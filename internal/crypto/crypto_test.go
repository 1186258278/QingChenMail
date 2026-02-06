package crypto

import (
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	secret := "test-secret-key-12345"
	plaintext := "my-smtp-password"

	encrypted, err := Encrypt(plaintext, secret)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if encrypted == plaintext {
		t.Fatal("Encrypted text should differ from plaintext")
	}

	if !IsEncrypted(encrypted) {
		t.Fatal("IsEncrypted should return true for encrypted text")
	}

	decrypted, err := Decrypt(encrypted, secret)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Fatalf("Decrypted text mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptPlaintext(t *testing.T) {
	// Should pass through unencrypted values (backward compatibility)
	secret := "test-secret"
	plain := "old-unencrypted-password"

	result, err := Decrypt(plain, secret)
	if err != nil {
		t.Fatalf("Decrypt plain failed: %v", err)
	}
	if result != plain {
		t.Fatalf("Expected %q, got %q", plain, result)
	}
}

func TestEncryptEmpty(t *testing.T) {
	encrypted, err := Encrypt("", "secret")
	if err != nil {
		t.Fatalf("Encrypt empty failed: %v", err)
	}
	if encrypted != "" {
		t.Fatalf("Expected empty, got %q", encrypted)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	secret1 := "correct-key"
	secret2 := "wrong-key"

	encrypted, _ := Encrypt("test-data", secret1)
	_, err := Decrypt(encrypted, secret2)
	if err == nil {
		t.Fatal("Decrypt with wrong key should fail")
	}
}

func TestIsEncrypted(t *testing.T) {
	if IsEncrypted("plain-text") {
		t.Fatal("plain text should not be detected as encrypted")
	}
	if !IsEncrypted("enc:base64data") {
		t.Fatal("enc: prefixed string should be detected as encrypted")
	}
}
