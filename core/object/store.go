// Object 上传/下载管理
// 加密 → WebDAV 上传，下载 → 解密
package object

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/crypto"
	"github.com/user/cc-box/core/webdav"
)

// Store Object 存储管理器
type Store struct {
	client      *webdav.Client
	key         []byte
	cache       string // 本地缓存目录
	encrypt     bool
	knownHashes map[string]bool // 已知在远程存在的哈希（跳过 Exists 检查）
}

// NewStore 创建 Store 实例
func NewStore(client *webdav.Client, key []byte, cacheDir string) *Store {
	encrypt := true
	if cfg, err := config.Load(); err == nil {
		encrypt = cfg.Encryption.Enabled
	}
	return &Store{
		client:      client,
		key:         key,
		cache:       cacheDir,
		encrypt:     encrypt,
		knownHashes: make(map[string]bool),
	}
}

// SetKnownHashes 设置已知远程存在的哈希集合，跳过 Exists 检查
func (s *Store) SetKnownHashes(hashes map[string]bool) {
	s.knownHashes = hashes
}

// Upload 加密并上传文件内容到 WebDAV
func (s *Store) Upload(data []byte) (string, error) {
	hash := ComputeHash(data)
	objPath := ObjectPath(hash)

	// 先查本地已知集合（零网络请求）
	if s.knownHashes[hash] {
		return hash, nil
	}

	// 查本地缓存（同进程内可能已上传过）
	if s.knownHashes != nil {
		if _, already := s.knownHashes["_uploaded_"+hash]; already {
			return hash, nil
		}
	}

	// 检查远程是否已存在
	if exists, _ := s.Exists(hash); exists {
		if s.knownHashes != nil {
			s.knownHashes[hash] = true
		}
		return hash, nil
	}

	payload := data
	if s.encrypt {
		encrypted, err := crypto.Encrypt(data, s.key)
		if err != nil {
			return "", fmt.Errorf("加密失败: %w", err)
		}
		payload = encrypted
	}

	// 确保目录存在
	if err := s.client.EnsureDir(objPath); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	// 上传
	if _, err := s.client.PUT(objPath, payload, ""); err != nil {
		return "", fmt.Errorf("上传 object 失败: %w", err)
	}

	// 标记为已知，后续同 hash 跳过
	if s.knownHashes != nil {
		s.knownHashes["_uploaded_"+hash] = true
	}

	return hash, nil
}

// Download 从 WebDAV 下载并解密文件内容
func (s *Store) Download(hash string) ([]byte, error) {
	// 先查本地缓存
	if s.cache != "" {
		cached := s.cachePath(hash)
		if data, err := os.ReadFile(cached); err == nil {
			decrypted := data
			if s.encrypt {
				decrypted, err = crypto.Decrypt(data, s.key)
				if err != nil {
					decrypted = nil
				}
			}
			if decrypted != nil && ValidateHash(decrypted, hash) {
				return decrypted, nil
			}
			// 缓存损坏，继续从远程下载
		}
	}

	var encrypted []byte
	var err error
	for _, candidate := range objectPaths(hash) {
		encrypted, _, err = s.client.GET(candidate)
		if err == nil {
			break
		}
		if err != webdav.ErrNotFound {
			return nil, fmt.Errorf("下载 object 失败: %w", err)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("下载 object 失败: %w", err)
	}

	decrypted := encrypted
	if s.encrypt {
		var err error
		decrypted, err = crypto.Decrypt(encrypted, s.key)
		if err != nil {
			return nil, fmt.Errorf("解密失败: %w", err)
		}
	}
	if !ValidateHash(decrypted, hash) {
		return nil, fmt.Errorf("object hash 校验失败: %s", hash)
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
	for _, objPath := range objectPaths(hash) {
		exists, err := s.client.Exists(objPath)
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

// cachePath 返回本地缓存路径
func (s *Store) cachePath(hash string) string {
	prefix := HashPrefix(hash)
	return filepath.Join(s.cache, prefix, hash+".enc")
}

func objectPaths(hash string) []string {
	primary := ObjectPath(hash)
	legacy := filepath.ToSlash(filepath.Join("objects", "sh", hash+".enc"))
	if legacy == primary {
		return []string{primary}
	}
	return []string{primary, legacy}
}
