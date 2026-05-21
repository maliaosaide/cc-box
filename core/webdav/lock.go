// WebDAV 乐观锁
// 基于 ETag 的 compare-and-swap 机制
package webdav

import (
	"fmt"
)

// LockResult 乐观锁操作结果
type LockResult struct {
	Success   bool
	NewETag   string
	CurrentID string // 远程当前 HEAD ID
}

// CompareAndSwapHEAD 原子更新远程 HEAD 指针
// expectedETag 为空时表示首次写入（不检查）
func (c *Client) CompareAndSwapHEAD(headPath string, newID string, expectedETag string) (*LockResult, error) {
	// 读取当前 HEAD
	currentID, currentETag, err := c.readHEAD(headPath)
	if err != nil && err != ErrNotFound {
		return nil, fmt.Errorf("读取 HEAD 失败: %w", err)
	}

	result := &LockResult{CurrentID: currentID}

	// 如果期望的 ETag 不匹配（远程已被其他设备更新）
	if expectedETag != "" && currentETag != expectedETag {
		result.Success = false
		return result, nil
	}

	// 上传新 HEAD
	newETag, err := c.PUT(headPath, []byte(newID), expectedETag)
	if err == ErrConflict {
		result.Success = false
		return result, nil
	}
	if err != nil {
		return nil, fmt.Errorf("写入 HEAD 失败: %w", err)
	}

	result.Success = true
	result.NewETag = newETag
	return result, nil
}

// readHEAD 读取远程 HEAD 指针
func (c *Client) readHEAD(headPath string) (string, string, error) {
	data, etag, err := c.GET(headPath)
	if err != nil {
		return "", "", err
	}
	return string(data), etag, nil
}
