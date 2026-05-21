// Dashboard 后端绑定
// 概览页数据获取 + 快捷操作
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/user/cc-box/gui/internal/binary"
	"github.com/user/cc-box/gui/internal/config"
	"github.com/user/cc-box/gui/internal/crypto"
	"github.com/user/cc-box/gui/internal/object"
	"github.com/user/cc-box/gui/internal/snapshot"
	"github.com/user/cc-box/gui/internal/webdav"
)

// DashboardData 概览页数据
type DashboardData struct {
	SyncStatus    string        `json:"syncStatus"`
	SyncHealth    SyncHealth    `json:"syncHealth"`
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

type SyncHealth struct {
	Status     string `json:"status"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	CanRepair  bool   `json:"canRepair"`
	LocalHead  string `json:"localHead,omitempty"`
	RemoteHead string `json:"remoteHead,omitempty"`
}

var (
	errRemoteUninitialized = errors.New("远程尚未初始化")
	errKeyMismatch         = errors.New("密钥不匹配")
)

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
	Name      string `json:"name"`
	Version   string `json:"version"`
	Latest    bool   `json:"latest"`
	Installed bool   `json:"installed"`
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
		SyncStatus:    "idle",
		SyncHealth:    SyncHealth{Status: "idle", Code: "idle", Message: "尚未同步"},
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

	data.Binaries = collectInstalledBinaries(nil)

	client, key, err := a.loadClientKey(cfg)
	if err != nil {
		data.setSyncHealth("connection_error", "local_credentials_error", err.Error(), false, "", "")
		return data, nil
	}
	a.fillDashboardFromSnapshots(data, cfg, client, key)

	return data, nil
}

// fillDashboardFromSnapshots 从真实快照数据填充概览
func (a *App) fillDashboardFromSnapshots(data *DashboardData, cfg *config.Config, client *webdav.Client, key []byte) {
	localHeadStr, localHeadErr := readLocalHeadID()
	if localHeadErr != nil {
		localHeadStr = ""
	}
	if localHeadStr != "" {
		if err := validateSnapshotID(localHeadStr); err != nil {
			data.setSyncHealth("local_error", "local_head_invalid", err.Error(), false, localHeadStr, "")
			return
		}
	}

	headData, _, err := client.GET("HEAD")
	if err == webdav.ErrNotFound {
		data.setSyncHealth("remote_uninitialized", "remote_head_missing", "当前 WebDAV 根路径下没有 HEAD。请检查根路径，确认无误后可用本机数据初始化远程。", true, localHeadStr, "")
		return
	}
	if err != nil {
		data.setSyncHealth("connection_error", "remote_head_read_failed", fmt.Sprintf("读取远程 HEAD 失败: %v", err), false, localHeadStr, "")
		return
	}
	headID := strings.TrimSpace(string(headData))
	if headID == "" {
		data.setSyncHealth("remote_incomplete", "remote_head_empty", "远程 HEAD 为空。请检查 WebDAV 根路径或手动修复远程数据。", false, localHeadStr, "")
		return
	}
	if err := validateSnapshotID(headID); err != nil {
		data.setSyncHealth("remote_incomplete", "remote_head_invalid", err.Error(), false, localHeadStr, headID)
		return
	}

	// 加载最新快照获取备份信息和最近变更
	snap, snapErr := a.loadRemoteSnapByID(client, key, headID)
	if snapErr != nil || snap == nil {
		if errors.Is(snapErr, webdav.ErrNotFound) {
			data.setSyncHealth("remote_incomplete", "remote_snapshot_missing", "远程 HEAD 指向的快照不存在，请检查 WebDAV 根路径或手动修复远程数据。", false, localHeadStr, headID)
			return
		}
		if errors.Is(snapErr, errKeyMismatch) {
			data.setSyncHealth("key_mismatch", "key_mismatch", "本机加密密码无法解密远程快照，请确认加密密码或 WebDAV 根路径是否正确。", false, localHeadStr, headID)
			return
		}
		data.setSyncHealth("remote_incomplete", "remote_snapshot_invalid", fmt.Sprintf("远程快照无效: %v", snapErr), false, localHeadStr, headID)
		return
	}

	if localHeadStr != headID {
		data.setSyncHealth("pending", "head_mismatch", "本地与远程 HEAD 不一致，需要同步。", false, localHeadStr, headID)
	} else {
		data.setSyncHealth("synced", "synced", "本地与远程一致。", false, localHeadStr, headID)
	}

	// 上次同步时间
	data.LastSync = snap.Timestamp.Local().Format("2006-01-02 15:04")

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
						Time:   localSnap.Timestamp.Local().Format("15:04"),
					})
				}
			}
		}
	}

	data.Binaries = collectInstalledBinaries(client)

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

func (d *DashboardData) setSyncHealth(status, code, message string, canRepair bool, localHead, remoteHead string) {
	d.SyncStatus = status
	d.SyncHealth = SyncHealth{
		Status:     status,
		Code:       code,
		Message:    message,
		CanRepair:  canRepair,
		LocalHead:  localHead,
		RemoteHead: remoteHead,
	}
}

func (a *App) loadRemoteSnapByID(client *webdav.Client, key []byte, id string) (*snapshot.Snapshot, error) {
	id = strings.TrimSpace(id)
	if err := validateSnapshotID(id); err != nil {
		return nil, err
	}
	encrypted, _, err := client.GET("snapshots/" + id + ".json.enc")
	if err != nil {
		return nil, err
	}
	decrypted, err := decryptRemoteData(encrypted, key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errKeyMismatch, err)
	}
	snap, err := snapshot.Deserialize(decrypted)
	if err != nil {
		return nil, err
	}
	if snap.ID != id {
		return nil, fmt.Errorf("远程快照 ID 与 HEAD 不一致")
	}
	return snap, nil
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
	client := newConfiguredWebDAVClient(cfg, pass)
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

		currentBins := currentBinaryVersions()
		var changes []snapshot.Change
		binaryChanged := localSnap == nil || !binaryVersionsEqual(localSnap.Binary, currentBins)
		if localSnap != nil {
			currentSnap := snapshot.CreateSnapshot("", cfg.Device.ID, "", scanResult.Files)
			changes = localSnap.Diff(currentSnap)
		} else {
			for path, entry := range scanResult.Files {
				changes = append(changes, snapshot.Change{Path: path, Type: snapshot.Added, NewHash: entry.Hash, NewSize: entry.Size})
			}
		}

		if len(changes) == 0 && !binaryChanged {
			a.emitProgress(opID, "quick-push", 1, 1, 1, 1, "没有变更需要推送")
			return nil
		}

		_, headETag, err := remoteHeadETagForUpdate(client, localHeadStr)
		if err != nil {
			return err
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
			fullPath, err := safeClaudePath(c.Path)
			if err != nil {
				return err
			}
			data, err := readObjectData(fullPath)
			if err != nil {
				return fmt.Errorf("读取文件 %s 失败: %w", c.Path, err)
			}
			if hash, err := store.Upload(data); err != nil {
				return fmt.Errorf("上传文件 %s 失败: %w", c.Path, err)
			} else if hash != c.NewHash {
				return fmt.Errorf("文件 %s hash 不一致", c.Path)
			}
			uploaded++
			a.emitProgress(opID, "quick-push", int64(i+1), total, int(i+1), int(total), fmt.Sprintf("推送 %s", c.Path))
		}

		// 创建新快照
		newSnap := snapshot.CreateSnapshot(localHeadStr, cfg.Device.ID, "gui push", scanResult.Files)
		newSnap.Binary = currentBins
		snapData, _ := newSnap.Serialize()
		encrypted, err := encryptRemoteData(snapData, key)
		if err != nil {
			return fmt.Errorf("加密快照失败: %w", err)
		}
		if err := client.EnsureDir("snapshots/"); err != nil {
			return fmt.Errorf("创建快照目录失败: %w", err)
		}
		if _, err := client.PUT("snapshots/"+newSnap.ID+".json.enc", encrypted, ""); err != nil {
			return fmt.Errorf("上传快照失败: %w", err)
		}

		// 乐观锁更新 HEAD
		result, err := client.CompareAndSwapHEAD("HEAD", newSnap.ID, headETag)
		if err != nil {
			return fmt.Errorf("更新 HEAD 失败: %w", err)
		}
		if !result.Success {
			return fmt.Errorf("远程 HEAD 已变化，请先拉取")
		}

		// 更新本地
		if err := os.WriteFile(config.CCBoxDir()+"/HEAD", []byte(newSnap.ID), 0600); err != nil {
			return fmt.Errorf("更新本地 HEAD 失败: %w", err)
		}
		if err := os.WriteFile(config.CCBoxDir()+"/snapshots/"+newSnap.ID+".json", snapData, 0600); err != nil {
			return fmt.Errorf("缓存快照失败: %w", err)
		}

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
		if err == webdav.ErrNotFound {
			return fmt.Errorf("远程尚未初始化或当前 WebDAV 根路径下没有 HEAD，请先检查 WebDAV 根路径，或在概览页显式以本机为准初始化远程: %w", errRemoteUninitialized)
		}
		if err != nil {
			return fmt.Errorf("读取远程 HEAD 失败: %w", err)
		}
		remoteHead := strings.TrimSpace(string(remoteHeadData))
		if remoteHead == "" {
			return fmt.Errorf("远程 HEAD 为空，请检查 WebDAV 根路径或手动修复远程数据: %w", errRemoteUninitialized)
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

		UpdateTrayState(TraySyncing)
		result, err := a.applyRemoteSnapshot(ctx, opID, "quick-pull", cfg, client, key, remoteHead, remoteSnap)
		if err != nil {
			return err
		}
		if result.Conflicts > 0 {
			UpdateTrayState(TrayConflict)
			return fmt.Errorf("发现 %d 个冲突，请在文件页选择以本地或远程为准", result.Conflicts)
		}

		UpdateTrayState(TraySynced)
		if result.Applied == 0 {
			a.emitProgress(opID, "quick-pull", 1, 1, 1, 1, "已是最新")
			return nil
		}
		a.emitProgress(opID, "quick-pull", int64(result.Applied), int64(result.Total), result.Applied, result.Total, fmt.Sprintf("已拉取 %d 个文件", result.Applied))
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
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		opCancelMu.Lock()
		pullErr := opResults[pullID]
		opCancelMu.Unlock()
		if pullErr != nil {
			return fmt.Errorf("拉取失败: %w", pullErr)
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
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		opCancelMu.Lock()
		pushErr := opResults[pushID]
		opCancelMu.Unlock()
		if pushErr != nil {
			return fmt.Errorf("推送失败: %w", pushErr)
		}

		a.emitProgress(opID, "quick-sync", 2, 2, 2, 2, "同步完成")
		return nil
	})
}

func (a *App) RepairRemoteFromLocal() int64 {
	return a.StartAsync("repair-remote", func(ctx context.Context, opID int64) error {
		cfg, client, key, err := a.loadClients()
		if err != nil {
			return err
		}

		UpdateTrayState(TraySyncing)
		a.emitProgress(opID, "repair-remote", 0, 4, 0, 4, "检查远程 HEAD...")
		if err := ensureRemoteHeadMissing(client); err != nil {
			return err
		}

		localHead, err := readLocalHeadID()
		if err != nil {
			localHead = ""
		}
		if localHead != "" {
			if err := validateSnapshotID(localHead); err != nil {
				return err
			}
		}

		a.emitProgress(opID, "repair-remote", 1, 4, 1, 4, "检查加密 salt...")
		if err := ensureRemoteSaltFromLocal(client); err != nil {
			return err
		}

		scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
		scanResult, err := scanner.Scan()
		if err != nil {
			return fmt.Errorf("扫描失败: %w", err)
		}

		store := object.NewStore(client, key, config.CCBoxDir()+"/cache/objects")
		total := int64(len(scanResult.Files))
		var uploaded int64
		for path, entry := range scanResult.Files {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			fullPath, err := safeClaudePath(path)
			if err != nil {
				return err
			}
			data, err := readObjectData(fullPath)
			if err != nil {
				return fmt.Errorf("读取文件 %s 失败: %w", path, err)
			}
			hash, err := store.Upload(data)
			if err != nil {
				return fmt.Errorf("上传文件 %s 失败: %w", path, err)
			}
			if hash != entry.Hash {
				return fmt.Errorf("文件 %s hash 不一致", path)
			}
			uploaded++
			a.emitProgress(opID, "repair-remote", uploaded, total, 2, 4, fmt.Sprintf("上传 %s", path))
		}

		snap := snapshot.CreateSnapshot(localHead, cfg.Device.ID, "repair remote from local", scanResult.Files)
		snap.Binary = currentBinaryVersions()
		snapData, err := snap.Serialize()
		if err != nil {
			return fmt.Errorf("序列化快照失败: %w", err)
		}
		encrypted, err := encryptRemoteData(snapData, key)
		if err != nil {
			return fmt.Errorf("加密快照失败: %w", err)
		}
		if err := client.EnsureDir("snapshots/"); err != nil {
			return fmt.Errorf("创建快照目录失败: %w", err)
		}
		if _, err := client.PUT("snapshots/"+snap.ID+".json.enc", encrypted, ""); err != nil {
			return fmt.Errorf("上传快照失败: %w", err)
		}

		a.emitProgress(opID, "repair-remote", 3, 4, 3, 4, "写入远程 HEAD...")
		if err := ensureRemoteHeadMissing(client); err != nil {
			return err
		}
		if _, err := client.PUT("HEAD", []byte(snap.ID), ""); err != nil {
			return fmt.Errorf("写入远程 HEAD 失败: %w", err)
		}
		if err := os.WriteFile(config.CCBoxDir()+"/HEAD", []byte(snap.ID), 0600); err != nil {
			return fmt.Errorf("更新本地 HEAD 失败: %w", err)
		}
		if err := os.WriteFile(config.CCBoxDir()+"/snapshots/"+snap.ID+".json", snapData, 0600); err != nil {
			return fmt.Errorf("缓存快照失败: %w", err)
		}
		registerDeviceInfo(client, cfg)
		UpdateTrayState(TraySynced)
		a.emitProgress(opID, "repair-remote", 4, 4, 4, 4, "远程初始化完成")
		return nil
	})
}

func ensureRemoteHeadMissing(client *webdav.Client) error {
	headData, _, err := client.GET("HEAD")
	if err == webdav.ErrNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取远程 HEAD 失败: %w", err)
	}
	if strings.TrimSpace(string(headData)) == "" {
		return fmt.Errorf("远程 HEAD 已存在但为空，已停止修复以避免覆盖可能损坏的远程数据；请先检查 WebDAV 根路径")
	}
	return fmt.Errorf("远程 HEAD 已存在，已停止修复以避免覆盖远程数据；请先检查 WebDAV 根路径")
}

func ensureRemoteSaltFromLocal(client *webdav.Client) error {
	localSalt, err := os.ReadFile(config.CCBoxDir() + "/salt.bin")
	if err != nil {
		return fmt.Errorf("读取本地 salt 失败: %w", err)
	}
	remoteSalt, _, err := client.GET("salt.bin")
	if err == nil {
		if !bytes.Equal(remoteSalt, localSalt) {
			return fmt.Errorf("远程 salt 与本地不一致，请检查 WebDAV 根路径")
		}
		return nil
	}
	if err != webdav.ErrNotFound {
		return fmt.Errorf("读取远程 salt 失败: %w", err)
	}
	if _, err := client.PUT("salt.bin", localSalt, ""); err != nil {
		return fmt.Errorf("上传 salt 失败: %w", err)
	}
	return nil
}

// fillBinaryVersion 尝试从二进制文件检测版本号
func fillBinaryVersion(d *DashboardData, binPath string) (*DashboardData, error) {
	ver := detectBinVersion(binPath)
	if ver != "" {
		d.ClaudeVersion = ver
	}
	return d, nil
}

func collectInstalledBinaries(client *webdav.Client) []BinaryInfo {
	tools := []string{"claude", "uv", "uvx", "codex", "gemini"}
	versions := make([]BinaryInfo, 0, len(tools))
	var idx *binary.Index
	if client != nil {
		idx, _ = binary.LoadIndex(client)
	}
	platform := config.Platform()
	for _, name := range tools {
		binPath := binary.GetBinaryPath(name)
		version := ""
		if name == "claude" {
			resolution := binary.ResolveClaudeBinary()
			if !resolution.Valid {
				continue
			}
			binPath = resolution.CurrentPath
			version = resolution.Version
		} else {
			if _, err := os.Stat(binPath); err != nil {
				continue
			}
			version = detectBinVersion(binPath)
		}
		latest := true
		if idx != nil {
			if info := idx.GetBinaryInfo(platform, name); info != nil {
				latest = !hasNewerBinaryVersion(version, info.Versions)
			}
		}
		versions = append(versions, BinaryInfo{
			Name:      name,
			Version:   version,
			Latest:    latest,
			Installed: true,
		})
	}
	return versions
}

func currentBinaryVersions() map[string]map[string]string {
	platform := config.Platform()
	tools := []string{"claude", "uv", "uvx", "codex", "gemini"}
	versions := make(map[string]string)
	for _, name := range tools {
		version := ""
		if name == "claude" {
			resolution := binary.ResolveClaudeBinary()
			if !resolution.Valid || resolution.IsShim {
				continue
			}
			version = resolution.Version
		} else {
			binPath := binary.GetBinaryPath(name)
			if _, err := os.Stat(binPath); err != nil {
				continue
			}
			version = detectBinVersion(binPath)
		}
		if version != "" {
			versions[name] = version
		}
	}
	if len(versions) == 0 {
		return nil
	}
	return map[string]map[string]string{platform: versions}
}

func binaryVersionsEqual(a, b map[string]map[string]string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for platform, aTools := range a {
		bTools, ok := b[platform]
		if !ok || len(aTools) != len(bTools) {
			return false
		}
		for name, aVersion := range aTools {
			if bTools[name] != aVersion {
				return false
			}
		}
	}
	return true
}

func hasNewerBinaryVersion(current string, versions map[string]binary.Version) bool {
	if len(versions) == 0 {
		return false
	}
	if current == "" {
		return true
	}
	for remote := range versions {
		if remote == current {
			continue
		}
		cmp, ok := compareVersion(remote, current)
		if !ok || cmp > 0 {
			return true
		}
	}
	return false
}

func compareVersion(a, b string) (int, bool) {
	aParts, okA := versionParts(a)
	bParts, okB := versionParts(b)
	if !okA || !okB {
		return 0, false
	}
	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}
	for i := 0; i < maxLen; i++ {
		var av, bv int
		if i < len(aParts) {
			av = aParts[i]
		}
		if i < len(bParts) {
			bv = bParts[i]
		}
		if av > bv {
			return 1, true
		}
		if av < bv {
			return -1, true
		}
	}
	return 0, true
}

func versionParts(version string) ([]int, bool) {
	version = cleanVersionToken(version)
	if version == "" {
		return nil, false
	}
	segments := strings.Split(version, ".")
	parts := make([]int, 0, len(segments))
	for _, segment := range segments {
		segment = leadingDigits(segment)
		if segment == "" {
			break
		}
		value, err := strconv.Atoi(segment)
		if err != nil {
			return nil, false
		}
		parts = append(parts, value)
	}
	return parts, len(parts) > 0
}

func detectBinVersion(binPath string) string {
	version, err := binary.DetectVersion(binPath)
	if err != nil {
		return ""
	}
	return version
}

func cleanVersionToken(token string) string {
	token = strings.Trim(token, " \t\r\n,;()[]{}")
	token = strings.TrimPrefix(token, "v")
	if token == "" {
		return ""
	}
	if token[0] < '0' || token[0] > '9' {
		return ""
	}
	return token
}

func leadingDigits(value string) string {
	for i, r := range value {
		if r < '0' || r > '9' {
			return value[:i]
		}
	}
	return value
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
