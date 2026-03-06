package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

const encryptedPrefix = "enc:"

// Encryption salts for DeriveKey. Each domain that encrypts secrets should use
// its own salt so that the same master secret produces different AES keys.
const (
	StorageEncryptionSalt = "storage-secret-encryption"
	SMTPEncryptionSalt    = "smtp-secret-encryption"
)

// DeriveKey derives a 32-byte AES key from a secret and a purpose-specific salt.
func DeriveKey(secret []byte, salt string) []byte {
	hash := sha256.Sum256(append([]byte(salt+":"), secret...))
	return hash[:]
}

// Encrypt encrypts plaintext using AES-GCM and returns an "enc:" prefixed base64 string.
func Encrypt(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encryptedPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts an AES-GCM encrypted string. If the value has the "enc:" prefix,
// it is decrypted. Otherwise the value is returned as-is (legacy plaintext).
func Decrypt(encoded string, key []byte) (string, error) {
	if !strings.HasPrefix(encoded, encryptedPrefix) {
		return encoded, nil
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, encryptedPrefix))
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted value: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("encrypted value too short")
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt value: %w", err)
	}
	return string(plaintext), nil
}
