// 历史、设置、项目页面后端绑定
// 快照列表、配置读写、项目追踪
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/user/cc-box/core/binary"
	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/crypto"
	"github.com/user/cc-box/core/object"
	"github.com/user/cc-box/core/snapshot"
	"github.com/user/cc-box/core/webdav"
	"github.com/user/cc-box/gui/internal/project"
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
	ID        string                       `json:"id"`
	Timestamp string                       `json:"timestamp"`
	Device    string                       `json:"device"`
	Message   string                       `json:"message"`
	Parent    string                       `json:"parent"`
	Files     map[string]FileEntry         `json:"files"`
	Binary    map[string]map[string]string `json:"binary"`
}

type FileEntry struct {
	Hash     string `json:"hash"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

// GetLocalSnapshotList 返回本地快照历史列表
func (a *App) GetLocalSnapshotList(limit int) ([]SnapshotEntry, error) {
	if limit <= 0 {
		limit = 20
	}

	headID, err := readLocalHeadID()
	if err != nil || headID == "" {
		return nil, nil
	}

	return a.buildSnapshotEntries(headID, limit, func(id string) (*snapshot.Snapshot, error) {
		return a.loadLocalSnapByID(id)
	})
}

// GetSnapshotList 返回快照历史列表
func (a *App) GetSnapshotList(limit int) ([]SnapshotEntry, error) {
	if entries, err := a.GetLocalSnapshotList(limit); err == nil && len(entries) > 0 {
		return entries, nil
	}

	_, client, key, err := a.loadClients()
	if err != nil {
		return nil, err
	}
	client.SetTimeout(8 * time.Second)

	headData, _, err := client.GET("HEAD")
	if err != nil {
		return nil, fmt.Errorf("读取远程 HEAD 失败: %w", err)
	}
	currentID := strings.TrimSpace(string(headData))
	if currentID == "" {
		return nil, nil
	}

	return a.buildSnapshotEntries(currentID, limit, func(id string) (*snapshot.Snapshot, error) {
		return a.loadSnapByID(client, key, id)
	})
}

// GetSnapshotDetail 返回快照详情
func (a *App) GetSnapshotDetail(id string) (*SnapshotDetail, error) {
	snap, err := a.loadLocalSnapByID(id)
	if err != nil {
		_, client, key, clientErr := a.loadClients()
		if clientErr != nil {
			return nil, clientErr
		}
		snap, err = a.loadSnapByID(client, key, id)
		if err != nil {
			return nil, err
		}
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
		Binary:    snapshotDisplayBinaryVersions(snap.Binary),
	}, nil
}

func snapshotDisplayBinaryVersions(binaryVersions map[string]map[string]string) map[string]map[string]string {
	platform := config.Platform()
	tools, ok := binaryVersions[platform]
	if !ok {
		return nil
	}
	version := strings.TrimSpace(tools["claude"])
	if version == "" {
		return nil
	}
	return map[string]map[string]string{platform: {"claude": version}}
}

// loadLocalSnapByID 按 ID 加载本地缓存快照
func (a *App) loadLocalSnapByID(id string) (*snapshot.Snapshot, error) {
	id = strings.TrimSpace(id)
	if err := validateSnapshotID(id); err != nil {
		return nil, err
	}
	snapDir := config.CCBoxDir() + "/snapshots/"
	data, err := os.ReadFile(snapDir + id + ".json")
	if err != nil {
		return nil, err
	}
	return snapshot.Deserialize(data)
}

// readLocalHeadID 读取本地 HEAD 指向的快照 ID
func readLocalHeadID() (string, error) {
	data, err := os.ReadFile(config.CCBoxDir() + "/HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// buildSnapshotEntries 沿快照链构建列表
func (a *App) buildSnapshotEntries(headID string, limit int, loader func(id string) (*snapshot.Snapshot, error)) ([]SnapshotEntry, error) {
	var entries []SnapshotEntry
	snapID := headID
	for i := 0; i < limit && snapID != ""; i++ {
		snap, err := loader(snapID)
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

// loadSnapByID 按ID加载快照
func (a *App) loadSnapByID(client *webdav.Client, key []byte, id string) (*snapshot.Snapshot, error) {
	id = strings.TrimSpace(id)
	if err := validateSnapshotID(id); err != nil {
		return nil, err
	}
	if snap, err := a.loadLocalSnapByID(id); err == nil {
		return snap, nil
	}
	// 从远程下载
	snapPath := "snapshots/" + id + ".json.enc"
	encrypted, _, err := client.GET(snapPath)
	if err != nil {
		return nil, err
	}
	decrypted, err := decryptRemoteData(encrypted, key)
	if err != nil {
		return nil, err
	}
	snap, err := snapshot.Deserialize(decrypted)
	if err != nil {
		return nil, err
	}
	// 缓存到本地
	snapData, _ := snap.Serialize()
	os.WriteFile(config.CCBoxDir()+"/snapshots/"+id+".json", snapData, 0600)
	return snap, nil
}

func remoteEncryptionEnabled() bool {
	cfg, err := config.Load()
	if err != nil {
		return true
	}
	return cfg.Encryption.Enabled
}

func encryptRemoteData(data, key []byte) ([]byte, error) {
	if !remoteEncryptionEnabled() {
		return data, nil
	}
	return crypto.Encrypt(data, key)
}

func decryptRemoteData(data, key []byte) ([]byte, error) {
	if !remoteEncryptionEnabled() {
		return data, nil
	}
	return crypto.Decrypt(data, key)
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
	claudeJSONPath := v.GetString("claude.json_path")
	binDir := v.GetString("binary.bin_dir")
	if binDir == "" {
		binDir = v.GetString("binary.bindir")
	}
	verDir := v.GetString("binary.versions_dir")
	if verDir == "" {
		verDir = v.GetString("binary.versionsdir")
	}
	claudeBinaryPath := v.GetString("binary.claude_path")
	resolution := binary.ResolveClaudeBinaryCached()
	webdavBaseURL := ""
	webdavHeadURL := ""
	if strings.TrimSpace(cfg.WebDAV.URL) != "" {
		webdavBaseURL = config.ConfiguredWebDAVURL(cfg)
		webdavHeadURL = webdavBaseURL + "HEAD"
	}

	return &ConfigView{
		WebDAV: WebDAVView{
			URL:         cfg.WebDAV.URL,
			Username:    cfg.WebDAV.Username,
			Root:        cfg.WebDAV.Root,
			BaseURL:     webdavBaseURL,
			HeadURL:     webdavHeadURL,
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
			Encrypt:           cfg.Binary.Encrypt,
			ChunkMode:         cfg.Binary.ChunkMode,
			ChunkSizeMB:       cfg.Binary.ChunkSizeMB,
			ChunkThresholdMB:  cfg.Binary.ChunkThresholdMB,
			AutoUpload:        cfg.Binary.SyncEnabled,
			SyncEnabled:       cfg.Binary.SyncEnabled,
			AutoConfigurePath: cfg.Binary.AutoConfigurePath,
		},
		Sync: SyncView{
			SnapshotLimit:    cfg.Sync.SnapshotLimit,
			ConflictStrategy: cfg.Sync.ConflictStrategy,
			MergeRetryMax:    cfg.Sync.MergeRetryMax,
			AutoSyncInterval: cfg.Sync.AutoSyncInterval,
		},
		Exclude:                     cfg.Exclude.Patterns,
		ClaudeDir:                   config.ClaudeDir(),
		ClaudeDirRaw:                claudePath,
		ClaudeDirDefault:            config.DefaultClaudeDir(),
		ClaudeJSONPath:              config.ClaudeJSONPath(),
		ClaudeJSONPathRaw:           claudeJSONPath,
		ClaudeJSONPathDefault:       config.DefaultClaudeJSONPath(),
		BinDir:                      config.LocalBinDir(),
		BinDirRaw:                   binDir,
		VersionsDir:                 config.VersionsDir(),
		VersionsDirRaw:              verDir,
		ClaudeBinaryPath:            resolution.CurrentPath,
		ClaudeBinaryPathRaw:         claudeBinaryPath,
		ClaudeBinaryManagedPath:     resolution.ManagedPath,
		ClaudeBinaryPlaceholderPath: claudeBinaryPlaceholderPath(resolution),
		ClaudeBinarySource:          resolution.Source,
		ClaudeBinaryVersion:         resolution.Version,
		ClaudeBinaryValid:           resolution.Valid,
		ClaudeBinaryReadOnly:        resolution.ReadOnly,
		ClaudeBinaryShim:            resolution.IsShim,
		ClaudeBinaryError:           resolution.Error,
	}, nil
}

// SetConfigField 修改单个配置项
func (a *App) SetConfigField(section, key, value string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	clearClaudeCache := false

	switch section {
	case "webdav":
		switch key {
		case "url":
			cfg.WebDAV.URL = strings.TrimSpace(value)
		case "username":
			cfg.WebDAV.Username = value
		case "root":
			cfg.WebDAV.Root = config.NormalizeWebDAVRoot(value)
		}
	case "device":
		if key == "name" {
			cfg.Device.Name = value
		}
	case "encryption":
		if key == "enabled" {
			newValue := value == "true"
			if cfg.Encryption.Enabled != newValue {
				return fmt.Errorf("加密和明文同步互斥，初始化后不能直接切换，请新建同步组或执行完整迁移")
			}
		}
	case "claude":
		switch key {
		case "path":
			cfg.Claude.Path = value
		case "json_path":
			cfg.Claude.JSONPath = value
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
		case "auto_upload", "sync_enabled":
			cfg.Binary.SyncEnabled = value == "true"
			cfg.Binary.AutoUpload = cfg.Binary.SyncEnabled
		case "auto_configure_path":
			cfg.Binary.AutoConfigurePath = value == "true"
		case "bin_dir":
			cfg.Binary.BinDir = value
			clearClaudeCache = true
		case "versions_dir":
			cfg.Binary.VersionsDir = value
		case "claude_path":
			cfg.Binary.ClaudePath = value
			clearClaudeCache = true
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

	if err := config.Save(cfg); err != nil {
		return err
	}
	if clearClaudeCache {
		_ = binary.ClearClaudeResolutionCache()
	}
	a.emitDataChanged("config", "set-config-field")
	return nil
}

// SetWebDAVPassword 保存 WebDAV 密码到密钥环
func (a *App) SetWebDAVPassword(password string) error {
	return config.SaveWebDAVPassword(password)
}

// GetClaudeBinaryResolution 返回当前 Claude 二进制解析结果
func (a *App) GetClaudeBinaryResolution() *ClaudeBinaryResolution {
	return toClaudeBinaryResolution(binary.ResolveClaudeBinary())
}

// RedetectClaudeBinary 清除缓存并重新检测 Claude 二进制
func (a *App) RedetectClaudeBinary() *ClaudeBinaryResolution {
	result := toClaudeBinaryResolution(binary.RedetectClaudeBinary())
	a.emitDataChanged("binary", "redetect-claude")
	return result
}

func toClaudeBinaryResolution(res binary.ClaudeResolution) *ClaudeBinaryResolution {
	return &ClaudeBinaryResolution{
		CurrentPath: res.CurrentPath,
		ManagedPath: res.ManagedPath,
		Source:      res.Source,
		Version:     res.Version,
		Valid:       res.Valid,
		ReadOnly:    res.ReadOnly,
		IsShim:      res.IsShim,
		Error:       res.Error,
	}
}

func claudeBinaryPlaceholderPath(res binary.ClaudeResolution) string {
	if res.CurrentPath != "" {
		return res.CurrentPath
	}
	return res.ManagedPath
}

// GetClaudeDirectories 返回 Claude 配置目录下的一级目录
func (a *App) GetClaudeDirectories() ([]ClaudeDirectoryInfo, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	excluded := make(map[string]bool, len(cfg.Exclude.Patterns))
	for _, pattern := range cfg.Exclude.Patterns {
		excluded[pattern] = true
	}

	claudeDir := config.ClaudeDir()
	entries, err := os.ReadDir(claudeDir)
	if os.IsNotExist(err) {
		return []ClaudeDirectoryInfo{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取 Claude 配置目录失败: %w", err)
	}

	dirs := make([]ClaudeDirectoryInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		pattern := name + "/"
		dirs = append(dirs, ClaudeDirectoryInfo{
			Name:     name,
			Path:     filepath.Join(claudeDir, name),
			Pattern:  pattern,
			Excluded: excluded[pattern],
		})
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	return dirs, nil
}

// GetClaudeExcludeFiles 返回可单独排除的 Claude 配置文件
func (a *App) GetClaudeExcludeFiles() ([]ClaudeFileInfo, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	excluded := make(map[string]bool, len(cfg.Exclude.Patterns))
	for _, pattern := range cfg.Exclude.Patterns {
		excluded[pattern] = true
	}

	const settingsPattern = "settings.json"
	return []ClaudeFileInfo{{
		Name:     settingsPattern,
		Path:     filepath.Join(config.ClaudeDir(), settingsPattern),
		Pattern:  settingsPattern,
		Excluded: excluded[settingsPattern],
	}}, nil
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

const guiDisplayCacheTTL = 30 * time.Second

func (a *App) loadBinaryIndexCached(client *webdav.Client) (*binary.Index, error) {
	a.cacheMu.Lock()
	if a.binaryIndexCache != nil && time.Since(a.binaryIndexCached) < guiDisplayCacheTTL {
		idx := a.binaryIndexCache
		a.cacheMu.Unlock()
		return idx, nil
	}
	a.cacheMu.Unlock()

	idx, err := binary.LoadIndex(client)
	if err != nil {
		return nil, err
	}
	a.cacheMu.Lock()
	a.binaryIndexCache = idx
	a.binaryIndexCached = time.Now()
	a.cacheMu.Unlock()
	return idx, nil
}

func (a *App) clearBinaryIndexCache() {
	a.cacheMu.Lock()
	a.binaryIndexCache = nil
	a.binaryIndexCached = time.Time{}
	a.cacheMu.Unlock()
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

	client := newConfiguredWebDAVClient(cfg, pass)
	client.SetTimeout(8 * time.Second)
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
	if result, ok := a.cachedProjectList(); ok {
		go func() {
			refreshed, err := a.RefreshProjectList()
			if err == nil {
				a.eventsEmit("projects:updated", refreshed)
			}
		}()
		return result, nil
	}
	return a.RefreshProjectList()
}

func (a *App) RefreshProjectList() (*ProjectListResult, error) {
	result, err := a.discoverProjectList()
	if err != nil {
		return nil, err
	}
	a.storeProjectListCache(result)
	return result, nil
}

func (a *App) discoverProjectList() (*ProjectListResult, error) {
	result := &ProjectListResult{Projects: []ProjectInfo{}, Orphans: []OrphanInfo{}}

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
	if err := project.AddTrackedProject(dir); err != nil {
		return err
	}
	a.clearProjectListCache()
	a.emitDataChanged("projects", "add-project")
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
	if err := project.SaveOrphanIndex(idx); err != nil {
		return err
	}
	a.clearProjectListCache()
	a.emitDataChanged("projects", "delete-orphan")
	return nil
}

type ProjectListResult struct {
	Projects []ProjectInfo `json:"projects"`
	Orphans  []OrphanInfo  `json:"orphans"`
}

type projectListCacheFile struct {
	UpdatedAt time.Time          `json:"updatedAt"`
	Result    *ProjectListResult `json:"result"`
}

const projectListCacheTTL = 24 * time.Hour

func (a *App) cachedProjectList() (*ProjectListResult, bool) {
	a.cacheMu.Lock()
	if a.projectListCache != nil && time.Since(a.projectListCached) < projectListCacheTTL {
		result := a.projectListCache
		a.cacheMu.Unlock()
		return result, true
	}
	a.cacheMu.Unlock()

	data, err := os.ReadFile(projectListCachePath())
	if err != nil {
		return nil, false
	}
	var cached projectListCacheFile
	if err := json.Unmarshal(data, &cached); err != nil || cached.Result == nil || time.Since(cached.UpdatedAt) >= projectListCacheTTL {
		return nil, false
	}
	a.cacheMu.Lock()
	a.projectListCache = cached.Result
	a.projectListCached = cached.UpdatedAt
	a.cacheMu.Unlock()
	return cached.Result, true
}

func (a *App) storeProjectListCache(result *ProjectListResult) {
	now := time.Now()
	a.cacheMu.Lock()
	a.projectListCache = result
	a.projectListCached = now
	a.cacheMu.Unlock()
	path := projectListCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return
	}
	data, err := json.MarshalIndent(projectListCacheFile{UpdatedAt: now, Result: result}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0600)
}

func (a *App) clearProjectListCache() {
	a.cacheMu.Lock()
	a.projectListCache = nil
	a.projectListCached = time.Time{}
	a.cacheMu.Unlock()
	_ = os.Remove(projectListCachePath())
}

func projectListCachePath() string {
	return filepath.Join(config.CCBoxDir(), "cache", "project-list.json")
}

// ConfigView 配置视图
type ConfigView struct {
	WebDAV                      WebDAVView     `json:"webdav"`
	Device                      DeviceView     `json:"device"`
	Encryption                  EncryptionView `json:"encryption"`
	Binary                      BinaryView     `json:"binary"`
	Sync                        SyncView       `json:"sync"`
	Exclude                     []string       `json:"exclude"`
	ClaudeDir                   string         `json:"claudeDir"`
	ClaudeDirRaw                string         `json:"claudeDirRaw"`
	ClaudeDirDefault            string         `json:"claudeDirDefault"`
	ClaudeJSONPath              string         `json:"claudeJSONPath"`
	ClaudeJSONPathRaw           string         `json:"claudeJSONPathRaw"`
	ClaudeJSONPathDefault       string         `json:"claudeJSONPathDefault"`
	BinDir                      string         `json:"binDir"`
	BinDirRaw                   string         `json:"binDirRaw"`
	VersionsDir                 string         `json:"versionsDir"`
	VersionsDirRaw              string         `json:"versionsDirRaw"`
	ClaudeBinaryPath            string         `json:"claudeBinaryPath"`
	ClaudeBinaryPathRaw         string         `json:"claudeBinaryPathRaw"`
	ClaudeBinaryManagedPath     string         `json:"claudeBinaryManagedPath"`
	ClaudeBinaryPlaceholderPath string         `json:"claudeBinaryPlaceholderPath"`
	ClaudeBinarySource          string         `json:"claudeBinarySource"`
	ClaudeBinaryVersion         string         `json:"claudeBinaryVersion"`
	ClaudeBinaryValid           bool           `json:"claudeBinaryValid"`
	ClaudeBinaryReadOnly        bool           `json:"claudeBinaryReadOnly"`
	ClaudeBinaryShim            bool           `json:"claudeBinaryShim"`
	ClaudeBinaryError           string         `json:"claudeBinaryError"`
}

type ClaudeBinaryResolution struct {
	CurrentPath string `json:"currentPath"`
	ManagedPath string `json:"managedPath"`
	Source      string `json:"source"`
	Version     string `json:"version"`
	Valid       bool   `json:"valid"`
	ReadOnly    bool   `json:"readOnly"`
	IsShim      bool   `json:"isShim"`
	Error       string `json:"error,omitempty"`
}

type ClaudeDirectoryInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Pattern  string `json:"pattern"`
	Excluded bool   `json:"excluded"`
}

type ClaudeFileInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Pattern  string `json:"pattern"`
	Excluded bool   `json:"excluded"`
}

type WebDAVView struct {
	URL         string `json:"url"`
	Username    string `json:"username"`
	Root        string `json:"root"`
	BaseURL     string `json:"baseUrl"`
	HeadURL     string `json:"headUrl"`
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
	Encrypt           bool   `json:"encrypt"`
	ChunkMode         string `json:"chunkMode"`
	ChunkSizeMB       int    `json:"chunkSizeMB"`
	ChunkThresholdMB  int    `json:"chunkThresholdMB"`
	AutoUpload        bool   `json:"autoUpload"`
	SyncEnabled       bool   `json:"syncEnabled"`
	AutoConfigurePath bool   `json:"autoConfigurePath"`
}

type SyncView struct {
	SnapshotLimit    int    `json:"snapshotLimit"`
	ConflictStrategy string `json:"conflictStrategy"`
	MergeRetryMax    int    `json:"mergeRetryMax"`
	AutoSyncInterval string `json:"autoSyncInterval"`
}

type ConnectionTest struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Latency int64  `json:"latency"`
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
	CurrentVersion string                     `json:"currentVersion"`
	AllVersions    []BinaryVersionInfo        `json:"allVersions"`
	Versions       []BinaryVersionInfo        `json:"versions"`
	LocalVersions  []BinaryVersionInfo        `json:"localVersions"`
	Platform       string                     `json:"platform"`
	BinaryPath     string                     `json:"binaryPath"`
	ManagedPath    string                     `json:"managedPath"`
	BinarySource   string                     `json:"binarySource"`
	BinaryReadOnly bool                       `json:"binaryReadOnly"`
	BinaryShim     bool                       `json:"binaryShim"`
	BinaryError    string                     `json:"binaryError"`
	VersionsDir    string                     `json:"versionsDir"`
	LocalExists    bool                       `json:"localExists"`
	CommandStatus  binary.ClaudeCommandStatus `json:"commandStatus"`
}

// GetBinaryPage 返回二进制管理页面数据
func (a *App) GetBinaryPage() (*BinaryPageData, error) {
	platform := config.Platform()
	verDir := config.VersionsDir()
	resolution := binary.ResolveClaudeBinaryCached()
	if resolution.Valid && resolution.Version == "" {
		resolution = binary.ResolveClaudeBinary()
	}

	data := &BinaryPageData{
		Platform:       platform,
		BinaryPath:     resolution.CurrentPath,
		ManagedPath:    resolution.ManagedPath,
		BinarySource:   resolution.Source,
		BinaryReadOnly: resolution.ReadOnly,
		BinaryShim:     resolution.IsShim,
		BinaryError:    resolution.Error,
		VersionsDir:    verDir,
		LocalExists:    resolution.Valid,
		CurrentVersion: resolution.Version,
		CommandStatus:  binary.ClaudeCommandState(resolution.ManagedPath),
	}

	data.LocalVersions = scanLocalVersions(verDir, data.CurrentVersion)
	data.AllVersions = mergeBinaryVersions(data.LocalVersions, nil, data.CurrentVersion)

	_, client, _, err := a.loadClients()
	if err != nil {
		return data, nil
	}
	client.SetTimeout(8 * time.Second)

	idx, err := a.loadBinaryIndexCached(client)
	if err != nil {
		return data, nil
	}

	binInfo := idx.GetBinaryInfo(platform, "claude")
	if binInfo == nil {
		return data, nil
	}

	for ver, v := range binInfo.Versions {
		isCurrent := data.LocalExists && ver == data.CurrentVersion
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

	data.AllVersions = mergeBinaryVersions(data.LocalVersions, data.Versions, data.CurrentVersion)

	return data, nil
}

func (a *App) GetGitHubBinaryReleases(limit int) (*binary.GitHubClaudeReleaseList, error) {
	return binary.CachedGitHubClaudeReleases(limit)
}

func (a *App) RefreshGitHubBinaryReleases(limit int) int64 {
	return a.StartAsync("binary-github-refresh", func(ctx context.Context, opID int64) error {
		a.emitProgress(opID, "binary-github-refresh", 0, 1, 0, 1, "正在刷新 GitHub Release")
		if _, err := binary.RefreshGitHubClaudeReleases(ctx, limit); err != nil {
			return err
		}
		a.emitProgress(opID, "binary-github-refresh", 1, 1, 1, 1, "GitHub Release 已刷新")
		a.emitDataChanged("binary", "github-releases")
		return nil
	})
}

func (a *App) InstallOfficialClaude() int64 {
	return a.StartAsync("binary-official-install", func(ctx context.Context, opID int64) error {
		_, err := binary.InstallOfficialClaude(ctx, func(current, total int64, message string) {
			a.emitProgress(opID, "binary-official-install", current, total, int(current), int(total), message)
		})
		if err != nil {
			return err
		}
		a.clearBinaryIndexCache()
		a.emitDataChanged("binary", "official-install")
		return nil
	})
}

func (a *App) InstallGitHubClaude(version string) int64 {
	return a.StartAsync("binary-github-install", func(ctx context.Context, opID int64) error {
		result, err := binary.InstallGitHubClaude(ctx, version, func(current, total int64, message string) {
			a.emitProgress(opID, "binary-github-install", current, total, int(current), int(total), message)
		})
		if err != nil {
			return err
		}
		if result != nil && result.PathConfig != nil && result.PathConfig.Error != "" {
			a.emitProgress(opID, "binary-github-install", 5, 5, 5, 5, result.PathConfig.Message)
		}
		a.clearBinaryIndexCache()
		a.emitDataChanged("binary", "github-install")
		return nil
	})
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

// mergeBinaryVersions 按版本号合并本地和云端列表
func mergeBinaryVersions(local, remote []BinaryVersionInfo, currentVer string) []BinaryVersionInfo {
	merged := make(map[string]*BinaryVersionInfo)
	for _, v := range local {
		cp := v
		merged[cp.Version] = &cp
	}
	for _, v := range remote {
		if existing, ok := merged[v.Version]; ok {
			existing.IsRemote = true
			existing.UploadedBy = v.UploadedBy
			existing.UploadedAt = v.UploadedAt
			existing.Refs = v.Refs
			if existing.Size == 0 {
				existing.Size = v.Size
			}
		} else {
			cp := v
			merged[cp.Version] = &cp
		}
	}
	result := make([]BinaryVersionInfo, 0, len(merged))
	for _, v := range merged {
		v.IsCurrent = (v.Version == currentVer)
		result = append(result, *v)
	}
	return result
}

// SwitchBinaryVersion 切换本地 Claude 版本
// source: "local" 从本地版本目录切换, "remote" 从云端下载切换
func (a *App) SwitchBinaryVersion(version string, source string) error {
	binPath := binary.GetBinaryPath("claude")
	verDir := config.VersionsDir()

	currentVer := detectBinVersion(binPath)
	if currentVer != "" {
		backupPath := filepath.Join(verDir, currentVer)
		if err := binary.BackupFileIfMissing(binPath, backupPath); err != nil {
			return fmt.Errorf("备份当前版本失败: %w", err)
		}
	}

	var switchErr error
	if source == "local" {
		srcPath := filepath.Join(verDir, version)
		srcData, err := os.ReadFile(srcPath)
		if err != nil {
			switchErr = fmt.Errorf("读取本地版本 %s 失败: %w", version, err)
		} else if _, err := binary.InstallClaudeBinaryData(binPath, srcData, version); err != nil {
			switchErr = fmt.Errorf("切换本地版本 %s 失败: %w", version, err)
		}
	} else {
		_, client, key, err := a.loadClients()
		if err != nil {
			switchErr = err
		} else {
			client.SetLongTimeout()
				data, err := binary.DownloadData(client, key, "claude", version, nil)
			if err != nil {
				switchErr = fmt.Errorf("下载版本 %s 失败: %w", version, err)
			} else if _, err := binary.InstallClaudeBinaryData(binPath, data, version); err != nil {
				switchErr = fmt.Errorf("切换云端版本 %s 失败: %w", version, err)
			}
		}
	}

	if switchErr != nil {
		return switchErr
	}

	if source == "remote" {
		_, client, _, err := a.loadClients()
		if err != nil {
			return err
		}
		platform := config.Platform()
		if err := binary.UpdateIndex(client, func(idx *binary.Index) error {
			info := idx.GetBinaryInfo(platform, "claude")
			if info == nil {
				return fmt.Errorf("没有可用的 Claude 二进制")
			}
			if _, exists := info.Versions[version]; !exists {
				return fmt.Errorf("版本 %s 不存在云端", version)
			}
			info.Current = version
			return nil
		}); err != nil {
			return fmt.Errorf("更新远程二进制索引失败: %w", err)
		}
	}

	_ = binary.ClearClaudeResolutionCache()
	installedVersion := detectBinVersion(binPath)
	if installedVersion == "" {
		installedVersion = version
	}
	installSource := "local"
	if source == "remote" {
		installSource = "webdav"
	}
	_ = binary.RememberClaudeBinarySource(binPath, installSource, installedVersion)
	_ = binary.ConfigureClaudePathIfEnabledBestEffort()
	a.clearBinaryIndexCache()
	a.emitDataChanged("binary", "switch-binary")
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
		if err := binary.Upload(client, key, "claude", data, version, a.progressCallback(opID, "binary-upload")); err != nil {
			return err
		}
		a.clearBinaryIndexCache()
		return nil
	})
}

// UploadCurrentBinary 上传当前正在使用的 Claude 二进制到云端
func (a *App) UploadCurrentBinary() int64 {
	return a.StartAsync("binary-upload", func(ctx context.Context, opID int64) error {
		_, client, key, err := a.loadClients()
		if err != nil {
			return err
		}

		resolution := binary.ResolveClaudeBinary()
		if !resolution.Valid {
			return fmt.Errorf("%s", resolution.Error)
		}
		if resolution.IsShim {
			return fmt.Errorf("当前 Claude 路径是脚本 shim，不支持上传；请手动选择真实二进制或使用受管目录")
		}
		data, err := os.ReadFile(resolution.CurrentPath)
		if err != nil {
			return fmt.Errorf("读取当前二进制失败: %w", err)
		}

		version := resolution.Version
		if version == "" {
			return fmt.Errorf("无法识别当前 Claude 版本")
		}

		a.emitProgress(opID, "binary-upload", 0, int64(len(data)), 0, 1, "正在上传当前版本 "+version+"...")
		if err := binary.Upload(client, key, "claude", data, version, a.progressCallback(opID, "binary-upload")); err != nil {
			return err
		}
		a.clearBinaryIndexCache()
		return nil
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
		client.SetTimeout(8 * time.Second)
		idx, err := a.loadBinaryIndexCached(client)
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

// DeleteBinaryVersion 删除指定 Claude 版本的本地缓存和云端备份
func (a *App) DeleteBinaryVersion(version string) error {
	version, err := validateBinaryVersionName(version)
	if err != nil {
		return err
	}
	if resolution := binary.ResolveClaudeBinaryCached(); resolution.Valid && resolution.Version == version {
		return fmt.Errorf("当前正在使用 Claude %s，请先切换到其他版本再删除", version)
	}

	_, client, key, err := a.loadClients()
	if err != nil {
		return fmt.Errorf("无法连接云端确认并删除版本 %s: %w", version, err)
	}
	idx, err := binary.LoadIndex(client)
	if err != nil {
		return fmt.Errorf("读取云端二进制索引失败: %w", err)
	}
	if info := idx.GetBinaryInfo(config.Platform(), "claude"); info != nil {
		if _, ok := info.Versions[version]; ok {
			if err := binary.DeleteRemoteVersion(client, key, "claude", version, config.Platform()); err != nil {
				return fmt.Errorf("删除云端版本失败: %w", err)
			}
		}
	}

	if err := os.Remove(filepath.Join(config.VersionsDir(), version)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除本地版本失败: %w", err)
	}
	a.clearBinaryIndexCache()
	a.emitDataChanged("binary", "delete-binary-version")
	return nil
}

func validateBinaryVersionName(version string) (string, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "", fmt.Errorf("版本号不能为空")
	}
	if filepath.Base(version) != version || strings.ContainsAny(version, `/\\`) {
		return "", fmt.Errorf("版本号无效")
	}
	return version, nil
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

	scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
	scanResult, err := scanner.ScanPartial()
	if err != nil {
		return fmt.Errorf("扫描失败: %w", err)
	}
	if err := requireCompleteScan(scanResult); err != nil {
		return err
	}

	parentHead, _ := readLocalHeadID()
	_, headETag, err := remoteHeadETagForUpdate(client, parentHead)
	if err != nil {
		return err
	}

	var toRestore []string
	for path, entry := range snap.Files {
		localEntry, exists := scanResult.Files[path]
		if !exists || localEntry.Hash != entry.Hash {
			toRestore = append(toRestore, path)
		}
	}

	var toDelete []string
	for path := range scanResult.Files {
		if _, exists := snap.Files[path]; !exists {
			toDelete = append(toDelete, path)
		}
	}

	store := object.NewStore(client, key, config.CCBoxDir()+"/cache/objects")
	restoreData := make(map[string][]byte, len(toRestore))
	for _, path := range toRestore {
		entry := snap.Files[path]
		data, err := store.Download(entry.Hash)
		if err != nil {
			return fmt.Errorf("下载文件 %s 失败: %w", path, err)
		}
		restoreData[path] = data
	}

	type pathBackup struct {
		path   string
		data   []byte
		mode   os.FileMode
		exists bool
	}
	var backups []pathBackup
	seenBackups := make(map[string]bool)
	backupPath := func(path string) error {
		if seenBackups[path] {
			return nil
		}
		seenBackups[path] = true
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				backups = append(backups, pathBackup{path: path})
				return nil
			}
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		backups = append(backups, pathBackup{path: path, data: data, mode: info.Mode(), exists: true})
		return nil
	}
	restoreBackups := func() {
		for i := len(backups) - 1; i >= 0; i-- {
			backup := backups[i]
			if backup.exists {
				_ = os.MkdirAll(filepath.Dir(backup.path), 0755)
				_ = os.WriteFile(backup.path, backup.data, backup.mode)
				continue
			}
			_ = os.Remove(backup.path)
		}
	}
	fail := func(err error) error {
		restoreBackups()
		return err
	}

	for _, path := range toRestore {
		fullPath, err := safeClaudePath(path)
		if err != nil {
			return err
		}
		if err := backupPath(fullPath); err != nil {
			return fmt.Errorf("备份文件 %s 失败: %w", path, err)
		}
	}
	for _, path := range toDelete {
		fullPath, err := safeClaudePath(path)
		if err != nil {
			return err
		}
		if err := backupPath(fullPath); err != nil {
			return fmt.Errorf("备份文件 %s 失败: %w", path, err)
		}
	}
	claudePlan, err := binary.PlanClaudeRestore(client, key, snap, binary.ClaudeRestoreExact)
	if err != nil {
		return err
	}
	if claudePlan.Action == binary.ClaudeActionUnavailable {
		return fmt.Errorf("快照需要 Claude %s，但云端没有当前平台可用版本", claudePlan.TargetVersion)
	}
	if claudePlan.Action == binary.ClaudeActionDownload {
		if err := backupPath(claudePlan.TargetPath); err != nil {
			return fmt.Errorf("备份 Claude binary 失败: %w", err)
		}
	}

	for _, path := range toRestore {
		fullPath, err := safeClaudePath(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fail(fmt.Errorf("创建目录失败: %w", err))
		}
		if err := os.WriteFile(fullPath, restoreData[path], 0600); err != nil {
			return fail(fmt.Errorf("恢复文件 %s 失败: %w", path, err))
		}
	}
	for _, path := range toDelete {
		fullPath, err := safeClaudePath(path)
		if err != nil {
			return err
		}
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			return fail(fmt.Errorf("删除文件 %s 失败: %w", path, err))
		}
	}

	if err := binary.ApplyClaudeRestore(client, key, claudePlan, nil); err != nil {
		return fail(err)
	}

	newSnap := snapshot.CreateSnapshot(parentHead, cfg.Device.ID, "revert to "+shortSnapshotID(snapID), snap.Files)
	newSnap.Binary = binary.CloneSnapshotBinary(snap)
	snapData, err := newSnap.Serialize()
	if err != nil {
		return fail(fmt.Errorf("序列化恢复快照失败: %w", err))
	}
	encrypted, err := encryptRemoteData(snapData, key)
	if err != nil {
		return fail(fmt.Errorf("加密恢复快照失败: %w", err))
	}
	if err := client.EnsureDir("snapshots/"); err != nil {
		return fail(fmt.Errorf("创建快照目录失败: %w", err))
	}
	if _, err := client.PUT("snapshots/"+newSnap.ID+".json.enc", encrypted, ""); err != nil {
		return fail(fmt.Errorf("上传恢复快照失败: %w", err))
	}
	res, err := client.CompareAndSwapHEAD("HEAD", newSnap.ID, headETag)
	if err != nil {
		return fail(fmt.Errorf("更新远程 HEAD 失败: %w", err))
	}
	if !res.Success {
		return fail(fmt.Errorf("远程 HEAD 已变化，请先拉取后再回滚"))
	}

	if err := os.WriteFile(config.CCBoxDir()+"/HEAD", []byte(newSnap.ID), 0600); err != nil {
		return fmt.Errorf("更新本地 HEAD 失败: %w", err)
	}
	if err := os.WriteFile(config.CCBoxDir()+"/snapshots/"+newSnap.ID+".json", snapData, 0600); err != nil {
		return fmt.Errorf("缓存恢复快照失败: %w", err)
	}

	a.emitDataChanged("sync", "revert-snapshot")
	return nil
}

func shortSnapshotID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

// EncryptionStatus 加密状态信息
type EncryptionStatus struct {
	Enabled     bool   `json:"enabled"`
	Fingerprint string `json:"fingerprint"`
	HasKey      bool   `json:"hasKey"`
}

type EncryptionVerifyResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type EncryptionPasswordPreview struct {
	Status         string `json:"status"`
	Message        string `json:"message"`
	Fingerprint    string `json:"fingerprint"`
	MatchesCurrent bool   `json:"matchesCurrent"`
}

// GetEncryptionStatus 返回加密状态和密钥指纹
func (a *App) GetEncryptionStatus() (*EncryptionStatus, error) {
	status := &EncryptionStatus{}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	status.Enabled = cfg.Encryption.Enabled
	keyPath := config.CCBoxDir() + "/key.bin"
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		status.HasKey = false
		return status, nil
	}
	status.HasKey = true
	status.Fingerprint = crypto.KeyFingerprint(keyData)
	return status, nil
}

// VerifyEncryptionKey 验证本机加密密码能否解密远程数据
func (a *App) VerifyEncryptionKey() (*EncryptionVerifyResult, error) {
	_, client, key, err := a.loadClients()
	if err != nil {
		return nil, err
	}
	return verifyRemoteSnapshotWithKey(client, key)
}

func (a *App) PreviewEncryptionPassword(password string) (*EncryptionPasswordPreview, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	webdavPassword, err := config.LoadWebDAVPassword()
	if err != nil {
		return nil, err
	}
	client := newConfiguredWebDAVClient(cfg, webdavPassword)
	preview, _, _, err := buildEncryptionPasswordPreview(client, password)
	return preview, err
}

func (a *App) PreviewSetupEncryptionPassword(url, username, password, root, encryptionPassword string) (*EncryptionPasswordPreview, error) {
	client := webdav.NewClient(buildWebDAVURL(url, root), username, password)
	preview, _, _, err := buildEncryptionPasswordPreview(client, encryptionPassword)
	return preview, err
}

func (a *App) SaveEncryptionPassword(password string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	webdavPassword, err := config.LoadWebDAVPassword()
	if err != nil {
		return err
	}
	client := newConfiguredWebDAVClient(cfg, webdavPassword)
	preview, salt, key, err := buildEncryptionPasswordPreview(client, password)
	if err != nil {
		return err
	}
	if preview.Status != "success" {
		return fmt.Errorf("%s", preview.Message)
	}
	if err := os.WriteFile(filepath.Join(config.CCBoxDir(), "salt.bin"), salt, 0600); err != nil {
		return fmt.Errorf("保存 salt 失败: %w", err)
	}
	if err := crypto.SaveKey(key, config.KeyPath()); err != nil {
		return fmt.Errorf("保存加密密码失败: %w", err)
	}
	return nil
}

func buildEncryptionPasswordPreview(client *webdav.Client, password string) (*EncryptionPasswordPreview, []byte, []byte, error) {
	if password == "" {
		return &EncryptionPasswordPreview{Status: "empty", Message: "请输入加密密码"}, nil, nil, nil
	}

	remoteSalt, _, err := client.GET("salt.bin")
	if err == webdav.ErrNotFound {
		return nil, nil, nil, fmt.Errorf("远程缺少 salt.bin，请检查 WebDAV 根路径")
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("读取远程 salt 失败: %w", err)
	}

	key := crypto.DeriveKey(password, remoteSalt)
	preview := &EncryptionPasswordPreview{Fingerprint: crypto.KeyFingerprint(key)}
	if currentKey, err := os.ReadFile(config.KeyPath()); err == nil {
		preview.MatchesCurrent = crypto.ConstantTimeEqual(key, currentKey)
	}

	verify, err := verifyRemoteSnapshotWithKey(client, key)
	if err != nil {
		return nil, nil, nil, err
	}
	preview.Status = verify.Status
	preview.Message = verify.Message
	return preview, remoteSalt, key, nil
}

func verifyRemoteSnapshotWithKey(client *webdav.Client, key []byte) (*EncryptionVerifyResult, error) {
	headData, _, err := client.GET("HEAD")
	if err == webdav.ErrNotFound {
		return encryptionVerifyUnverified("远程尚未初始化或当前 WebDAV 根路径下没有 HEAD，无法验证加密密码；请先成功同步一次，或检查 WebDAV 根路径。"), nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取远程 HEAD 失败: %w", err)
	}
	snapID := strings.TrimSpace(string(headData))
	if snapID == "" {
		return encryptionVerifyUnverified("远程 HEAD 为空，无法验证加密密码；请先成功同步一次，或检查 WebDAV 根路径。"), nil
	}
	if err := validateSnapshotID(snapID); err != nil {
		return nil, fmt.Errorf("远程 HEAD 内容无效: %w", err)
	}
	encData, _, err := client.GET("snapshots/" + snapID + ".json.enc")
	if err == webdav.ErrNotFound {
		return encryptionVerifyUnverified("远程 HEAD 指向的快照不存在，无法验证加密密码；请先同步修复远程数据。"), nil
	}
	if err != nil {
		return nil, fmt.Errorf("下载远程快照失败: %w", err)
	}
	plainData, err := decryptRemoteData(encData, key)
	if err != nil {
		return &EncryptionVerifyResult{Status: "mismatch", Message: "输入的加密密码无法解密远程数据"}, nil
	}
	if _, err := snapshot.Deserialize(plainData); err != nil {
		return &EncryptionVerifyResult{Status: "mismatch", Message: "解密结果不是有效快照，可能加密密码不匹配或远程快照已损坏"}, nil
	}
	return &EncryptionVerifyResult{Status: "success", Message: "加密密码有效，可以解密远程数据"}, nil
}

func encryptionVerifyUnverified(message string) *EncryptionVerifyResult {
	return &EncryptionVerifyResult{Status: "unverified", Message: message}
}

// rotateRemoteData 用旧密钥解密远程加密数据并用新密钥重新加密
func (a *App) rotateRemoteData(client *webdav.Client, oldKey, newKey []byte) error {
	type rotationFile struct {
		path    string
		oldData []byte
		oldETag string
		newData []byte
		newETag string
	}

	roots := []string{"snapshots/", "objects/", "projects/", "binaries/"}
	var rotations []rotationFile
	for _, root := range roots {
		files, err := listRemoteFilesRecursive(client, root)
		if err != nil {
			return err
		}
		for _, remotePath := range files {
			if !strings.HasSuffix(remotePath, ".enc") {
				continue
			}
			encData, etag, err := client.GET(remotePath)
			if err != nil {
				return fmt.Errorf("读取 %s 失败: %w", remotePath, err)
			}
			plainData, err := crypto.Decrypt(encData, oldKey)
			if err != nil {
				return fmt.Errorf("解密 %s 失败: %w", remotePath, err)
			}
			newEncData, err := crypto.Encrypt(plainData, newKey)
			if err != nil {
				return fmt.Errorf("加密 %s 失败: %w", remotePath, err)
			}
			rotations = append(rotations, rotationFile{path: remotePath, oldData: encData, oldETag: etag, newData: newEncData})
		}
	}

	rollback := func(applied []rotationFile) error {
		var rollbackErr error
		for i := len(applied) - 1; i >= 0; i-- {
			file := applied[i]
			if _, err := client.PUT(file.path, file.oldData, file.newETag); err != nil && rollbackErr == nil {
				rollbackErr = fmt.Errorf("回滚 %s 失败: %w", file.path, err)
			}
		}
		return rollbackErr
	}

	applied := make([]rotationFile, 0, len(rotations))
	for _, file := range rotations {
		newETag, err := client.PUT(file.path, file.newData, file.oldETag)
		if err != nil {
			if rollbackErr := rollback(applied); rollbackErr != nil {
				return fmt.Errorf("轮换 %s 失败: %w；%v", file.path, err, rollbackErr)
			}
			return fmt.Errorf("轮换 %s 失败: %w", file.path, err)
		}
		file.newETag = newETag
		applied = append(applied, file)
	}
	return nil
}

func listRemoteFilesRecursive(client *webdav.Client, dir string) ([]string, error) {
	entries, err := client.PROPFIND(dir, 1)
	if err == webdav.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		remotePath := normalizeRemoteChildPath(dir, entry.Path)
		if strings.TrimSuffix(remotePath, "/") == strings.TrimSuffix(strings.TrimPrefix(dir, "/"), "/") {
			continue
		}
		if entry.IsDir {
			if !strings.HasSuffix(remotePath, "/") {
				remotePath += "/"
			}
			children, err := listRemoteFilesRecursive(client, remotePath)
			if err != nil {
				return nil, err
			}
			files = append(files, children...)
			continue
		}
		files = append(files, remotePath)
	}
	return files, nil
}

func normalizeRemoteChildPath(dir, child string) string {
	child = strings.TrimPrefix(child, "/")
	dir = strings.TrimPrefix(dir, "/")
	if strings.HasPrefix(child, dir) {
		return child
	}
	return dir + child
}

// ChangeEncryptionPassword 修改加密密码
func (a *App) ChangeEncryptionPassword(oldPassword, newPassword string) error {
	keyPath := config.CCBoxDir() + "/key.bin"
	saltPath := config.CCBoxDir() + "/salt.bin"
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("无法读取密钥: %w", err)
	}
	saltData, err := os.ReadFile(saltPath)
	if err != nil {
		cfg, cfgErr := config.Load()
		if cfgErr != nil {
			return fmt.Errorf("无法读取 salt: %w", err)
		}
		pass, passErr := config.LoadWebDAVPassword()
		if passErr != nil {
			return fmt.Errorf("无法读取 salt: %w", err)
		}
		client := newConfiguredWebDAVClient(cfg, pass)
		remoteSalt, _, saltErr := client.GET("salt.bin")
		if saltErr != nil {
			return fmt.Errorf("无法读取 salt: %w", err)
		}
		saltData = remoteSalt
		_ = os.WriteFile(saltPath, saltData, 0600)
	}
	// 验证旧密码
	oldKey := crypto.DeriveKey(oldPassword, saltData)
	if !crypto.ConstantTimeEqual(oldKey, keyData) {
		return fmt.Errorf("当前加密密码不正确")
	}
	// 生成新 salt 和密钥
	newSalt, err := crypto.GenerateSalt()
	if err != nil {
		return err
	}
	newKey := crypto.DeriveKey(newPassword, newSalt)
	// 轮换远程加密数据
	_, client, _, err := a.loadClients()
	if err != nil {
		return err
	}
	err = a.rotateRemoteData(client, keyData, newKey)
	if err != nil {
		return fmt.Errorf("轮换远程数据失败: %w", err)
	}
	rollbackRemote := func(cause error) error {
		if rollbackErr := a.rotateRemoteData(client, newKey, keyData); rollbackErr != nil {
			return fmt.Errorf("%w；回滚远程数据失败: %v", cause, rollbackErr)
		}
		_, _ = client.PUT("salt.bin", saltData, "")
		_ = os.WriteFile(saltPath, saltData, 0600)
		_ = os.WriteFile(keyPath, keyData, 0600)
		return cause
	}
	if _, err := client.PUT("salt.bin", newSalt, ""); err != nil {
		return rollbackRemote(fmt.Errorf("上传新 salt 失败: %w", err))
	}
	if err := os.WriteFile(saltPath, newSalt, 0600); err != nil {
		return rollbackRemote(fmt.Errorf("保存新 salt 失败: %w", err))
	}
	if err := os.WriteFile(keyPath, newKey, 0600); err != nil {
		return rollbackRemote(fmt.Errorf("保存新密钥失败: %w", err))
	}
	return nil
}
