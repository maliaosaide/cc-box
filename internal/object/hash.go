// Object 存储
// 文件内容加密后上传/下载到 WebDAV，按哈希前缀分目录
package object

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
)

// HashPrefix 返回哈希值的前 2 字符作为目录前缀
func HashPrefix(hash string) string {
	if len(hash) < 2 {
		return "00"
	}
	return hash[:2]
}

// ObjectPath 返回 object 在 WebDAV 上的路径
// 格式: objects/ab/c1234def....enc
func ObjectPath(hash string) string {
	prefix := HashPrefix(hash)
	return path.Join("objects", prefix, hash+".enc")
}

// ComputeHash 计算数据的 SHA-256 哈希
func ComputeHash(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

// ValidateHash 验证数据与哈希是否匹配
func ValidateHash(data []byte, expectedHash string) bool {
	return ComputeHash(data) == expectedHash
}

// HashShort 返回哈希的短格式（前 12 字符）
func HashShort(hash string) string {
	if len(hash) > 16 {
		return hash[:16] + "..."
	}
	return hash
}

// ParseHash 解析 sha256: 前缀
func ParseHash(hash string) (string, error) {
	if len(hash) < 7 || hash[:7] != "sha256:" {
		return "", fmt.Errorf("无效的哈希格式: %s", hash)
	}
	return hash[7:], nil
}
