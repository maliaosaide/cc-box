// 历史、设置、项目页面后端绑定
// 快照列表、配置读写、项目追踪
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/user/cc-box/internal/binary"
	"github.com/user/cc-box/internal/config"
	"github.com/user/cc-box/internal/crypto"
	"github.com/user/cc-box/internal/object"
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
			Timestamp: snap.Timestamp.Local().Format("2006-01-02 15:04"),
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
			Modified: entry.Modified.Local().Format("2006-01-02 15:04"),
		}
	}

	return &SnapshotDetail{
		ID:        snap.ID,
		Timestamp: snap.Timestamp.Local().Format("2006-01-02 15:04:05"),
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

	hasPassword := false
	if _, err := config.LoadWebDAVPassword(); err == nil {
		hasPassword = true
	}

	// 读取原始配置中的路径字段（用于编辑回显）
	v := config.LoadRaw()
	claudePath := v.GetString("claude.path")
	binDir := v.GetString("binary.bindir")
	verDir := v.GetString("binary.versionsdir")

	return &ConfigView{
		WebDAV: WebDAVView{
			URL:         cfg.WebDAV.URL,
			Username:    cfg.WebDAV.Username,
			Root:        cfg.WebDAV.Root,
			HasPassword: hasPassword,
		},
		Device: DeviceView{
			ID:   cfg.Device.ID,
			Name: cfg.Device.Name,
		},
		Encryption: EncryptionView{
			Enabled: cfg.Encryption.Enabled,
		},
		Binary: BinaryView{
			Encrypt:          cfg.Binary.Encrypt,
			ChunkMode:        cfg.Binary.ChunkMode,
			ChunkSizeMB:      cfg.Binary.ChunkSizeMB,
			ChunkThresholdMB: cfg.Binary.ChunkThresholdMB,
			AutoUpload:       cfg.Binary.AutoUpload,
		},
		Sync: SyncView{
			SnapshotLimit:     cfg.Sync.SnapshotLimit,
			ConflictStrategy:  cfg.Sync.ConflictStrategy,
			MergeRetryMax:     cfg.Sync.MergeRetryMax,
			AutoSyncInterval:  cfg.Sync.AutoSyncInterval,
		},
		Exclude:        cfg.Exclude.Patterns,
		ClaudeDir:      config.ClaudeDir(),
		ClaudeDirRaw:   claudePath,
		BinDir:         config.LocalBinDir(),
		BinDirRaw:      binDir,
		VersionsDir:    config.VersionsDir(),
		VersionsDirRaw: verDir,
	}, nil
}

// SetConfigField 修改单个配置项
func (a *App) SetConfigField(section, key, value string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	switch section {
	case "webdav":
		switch key {
		case "url":
			cfg.WebDAV.URL = value
		case "username":
			cfg.WebDAV.Username = value
		case "root":
			cfg.WebDAV.Root = value
		}
	case "device":
		if key == "name" {
			cfg.Device.Name = value
		}
	case "encryption":
		if key == "enabled" {
			cfg.Encryption.Enabled = value == "true"
		}
	case "claude":
		if key == "path" {
			cfg.Claude.Path = value
		}
	case "binary":
		switch key {
		case "encrypt":
			cfg.Binary.Encrypt = value == "true"
		case "chunk_mode":
			cfg.Binary.ChunkMode = value
		case "chunk_size_mb":
			if v, e := parseInt(value); e == nil {
				cfg.Binary.ChunkSizeMB = v
			}
		case "chunk_threshold_mb":
			if v, e := parseInt(value); e == nil {
				cfg.Binary.ChunkThresholdMB = v
			}
		case "auto_upload":
			cfg.Binary.AutoUpload = value == "true"
		case "bin_dir":
			cfg.Binary.BinDir = value
		case "versions_dir":
			cfg.Binary.VersionsDir = value
		}
	case "sync":
		switch key {
		case "conflict_strategy":
			cfg.Sync.ConflictStrategy = value
		case "snapshot_limit":
			if v, e := parseInt(value); e == nil {
				cfg.Sync.SnapshotLimit = v
			}
		case "merge_retry_max":
			if v, e := parseInt(value); e == nil {
				cfg.Sync.MergeRetryMax = v
			}
		case "auto_sync_interval":
			cfg.Sync.AutoSyncInterval = value
		}
	}

	return config.Save(cfg)
}

// SetWebDAVPassword 保存 WebDAV 密码到密钥环
func (a *App) SetWebDAVPassword(password string) error {
	return config.SaveWebDAVPassword(password)
}

// AddExcludePattern 添加排除规则
func (a *App) AddExcludePattern(pattern string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	for _, p := range cfg.Exclude.Patterns {
		if p == pattern {
			return nil
		}
	}
	cfg.Exclude.Patterns = append(cfg.Exclude.Patterns, pattern)
	return config.Save(cfg)
}

// RemoveExcludePattern 删除排除规则
func (a *App) RemoveExcludePattern(pattern string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	filtered := make([]string, 0, len(cfg.Exclude.Patterns))
	for _, p := range cfg.Exclude.Patterns {
		if p != pattern {
			filtered = append(filtered, p)
		}
	}
	cfg.Exclude.Patterns = filtered
	return config.Save(cfg)
}

func parseInt(s string) (int, error) {
	v := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			v = v*10 + int(c-'0')
		} else {
			return 0, fmt.Errorf("invalid int")
		}
	}
	return v, nil
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

// GetProjectDetail 获取项目 .claude.json 内容
func (a *App) GetProjectDetail(projectPath string) (string, error) {
	content, err := project.LoadClaudeJSON(projectPath)
	if err != nil {
		return "", fmt.Errorf("读取 .claude.json 失败: %w", err)
	}
	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// AddProjectPath 添加项目路径
func (a *App) AddProjectPath(dir string) error {
	claudeJSON := filepath.Join(dir, ".claude.json")
	if _, err := os.Stat(claudeJSON); os.IsNotExist(err) {
		return fmt.Errorf("该目录下没有 .claude.json 文件")
	}
	return nil
}

// DeleteOrphan 删除 orphan 记录
func (a *App) DeleteOrphan(remote string) error {
	idx, err := project.LoadOrphanIndex()
	if err != nil {
		return err
	}
	filtered := make([]project.OrphanProject, 0)
	for _, o := range idx.Orphans {
		if o.RemoteURL != remote {
			filtered = append(filtered, o)
		}
	}
	idx.Orphans = filtered
	return project.SaveOrphanIndex(idx)
}
type ProjectListResult struct {
	Projects []ProjectInfo `json:"projects"`
	Orphans  []OrphanInfo  `json:"orphans"`
}

// ConfigView 配置视图
type ConfigView struct {
	WebDAV         WebDAVView     `json:"webdav"`
	Device         DeviceView     `json:"device"`
	Encryption     EncryptionView `json:"encryption"`
	Binary         BinaryView     `json:"binary"`
	Sync           SyncView       `json:"sync"`
	Exclude        []string       `json:"exclude"`
	ClaudeDir      string         `json:"claudeDir"`
	ClaudeDirRaw   string         `json:"claudeDirRaw"`
	BinDir         string         `json:"binDir"`
	BinDirRaw      string         `json:"binDirRaw"`
	VersionsDir    string         `json:"versionsDir"`
	VersionsDirRaw string         `json:"versionsDirRaw"`
}

type WebDAVView struct {
	URL         string `json:"url"`
	Username    string `json:"username"`
	Root        string `json:"root"`
	HasPassword bool   `json:"hasPassword"`
}

type DeviceView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type EncryptionView struct {
	Enabled bool `json:"enabled"`
}

type BinaryView struct {
	Encrypt          bool   `json:"encrypt"`
	ChunkMode        string `json:"chunkMode"`
	ChunkSizeMB      int    `json:"chunkSizeMB"`
	ChunkThresholdMB int    `json:"chunkThresholdMB"`
	AutoUpload       bool   `json:"autoUpload"`
}

type SyncView struct {
	SnapshotLimit     int    `json:"snapshotLimit"`
	ConflictStrategy  string `json:"conflictStrategy"`
	MergeRetryMax     int    `json:"mergeRetryMax"`
	AutoSyncInterval  string `json:"autoSyncInterval"`
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
	LocalVersions  []BinaryVersionInfo `json:"localVersions"`
	Platform       string              `json:"platform"`
	BinaryPath     string              `json:"binaryPath"`
	VersionsDir    string              `json:"versionsDir"`
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
	verDir := config.VersionsDir()

	data := &BinaryPageData{
		Platform:    platform,
		BinaryPath:  binPath,
		VersionsDir: verDir,
	}

	// 检查主二进制文件
	if info, statErr := os.Stat(binPath); statErr == nil {
		data.LocalExists = true
		data.CurrentVersion = detectBinVersion(binPath)
		_ = info
	}

	// 扫描本地版本目录
	data.LocalVersions = scanLocalVersions(verDir, data.CurrentVersion)

	// 从 WebDAV 加载索引
	idx, err := binary.LoadIndex(client)
	if err != nil {
		return data, nil
	}

	binInfo := idx.GetBinaryInfo(platform, "claude")
	if binInfo == nil {
		return data, nil
	}

	if data.CurrentVersion == "" {
		data.CurrentVersion = binInfo.Current
	}

	for ver, v := range binInfo.Versions {
		isCurrent := (ver == binInfo.Current)
		data.Versions = append(data.Versions, BinaryVersionInfo{
			Version:    ver,
			Size:       v.Size,
			Refs:       v.Refs,
			UploadedBy: v.UploadedBy,
			UploadedAt: v.Uploaded.Local().Format("2006-01-02 15:04"),
			IsCurrent:  isCurrent,
			IsRemote:   true,
		})
	}

	return data, nil
}

// scanLocalVersions 扫描本地版本目录
func scanLocalVersions(dir string, currentVersion string) []BinaryVersionInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var versions []BinaryVersionInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 跳过非版本格式文件
		if name == "" || name[0] == '.' {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		versions = append(versions, BinaryVersionInfo{
			Version:   name,
			Size:      info.Size(),
			IsLocal:   true,
			IsCurrent: name == currentVersion,
		})
	}
	return versions
}

// SwitchBinaryVersion 切换本地 Claude 版本
// source: "local" 从本地版本目录切换, "remote" 从云端下载切换
func (a *App) SwitchBinaryVersion(version string, source string) error {
	binPath := binary.GetBinaryPath("claude")
	verDir := config.VersionsDir()

	// 备份当前版本到 versions 目录（仅移走，不删除）
	os.MkdirAll(verDir, 0755)
	currentVer := detectBinVersion(binPath)
	var backupPath string
	if currentVer != "" {
		backupPath = filepath.Join(verDir, currentVer)
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			os.Rename(binPath, backupPath)
		} else {
			// 已有同名备份，直接移除当前（备份已在）
			os.Remove(binPath)
			backupPath = ""
		}
	}

	var switchErr error
	if source == "local" {
		srcPath := filepath.Join(verDir, version)
		srcData, err := os.ReadFile(srcPath)
		if err != nil {
			switchErr = fmt.Errorf("读取本地版本 %s 失败: %w", version, err)
		} else if err := os.WriteFile(binPath, srcData, 0755); err != nil {
			switchErr = fmt.Errorf("写入失败: %w", err)
		}
	} else {
		_, client, key, err := a.loadClients()
		if err != nil {
			switchErr = err
		} else {
			err = binary.Download(client, key, "claude", version, binPath, nil)
			if err != nil {
				switchErr = fmt.Errorf("下载版本 %s 失败: %w", version, err)
			}
		}
	}

	// 切换失败时回滚
	if switchErr != nil && backupPath != "" {
		os.Rename(backupPath, binPath)
	}
	if switchErr != nil {
		return switchErr
	}

	return nil
}

// UploadBinaryVersion 上传指定本地版本到云端（异步，带进度）
func (a *App) UploadBinaryVersion(version string) int64 {
	return a.StartAsync("binary-upload", func(ctx context.Context, opID int64) error {
		_, client, key, err := a.loadClients()
		if err != nil {
			return err
		}

		verDir := config.VersionsDir()
		srcPath := filepath.Join(verDir, version)

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("读取版本文件 %s 失败: %w", version, err)
		}

		a.emitProgress(opID, "binary-upload", 0, int64(len(data)), 0, 1, "正在上传 "+version+"...")
		return binary.Upload(client, key, "claude", data, version, a.progressCallback(opID, "binary-upload"))
	})
}

// BinaryStorageInfo 二进制存储空间统计
type BinaryStorageInfo struct {
	LocalTotal int64 `json:"localTotal"`
	CloudTotal int64 `json:"cloudTotal"`
	LocalCount int   `json:"localCount"`
	CloudCount int   `json:"cloudCount"`
}

// GetBinaryStorage 返回二进制存储统计
func (a *App) GetBinaryStorage() (*BinaryStorageInfo, error) {
	info := &BinaryStorageInfo{}
	verDir := config.VersionsDir()
	entries, err := os.ReadDir(verDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && e.Name()[0] != '.' {
				if fi, err := e.Info(); err == nil {
					info.LocalTotal += fi.Size()
					info.LocalCount++
				}
			}
		}
	}
	binPath := binary.GetBinaryPath("claude")
	if fi, err := os.Stat(binPath); err == nil {
		info.LocalTotal += fi.Size()
		info.LocalCount++
	}
	_, client, _, err := a.loadClients()
	if err == nil {
		idx, err := binary.LoadIndex(client)
		if err == nil {
			platform := config.Platform()
			binInfo := idx.GetBinaryInfo(platform, "claude")
			if binInfo != nil {
				for _, v := range binInfo.Versions {
					info.CloudTotal += v.Size
					info.CloudCount++
				}
			}
		}
	}
	return info, nil
}

// DeleteLocalVersion 删除本地版本文件
func (a *App) DeleteLocalVersion(version string) error {
	verDir := config.VersionsDir()
	return os.Remove(filepath.Join(verDir, version))
}

// RevertToSnapshot 回滚到指定快照版本
func (a *App) RevertToSnapshot(snapID string) error {
	cfg, client, key, err := a.loadClients()
	if err != nil {
		return err
	}

	snap, err := a.loadSnapByID(client, key, snapID)
	if err != nil {
		return fmt.Errorf("加载快照失败: %w", err)
	}

	// 扫描本地文件
	scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
	scanResult, err := scanner.Scan()
	if err != nil {
		return fmt.Errorf("扫描失败: %w", err)
	}

	// 计算需要恢复的文件
	var toRestore []string
	for path, entry := range snap.Files {
		localEntry, exists := scanResult.Files[path]
		if !exists || localEntry.Hash != entry.Hash {
			toRestore = append(toRestore, path)
		}
	}

	if len(toRestore) == 0 {
		return nil
	}

	// 下载并恢复文件
	store := object.NewStore(client, key, config.CCBoxDir()+"/cache/objects")
	for _, path := range toRestore {
		entry, ok := snap.Files[path]
		if !ok {
			continue
		}
		data, err := store.Download(entry.Hash)
		if err != nil {
			continue
		}
		fullPath := filepath.Join(config.ClaudeDir(), filepath.FromSlash(path))
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, data, 0600)
	}

	// 删除快照中不存在的文件
	for path := range scanResult.Files {
		if _, exists := snap.Files[path]; !exists {
			fullPath := filepath.Join(config.ClaudeDir(), filepath.FromSlash(path))
			os.Remove(fullPath)
		}
	}

	// 创建恢复快照
	newSnap := snapshot.CreateSnapshot(snap.ID, cfg.Device.ID, "revert to "+snapID[:12], snap.Files)
	snapData, _ := newSnap.Serialize()
	encrypted, _ := crypto.Encrypt(snapData, key)
	client.EnsureDir("snapshots/")
	client.PUT("snapshots/"+newSnap.ID+".json.enc", encrypted, "")

	// 更新本地 HEAD
	os.WriteFile(config.CCBoxDir()+"/HEAD", []byte(newSnap.ID), 0600)
	os.WriteFile(config.CCBoxDir()+"/snapshots/"+newSnap.ID+".json", snapData, 0600)

	return nil
}
