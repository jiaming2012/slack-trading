package utils

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"io"
)

// Define maximum input size limit
const MaxInputSize = 120 // Safe limit to ensure final ciphertext ≤ 255 characters

// Encrypts a message with AES-GCM, compresses it, and encodes it safely
func encryptMessage(plaintext string, key []byte) (string, error) {
	// Check input length
	if len(plaintext) > MaxInputSize {
		return "", fmt.Errorf("input too long: maximum allowed is %d characters", MaxInputSize)
	}

	// Compress the plaintext
	compressedData, err := gzipCompress([]byte(plaintext))
	if err != nil {
		return "", err
	}

	// AES-GCM encryption setup
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Generate a random nonce
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Encrypt the compressed message
	ciphertext := aesGCM.Seal(nonce, nonce, compressedData, nil)

	// Encode using Base32 (RFC 4648, no padding)
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(ciphertext)

	// Ensure ciphertext length is within 255 characters
	if len(encoded) > 255 {
		return "", fmt.Errorf("encrypted message too long: expected ≤ 255 characters, got %d", len(encoded))
	}

	return encoded, nil
}

// Decrypts a Base32-encoded, AES-GCM encrypted message and decompresses it
func decryptMessage(encoded string, key []byte) (string, error) {
	// Decode Base32
	ciphertext, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(encoded)
	if err != nil {
		return "", err
	}

	// AES-GCM decryption setup
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Extract nonce and actual ciphertext
	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("invalid ciphertext")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt the message
	decompressedData, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	// Decompress the decrypted data
	plaintext, err := gzipDecompress(decompressedData)
	if err != nil {
		return "", err
	}

	return plaintext, nil
}

// Compresses data using gzip
func gzipCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write(data)
	if err != nil {
		return nil, err
	}
	gz.Close()
	return buf.Bytes(), nil
}

// Decompresses gzip-compressed data
func gzipDecompress(data []byte) (string, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	reader.Close()
	return string(decompressed), nil
}

// func main() {
// 	// 256-bit AES key (must be 32 bytes for AES-256)
// 	key := make([]byte, 32)
// 	_, _ = rand.Read(key)

// 	// Example messages
// 	messageShort := "Hello, Golang Encryption!"
// 	messageLong := "This message is intentionally too long to fit within the encryption limit of 120 characters, and should trigger an error."

// 	// Encrypt short message
// 	ciphertext, err := encryptMessage(messageShort, key)
// 	if err != nil {
// 		fmt.Println("Encryption error:", err)
// 	} else {
// 		fmt.Println("Encrypted:", ciphertext)
// 		fmt.Println("Ciphertext Length:", len(ciphertext))

// 		// Decrypt
// 		decrypted, err := decryptMessage(ciphertext, key)
// 		if err != nil {
// 			fmt.Println("Decryption error:", err)
// 		} else {
// 			fmt.Println("Decrypted:", decrypted)
// 		}
// 	}

// 	// Try encrypting an overly long message
// 	ciphertext, err = encryptMessage(messageLong, key)
// 	if err != nil {
// 		fmt.Println("Encryption error (expected for long input):", err)
// 	}
// }
