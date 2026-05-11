// 二进制远程版本删除
package binary

import (
	"fmt"

	"github.com/user/cc-box/internal/webdav"
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

	// 删除云端文件
	if v.Chunked {
		// 分块存储：删除 manifest 和 parts 目录
		basePath := fmt.Sprintf("binaries/parts/%s/", v.Hash)
		client.DELETE(basePath + "manifest.json")
		ext := extForEncrypted(v.Encrypted)
		for i := 0; i < 1000; i++ {
			partPath := fmt.Sprintf("%spart-%03d%s", basePath, i, ext)
			if err := client.DELETE(partPath); err != nil {
				break
			}
		}
	} else {
		ext := extForEncrypted(v.Encrypted)
		path := fmt.Sprintf("binaries/%s/%s-%s%s", platform, name, version, ext)
		client.DELETE(path)
	}

	// 从索引中移除
	delete(info.Versions, version)

	return SaveIndex(client, idx)
}
