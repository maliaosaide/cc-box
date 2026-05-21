// 端到端加密
// Argon2id 密钥派生 + AES-256-GCM 加解密
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	// Argon2id 参数
	argonTime    = 3
	argonMemory  = 64 * 1024 // 64MB
	argonThreads = 4
	argonKeyLen  = 32 // 256-bit
	saltSize     = 16

	// AES-256-GCM nonce 大小
	nonceSize = 12

	// 加密格式版本标记
	versionByte = 0x01
)

// GenerateSalt 生成随机 salt
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("生成 salt 失败: %w", err)
	}
	return salt, nil
}

// DeriveKey 从密码和 salt 派生 256-bit 密钥
func DeriveKey(password string, salt []byte) []byte {
	return argon2.IDKey(
		[]byte(password),
		salt,
		argonTime,
		argonMemory,
		uint8(argonThreads),
		argonKeyLen,
	)
}

// KeyFingerprint 返回密钥的 SHA-256 指纹前 8 字符，用于验证
func KeyFingerprint(key []byte) string {
	h := sha256.Sum256(key)
	return hex.EncodeToString(h[:])[:8]
}

// Encrypt 使用 AES-256-GCM 加密数据
// 格式: [version(1)] [nonce(12)] [ciphertext+tag]
func Encrypt(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成 nonce 失败: %w", err)
	}

	// 版本字节 + nonce + 密文
	out := make([]byte, 0, 1+nonceSize+len(plaintext)+16)
	out = append(out, versionByte)
	out = append(out, nonce...)
	out = aesgcm.Seal(out, nonce, plaintext, nil)

	return out, nil
}

// Decrypt 使用 AES-256-GCM 解密数据
func Decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	if len(ciphertext) < 1+nonceSize+16 {
		return nil, fmt.Errorf("密文太短")
	}

	// 检查版本
	if ciphertext[0] != versionByte {
		return nil, fmt.Errorf("不支持的加密格式版本: %d", ciphertext[0])
	}

	nonce := ciphertext[1 : 1+nonceSize]
	data := ciphertext[1+nonceSize:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	plaintext, err := aesgcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败（密钥错误或数据损坏）: %w", err)
	}

	return plaintext, nil
}

// SaveKey 将密钥保存到文件
func SaveKey(key []byte, path string) error {
	return writeSecretFile(path, key)
}

// LoadKey 从文件加载密钥
func LoadKey(path string) ([]byte, error) {
	data, err := readSecretFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取密钥失败: %w", err)
	}
	return data, nil
}

// writeSecretFile 写入文件并设置 0600 权限
func writeSecretFile(path string, data []byte) error {
	if err := writeFileSecure(path, data, 0600); err != nil {
		return fmt.Errorf("写入密钥文件失败: %w", err)
	}
	return nil
}

// readSecretFile 读取密钥文件
func readSecretFile(path string) ([]byte, error) {
	data, err := readFileSecure(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}


// ConstantTimeEqual 常量时间比较两个字节切片
func ConstantTimeEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}
