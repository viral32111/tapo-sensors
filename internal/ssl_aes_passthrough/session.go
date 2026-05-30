package ssl_aes_passthrough

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/viral32111/tapo-sensors/internal/helpers"
)

// Session holds the state of an authenticated SSL-AES (smartcam) connection to a Tapo hub.
type Session struct {
	HttpClient *http.Client

	IPAddress string
	BaseUrl   string
	TokenUrl  string

	Key []byte // The AES-128 session key (lsk).
	IV  []byte // The AES-128 initialisation vector (ivb).

	PasswordHash string
	LocalNonce   string
	Seq          int
}

// New performs the full SSL-AES handshake against the hub and returns a ready-to-use session.
func New(ipAddress string, emailAddress string, password string) (*Session, error) {
	baseUrl := fmt.Sprintf("https://%s:443/", ipAddress)
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			// The hub presents a self-signed device certificate (CN=TPRI-DEVICE, O=TPRI).
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},

			// The hub closes the TCP connection after every response despite advertising
			// "Connection: keep-alive", so a pooled connection is always dead on reuse and the
			// next request fails with EOF. Force a fresh connection per request.
			DisableKeepAlives: true,
		},
	}

	localNonce, err := GenerateLocalNonce()
	if err != nil {
		return nil, err
	}

	// python-kasa tries the cloud email first, then falls back to the built-in "admin" username,
	// which is the one the H200 actually accepts. We mirror that order for robustness.
	usernames := []string{emailAddress, USERNAME_ADMIN}

	var (
		username      string
		serverNonce   string
		deviceConfirm string
		handshakeErr  error
	)
	for _, candidate := range usernames {
		serverNonce, deviceConfirm, handshakeErr = handshake1(httpClient, baseUrl, ipAddress, candidate, localNonce)
		if handshakeErr == nil {
			username = candidate
			break
		}
	}
	if username == "" {
		return nil, fmt.Errorf("Handshake1 rejected every username (is 'Third-Party Compatibility' enabled in the Tapo app?): %w", handshakeErr)
	}

	// Verify the password and, in doing so, select the hashing algorithm the hub uses. The
	// device_confirm hash does not depend on the username, only on the password hash and nonces.
	passwordHash := SHA256HexUpper([]byte(password))
	if deviceConfirm != GenerateConfirmHash(localNonce, serverNonce, passwordHash) {
		passwordHash = MD5HexUpper([]byte(password))
		if deviceConfirm != GenerateConfirmHash(localNonce, serverNonce, passwordHash) {
			return nil, fmt.Errorf("Device confirmation hash did not match (incorrect password?)")
		}
	}

	digestPassword := GenerateDigestPassword(localNonce, serverNonce, passwordHash)
	startSeq, stok, err := handshake2(httpClient, baseUrl, ipAddress, username, localNonce, digestPassword)
	if err != nil {
		return nil, err
	}

	return &Session{
		HttpClient: httpClient,

		IPAddress: ipAddress,
		BaseUrl:   baseUrl,
		TokenUrl:  fmt.Sprintf("%sstok=%s/ds", baseUrl, stok),

		Key: GenerateEncryptionToken("lsk", localNonce, serverNonce, passwordHash),
		IV:  GenerateEncryptionToken("ivb", localNonce, serverNonce, passwordHash),

		PasswordHash: passwordHash,
		LocalNonce:   localNonce,
		Seq:          startSeq,
	}, nil
}

// Request sends an inner request through the secure passthrough and returns the decrypted inner
// response. The caller is responsible for parsing the inner JSON, whose shape is method-specific.
func (session *Session) Request(payload any) (json.RawMessage, error) {
	inner, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("Error encoding inner request as JSON: %w", err)
	}

	encryptedRequest, err := Encrypt(inner, session.Key, session.IV)
	if err != nil {
		return nil, fmt.Errorf("Error encrypting inner request: %w", err)
	}

	passthrough, err := json.Marshal(map[string]any{
		"method": "securePassthrough",
		"params": map[string]any{
			"request": encryptedRequest,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Error encoding secure passthrough request as JSON: %w", err)
	}

	// The tag is computed over the exact bytes that are sent, using the sequence number for this
	// request; the sequence then increments for the next request.
	seq := session.Seq
	tag := GenerateTag(string(passthrough), session.LocalNonce, session.PasswordHash, seq)
	session.Seq++

	body, statusCode, err := doPost(session.HttpClient, session.TokenUrl, session.IPAddress, passthrough, map[string]string{
		"Seq":      strconv.Itoa(seq),
		"Tapo_tag": tag,
	})
	if err != nil {
		return nil, fmt.Errorf("Error sending secure passthrough request: %w", err)
	}
	if statusCode != http.StatusOK {
		// A stale session can return HTTP 500 (e.g. another client logged in); the caller
		// should re-handshake and retry.
		return nil, fmt.Errorf("Unexpected HTTP status code %d for secure passthrough request: %s", statusCode, body)
	}

	var outer struct {
		ErrorCode int             `json:"error_code"`
		Result    json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &outer); err != nil {
		return nil, fmt.Errorf("Error parsing secure passthrough response: %w", err)
	}
	if outer.ErrorCode != ERROR_SUCCESS {
		return nil, fmt.Errorf("Hub returned error code %d for secure passthrough request", outer.ErrorCode)
	}

	var encrypted struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(outer.Result, &encrypted); err == nil && encrypted.Response != "" {
		decrypted, err := Decrypt(encrypted.Response, session.Key, session.IV)
		if err != nil {
			return nil, fmt.Errorf("Error decrypting secure passthrough response: %w", err)
		}

		return decrypted, nil
	}

	// Some single requests answer unencrypted - treat the result itself as the response.
	return outer.Result, nil
}

// Encrypt performs AES-128-CBC with PKCS#7 padding and returns the base64-encoded cipher-text.
func Encrypt(plainText []byte, key []byte, initVector []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	padded := helpers.PKCS7Pad(plainText, aes.BlockSize)
	cipherText := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, initVector).CryptBlocks(cipherText, padded)

	return base64.StdEncoding.EncodeToString(cipherText), nil
}

// Decrypt reverses Encrypt: base64-decode, AES-128-CBC decrypt, then strip PKCS#7 padding.
func Decrypt(cipherTextB64 string, key []byte, initVector []byte) ([]byte, error) {
	cipherText, err := base64.StdEncoding.DecodeString(cipherTextB64)
	if err != nil {
		return nil, fmt.Errorf("Error decoding base64 cipher-text: %w", err)
	}

	if len(cipherText)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("Cipher-text length %d is not a multiple of AES block size %d", len(cipherText), aes.BlockSize)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	plainText := make([]byte, len(cipherText))
	cipher.NewCBCDecrypter(block, initVector).CryptBlocks(plainText, cipherText)

	return helpers.PKCS7Unpad(plainText)
}

// setCommonHeaders applies the headers the smartcam transport expects on every request.
func setCommonHeaders(request *http.Request, ipAddress string) {
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	request.Header.Set("requestByApp", "true")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Tapo CameraClient Android")
	request.Header.Set("Referer", fmt.Sprintf("https://%s:443", ipAddress))

	// Go ignores a "Host" entry in the header map, so it must be set on the request directly.
	request.Host = fmt.Sprintf("%s:443", ipAddress)
}

// doPost sends a JSON POST with the common headers plus any extra headers, returning the response
// body and HTTP status code.
func doPost(client *http.Client, url string, ipAddress string, body []byte, extraHeaders map[string]string) ([]byte, int, error) {
	request, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}

	setCommonHeaders(request, ipAddress)
	for name, value := range extraHeaders {
		request.Header.Set(name, value)
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, response.StatusCode, err
	}

	return responseBody, response.StatusCode, nil
}
