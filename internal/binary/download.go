// 二进制下载
// 支持分块下载和断点续传
package binary

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/user/cc-box/internal/config"
	"github.com/user/cc-box/internal/crypto"
	"github.com/user/cc-box/internal/webdav"
)

// DownloadProgress 下载进度回调
type DownloadProgress func(total, downloaded int64, partIndex, totalParts int)

// Download 从 WebDAV 下载二进制文件
func Download(client *webdav.Client, key []byte, name string, version string, targetPath string, progress DownloadProgress) error {
	platform := config.Platform()
	idx, err := LoadIndex(client)
	if err != nil {
		return err
	}

	info := idx.GetBinaryInfo(platform, name)
	if info == nil {
		return fmt.Errorf("平台 %s 上没有 %s 的记录", platform, name)
	}

	v, exists := info.Versions[version]
	if !exists {
		return fmt.Errorf("版本 %s 不存在", version)
	}

	var data []byte

	if ShouldChunk(v.Size) {
		data, err = downloadChunked(client, key, v.Hash, v.Size, progress)
	} else {
		data, err = downloadWhole(client, key, name, platform)
	}
	if err != nil {
		return err
	}

	// 写入目标文件
	os.MkdirAll(filepath.Dir(targetPath), 0755)
	if err := os.WriteFile(targetPath, data, 0755); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

func downloadChunked(client *webdav.Client, key []byte, hash string, totalSize int64, progress DownloadProgress) ([]byte, error) {
	// 下载 manifest
	manifestPath := fmt.Sprintf("binaries/parts/%s/manifest.json", hash)
	manifestData, _, err := client.GET(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("下载 manifest 失败: %w", err)
	}

	manifest, err := DeserializeManifest(manifestData)
	if err != nil {
		return nil, err
	}

	result := make([]byte, 0, totalSize)
	for i := 0; i < manifest.TotalParts; i++ {
		partPath := fmt.Sprintf("binaries/parts/%s/part-%03d.enc", hash, i)

		encrypted, _, err := client.GET(partPath)
		if err != nil {
			return nil, fmt.Errorf("下载分块 %d 失败: %w", i, err)
		}

		chunk, err := crypto.Decrypt(encrypted, key)
		if err != nil {
			return nil, fmt.Errorf("解密分块 %d 失败: %w", i, err)
		}

		result = append(result, chunk...)

		if progress != nil {
			progress(totalSize, int64(len(result)), i+1, manifest.TotalParts)
		}
	}

	return result, nil
}

func downloadWhole(client *webdav.Client, key []byte, name string, platform string) ([]byte, error) {
	path := fmt.Sprintf("binaries/%s/%s.enc", platform, name)
	encrypted, _, err := client.GET(path)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}

	return crypto.Decrypt(encrypted, key)
}
