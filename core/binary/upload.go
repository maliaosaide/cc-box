// 二进制上传
// 支持加密/不加密 × 分块/整体 四种组合
package binary

import (
	"fmt"
	"time"

	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/crypto"
	"github.com/user/cc-box/core/webdav"
)

// UploadProgress 上传进度回调
type UploadProgress func(total, uploaded int64, partIndex, totalParts int)

// Upload 上传二进制文件到 WebDAV
func Upload(client *webdav.Client, key []byte, name string, data []byte, version string, progress UploadProgress) error {
	cfg := loadBinaryConfig()
	platform := config.Platform()

	idx, err := LoadIndex(client)
	if err != nil {
		return err
	}

	info := idx.EnsureBinaryInfo(platform, name)

	// 去重检查
	manifest, _ := computeManifest(data)
	if v, exists := info.Versions[version]; exists && v.Hash == manifest.Hash {
		return fmt.Errorf("版本 %s 已存在云端", version)
	}

	chunkSize := cfg.ChunkSizeMB * 1024 * 1024
	thresholdBytes := int64(cfg.ChunkThresholdMB) * 1024 * 1024
	shouldChunk := ShouldChunk(int64(len(data)), cfg.ChunkMode, thresholdBytes)

	if shouldChunk {
		err = uploadChunked(client, key, data, manifest, cfg.Encrypt, chunkSize, progress)
	} else {
		err = uploadWhole(client, key, name, version, data, platform, cfg.Encrypt)
	}
	if err != nil {
		return err
	}

	uploadedBy := ""
	if appCfg, err := config.Load(); err == nil {
		uploadedBy = appCfg.Device.ID
	}

	// 更新索引
	info.Versions[version] = Version{
		Hash:       manifest.Hash,
		Size:       int64(len(data)),
		Refs:       0,
		Uploaded:   now(),
		UploadedBy: uploadedBy,
		Encrypted:  cfg.Encrypt,
		Chunked:    shouldChunk,
	}

	return SaveIndex(client, idx)
}

func uploadChunked(client *webdav.Client, key []byte, data []byte, manifest *Manifest, encrypt bool, chunkSize int, progress UploadProgress) error {
	chunkResult := Split(data, chunkSize)
	basePath := fmt.Sprintf("binaries/parts/%s/", manifest.Hash)
	ext := extForEncrypted(encrypt)

	// 上传 manifest
	manifestData, _ := SerializeManifest(chunkResult.Manifest)
	client.EnsureDir(basePath + "manifest.json")
	client.PUT(basePath+"manifest.json", manifestData, "")

	// 逐块上传
	for i, chunk := range chunkResult.Chunks {
		partPath := basePath + fmt.Sprintf("part-%03d%s", i, ext)

		// 断点续传：检查是否已存在
		if exists, _ := client.Exists(partPath); exists {
			if progress != nil {
				progress(int64(len(data)), int64(i*chunkSize), i, chunkResult.Manifest.TotalParts)
			}
			continue
		}

		// 处理数据（加密或不加密）
		payload := chunk
		if encrypt {
			encrypted, err := crypto.Encrypt(chunk, key)
			if err != nil {
				return fmt.Errorf("加密分块 %d 失败: %w", i, err)
			}
			payload = encrypted
		}

		client.EnsureDir(partPath)
		if _, err := client.PUT(partPath, payload, ""); err != nil {
			return fmt.Errorf("上传分块 %d 失败: %w", i, err)
		}

		if progress != nil {
			progress(int64(len(data)), int64((i+1)*chunkSize), i+1, chunkResult.Manifest.TotalParts)
		}
	}

	return nil
}

func uploadWhole(client *webdav.Client, key []byte, name string, version string, data []byte, platform string, encrypt bool) error {
	payload := data
	if encrypt {
		encrypted, err := crypto.Encrypt(data, key)
		if err != nil {
			return fmt.Errorf("加密失败: %w", err)
		}
		payload = encrypted
	}

	ext := extForEncrypted(encrypt)
	path := fmt.Sprintf("binaries/%s/%s-%s%s", platform, name, version, ext)
	client.EnsureDir(path)
	_, err := client.PUT(path, payload, "")
	return err
}

func computeManifest(data []byte) (*Manifest, error) {
	result := Split(data, DefaultChunkSize)
	return result.Manifest, nil
}

func now() time.Time {
	return time.Now().UTC()
}

// extForEncrypted 根据是否加密返回文件扩展名
func extForEncrypted(encrypt bool) string {
	if encrypt {
		return ".enc"
	}
	return ".bin"
}

// loadBinaryConfig 从配置文件加载二进制配置，使用默认值兜底
func loadBinaryConfig() config.BinaryConfig {
	cfg, err := config.Load()
	if err != nil {
		return config.BinaryConfig{
			Encrypt:          false,
			ChunkMode:        "auto",
			ChunkSizeMB:      10,
			ChunkThresholdMB: 50,
		}
	}
	bc := cfg.Binary
	if bc.ChunkMode == "" {
		bc.ChunkMode = "auto"
	}
	if bc.ChunkSizeMB == 0 {
		bc.ChunkSizeMB = 10
	}
	if bc.ChunkThresholdMB == 0 {
		bc.ChunkThresholdMB = 50
	}
	return bc
}
