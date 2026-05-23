// 二进制版本索引管理
// 管理 WebDAV 上的 binaries/index.json
package binary

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/webdav"
)

// Index 二进制版本索引
type Index struct {
	Platforms map[string]PlatformBins `json:"platforms"`
}

// PlatformBins 某平台的二进制版本
type PlatformBins struct {
	Claude *BinaryInfo            `json:"claude,omitempty"`
	UV     *BinaryInfo            `json:"uv,omitempty"`
	UVX    *BinaryInfo            `json:"uvx,omitempty"`
	UVW    *BinaryInfo            `json:"uvw,omitempty"`
	Custom map[string]*BinaryInfo `json:"custom,omitempty"`
}

// BinaryInfo 单个二进制的版本信息
type BinaryInfo struct {
	Current  string             `json:"current"`
	Versions map[string]Version `json:"versions"`
}

// Version 版本详情
type Version struct {
	Hash       string    `json:"hash"`
	Size       int64     `json:"size"`
	Refs       int       `json:"refs"`
	Uploaded   time.Time `json:"uploaded"`
	UploadedBy string    `json:"uploaded_by"`
	Encrypted  bool      `json:"encrypted"`
	Chunked    bool      `json:"chunked"`
}

// NewIndex 创建空索引
func NewIndex() *Index {
	return &Index{
		Platforms: make(map[string]PlatformBins),
	}
}

const (
	indexPath             = "binaries/index.json"
	maxIndexUpdateRetries = 5
)

// IndexRevision 是带 ETag 的索引快照。
type IndexRevision struct {
	Index  *Index
	ETag   string
	Exists bool
}

// LoadIndex 从 WebDAV 加载索引。
func LoadIndex(client *webdav.Client) (*Index, error) {
	rev, err := LoadIndexRevision(client)
	if err != nil {
		return nil, err
	}
	return rev.Index, nil
}

// LoadIndexRevision 从 WebDAV 加载索引和写入前置条件。
func LoadIndexRevision(client *webdav.Client) (*IndexRevision, error) {
	data, etag, err := client.GET(indexPath)
	if err == webdav.ErrNotFound {
		return &IndexRevision{Index: NewIndex()}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("下载索引失败: %w", err)
	}

	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("解析索引失败: %w", err)
	}
	normalizeIndex(&idx)
	return &IndexRevision{Index: &idx, ETag: etag, Exists: true}, nil
}

// SaveIndex 使用给定的 ETag 前置条件保存索引到 WebDAV。
func SaveIndex(client *webdav.Client, idx *Index, expectedETag string, exists bool) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化索引失败: %w", err)
	}
	if err := client.EnsureDir(indexPath); err != nil {
		return err
	}
	if !exists {
		_, err = client.PUTIfAbsent(indexPath, data)
		return err
	}
	if expectedETag == "" {
		return fmt.Errorf("远程二进制索引没有 ETag，无法安全更新")
	}
	_, err = client.PUT(indexPath, data, expectedETag)
	return err
}

// UpdateIndex 对索引执行 CAS 更新；遇到并发冲突会重新加载并重试。
func UpdateIndex(client *webdav.Client, mutate func(*Index) error) error {
	for i := 0; i < maxIndexUpdateRetries; i++ {
		rev, err := LoadIndexRevision(client)
		if err != nil {
			return err
		}
		if err := mutate(rev.Index); err != nil {
			return err
		}
		if err := SaveIndex(client, rev.Index, rev.ETag, rev.Exists); err != nil {
			if err == webdav.ErrConflict {
				continue
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("二进制索引被其他设备持续更新，请重试")
}

func normalizeIndex(idx *Index) {
	if idx.Platforms == nil {
		idx.Platforms = make(map[string]PlatformBins)
	}
}

// GetBinaryInfo 获取指定平台和二进制的版本信息
func (idx *Index) GetBinaryInfo(platform, name string) *BinaryInfo {
	p, ok := idx.Platforms[platform]
	if !ok {
		return nil
	}

	switch name {
	case "claude":
		return p.Claude
	case "uv":
		return p.UV
	case "uvx":
		return p.UVX
	case "uvw":
		return p.UVW
	default:
		if p.Custom != nil {
			return p.Custom[name]
		}
		return nil
	}
}

// EnsureBinaryInfo 确保版本信息存在
func (idx *Index) EnsureBinaryInfo(platform, name string) *BinaryInfo {
	if idx.Platforms == nil {
		idx.Platforms = make(map[string]PlatformBins)
	}

	p := idx.Platforms[platform]

	var info *BinaryInfo
	switch name {
	case "claude":
		if p.Claude == nil {
			p.Claude = &BinaryInfo{Versions: make(map[string]Version)}
		}
		info = p.Claude
	case "uv":
		if p.UV == nil {
			p.UV = &BinaryInfo{Versions: make(map[string]Version)}
		}
		info = p.UV
	case "uvx":
		if p.UVX == nil {
			p.UVX = &BinaryInfo{Versions: make(map[string]Version)}
		}
		info = p.UVX
	case "uvw":
		if p.UVW == nil {
			p.UVW = &BinaryInfo{Versions: make(map[string]Version)}
		}
		info = p.UVW
	default:
		if p.Custom == nil {
			p.Custom = make(map[string]*BinaryInfo)
		}
		if p.Custom[name] == nil {
			p.Custom[name] = &BinaryInfo{Versions: make(map[string]Version)}
		}
		info = p.Custom[name]
	}

	idx.Platforms[platform] = p
	return info
}

// GetBinaryPath 获取二进制文件的受管写入路径
func GetBinaryPath(name string) string {
	if name == "claude" {
		return ResolveClaudeManagedPath()
	}
	return filepath.Join(config.LocalBinDir(), executableName(name))
}
