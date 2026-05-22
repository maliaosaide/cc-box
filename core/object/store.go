// Object 上传/下载管理
// 加密 → WebDAV 上传，下载 → 解密
package object

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	chunkMode   string
	chunkSize   int
	threshold   int64
	knownHashes map[string]bool // 已知在远程存在的哈希（跳过 Exists 检查）
}

type objectManifest struct {
	Hash       string   `json:"hash"`
	TotalParts int      `json:"total_parts"`
	PartHashes []string `json:"part_hashes"`
	TotalSize  int64    `json:"total_size"`
}

const defaultObjectChunkSize = 10 * 1024 * 1024

// NewStore 创建 Store 实例
func NewStore(client *webdav.Client, key []byte, cacheDir string) *Store {
	encrypt := true
	chunkMode := "auto"
	chunkSize := defaultObjectChunkSize
	threshold := int64(50 * 1024 * 1024)
	if cfg, err := config.Load(); err == nil {
		encrypt = cfg.Encryption.Enabled
		if cfg.Binary.ChunkMode != "" {
			chunkMode = cfg.Binary.ChunkMode
		}
		if cfg.Binary.ChunkSizeMB > 0 {
			chunkSize = cfg.Binary.ChunkSizeMB * 1024 * 1024
		}
		if cfg.Binary.ChunkThresholdMB > 0 {
			threshold = int64(cfg.Binary.ChunkThresholdMB) * 1024 * 1024
		}
	}
	return &Store{
		client:      client,
		key:         key,
		cache:       cacheDir,
		encrypt:     encrypt,
		chunkMode:   chunkMode,
		chunkSize:   chunkSize,
		threshold:   threshold,
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

	if shouldChunkObject(int64(len(data)), s.chunkMode, s.threshold) {
		if err := s.uploadChunked(hash, data); err != nil {
			return "", err
		}
	} else {
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
		chunked, chunkErr := s.downloadChunked(hash)
		if chunkErr != nil {
			if chunkErr == webdav.ErrNotFound {
				return nil, fmt.Errorf("下载 object 失败: %w", err)
			}
			return nil, fmt.Errorf("下载 object 分块失败: %w", chunkErr)
		}
		if s.cache != "" {
			cached := s.cachePath(hash)
			payload := chunked
			if s.encrypt {
				if encryptedCache, cacheErr := crypto.Encrypt(chunked, s.key); cacheErr == nil {
					payload = encryptedCache
				}
			}
			os.MkdirAll(filepath.Dir(cached), 0700)
			os.WriteFile(cached, payload, 0600)
		}
		return chunked, nil
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

func (s *Store) downloadChunked(hash string) ([]byte, error) {
	manifestData, _, err := s.client.GET(objectManifestPath(hash))
	if err != nil {
		return nil, err
	}
	var manifest objectManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("解析 object manifest 失败: %w", err)
	}
	if manifest.Hash != hash {
		return nil, fmt.Errorf("object manifest hash 不匹配: %s", manifest.Hash)
	}
	if len(manifest.PartHashes) != manifest.TotalParts {
		return nil, fmt.Errorf("object manifest 分块 hash 数量不匹配")
	}
	result := make([]byte, 0, manifest.TotalSize)
	for i := 0; i < manifest.TotalParts; i++ {
		payload, _, err := s.client.GET(objectPartPath(hash, i, s.encrypt))
		if err != nil {
			return nil, fmt.Errorf("下载 object 分块 %d 失败: %w", i, err)
		}
		chunk := payload
		if s.encrypt {
			decrypted, err := crypto.Decrypt(payload, s.key)
			if err != nil {
				return nil, fmt.Errorf("解密 object 分块 %d 失败: %w", i, err)
			}
			chunk = decrypted
		}
		partHash := sha256.Sum256(chunk)
		if hex.EncodeToString(partHash[:]) != manifest.PartHashes[i] {
			return nil, fmt.Errorf("object 分块 %d hash 校验失败", i)
		}
		result = append(result, chunk...)
	}
	if int64(len(result)) != manifest.TotalSize {
		return nil, fmt.Errorf("object 下载大小不匹配: %d", len(result))
	}
	if !ValidateHash(result, hash) {
		return nil, fmt.Errorf("object hash 校验失败: %s", hash)
	}
	return result, nil
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
	return s.chunkedExists(hash)
}

func (s *Store) chunkedExists(hash string) (bool, error) {
	manifestData, _, err := s.client.GET(objectManifestPath(hash))
	if err == webdav.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var manifest objectManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil || manifest.Hash != hash || len(manifest.PartHashes) != manifest.TotalParts {
		return false, nil
	}
	for i := 0; i < manifest.TotalParts; i++ {
		exists, err := s.client.Exists(objectPartPath(hash, i, s.encrypt))
		if err != nil {
			return false, err
		}
		if !exists {
			return false, nil
		}
	}
	return true, nil
}

// cachePath 返回本地缓存路径
func (s *Store) cachePath(hash string) string {
	prefix := HashPrefix(hash)
	return filepath.Join(s.cache, prefix, hash+".enc")
}

func (s *Store) uploadChunked(hash string, data []byte) error {
	manifest, chunks := splitObject(data, s.chunkSize)
	manifest.Hash = hash
	basePath := objectPartsBasePath(hash)
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 object manifest 失败: %w", err)
	}
	if err := s.client.EnsureDir(basePath + "manifest.json"); err != nil {
		return fmt.Errorf("创建 object manifest 目录失败: %w", err)
	}
	if _, err := s.client.PUT(basePath+"manifest.json", manifestData, ""); err != nil {
		return fmt.Errorf("上传 object manifest 失败: %w", err)
	}
	for i, chunk := range chunks {
		partPath := objectPartPath(hash, i, s.encrypt)
		if exists, _ := s.client.Exists(partPath); exists {
			continue
		}
		payload := chunk
		if s.encrypt {
			encrypted, err := crypto.Encrypt(chunk, s.key)
			if err != nil {
				return fmt.Errorf("加密 object 分块 %d 失败: %w", i, err)
			}
			payload = encrypted
		}
		if err := s.client.EnsureDir(partPath); err != nil {
			return fmt.Errorf("创建 object 分块目录失败: %w", err)
		}
		if _, err := s.client.PUT(partPath, payload, ""); err != nil {
			return fmt.Errorf("上传 object 分块 %d 失败: %w", i, err)
		}
	}
	return nil
}

func splitObject(data []byte, chunkSize int) (*objectManifest, [][]byte) {
	if chunkSize <= 0 {
		chunkSize = defaultObjectChunkSize
	}
	totalSize := len(data)
	totalParts := (totalSize + chunkSize - 1) / chunkSize
	if totalParts == 0 {
		totalParts = 1
	}
	chunks := make([][]byte, totalParts)
	partHashes := make([]string, totalParts)
	for i := 0; i < totalParts; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > totalSize {
			end = totalSize
		}
		chunks[i] = data[start:end]
		h := sha256.Sum256(chunks[i])
		partHashes[i] = hex.EncodeToString(h[:])
	}
	return &objectManifest{Hash: ComputeHash(data), TotalParts: totalParts, PartHashes: partHashes, TotalSize: int64(totalSize)}, chunks
}

func shouldChunkObject(size int64, mode string, threshold int64) bool {
	switch mode {
	case "always":
		return true
	case "never":
		return false
	default:
		return size > threshold
	}
}

func objectPaths(hash string) []string {
	primary := ObjectPath(hash)
	legacy := filepath.ToSlash(filepath.Join("objects", "sh", hash+".enc"))
	if legacy == primary {
		return []string{primary}
	}
	return []string{primary, legacy}
}

func objectPartsBasePath(hash string) string {
	return filepath.ToSlash(filepath.Join("objects", "parts", hash)) + "/"
}

func objectManifestPath(hash string) string {
	return objectPartsBasePath(hash) + "manifest.json"
}

func objectPartPath(hash string, index int, encrypted bool) string {
	ext := ".bin"
	if encrypted {
		ext = ".enc"
	}
	return objectPartsBasePath(hash) + fmt.Sprintf("part-%03d%s", index, ext)
}
