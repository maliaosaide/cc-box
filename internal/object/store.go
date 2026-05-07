// Object 上传/下载管理
// 加密 → WebDAV 上传，下载 → 解密
package object

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/user/cc-box/internal/crypto"
	"github.com/user/cc-box/internal/webdav"
)

// Store Object 存储管理器
type Store struct {
	client *webdav.Client
	key    []byte
	cache  string // 本地缓存目录
}

// NewStore 创建 Store 实例
func NewStore(client *webdav.Client, key []byte, cacheDir string) *Store {
	return &Store{
		client: client,
		key:    key,
		cache:  cacheDir,
	}
}

// Upload 加密并上传文件内容到 WebDAV
func (s *Store) Upload(data []byte) (string, error) {
	hash := ComputeHash(data)
	objPath := ObjectPath(hash)

	// 检查是否已存在（去重）
	if exists, _ := s.client.Exists(objPath); exists {
		return hash, nil
	}

	// 加密
	encrypted, err := crypto.Encrypt(data, s.key)
	if err != nil {
		return "", fmt.Errorf("加密失败: %w", err)
	}

	// 确保目录存在
	if err := s.client.EnsureDir(objPath); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	// 上传
	if _, err := s.client.PUT(objPath, encrypted, ""); err != nil {
		return "", fmt.Errorf("上传 object 失败: %w", err)
	}

	return hash, nil
}

// Download 从 WebDAV 下载并解密文件内容
func (s *Store) Download(hash string) ([]byte, error) {
	// 先查本地缓存
	if s.cache != "" {
		cached := s.cachePath(hash)
		if data, err := os.ReadFile(cached); err == nil {
			decrypted, err := crypto.Decrypt(data, s.key)
			if err == nil {
				return decrypted, nil
			}
			// 缓存损坏，继续从远程下载
		}
	}

	objPath := ObjectPath(hash)
	encrypted, _, err := s.client.GET(objPath)
	if err != nil {
		return nil, fmt.Errorf("下载 object 失败: %w", err)
	}

	decrypted, err := crypto.Decrypt(encrypted, s.key)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}

	// 写入缓存
	if s.cache != "" {
		cached := s.cachePath(hash)
		os.MkdirAll(filepath.Dir(cached), 0700)
		os.WriteFile(cached, encrypted, 0600)
	}

	return decrypted, nil
}

// Key 返回加密密钥
func (s *Store) Key() []byte {
	return s.key
}

// Exists 检查 object 是否存在
func (s *Store) Exists(hash string) (bool, error) {
	objPath := ObjectPath(hash)
	return s.client.Exists(objPath)
}

// cachePath 返回本地缓存路径
func (s *Store) cachePath(hash string) string {
	prefix := HashPrefix(hash)
	return filepath.Join(s.cache, prefix, hash+".enc")
}
