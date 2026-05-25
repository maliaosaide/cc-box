// 二进制远程版本删除
package binary

import (
	"fmt"

	"github.com/user/cc-box/core/webdav"
)

// DeleteRemoteVersion 从云端删除指定二进制版本
func DeleteRemoteVersion(client *webdav.Client, key []byte, name, version, platform string) error {
	return withBinaryVersionLock(client, platform, name, version, func() error {
		current, err := loadVersionForDelete(client, platform, name, version)
		if err != nil {
			return err
		}
		deleteWithIndex := func() error {
			v, deletePhysical, err := removeVersionFromIndex(client, platform, name, version)
			if err != nil || !deletePhysical {
				return err
			}
			return deleteBinaryPayload(client, platform, name, version, v)
		}
		if current.Chunked {
			return withBinaryHashLock(client, current.Hash, deleteWithIndex)
		}
		return deleteWithIndex()
	})
}

func loadVersionForDelete(client *webdav.Client, platform, name, version string) (Version, error) {
	idx, err := LoadIndex(client)
	if err != nil {
		return Version{}, err
	}
	info := idx.GetBinaryInfo(platform, name)
	if info == nil {
		return Version{}, fmt.Errorf("平台 %s 上没有 %s 的记录", platform, name)
	}
	v, exists := info.Versions[version]
	if !exists {
		return Version{}, fmt.Errorf("版本 %s 不存在", version)
	}
	return v, nil
}

func removeVersionFromIndex(client *webdav.Client, platform, name, version string) (Version, bool, error) {
	var removed Version
	deletePhysical := false
	if err := UpdateIndex(client, func(idx *Index) error {
		info := idx.GetBinaryInfo(platform, name)
		if info == nil {
			return fmt.Errorf("平台 %s 上没有 %s 的记录", platform, name)
		}

		current, exists := info.Versions[version]
		if !exists {
			return fmt.Errorf("版本 %s 不存在", version)
		}

		removed = current
		deletePhysical = !removed.Chunked || countBinaryHashRefs(idx, removed.Hash) == 1
		delete(info.Versions, version)
		if info.Current == version {
			info.Current = nextBinaryCurrent(info)
		}
		return nil
	}); err != nil {
		return Version{}, false, err
	}
	return removed, deletePhysical, nil
}

func nextBinaryCurrent(info *BinaryInfo) string {
	if info == nil || len(info.Versions) == 0 {
		return ""
	}
	selected := ""
	var selectedUploaded int64
	for version, item := range info.Versions {
		uploaded := item.Uploaded.UnixNano()
		if selected == "" || uploaded > selectedUploaded || (uploaded == selectedUploaded && version > selected) {
			selected = version
			selectedUploaded = uploaded
		}
	}
	return selected
}

func deleteBinaryPayload(client *webdav.Client, platform, name, version string, v Version) error {
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
