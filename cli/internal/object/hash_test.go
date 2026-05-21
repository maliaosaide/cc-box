// Object 哈希测试
package object

import (
	"testing"
)

func TestComputeHash(t *testing.T) {
	data := []byte("hello world")
	hash := ComputeHash(data)
	if hash == "" {
		t.Error("ComputeHash returned empty")
	}
	if hash[:7] != "sha256:" {
		t.Errorf("hash should start with sha256:, got %s", hash[:7])
	}
}

func TestComputeHashDeterministic(t *testing.T) {
	data := []byte("test data")
	h1 := ComputeHash(data)
	h2 := ComputeHash(data)
	if h1 != h2 {
		t.Error("same data should produce same hash")
	}
}

func TestValidateHash(t *testing.T) {
	data := []byte("test")
	hash := ComputeHash(data)
	if !ValidateHash(data, hash) {
		t.Error("ValidateHash should return true for matching data")
	}
	if ValidateHash([]byte("other"), hash) {
		t.Error("ValidateHash should return false for different data")
	}
}

func TestObjectPath(t *testing.T) {
	hash := "sha256:abcdef1234567890"
	path := ObjectPath(hash)
	if path != "objects/ab/sha256:abcdef1234567890.enc" {
		t.Errorf("ObjectPath() = %s, unexpected", path)
	}
}

func TestHashPrefix(t *testing.T) {
	if HashPrefix("sha256:abcdef") != "ab" {
		t.Error("HashPrefix should return first 2 digest chars")
	}
	if HashPrefix("abcdef") != "00" {
		t.Error("HashPrefix should reject hashes without sha256 prefix")
	}
	if HashPrefix("sha256:a") != "00" {
		t.Error("HashPrefix should return 00 for short digest")
	}
}

func TestParseHash(t *testing.T) {
	hash, err := ParseHash("sha256:abc123")
	if err != nil || hash != "abc123" {
		t.Errorf("ParseHash() = %s, %v; want abc123, nil", hash, err)
	}

	_, err = ParseHash("invalid")
	if err == nil {
		t.Error("ParseHash should reject non-sha256 prefix")
	}
}
