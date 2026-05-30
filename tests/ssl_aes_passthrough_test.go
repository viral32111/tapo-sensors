package tests

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/joho/godotenv"

	"github.com/viral32111/tapo-sensors/internal/hub"
	"github.com/viral32111/tapo-sensors/internal/ssl_aes_passthrough"
)

func TestMain(m *testing.M) {
	currentDirectory, _ := os.Getwd()

	for {
		dotEnvFilePath := filepath.Join(currentDirectory, ".env")
		if _, err := os.Stat(dotEnvFilePath); err == nil {
			godotenv.Load(dotEnvFilePath)
			break
		}

		parentDirectory := filepath.Dir(currentDirectory)
		if parentDirectory == currentDirectory {
			break
		}

		currentDirectory = parentDirectory
	}

	os.Exit(m.Run())
}

// Fixed inputs and the expected outputs were computed independently with Python (the python-kasa
// reference algorithm), so these vectors cross-check the Go port of the SSL-AES crypto.
const (
	testLocalNonce  = "1234567890ABCDEF"
	testServerNonce = "FEDCBA0987654321"
	testPassword    = "P4ssw0rd!"

	expectedPwdSHA256 = "45279BBC6FF0112676B4B840A495A19633F54077C8DBC8E182F37F7B96ACB7CF"
	expectedPwdMD5    = "8AE1DD156C62F4F0B0B31C29B46F8E48"
	expectedConfirm   = "DC792268467F233FF5479EF4C659E730ED0CD7E67BFA21B33F71016214B22E01FEDCBA09876543211234567890ABCDEF"
	expectedDigest    = "D9F0F9B315B5AF0F4DBA3E719B9037AB24AAD0E6B1D6BA6AD50064CBA8598A2A1234567890ABCDEFFEDCBA0987654321"
	expectedLSK       = "cf8ce95d59c7d4d1b04d93c460761d8c"
	expectedIVB       = "c9bfcd8e2d90776cd9552dc3d95bfdcc"
	expectedTag       = "83AC59BFB9D2B7870B5FE1BFFD98769E67BB21B7230A130A1E6C95911049A172"

	testTagRequest = `{"method":"securePassthrough","params":{"request":"QUJDRA=="}}`
	testTagSeq     = 100
)

func TestPasswordHash(test *testing.T) {
	if got := ssl_aes_passthrough.SHA256HexUpper([]byte(testPassword)); got != expectedPwdSHA256 {
		test.Errorf("SHA256HexUpper mismatch\nActual:   %q\nExpected: %q", got, expectedPwdSHA256)
	}

	if got := ssl_aes_passthrough.MD5HexUpper([]byte(testPassword)); got != expectedPwdMD5 {
		test.Errorf("MD5HexUpper mismatch\nActual:   %q\nExpected: %q", got, expectedPwdMD5)
	}
}

func TestGenerateConfirmHash(test *testing.T) {
	got := ssl_aes_passthrough.GenerateConfirmHash(testLocalNonce, testServerNonce, expectedPwdSHA256)
	if got != expectedConfirm {
		test.Errorf("GenerateConfirmHash mismatch\nActual:   %q\nExpected: %q", got, expectedConfirm)
	}

	// 64 hex chars of SHA-256, then server_nonce (16) + local_nonce (16) = 96.
	if len(got) != 96 {
		test.Errorf("Confirm hash length %d, expected 96", len(got))
	}
}

func TestGenerateDigestPassword(test *testing.T) {
	got := ssl_aes_passthrough.GenerateDigestPassword(testLocalNonce, testServerNonce, expectedPwdSHA256)
	if got != expectedDigest {
		test.Errorf("GenerateDigestPassword mismatch\nActual:   %q\nExpected: %q", got, expectedDigest)
	}
}

func TestGenerateEncryptionToken(test *testing.T) {
	lsk := ssl_aes_passthrough.GenerateEncryptionToken("lsk", testLocalNonce, testServerNonce, expectedPwdSHA256)
	ivb := ssl_aes_passthrough.GenerateEncryptionToken("ivb", testLocalNonce, testServerNonce, expectedPwdSHA256)

	if len(lsk) != 16 {
		test.Errorf("Key (lsk) length %d, expected 16", len(lsk))
	}
	if len(ivb) != 16 {
		test.Errorf("IV (ivb) length %d, expected 16", len(ivb))
	}

	if got := hex.EncodeToString(lsk); got != expectedLSK {
		test.Errorf("lsk mismatch\nActual:   %q\nExpected: %q", got, expectedLSK)
	}
	if got := hex.EncodeToString(ivb); got != expectedIVB {
		test.Errorf("ivb mismatch\nActual:   %q\nExpected: %q", got, expectedIVB)
	}

	if bytes.Equal(lsk, ivb) {
		test.Errorf("Key and IV should differ (token_type must change the output)")
	}
}

func TestGenerateTag(test *testing.T) {
	got := ssl_aes_passthrough.GenerateTag(testTagRequest, testLocalNonce, expectedPwdSHA256, testTagSeq)
	if got != expectedTag {
		test.Errorf("GenerateTag mismatch\nActual:   %q\nExpected: %q", got, expectedTag)
	}
}

func TestEncryptDecrypt(test *testing.T) {
	key := bytes.Repeat([]byte{0x01}, 16)
	initVector := bytes.Repeat([]byte{0x02}, 16)

	payload := []byte(`{"method":"get","device_info":{}}`)

	encrypted, err := ssl_aes_passthrough.Encrypt(payload, key, initVector)
	if err != nil {
		test.Fatalf("Error while encrypting: %v", err)
	}

	decrypted, err := ssl_aes_passthrough.Decrypt(encrypted, key, initVector)
	if err != nil {
		test.Fatalf("Error while decrypting: %v", err)
	}

	if !bytes.Equal(payload, decrypted) {
		test.Errorf("Encrypt/decrypt mismatch\nActual:   %q\nExpected: %q", decrypted, payload)
	}
}

// hubEnvironment returns the hub IP and account credentials, skipping the test if any are unset.
func hubEnvironment(test *testing.T) (ipAddress string, email string, password string) {
	test.Helper()

	ipAddress = os.Getenv("TAPO_HUB_IP_ADDRESS")
	if ipAddress == "" {
		test.Skip("TAPO_HUB_IP_ADDRESS environment variable not set, skipping live hub test")
	}

	email = os.Getenv("TAPO_ACCOUNT_EMAIL")
	if email == "" {
		test.Skip("TAPO_ACCOUNT_EMAIL environment variable not set, skipping live hub test")
	}

	password = os.Getenv("TAPO_ACCOUNT_PASSWORD")
	if password == "" {
		test.Skip("TAPO_ACCOUNT_PASSWORD environment variable not set, skipping live hub test")
	}

	return ipAddress, email, password
}

func TestLogin(test *testing.T) {
	ipAddress, email, password := hubEnvironment(test)

	session, err := ssl_aes_passthrough.New(ipAddress, email, password)
	if err != nil {
		// Never dereference session after an error.
		test.Fatalf("Error during login: %v", err)
	}

	if session.TokenUrl == "" {
		test.Errorf("Session token URL is empty after login")
	}
	if len(session.Key) != 16 {
		test.Errorf("Session key length %d, expected 16", len(session.Key))
	}
	if len(session.IV) != 16 {
		test.Errorf("Session IV length %d, expected 16", len(session.IV))
	}
}

func TestInvalidLogin(test *testing.T) {
	ipAddress := os.Getenv("TAPO_HUB_IP_ADDRESS")
	if ipAddress == "" {
		test.Skip("TAPO_HUB_IP_ADDRESS environment variable not set, skipping live hub test")
	}

	_, err := ssl_aes_passthrough.New(ipAddress, "user@example.com", "definitely-the-wrong-password")
	if err == nil {
		test.Errorf("Expected an error logging in with invalid credentials, but got none")
	} else {
		test.Logf("Received expected error during login with invalid credentials: %v", err)
	}
}

func TestGetChildDeviceList(test *testing.T) {
	ipAddress, email, password := hubEnvironment(test)

	session, err := ssl_aes_passthrough.New(ipAddress, email, password)
	if err != nil {
		test.Fatalf("Error during login: %v", err)
	}

	children, err := hub.GetChildDeviceList(session)
	if err != nil {
		test.Fatalf("Error fetching child device list: %v", err)
	}

	if len(children) == 0 {
		test.Fatal("Expected at least one child sensor, got none")
	}

	for _, child := range children {
		if child.DeviceID == "" {
			test.Errorf("Child sensor has an empty device ID")
		}
		// The nickname must decode to a non-empty human-readable name.
		if child.Name() == "" {
			test.Errorf("Child sensor %s decoded to an empty name (nickname %q)", child.DeviceID, child.Nickname)
		}
		test.Logf("Sensor %s: name=%q model=%s category=%s status=%s", child.DeviceID, child.Name(), child.Model, child.Category, child.Status)
	}
}
