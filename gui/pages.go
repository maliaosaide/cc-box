// 历史、设置、项目页面后端绑定
// 快照列表、配置读写、项目追踪
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/user/cc-box/internal/binary"
	"github.com/user/cc-box/internal/config"
	"github.com/user/cc-box/internal/crypto"
	"github.com/user/cc-box/internal/project"
	"github.com/user/cc-box/internal/snapshot"
	"github.com/user/cc-box/internal/webdav"
)

// SnapshotEntry 快照历史条目
type SnapshotEntry struct {
	ID        string `json:"id"`
	ShortID   string `json:"shortId"`
	Parent    string `json:"parent"`
	Timestamp string `json:"timestamp"`
	Device    string `json:"device"`
	Message   string `json:"message"`
	FileCount int    `json:"fileCount"`
}

// SnapshotDetail 快照详情
type SnapshotDetail struct {
	ID        string                 `json:"id"`
	Timestamp string                 `json:"timestamp"`
	Device    string                 `json:"device"`
	Message   string                 `json:"message"`
	Parent    string                 `json:"parent"`
	Files     map[string]FileEntry   `json:"files"`
	Binary    map[string]map[string]string `json:"binary"`
}

type FileEntry struct {
	Hash     string `json:"hash"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

// GetSnapshotList 返回快照历史列表
func (a *App) GetSnapshotList(limit int) ([]SnapshotEntry, error) {
	_, client, key, err := a.loadClients()
	if err != nil {
		return nil, err
	}

	headData, _, err := client.GET("HEAD")
	if err != nil {
		return nil, fmt.Errorf("读取远程 HEAD 失败: %w", err)
	}
	currentID := strings.TrimSpace(string(headData))
	if currentID == "" {
		return nil, nil
	}

	if limit <= 0 {
		limit = 20
	}

	var entries []SnapshotEntry
	snapID := currentID
	for i := 0; i < limit && snapID != ""; i++ {
		snap, err := a.loadSnapByID(client, key, snapID)
		if err != nil {
			break
		}

		shortID := snap.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}

		entries = append(entries, SnapshotEntry{
			ID:        snap.ID,
			ShortID:   shortID,
			Parent:    snap.Parent,
			Timestamp: snap.Timestamp.Format("2006-01-02 15:04"),
			Device:    snap.Device,
			Message:   snap.Message,
			FileCount: len(snap.Files),
		})
		snapID = snap.Parent
	}

	return entries, nil
}

// GetSnapshotDetail 返回快照详情
func (a *App) GetSnapshotDetail(id string) (*SnapshotDetail, error) {
	_, client, key, err := a.loadClients()
	if err != nil {
		return nil, err
	}

	snap, err := a.loadSnapByID(client, key, id)
	if err != nil {
		return nil, err
	}

	files := make(map[string]FileEntry)
	for path, entry := range snap.Files {
		files[path] = FileEntry{
			Hash:     entry.Hash[:16],
			Size:     entry.Size,
			Modified: entry.Modified.Format("2006-01-02 15:04"),
		}
	}

	return &SnapshotDetail{
		ID:        snap.ID,
		Timestamp: snap.Timestamp.Format("2006-01-02 15:04:05"),
		Device:    snap.Device,
		Message:   snap.Message,
		Parent:    snap.Parent,
		Files:     files,
		Binary:    snap.Binary,
	}, nil
}

// loadSnapByID 按ID加载快照
func (a *App) loadSnapByID(client *webdav.Client, key []byte, id string) (*snapshot.Snapshot, error) {
	// 先本地缓存
	snapDir := config.CCBoxDir() + "/snapshots/"
	data, err := os.ReadFile(snapDir + id + ".json")
	if err == nil {
		return snapshot.Deserialize(data)
	}
	// 从远程下载
	snapPath := "snapshots/" + id + ".json.enc"
	encrypted, _, err := client.GET(snapPath)
	if err != nil {
		return nil, err
	}
	decrypted, err := crypto.Decrypt(encrypted, key)
	if err != nil {
		return nil, err
	}
	snap, err := snapshot.Deserialize(decrypted)
	if err != nil {
		return nil, err
	}
	// 缓存到本地
	snapData, _ := snap.Serialize()
	os.WriteFile(snapDir+id+".json", snapData, 0600)
	return snap, nil
}

// GetConfig 返回当前完整配置
func (a *App) GetConfig() (*ConfigView, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return &ConfigView{
		WebDAV: WebDAVView{
			URL:      cfg.WebDAV.URL,
			Username: cfg.WebDAV.Username,
			Root:     cfg.WebDAV.Root,
		},
		Device: DeviceView{
			ID:   cfg.Device.ID,
			Name: cfg.Device.Name,
		},
		Encryption: EncryptionView{
			Enabled: cfg.Encryption.Enabled,
		},
		Sync: SyncView{
			SnapshotLimit:   cfg.Sync.SnapshotLimit,
			ConflictStrategy: cfg.Sync.ConflictStrategy,
			MergeRetryMax:   cfg.Sync.MergeRetryMax,
		},
		Exclude: cfg.Exclude.Patterns,
		ClaudeDir: config.ClaudeDir(),
	}, nil
}

// SetConfigField 修改单个配置项
func (a *App) SetConfigField(section, key, value string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	switch section {
	case "device":
		if key == "name" {
			cfg.Device.Name = value
		}
	case "sync":
		if key == "conflict_strategy" {
			cfg.Sync.ConflictStrategy = value
		}
	}

	return config.Save(cfg)
}

// TestConnection 测试 WebDAV 连接
func (a *App) TestConnection() (*ConnectionTest, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	pass, err := config.LoadWebDAVPassword()
	if err != nil {
		return &ConnectionTest{Success: false, Error: "未配置密码"}, nil
	}

	client := webdav.NewClient(cfg.WebDAV.URL, cfg.WebDAV.Username, pass)
	start := time.Now()

	_, _, err = client.GET("HEAD")
	elapsed := time.Since(start)

	if err != nil && err != webdav.ErrNotFound {
		return &ConnectionTest{Success: false, Error: err.Error(), Latency: elapsed.Milliseconds()}, nil
	}

	return &ConnectionTest{Success: true, Latency: elapsed.Milliseconds()}, nil
}

// GetProjectList 返回已追踪项目列表和 orphan 列表
func (a *App) GetProjectList() (*ProjectListResult, error) {
	result := &ProjectListResult{}

	// 发现本地项目
	projects, err := project.DiscoverProjects()
	if err != nil {
		return nil, fmt.Errorf("扫描项目失败: %w", err)
	}

	for _, p := range projects {
		name := filepath.Base(p.LocalPath)
		mcpCount := 0
		claudeJSON, err := project.LoadClaudeJSON(p.LocalPath)
		if err == nil {
			if servers, ok := claudeJSON["mcpServers"].(map[string]interface{}); ok {
				mcpCount = len(servers)
			}
		}

		result.Projects = append(result.Projects, ProjectInfo{
			Name:       name,
			Path:       p.LocalPath,
			Remote:     p.RemoteURL,
			RemoteName: p.RemoteName,
			MCPCount:   mcpCount,
			HasLocal:   true,
		})
	}

	// 加载 orphan 项目
	orphanIdx, err := project.LoadOrphanIndex()
	if err == nil {
		for _, o := range orphanIdx.Orphans {
			result.Orphans = append(result.Orphans, OrphanInfo{
				Remote:     o.RemoteURL,
				Discovered: o.Discovered,
			})
		}
	}

	return result, nil
}

// ProjectListResult 项目列表结果
type ProjectListResult struct {
	Projects []ProjectInfo `json:"projects"`
	Orphans  []OrphanInfo  `json:"orphans"`
}

// ConfigView 配置视图
type ConfigView struct {
	WebDAV     WebDAVView    `json:"webdav"`
	Device     DeviceView    `json:"device"`
	Encryption EncryptionView `json:"encryption"`
	Sync       SyncView      `json:"sync"`
	Exclude    []string      `json:"exclude"`
	ClaudeDir  string        `json:"claudeDir"`
}

type WebDAVView struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Root     string `json:"root"`
}

type DeviceView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type EncryptionView struct {
	Enabled bool `json:"enabled"`
}

type SyncView struct {
	SnapshotLimit   int    `json:"snapshotLimit"`
	ConflictStrategy string `json:"conflictStrategy"`
	MergeRetryMax   int    `json:"mergeRetryMax"`
}

type ConnectionTest struct {
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
	Latency  int64  `json:"latency"`
}

type ProjectInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Remote     string `json:"remote"`
	RemoteName string `json:"remoteName"`
	LastSync   string `json:"lastSync"`
	MCPCount   int    `json:"mcpCount"`
	HasLocal   bool   `json:"hasLocal"`
	HasRemote  bool   `json:"hasRemote"`
	IsOrphan   bool   `json:"isOrphan"`
}

type OrphanInfo struct {
	Remote     string `json:"remote"`
	Discovered string `json:"discovered"`
}

// BinaryVersionInfo 二进制版本条目
type BinaryVersionInfo struct {
	Version    string `json:"version"`
	Size       int64  `json:"size"`
	Refs       int    `json:"refs"`
	UploadedBy string `json:"uploadedBy"`
	UploadedAt string `json:"uploadedAt"`
	IsCurrent  bool   `json:"isCurrent"`
	IsLocal    bool   `json:"isLocal"`
	IsRemote   bool   `json:"isRemote"`
}

// BinaryPageData 二进制页面数据
type BinaryPageData struct {
	CurrentVersion string              `json:"currentVersion"`
	Versions       []BinaryVersionInfo `json:"versions"`
	Platform       string              `json:"platform"`
	BinaryPath     string              `json:"binaryPath"`
	LocalExists    bool                `json:"localExists"`
}

// GetBinaryPage 返回二进制管理页面数据
func (a *App) GetBinaryPage() (*BinaryPageData, error) {
	_, client, _, err := a.loadClients()
	if err != nil {
		return nil, err
	}

	platform := config.Platform()
	binPath := binary.GetBinaryPath("claude")

	data := &BinaryPageData{
		Platform:   platform,
		BinaryPath: binPath,
	}

	// 检查本地文件
	info, statErr := os.Stat(binPath)
	if statErr == nil {
		data.LocalExists = true
		// 尝试读取版本
		ver := detectBinVersion(binPath)
		data.CurrentVersion = ver
		_ = info
	}

	// 从 WebDAV 加载索引
	idx, err := binary.LoadIndex(client)
	if err != nil {
		return data, nil
	}

	binInfo := idx.GetBinaryInfo(platform, "claude")
	if binInfo == nil {
		return data, nil
	}

	data.CurrentVersion = binInfo.Current

	for ver, v := range binInfo.Versions {
		isCurrent := (ver == binInfo.Current)
		data.Versions = append(data.Versions, BinaryVersionInfo{
			Version:    ver,
			Size:       v.Size,
			Refs:       v.Refs,
			UploadedBy: v.UploadedBy,
			UploadedAt: v.Uploaded.Format("2006-01-02 15:04"),
			IsCurrent:  isCurrent,
			IsRemote:   true,
		})
	}

	return data, nil
}
