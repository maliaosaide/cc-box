// 二进制版本索引管理
// 管理 WebDAV 上的 binaries/index.json
package binary

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/user/cc-box/internal/config"
	"github.com/user/cc-box/internal/webdav"
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

// LoadIndex 从 WebDAV 加载索引
func LoadIndex(client *webdav.Client) (*Index, error) {
	data, _, err := client.GET("binaries/index.json")
	if err == webdav.ErrNotFound {
		return NewIndex(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("下载索引失败: %w", err)
	}

	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("解析索引失败: %w", err)
	}
	if idx.Platforms == nil {
		idx.Platforms = make(map[string]PlatformBins)
	}
	return &idx, nil
}

// SaveIndex 保存索引到 WebDAV
func SaveIndex(client *webdav.Client, idx *Index) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化索引失败: %w", err)
	}

	client.EnsureDir("binaries/index.json")
	_, err = client.PUT("binaries/index.json", data, "")
	return err
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

// GetBinaryPath 获取二进制文件的本地路径
func GetBinaryPath(name string) string {
	ext := ""
	if config.Platform()[:7] == "windows" {
		ext = ".exe"
	}
	return config.LocalBinDir() + "/" + name + ext
}
