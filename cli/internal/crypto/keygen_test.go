// 端到端加密测试
package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := DeriveKey("test-password", []byte("0123456789abcdef"))
	plaintext := []byte("Hello, CC-Box! 这是一段测试文本。")

	encrypted, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// 密文应与明文不同
	if bytes.Equal(encrypted[1+12:], plaintext) {
		t.Error("encrypted data should differ from plaintext")
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptDecryptEmpty(t *testing.T) {
	key := DeriveKey("test", []byte("salt123456789012"))
	plaintext := []byte("")

	encrypted, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt empty failed: %v", err)
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt empty failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("roundtrip failed for empty data")
	}
}

func TestEncryptDecryptLarge(t *testing.T) {
	key := DeriveKey("test", []byte("salt123456789012"))
	plaintext := make([]byte, 1024*1024) // 1MB
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	encrypted, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt large failed: %v", err)
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt large failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("roundtrip failed for large data")
	}
}

func TestWrongKeyFails(t *testing.T) {
	key1 := DeriveKey("password1", []byte("salt123456789012"))
	key2 := DeriveKey("password2", []byte("salt123456789012"))

	encrypted, _ := Encrypt([]byte("secret"), key1)

	_, err := Decrypt(encrypted, key2)
	if err == nil {
		t.Error("decrypting with wrong key should fail")
	}
}

func TestDeriveKeyDeterministic(t *testing.T) {
	salt := []byte("0123456789abcdef")
	key1 := DeriveKey("same-password", salt)
	key2 := DeriveKey("same-password", salt)

	if !bytes.Equal(key1, key2) {
		t.Error("same password + salt should produce same key")
	}
}

func TestDeriveKeyDifferentSalt(t *testing.T) {
	key1 := DeriveKey("same-password", []byte("0123456789abcdef"))
	key2 := DeriveKey("same-password", []byte("fedcba9876543210"))

	if bytes.Equal(key1, key2) {
		t.Error("different salts should produce different keys")
	}
}

func TestKeyFingerprint(t *testing.T) {
	key := DeriveKey("test", []byte("0123456789abcdef"))
	fp := KeyFingerprint(key)
	if len(fp) != 8 {
		t.Errorf("KeyFingerprint length = %d, want 8", len(fp))
	}
}

func TestGenerateSalt(t *testing.T) {
	salt1, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt failed: %v", err)
	}
	if len(salt1) != 16 {
		t.Errorf("salt length = %d, want 16", len(salt1))
	}

	salt2, _ := GenerateSalt()
	if bytes.Equal(salt1, salt2) {
		t.Error("two generated salts should differ")
	}
}
