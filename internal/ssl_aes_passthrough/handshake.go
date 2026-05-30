package ssl_aes_passthrough

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const (
	// The local login username is always the literal string "admin", never the cloud account
	// email - the H200 rejects the email with inner code -60502. The password is still the
	// cloud-account password. See PLAN.md "Connection constants".
	USERNAME_ADMIN = "admin"

	// Encryption type "3" selects the SSL-AES (smartcam) transport during the handshake.
	ENCRYPT_TYPE = "3"
)

// TP-Link structured protocol error codes relevant to the handshake (kasa SmartErrorCode).
const (
	ERROR_SUCCESS            = 0      // ok
	ERROR_INVALID_NONCE      = -40413 // expected handshake1 reply - "do secure login", carries nonce + device_confirm
	ERROR_SESSION_EXPIRED    = -40401 // legacy-transport rejection / generic session error
	ERROR_UNSUPPORTED_METHOD = -40106 // wrong method name or envelope
	ERROR_BAD_USERNAME       = -40411 // username not recognised
	ERROR_INVALID_USERNAME   = -60502 // returned when handshake1 username is the cloud email instead of "admin"
)

// sha256Raw returns the raw SHA-256 digest bytes of the input.
func sha256Raw(input []byte) []byte {
	digest := sha256.Sum256(input)
	return digest[:]
}

// SHA256HexUpper returns the SHA-256 digest of the input as an upper-case hex string.
func SHA256HexUpper(input []byte) string {
	return strings.ToUpper(hex.EncodeToString(sha256Raw(input)))
}

// MD5HexUpper returns the MD5 digest of the input as an upper-case hex string.
func MD5HexUpper(input []byte) string {
	digest := md5.Sum(input)
	return strings.ToUpper(hex.EncodeToString(digest[:]))
}

// GenerateLocalNonce produces a client nonce: 8 random bytes rendered as 16 upper-case hex characters.
func GenerateLocalNonce() (string, error) {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("Error generating local nonce: %w", err)
	}

	return strings.ToUpper(hex.EncodeToString(buffer)), nil
}

// GenerateConfirmHash reproduces the device confirmation hash, proving we know the password.
// It must equal the device_confirm value returned by handshake1.
func GenerateConfirmHash(localNonce string, serverNonce string, passwordHash string) string {
	hash := SHA256HexUpper([]byte(localNonce + passwordHash + serverNonce))
	return hash + serverNonce + localNonce
}

// GenerateDigestPassword produces the proof of password knowledge sent back in handshake2.
func GenerateDigestPassword(localNonce string, serverNonce string, passwordHash string) string {
	hash := SHA256HexUpper([]byte(passwordHash + localNonce + serverNonce))
	return hash + localNonce + serverNonce
}

// GenerateEncryptionToken derives a 16-byte AES key ("lsk") or IV ("ivb") for the secure session.
func GenerateEncryptionToken(tokenType string, localNonce string, serverNonce string, passwordHash string) []byte {
	hashedKey := SHA256HexUpper([]byte(localNonce + passwordHash + serverNonce))
	token := sha256Raw([]byte(tokenType + localNonce + serverNonce + hashedKey))
	return token[:16]
}

// GenerateTag computes the per-request integrity tag sent in the "Tapo_tag" header. The request
// argument is the exact securePassthrough JSON string that is sent on the wire.
func GenerateTag(request string, localNonce string, passwordHash string, seq int) string {
	passwordNonceHash := SHA256HexUpper([]byte(passwordHash + localNonce))
	return SHA256HexUpper([]byte(passwordNonceHash + request + strconv.Itoa(seq)))
}

// handshake1 sends the client nonce and, on success, returns the server nonce and device
// confirmation hash. An error is returned (so the caller can try another username) if the hub
// does not respond with a usable nonce.
func handshake1(client *http.Client, baseUrl string, ipAddress string, username string, localNonce string) (serverNonce string, deviceConfirm string, err error) {
	payload, err := json.Marshal(map[string]any{
		"method": "login",
		"params": map[string]any{
			"cnonce":       localNonce,
			"encrypt_type": ENCRYPT_TYPE,
			"username":     username,
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("Error encoding handshake1 request as JSON: %w", err)
	}

	body, statusCode, err := doPost(client, baseUrl, ipAddress, payload, nil)
	if err != nil {
		return "", "", fmt.Errorf("Error sending handshake1 request: %w", err)
	}
	if statusCode != http.StatusOK {
		return "", "", fmt.Errorf("Unexpected HTTP status code %d for handshake1: %s", statusCode, body)
	}

	var data struct {
		ErrorCode int `json:"error_code"`
		Result    struct {
			Data struct {
				Nonce         string `json:"nonce"`
				DeviceConfirm string `json:"device_confirm"`
				Code          int    `json:"code"`
			} `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", "", fmt.Errorf("Error parsing handshake1 response: %w", err)
	}

	// A usable reply carries both the server nonce and the device confirmation hash. The cloud
	// email is rejected here with inner code -60502 and no nonce, which signals "try 'admin'".
	if data.Result.Data.Nonce == "" || data.Result.Data.DeviceConfirm == "" {
		return "", "", fmt.Errorf("Hub did not return a nonce for username %q (error_code %d, inner code %d)", username, data.ErrorCode, data.Result.Data.Code)
	}

	return data.Result.Data.Nonce, data.Result.Data.DeviceConfirm, nil
}

// handshake2 proves password knowledge with the digest and, on success, returns the starting
// sequence number and session token (stok).
func handshake2(client *http.Client, baseUrl string, ipAddress string, username string, localNonce string, digestPassword string) (startSeq int, stok string, err error) {
	payload, err := json.Marshal(map[string]any{
		"method": "login",
		"params": map[string]any{
			"cnonce":        localNonce,
			"encrypt_type":  ENCRYPT_TYPE,
			"digest_passwd": digestPassword,
			"username":      username,
		},
	})
	if err != nil {
		return 0, "", fmt.Errorf("Error encoding handshake2 request as JSON: %w", err)
	}

	body, statusCode, err := doPost(client, baseUrl, ipAddress, payload, nil)
	if err != nil {
		return 0, "", fmt.Errorf("Error sending handshake2 request: %w", err)
	}
	if statusCode != http.StatusOK {
		return 0, "", fmt.Errorf("Unexpected HTTP status code %d for handshake2: %s", statusCode, body)
	}

	var data struct {
		ErrorCode int `json:"error_code"`
		Result    struct {
			StartSeq int    `json:"start_seq"`
			Stok     string `json:"stok"`
			// user_group is intentionally omitted: it is unused, and the hub returns it as a
			// string here rather than the integer some documentation suggests.
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, "", fmt.Errorf("Error parsing handshake2 response: %w", err)
	}
	if data.ErrorCode != ERROR_SUCCESS {
		return 0, "", fmt.Errorf("Hub reported error code %d during handshake2", data.ErrorCode)
	}
	if data.Result.Stok == "" {
		return 0, "", fmt.Errorf("No session token (stok) in handshake2 response")
	}

	return data.Result.StartSeq, data.Result.Stok, nil
}
