package helpers

import (
	"bytes"
	"crypto/aes"
	"fmt"
)

// Pads data to a multiple of blockSize using PKCS#7
func PKCS7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	return append(data, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

// Remove PKCS7 padding
func PKCS7Unpad(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 || length%aes.BlockSize != 0 {
		return nil, fmt.Errorf("Invalid padded length: %d", length)
	}

	padding := int(data[length-1])
	if padding == 0 || padding > aes.BlockSize {
		return nil, fmt.Errorf("Invalid PKCS7 padding byte: %d", padding)
	}

	return data[:length-padding], nil
}
