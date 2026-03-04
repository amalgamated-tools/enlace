package crypto

import (
	"strings"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := DeriveKey([]byte("test-secret"), "test-salt")
	plaintext := "my-s3-secret-key"

	encrypted, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if !strings.HasPrefix(encrypted, "enc:") {
		t.Errorf("expected encrypted value to have 'enc:' prefix, got %q", encrypted)
	}

	if encrypted == plaintext {
		t.Error("encrypted value should differ from plaintext")
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected decrypted value %q, got %q", plaintext, decrypted)
	}
}

func TestDecrypt_LegacyPlaintext(t *testing.T) {
	key := DeriveKey([]byte("test-secret"), "test-salt")

	// Value without "enc:" prefix should be returned as-is
	result, err := Decrypt("plain-value", key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if result != "plain-value" {
		t.Errorf("expected %q, got %q", "plain-value", result)
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := DeriveKey([]byte("secret-1"), "salt")
	key2 := DeriveKey([]byte("secret-2"), "salt")

	encrypted, err := Encrypt("sensitive", key1)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(encrypted, key2)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}

func TestDeriveKey_DifferentSalts(t *testing.T) {
	secret := []byte("same-secret")
	key1 := DeriveKey(secret, "salt-a")
	key2 := DeriveKey(secret, "salt-b")

	if string(key1) == string(key2) {
		t.Error("expected different keys for different salts")
	}
}
