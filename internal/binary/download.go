// 二进制下载
// 根据 index 中记录的 encrypted/chunked 元数据选择对应策略
package binary

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/user/cc-box/internal/config"
	"github.com/user/cc-box/internal/crypto"
	"github.com/user/cc-box/internal/object"
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

	if v.Chunked {
		data, err = downloadChunked(client, key, v.Hash, v.Size, v.Encrypted, progress)
	} else {
		data, err = downloadWhole(client, key, name, version, platform, v.Encrypted)
	}
	if err != nil {
		return err
	}
	if !object.ValidateHash(data, v.Hash) {
		return fmt.Errorf("二进制 hash 校验失败: %s", v.Hash)
	}

	if err := WriteFileAtomic(targetPath, data, 0755); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

func WriteFileAtomic(targetPath string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(targetPath)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if err := os.Rename(tmpPath, targetPath); err != nil {
			return err
		}
		cleanupTemp = false
		return nil
	}
	return replaceFileWindows(tmpPath, targetPath, &cleanupTemp)
}

func replaceFileWindows(tmpPath, targetPath string, cleanupTemp *bool) error {
	backupPath := targetPath + ".old"
	for i := 0; ; i++ {
		candidate := fmt.Sprintf("%s.%d", backupPath, i)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			backupPath = candidate
			break
		}
	}
	hadTarget := false
	if _, err := os.Stat(targetPath); err == nil {
		hadTarget = true
		if err := os.Rename(targetPath, backupPath); err != nil {
			return err
		}
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		if hadTarget {
			_ = os.Rename(backupPath, targetPath)
		}
		return err
	}
	*cleanupTemp = false
	if hadTarget {
		_ = os.Remove(backupPath)
	}
	return nil
}

func BackupFileIfMissing(srcPath, backupPath string) error {
	if _, err := os.Stat(backupPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	return WriteFileAtomic(backupPath, data, 0755)
}

func downloadChunked(client *webdav.Client, key []byte, hash string, totalSize int64, encrypted bool, progress DownloadProgress) ([]byte, error) {
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
	if manifest.Hash != hash {
		return nil, fmt.Errorf("manifest hash 不匹配: %s", manifest.Hash)
	}
	if manifest.TotalSize != totalSize {
		return nil, fmt.Errorf("manifest size 不匹配: %d", manifest.TotalSize)
	}
	if len(manifest.PartHashes) != manifest.TotalParts {
		return nil, fmt.Errorf("manifest 分块 hash 数量不匹配")
	}

	ext := extForEncrypted(encrypted)
	result := make([]byte, 0, totalSize)
	for i := 0; i < manifest.TotalParts; i++ {
		partPath := fmt.Sprintf("binaries/parts/%s/part-%03d%s", hash, i, ext)

		payload, _, err := client.GET(partPath)
		if err != nil {
			return nil, fmt.Errorf("下载分块 %d 失败: %w", i, err)
		}

		var chunk []byte
		if encrypted {
			chunk, err = crypto.Decrypt(payload, key)
			if err != nil {
				return nil, fmt.Errorf("解密分块 %d 失败: %w", i, err)
			}
		} else {
			chunk = payload
		}

		partHash := sha256.Sum256(chunk)
		if hex.EncodeToString(partHash[:]) != manifest.PartHashes[i] {
			return nil, fmt.Errorf("分块 %d hash 校验失败", i)
		}

		result = append(result, chunk...)

		if progress != nil {
			progress(totalSize, int64(len(result)), i+1, manifest.TotalParts)
		}
	}

	if int64(len(result)) != totalSize {
		return nil, fmt.Errorf("下载大小不匹配: %d", len(result))
	}
	return result, nil
}

func downloadWhole(client *webdav.Client, key []byte, name string, version string, platform string, encrypted bool) ([]byte, error) {
	ext := extForEncrypted(encrypted)
	path := fmt.Sprintf("binaries/%s/%s-%s%s", platform, name, version, ext)

	payload, _, err := client.GET(path)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}

	if encrypted {
		return crypto.Decrypt(payload, key)
	}
	return payload, nil
}
