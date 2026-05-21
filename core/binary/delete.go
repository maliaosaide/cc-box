// 二进制远程版本删除
package binary

import (
	"fmt"

	"github.com/user/cc-box/core/webdav"
)

// DeleteRemoteVersion 从云端删除指定二进制版本
func DeleteRemoteVersion(client *webdav.Client, key []byte, name, version, platform string) error {
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

	deletePhysical := !v.Chunked || countBinaryHashRefs(idx, v.Hash) == 1

	// 先移除索引，避免 SaveIndex 失败时留下指向已删除实体的版本记录
	delete(info.Versions, version)
	if err := SaveIndex(client, idx); err != nil {
		return err
	}
	if !deletePhysical {
		return nil
	}

	if v.Chunked {
		basePath := fmt.Sprintf("binaries/parts/%s/", v.Hash)
		manifestData, _, err := client.GET(basePath + "manifest.json")
		totalParts := 0
		if err == nil {
			if manifest, err := DeserializeManifest(manifestData); err == nil {
				totalParts = manifest.TotalParts
			}
		}
		ext := extForEncrypted(v.Encrypted)
		for i := 0; i < totalParts; i++ {
			partPath := fmt.Sprintf("%spart-%03d%s", basePath, i, ext)
			if err := client.DELETE(partPath); err != nil && err != webdav.ErrNotFound {
				return err
			}
		}
		if err := client.DELETE(basePath + "manifest.json"); err != nil && err != webdav.ErrNotFound {
			return err
		}
		return nil
	}

	ext := extForEncrypted(v.Encrypted)
	path := fmt.Sprintf("binaries/%s/%s-%s%s", platform, name, version, ext)
	if err := client.DELETE(path); err != nil && err != webdav.ErrNotFound {
		return err
	}
	return nil
}

func countBinaryHashRefs(idx *Index, hash string) int {
	count := 0
	for _, platform := range idx.Platforms {
		infos := []*BinaryInfo{platform.Claude, platform.UV, platform.UVX, platform.UVW}
		for _, info := range platform.Custom {
			infos = append(infos, info)
		}
		for _, info := range infos {
			if info == nil {
				continue
			}
			for _, version := range info.Versions {
				if version.Hash == hash {
					count++
				}
			}
		}
	}
	return count
}
