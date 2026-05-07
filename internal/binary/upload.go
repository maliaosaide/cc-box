// 二进制上传
// 分块上传 + 断点续传
package binary

import (
	"fmt"
	"time"

	"github.com/user/cc-box/internal/config"
	"github.com/user/cc-box/internal/crypto"
	"github.com/user/cc-box/internal/webdav"
)

// UploadProgress 上传进度回调
type UploadProgress func(total, uploaded int64, partIndex, totalParts int)

// Upload 上传二进制文件到 WebDAV
func Upload(client *webdav.Client, key []byte, name string, data []byte, version string, progress UploadProgress) error {
	platform := config.Platform()

	// 加载索引
	idx, err := LoadIndex(client)
	if err != nil {
		return err
	}

	info := idx.EnsureBinaryInfo(platform, name)

	// 检查是否已上传（去重）
	manifest, _ := computeManifest(data)
	if v, exists := info.Versions[version]; exists && v.Hash == manifest.Hash {
		return fmt.Errorf("版本 %s 已存在云端", version)
	}

	// 判断是否分块
	if ShouldChunk(int64(len(data))) {
		err = uploadChunked(client, key, data, manifest, progress)
	} else {
		err = uploadWhole(client, key, name, data, platform)
	}
	if err != nil {
		return err
	}

	// 更新索引
	info.Versions[version] = Version{
		Hash:       manifest.Hash,
		Size:       int64(len(data)),
		Refs:       0,
		Uploaded:   now(),
		UploadedBy: "", // 从 config 加载
	}

	return SaveIndex(client, idx)
}

func uploadChunked(client *webdav.Client, key []byte, data []byte, manifest *Manifest, progress UploadProgress) error {
	chunkResult := Split(data, DefaultChunkSize)
	basePath := fmt.Sprintf("binaries/parts/%s/", manifest.Hash)

	// 上传 manifest
	manifestData, _ := SerializeManifest(chunkResult.Manifest)
	client.EnsureDir(basePath + "manifest.json")
	client.PUT(basePath+"manifest.json", manifestData, "")

	// 逐块上传
	for i, chunk := range chunkResult.Chunks {
		partPath := basePath + fmt.Sprintf("part-%03d.enc", i)

		// 断点续传：检查是否已存在
		if exists, _ := client.Exists(partPath); exists {
			if progress != nil {
				progress(int64(len(data)), int64(i*DefaultChunkSize), i, chunkResult.Manifest.TotalParts)
			}
			continue
		}

		// 加密
		encrypted, err := crypto.Encrypt(chunk, key)
		if err != nil {
			return fmt.Errorf("加密分块 %d 失败: %w", i, err)
		}

		client.EnsureDir(partPath)
		if _, err := client.PUT(partPath, encrypted, ""); err != nil {
			return fmt.Errorf("上传分块 %d 失败: %w", i, err)
		}

		if progress != nil {
			progress(int64(len(data)), int64((i+1)*DefaultChunkSize), i+1, chunkResult.Manifest.TotalParts)
		}
	}

	return nil
}

func uploadWhole(client *webdav.Client, key []byte, name string, data []byte, platform string) error {
	encrypted, err := crypto.Encrypt(data, key)
	if err != nil {
		return fmt.Errorf("加密失败: %w", err)
	}

	path := fmt.Sprintf("binaries/%s/%s.enc", platform, name)
	client.EnsureDir(path)
	_, err = client.PUT(path, encrypted, "")
	return err
}

func computeManifest(data []byte) (*Manifest, error) {
	result := Split(data, DefaultChunkSize)
	return result.Manifest, nil
}

func now() time.Time {
	return time.Now().UTC()
}
