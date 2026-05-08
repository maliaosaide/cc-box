// Dashboard 后端绑定
// 概览页数据获取 + 快捷操作
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/user/cc-box/internal/binary"
	"github.com/user/cc-box/internal/config"
	"github.com/user/cc-box/internal/crypto"
	"github.com/user/cc-box/internal/object"
	"github.com/user/cc-box/internal/snapshot"
	"github.com/user/cc-box/internal/webdav"
)

// DashboardData 概览页数据
type DashboardData struct {
	SyncStatus    string        `json:"syncStatus"`
	LastSync      string        `json:"lastSync"`
	ClaudeVersion string        `json:"claudeVersion"`
	ClaudeLatest  bool          `json:"claudeLatest"`
	Conflicts     int           `json:"conflicts"`
	ConflictFiles []ConflictRef `json:"conflictFiles"`
	Devices       []DeviceInfo  `json:"devices"`
	RecentChanges []ChangeInfo  `json:"recentChanges"`
	Backups       []BackupInfo  `json:"backups"`
	Binaries      []BinaryInfo  `json:"binaries"`
}

type ConflictRef struct {
	Path   string `json:"path"`
	Detail string `json:"detail"`
}

type DeviceInfo struct {
	Name       string `json:"name"`
	Platform   string `json:"platform"`
	Version    string `json:"version"`
	LastActive string `json:"lastActive"`
	IsCurrent  bool   `json:"isCurrent"`
}

type ChangeInfo struct {
	Status string `json:"status"`
	Path   string `json:"path"`
	Time   string `json:"time"`
}

type BackupInfo struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Device  string `json:"device"`
	Time    string `json:"time"`
}

type BinaryInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Latest  bool   `json:"latest"`
}

// GetDashboard 返回概览页数据
func (a *App) GetDashboard() (*DashboardData, error) {
	if !config.IsInitialized() {
		return nil, fmt.Errorf("未初始化")
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	data := &DashboardData{
		SyncStatus:    "synced",
		ClaudeVersion: "-",
		ClaudeLatest:  true,
		Conflicts:     0,
		Devices:       []DeviceInfo{},
		RecentChanges: []ChangeInfo{},
		Backups:       []BackupInfo{},
		Binaries:      []BinaryInfo{},
	}

	// 当前设备
	data.Devices = append(data.Devices, DeviceInfo{
		Name:       cfg.Device.Name,
		Platform:   config.Platform(),
		Version:    "-",
		LastActive: "刚刚",
		IsCurrent:  true,
	})

	// 检测 claude 二进制版本
	binPath := binary.GetBinaryPath("claude")
	data, _ = fillBinaryVersion(data, binPath)
	data.Binaries = []BinaryInfo{
		{Name: "uv", Version: "-", Latest: true},
	}

	// 尝试加载真实数据
	client, key, err := a.loadClientKey(cfg)
	if err == nil {
		a.fillDashboardFromSnapshots(data, cfg, client, key)
	}

	return data, nil
}

// fillDashboardFromSnapshots 从真实快照数据填充概览
func (a *App) fillDashboardFromSnapshots(data *DashboardData, cfg *config.Config, client *webdav.Client, key []byte) {
	// 加载快照列表
	headData, _, err := client.GET("HEAD")
	if err != nil || string(headData) == "" {
		return
	}
	headID := strings.TrimSpace(string(headData))

	// 本地 HEAD
	localHead, _ := os.ReadFile(config.CCBoxDir() + "/HEAD")
	localHeadStr := strings.TrimSpace(string(localHead))

	if localHeadStr != headID {
		data.SyncStatus = "pending"
	}

	// 加载最新快照获取备份信息和最近变更
	_, _ = a.loadSnapByID(client, key, headID)
	if err != nil {
		return
	}

	// 备份列表（最多3个）
	entries, _ := a.GetSnapshotList(3)
	for i, e := range entries {
		if i >= 3 {
			break
		}
		deviceName := e.Device
		// 找设备友好名称
		if deviceName == cfg.Device.ID {
			deviceName = cfg.Device.Name
		}
		data.Backups = append(data.Backups, BackupInfo{
			ID:      e.ID,
			Message: e.Message,
			Device:  deviceName,
			Time:    e.Timestamp,
		})
	}

	// 最近变更：对比最新快照和本地文件
	if localHeadStr != "" {
		localSnap, err := a.loadSnapByID(client, key, localHeadStr)
		if err == nil && localSnap != nil {
			scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
			scanResult, err := scanner.Scan()
			if err == nil {
				currentSnap := snapshot.CreateSnapshot("", cfg.Device.ID, "", scanResult.Files)
				changes := localSnap.Diff(currentSnap)
				for i, c := range changes {
					if i >= 5 {
						break
					}
					status := "M"
					switch c.Type {
					case snapshot.Added:
						status = "A"
					case snapshot.Deleted:
						status = "D"
					}
					data.RecentChanges = append(data.RecentChanges, ChangeInfo{
						Status: status,
						Path:   c.Path,
						Time:   "刚刚",
					})
				}
			}
		}
	}

	// 冲突
// 从远程加载设备列表
	a.loadRemoteDevices(data, cfg, client)

	conflictFiles := listConflicts()
	data.Conflicts = len(conflictFiles)
	for path := range conflictFiles {
		if len(data.ConflictFiles) < 10 {
			data.ConflictFiles = append(data.ConflictFiles, ConflictRef{Path: path})
		}
	}
}

func (a *App) loadClientKey(cfg *config.Config) (*webdav.Client, []byte, error) {
	key, err := crypto.LoadKey(config.KeyPath())
	if err != nil {
		return nil, nil, err
	}
	pass, err := config.LoadWebDAVPassword()
	if err != nil {
		return nil, nil, err
	}
	client := webdav.NewClient(cfg.WebDAV.URL, cfg.WebDAV.Username, pass)
	return client, key, nil
}

// QuickPush 真正推送配置到云端
func (a *App) QuickPush() int64 {
	return a.StartAsync("quick-push", func(ctx context.Context, opID int64) error {
		cfg, client, key, err := a.loadClients()
		if err != nil {
			return err
		}

		scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
		scanResult, err := scanner.Scan()
		if err != nil {
			return fmt.Errorf("扫描失败: %w", err)
		}

		localHead, _ := os.ReadFile(config.CCBoxDir() + "/HEAD")
		localHeadStr := strings.TrimSpace(string(localHead))

		var localSnap *snapshot.Snapshot
		if localHeadStr != "" {
			localSnap, _ = a.loadSnapByID(client, key, localHeadStr)
		}

		var changes []snapshot.Change
		if localSnap != nil {
			currentSnap := snapshot.CreateSnapshot("", cfg.Device.ID, "", scanResult.Files)
			changes = localSnap.Diff(currentSnap)
		} else {
			for path, entry := range scanResult.Files {
				changes = append(changes, snapshot.Change{Path: path, Type: snapshot.Added, NewHash: entry.Hash, NewSize: entry.Size})
			}
		}

		if len(changes) == 0 {
			a.emitProgress(opID, "quick-push", 1, 1, 1, 1, "没有变更需要推送")
			return nil
		}

		total := int64(len(changes))
		store := object.NewStore(client, key, config.CCBoxDir()+"/cache/objects")
		uploaded := 0

		for i, c := range changes {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if c.Type == snapshot.Deleted {
				continue
			}
			fullPath := filepath.Join(config.ClaudeDir(), filepath.FromSlash(c.Path))
			data, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}
			if _, err := store.Upload(data); err != nil {
				continue
			}
			uploaded++
			a.emitProgress(opID, "quick-push", int64(i+1), total, int(i+1), int(total), fmt.Sprintf("推送 %s", c.Path))
		}

		// 创建新快照
		newSnap := snapshot.CreateSnapshot(localHeadStr, cfg.Device.ID, "gui push", scanResult.Files)
		snapData, _ := newSnap.Serialize()
		encrypted, err := crypto.Encrypt(snapData, key)
		if err != nil {
			return fmt.Errorf("加密快照失败: %w", err)
		}
		client.EnsureDir("snapshots/")
		if _, err := client.PUT("snapshots/"+newSnap.ID+".json.enc", encrypted, ""); err != nil {
			return fmt.Errorf("上传快照失败: %w", err)
		}

		// 乐观锁更新 HEAD
		for attempt := 0; attempt < cfg.Sync.MergeRetryMax; attempt++ {
			_, currentETag, err := client.GET("HEAD")
			if err != nil && err != webdav.ErrNotFound {
				return fmt.Errorf("读取远程 HEAD 失败: %w", err)
			}
			result, err := client.CompareAndSwapHEAD("HEAD", newSnap.ID, currentETag)
			if err != nil {
				return fmt.Errorf("更新 HEAD 失败: %w", err)
			}
			if result.Success {
				break
			}
			time.Sleep(time.Duration(attempt+1) * time.Second)
			if attempt == cfg.Sync.MergeRetryMax-1 {
				return fmt.Errorf("推送冲突，请先拉取")
			}
		}

		// 更新本地
		os.WriteFile(config.CCBoxDir()+"/HEAD", []byte(newSnap.ID), 0600)
		os.WriteFile(config.CCBoxDir()+"/snapshots/"+newSnap.ID+".json", snapData, 0600)

		UpdateTrayState(TraySynced)
		a.emitProgress(opID, "quick-push", total, total, int(total), int(total), fmt.Sprintf("已推送 %d 个变更", uploaded))
		return nil
	})
}

// QuickPull 真正拉取远程配置
func (a *App) QuickPull() int64 {
	return a.StartAsync("quick-pull", func(ctx context.Context, opID int64) error {
		cfg, client, key, err := a.loadClients()
		if err != nil {
			return err
		}

		remoteHeadData, _, err := client.GET("HEAD")
		if err != nil {
			return fmt.Errorf("读取远程 HEAD 失败: %w", err)
		}
		remoteHead := strings.TrimSpace(string(remoteHeadData))
		if remoteHead == "" {
			return fmt.Errorf("远程没有数据")
		}

		localHead, _ := os.ReadFile(config.CCBoxDir() + "/HEAD")
		if strings.TrimSpace(string(localHead)) == remoteHead {
			a.emitProgress(opID, "quick-pull", 1, 1, 1, 1, "已是最新")
			return nil
		}

		remoteSnap, err := a.loadSnapByID(client, key, remoteHead)
		if err != nil {
			return fmt.Errorf("加载远程快照失败: %w", err)
		}

		// 扫描本地，计算需要下载的文件
		scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
		scanResult, err := scanner.Scan()
		if err != nil {
			return fmt.Errorf("扫描失败: %w", err)
		}

		var toDownload []string
		for path, remoteEntry := range remoteSnap.Files {
			localEntry, exists := scanResult.Files[path]
			if !exists || localEntry.Hash != remoteEntry.Hash {
				toDownload = append(toDownload, path)
			}
		}

		if len(toDownload) == 0 {
			os.WriteFile(config.CCBoxDir()+"/HEAD", []byte(remoteHead), 0600)
			a.emitProgress(opID, "quick-pull", 1, 1, 1, 1, "已是最新")
			return nil
		}

		UpdateTrayState(TraySyncing)
		store := object.NewStore(client, key, config.CCBoxDir()+"/cache/objects")
		total := int64(len(toDownload))
		applied := 0

		for i, path := range toDownload {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			remoteEntry, ok := remoteSnap.Files[path]
			if !ok {
				continue
			}
			data, err := store.Download(remoteEntry.Hash)
			if err != nil {
				continue
			}
			fullPath := filepath.Join(config.ClaudeDir(), filepath.FromSlash(path))
			os.MkdirAll(filepath.Dir(fullPath), 0755)
			os.WriteFile(fullPath, data, 0600)
			applied++
			a.emitProgress(opID, "quick-pull", int64(i+1), total, int(i+1), int(total), fmt.Sprintf("拉取 %s", path))
		}

		os.WriteFile(config.CCBoxDir()+"/HEAD", []byte(remoteHead), 0600)
		snapData, _ := remoteSnap.Serialize()
		os.WriteFile(config.CCBoxDir()+"/snapshots/"+remoteHead+".json", snapData, 0600)

		UpdateTrayState(TraySynced)
		a.emitProgress(opID, "quick-pull", total, total, int(total), int(total), fmt.Sprintf("已拉取 %d 个文件", applied))
		return nil
	})
}

// QuickSync pull + push 一步完成
func (a *App) QuickSync() int64 {
	return a.StartAsync("quick-sync", func(ctx context.Context, opID int64) error {
		UpdateTrayState(TraySyncing)
		a.emitProgress(opID, "quick-sync", 0, 2, 0, 2, "正在拉取...")

		pullID := a.QuickPull()
		// 等待 pull 完成
		for {
			time.Sleep(200 * time.Millisecond)
			opCancelMu.Lock()
			_, running := opCancels[pullID]
			opCancelMu.Unlock()
			if !running {
				break
			}
		}

		a.emitProgress(opID, "quick-sync", 1, 2, 1, 2, "正在推送...")
		pushID := a.QuickPush()
		for {
			time.Sleep(200 * time.Millisecond)
			opCancelMu.Lock()
			_, running := opCancels[pushID]
			opCancelMu.Unlock()
			if !running {
				break
			}
		}

		a.emitProgress(opID, "quick-sync", 2, 2, 2, 2, "同步完成")
		return nil
	})
}

// fillBinaryVersion 尝试从二进制文件检测版本号
func fillBinaryVersion(d *DashboardData, binPath string) (*DashboardData, error) {
	ver := detectBinVersion(binPath)
	if ver != "" {
		d.ClaudeVersion = ver
	}
	return d, nil
}

func detectBinVersion(binPath string) string {
	cmd := exec.Command(binPath, "--version")
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = hideWindowAttr()
	}
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(output))
	if len(fields) > 0 {
		return fields[0]
	}
	return ""
}

func hideWindowAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
}

// loadRemoteDevices 从远程 devices/ 目录加载设备列表
func (a *App) loadRemoteDevices(data *DashboardData, cfg *config.Config, client *webdav.Client) {
	type remoteDeviceInfo struct {
		ID       string    `json:"id"`
		Name     string    `json:"name"`
		Platform string    `json:"platform"`
		LastSeen time.Time `json:"last_seen"`
	}

	files, err := client.PROPFIND("devices/", 1)
	if err != nil {
		return
	}

	for _, f := range files {
		if f.IsDir || !strings.HasSuffix(f.Path, ".json") {
			continue
		}
		fileName := f.Path
		if idx := strings.LastIndex(fileName, "/"); idx >= 0 {
			fileName = fileName[idx+1:]
		}
		devData, _, err := client.GET("devices/" + fileName)
		if err != nil {
			continue
		}
		var info remoteDeviceInfo
		if err := json.Unmarshal(devData, &info); err != nil {
			continue
		}
		if info.ID == cfg.Device.ID {
			continue // 本机已添加
		}
		ago := formatTimeAgo(info.LastSeen)
		data.Devices = append(data.Devices, DeviceInfo{
			Name:       info.Name,
			Platform:   info.Platform,
			LastActive: ago,
			IsCurrent:  false,
		})
	}
}

func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "未知"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟前", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时前", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d 天前", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}
