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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/user/cc-box/core/binary"
	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/crypto"
	"github.com/user/cc-box/core/normalize"
	"github.com/user/cc-box/core/object"
	"github.com/user/cc-box/core/snapshot"
	"github.com/user/cc-box/core/webdav"
)

// DashboardData 概览页数据
type DashboardData struct {
	SyncStatus    string           `json:"syncStatus"`
	SyncHealth    SyncHealth       `json:"syncHealth"`
	LastSync      string           `json:"lastSync"`
	ClaudeVersion string           `json:"claudeVersion"`
	ClaudeLatest  bool             `json:"claudeLatest"`
	ClaudeBinary  ClaudeBinaryInfo `json:"claudeBinary"`
	ConfigStatus  ConfigStatus     `json:"configStatus"`
	Conflicts     int              `json:"conflicts"`
	ConflictFiles []ConflictRef    `json:"conflictFiles"`
	Devices       []DeviceInfo     `json:"devices"`
	RecentChanges []ChangeInfo     `json:"recentChanges"`
	Backups       []BackupInfo     `json:"backups"`
	Binaries      []BinaryInfo     `json:"binaries"`
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

type ClaudeBinaryInfo struct {
	Platform      string `json:"platform"`
	PlatformLabel string `json:"platformLabel"`
	LocalVersion  string `json:"localVersion"`
	RemoteVersion string `json:"remoteVersion"`
	Installed     bool   `json:"installed"`
	Status        string `json:"status"`
	StatusLabel   string `json:"statusLabel"`
}

type ConfigStatus struct {
	OK                bool   `json:"ok"`
	WebDAVConfigured  bool   `json:"webdavConfigured"`
	PasswordAvailable bool   `json:"passwordAvailable"`
	ClaudeDirExists   bool   `json:"claudeDirExists"`
	Message           string `json:"message"`
}

// GetDashboard 返回完整概览页数据，保留给旧调用使用
func (a *App) GetDashboard() (*DashboardData, error) {
	return a.RefreshDashboardRemote()
}

// GetDashboardLocal 返回不依赖远程请求的首屏概览数据
func (a *App) GetDashboardLocal() (*DashboardData, error) {
	if !config.IsInitialized() {
		return nil, fmt.Errorf("未初始化")
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	return a.buildDashboardBase(cfg), nil
}

// RefreshDashboardRemote 在本地概览数据基础上补充远程同步状态
func (a *App) RefreshDashboardRemote() (*DashboardData, error) {
	if !config.IsInitialized() {
		return nil, fmt.Errorf("未初始化")
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	data := a.buildDashboardBase(cfg)
	if data.SyncStatus == "local_error" {
		return data, nil
	}

	client, key, err := a.loadClientKey(cfg)
	if err != nil {
		data.setSyncHealth("local_error", "local_credentials_error", err.Error(), false, "", "")
		return data, nil
	}
	a.fillDashboardFromSnapshots(data, cfg, client, key)

	return data, nil
}

func (a *App) buildDashboardBase(cfg *config.Config) *DashboardData {
	claudeBinary := a.collectClaudeBinaryInfo(nil)
	data := &DashboardData{
		SyncStatus:    "checking",
		SyncHealth:    SyncHealth{Status: "checking", Code: "checking_remote", Message: "本地数据已加载，正在检查远程..."},
		ClaudeVersion: claudeBinary.LocalVersion,
		ClaudeLatest:  claudeBinary.Status == "latest",
		ClaudeBinary:  claudeBinary,
		ConfigStatus:  buildConfigStatus(cfg),
		Devices:       []DeviceInfo{},
		RecentChanges: []ChangeInfo{},
		Backups:       []BackupInfo{},
		Binaries:      []BinaryInfo{},
	}

	data.Devices = append(data.Devices, DeviceInfo{
		Name:       cfg.Device.Name,
		Platform:   config.Platform(),
		Version:    "-",
		LastActive: "刚刚",
		IsCurrent:  true,
	})

	if !data.ConfigStatus.OK {
		data.setSyncHealth("local_error", "local_config_invalid", data.ConfigStatus.Message, false, "", "")
	}

	localHeadStr, err := readLocalHeadID()
	if err == nil && localHeadStr != "" {
		if err := validateSnapshotID(localHeadStr); err != nil {
			data.setSyncHealth("local_error", "local_head_invalid", err.Error(), false, localHeadStr, "")
		}
	}

	entries, _ := a.GetLocalSnapshotList(3)
	data.fillBackups(cfg, entries)
	fillDashboardConflicts(data)

	return data
}

func (d *DashboardData) fillBackups(cfg *config.Config, entries []SnapshotEntry) {
	d.Backups = []BackupInfo{}
	if d.LastSync == "" && len(entries) > 0 {
		d.LastSync = entries[0].Timestamp
	}
	for i, e := range entries {
		if i >= 3 {
			break
		}
		deviceName := e.Device
		if deviceName == cfg.Device.ID {
			deviceName = cfg.Device.Name
		}
		d.Backups = append(d.Backups, BackupInfo{
			ID:      e.ID,
			Message: e.Message,
			Device:  deviceName,
			Time:    e.Timestamp,
		})
	}
}

func fillDashboardConflicts(data *DashboardData) {
	conflictFiles := listConflicts()
	data.Conflicts = len(conflictFiles)
	data.ConflictFiles = []ConflictRef{}
	for path := range conflictFiles {
		if len(data.ConflictFiles) < 10 {
			data.ConflictFiles = append(data.ConflictFiles, ConflictRef{Path: path})
		}
	}
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

	headData, _, err := getDashboardRemoteHead(client)
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

	// 加载最新快照获取备份信息
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
		uncommitted := detectUncommittedChanges(cfg.Exclude.Patterns, snap)
		if uncommitted {
			data.setSyncHealth("pending", "local_uncommitted", "本地有未同步的变更，请推送。", false, localHeadStr, headID)
		} else {
			data.setSyncHealth("synced", "synced", "本地与远程一致。", false, localHeadStr, headID)
		}
	}

	// 上次同步时间
	data.LastSync = snap.Timestamp.Local().Format("2006-01-02 15:04")

	entries, _ := a.GetSnapshotList(3)
	data.fillBackups(cfg, entries)

	data.ClaudeBinary = a.collectClaudeBinaryRemoteInfo(data.ClaudeBinary, client)
	data.ClaudeVersion = data.ClaudeBinary.LocalVersion
	data.ClaudeLatest = data.ClaudeBinary.Status == "latest"

	a.loadRemoteDevices(data, cfg, client)
	fillDashboardConflicts(data)
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

const dashboardRemoteHeadAttempts = 3

var dashboardRemoteHeadRetryDelay = 300 * time.Millisecond

func getDashboardRemoteHead(client *webdav.Client) ([]byte, string, error) {
	var data []byte
	var etag string
	var err error
	for attempt := 0; attempt < dashboardRemoteHeadAttempts; attempt++ {
		data, etag, err = client.GET("HEAD")
		if err == nil || err == webdav.ErrNotFound {
			return data, etag, err
		}
		if attempt+1 < dashboardRemoteHeadAttempts {
			time.Sleep(dashboardRemoteHeadRetryDelay)
		}
	}
	return data, etag, err
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
	client.SetTimeout(8 * time.Second)
	return client, key, nil
}

// QuickPush 真正推送配置到云端
func (a *App) QuickPush() int64 {
	return a.StartAsync("quick-push", func(ctx context.Context, opID int64) error {
		cfg, client, key, err := a.loadClients()
		if err != nil {
			return err
		}

		scanner := newClaudeScanner(cfg.Exclude.Patterns)
		scanResult, err := scanner.ScanPartial()
		if err != nil {
			return fmt.Errorf("扫描失败: %w", err)
		}
		if err := requireCompleteScan(scanResult); err != nil {
			return err
		}

		localHead, _ := os.ReadFile(config.CCBoxDir() + "/HEAD")
		localHeadStr := strings.TrimSpace(string(localHead))

		var localSnap *snapshot.Snapshot
		if localHeadStr != "" {
			localSnap, _ = a.loadSnapByID(client, key, localHeadStr)
		}

		var currentBins map[string]map[string]string
		var changes []snapshot.Change
		binaryChanged := false
		if cfg.Binary.SyncEnabled {
			version, err := binary.CurrentClaudeVersion()
			if err != nil {
				return err
			}
			currentBins = map[string]map[string]string{config.Platform(): {"claude": version}}
		}
		if localSnap != nil {
			currentSnap := snapshot.CreateSnapshot("", cfg.Device.ID, "", scanResult.Files)
			changes = localSnap.Diff(currentSnap)
			binaryChanged = cfg.Binary.SyncEnabled && !binaryVersionsEqual(localSnap.Binary, currentBins)
		} else {
			for path, entry := range scanResult.Files {
				changes = append(changes, snapshot.Change{Path: path, Type: snapshot.Added, NewHash: entry.Hash, NewSize: entry.Size})
			}
			binaryChanged = cfg.Binary.SyncEnabled
		}

		if len(changes) == 0 && !binaryChanged {
			repaired, err := a.ensureRemoteSnapshotObjects(ctx, opID, "quick-push", client, key, scanResult.Files)
			if err != nil {
				return err
			}
			if repaired > 0 {
				a.emitProgress(opID, "quick-push", 1, 1, 1, 1, fmt.Sprintf("已补传 %d 个缺失文件", repaired))
				return nil
			}
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

		if cfg.Binary.SyncEnabled {
			version, uploadedBinary, err := binary.EnsureCurrentClaudeUploaded(client, key, a.progressCallback(opID, "quick-push"))
			if err != nil {
				return err
			}
			currentBins = map[string]map[string]string{config.Platform(): {"claude": version}}
			if uploadedBinary {
				a.clearBinaryIndexCache()
			}
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
		if err := config.WriteFileEnsureDir(config.CCBoxDir()+"/HEAD", []byte(newSnap.ID), 0600); err != nil {
			return fmt.Errorf("更新本地 HEAD 失败: %w", err)
		}
		if err := config.WriteFileEnsureDir(config.CCBoxDir()+"/snapshots/"+newSnap.ID+".json", snapData, 0600); err != nil {
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
		if strings.TrimSpace(string(localHead)) == remoteHead && !cfg.Binary.SyncEnabled {
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
			if result.BinaryApplied {
				a.emitProgress(opID, "quick-pull", 1, 1, 1, 1, "已恢复 Claude binary")
			} else {
				a.emitProgress(opID, "quick-pull", 1, 1, 1, 1, "已是最新")
			}
			return nil
		}
		if result.BinaryApplied {
			a.emitProgress(opID, "quick-pull", int64(result.Applied), int64(result.Total), result.Applied, result.Total, fmt.Sprintf("已拉取 %d 个文件并恢复 Claude binary", result.Applied))
		} else {
			a.emitProgress(opID, "quick-pull", int64(result.Applied), int64(result.Total), result.Applied, result.Total, fmt.Sprintf("已拉取 %d 个文件", result.Applied))
		}
		return nil
	})
}

// QuickSync pull + push 一步完成
func (a *App) QuickSync() int64 {
	return a.StartAsync("quick-sync", func(ctx context.Context, opID int64) error {
		cfg, client, key, err := a.loadClients()
		if err != nil {
			return err
		}
		client.SetTimeout(5 * time.Second)

		// 快速检查：HEAD 一致且无未提交变更 → 跳过
		localHeadStr, _ := readLocalHeadID()
		if localHeadStr != "" {
			remoteHeadData, _, headErr := getDashboardRemoteHead(client)
			if headErr == nil {
				remoteHead := strings.TrimSpace(string(remoteHeadData))
				if localHeadStr == remoteHead {
					if snap, snapErr := a.loadSnapByID(client, key, localHeadStr); snapErr == nil && snap != nil {
						if !detectUncommittedChanges(cfg.Exclude.Patterns, snap) {
							UpdateTrayState(TraySynced)
							a.emitProgress(opID, "quick-sync", 1, 1, 1, 1, "已是最新，无需同步")
							return nil
						}
					}
				}
			}
		}

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
		pullErr, ok := takeOpResult(pullID)
		if !ok {
			return fmt.Errorf("拉取结果丢失")
		}
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
		pushErr, ok := takeOpResult(pushID)
		if !ok {
			return fmt.Errorf("推送结果丢失")
		}
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
		releaseInitLock, err := acquireRemoteInitLock(client, cfg.Device.ID)
		if err != nil {
			return err
		}
		defer releaseInitLock()

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

		scanner := newClaudeScanner(cfg.Exclude.Patterns)
		scanResult, err := scanner.ScanPartial()
		if err != nil {
			return fmt.Errorf("扫描失败: %w", err)
		}
		if err := requireCompleteScan(scanResult); err != nil {
			return err
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
		if cfg.Binary.SyncEnabled {
			version, _, err := binary.EnsureCurrentClaudeUploaded(client, key, a.progressCallback(opID, "repair-remote"))
			if err != nil {
				return err
			}
			binary.SetSnapshotClaudeVersion(snap, version)
		}
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
		if _, err := client.PUTIfAbsent("HEAD", []byte(snap.ID)); err != nil {
			if err == webdav.ErrConflict {
				return fmt.Errorf("远程 HEAD 已存在，已停止修复以避免覆盖远程数据；请先检查 WebDAV 根路径")
			}
			return fmt.Errorf("写入远程 HEAD 失败: %w", err)
		}
		if err := config.WriteFileEnsureDir(config.CCBoxDir()+"/HEAD", []byte(snap.ID), 0600); err != nil {
			return fmt.Errorf("更新本地 HEAD 失败: %w", err)
		}
		if err := config.WriteFileEnsureDir(config.CCBoxDir()+"/snapshots/"+snap.ID+".json", snapData, 0600); err != nil {
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
	if _, err := client.PUTIfAbsent("salt.bin", localSalt); err != nil {
		if err == webdav.ErrConflict {
			remoteSalt, _, readErr := client.GET("salt.bin")
			if readErr == nil && bytes.Equal(remoteSalt, localSalt) {
				return nil
			}
			return fmt.Errorf("远程 salt 与本地不一致，请检查 WebDAV 根路径")
		}
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

func buildConfigStatus(cfg *config.Config) ConfigStatus {
	status := ConfigStatus{
		WebDAVConfigured: strings.TrimSpace(cfg.WebDAV.URL) != "" && strings.TrimSpace(cfg.WebDAV.Username) != "",
		ClaudeDirExists:  dirExists(config.ClaudeDir()),
	}
	_, err := config.LoadWebDAVPassword()
	status.PasswordAvailable = err == nil
	status.OK = status.WebDAVConfigured && status.PasswordAvailable && status.ClaudeDirExists
	switch {
	case status.OK:
		status.Message = "配置正常"
	case !status.WebDAVConfigured:
		status.Message = "WebDAV 未配置"
	case !status.PasswordAvailable:
		status.Message = "加密密码不可用"
	case !status.ClaudeDirExists:
		status.Message = "Claude 配置目录不存在"
	}
	return status
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (a *App) collectClaudeBinaryInfo(client *webdav.Client) ClaudeBinaryInfo {
	platform := config.Platform()
	info := ClaudeBinaryInfo{
		Platform:      platform,
		PlatformLabel: platformLabel(platform),
		Status:        "missing_remote",
		StatusLabel:   "当前平台暂无云端版本",
	}

	resolution := binary.ResolveClaudeBinaryCached()
	if resolution.Valid {
		info.Installed = true
		info.LocalVersion = resolution.Version
	}

	return a.collectClaudeBinaryRemoteInfo(info, client)
}

func (a *App) collectClaudeBinaryRemoteInfo(info ClaudeBinaryInfo, client *webdav.Client) ClaudeBinaryInfo {
	platform := info.Platform
	if platform == "" {
		platform = config.Platform()
		info.Platform = platform
	}
	if info.PlatformLabel == "" {
		info.PlatformLabel = platformLabel(platform)
	}

	if client != nil {
		if idx, err := a.loadBinaryIndexCached(client); err == nil {
			info.RemoteVersion = highestRemoteBinaryVersion(idx.GetBinaryInfo(platform, "claude"))
		}
	}

	info.Status, info.StatusLabel = claudeBinaryStatus(info.LocalVersion, info.RemoteVersion, info.Installed)
	return info
}

func platformLabel(platform string) string {
	switch platform {
	case "windows-amd64":
		return "Windows"
	case "darwin-arm64":
		return "Mac M 系列"
	case "linux-amd64":
		return "Linux"
	default:
		return platform
	}
}

func highestRemoteBinaryVersion(info *binary.BinaryInfo) string {
	if info == nil {
		return ""
	}
	highest := strings.TrimSpace(info.Current)
	for version := range info.Versions {
		version = strings.TrimSpace(version)
		if version == "" {
			continue
		}
		if highest == "" {
			highest = version
			continue
		}
		if cmp, ok := compareVersion(version, highest); ok && cmp > 0 {
			highest = version
		}
	}
	return highest
}

func claudeBinaryStatus(localVersion, remoteVersion string, installed bool) (string, string) {
	if !installed {
		return "missing_local", "未检测到本地版本"
	}
	if remoteVersion == "" {
		return "missing_remote", "当前平台暂无云端版本"
	}
	cmp, ok := compareVersion(remoteVersion, localVersion)
	if !ok {
		if remoteVersion == localVersion {
			return "latest", "已是最新"
		}
		return "unknown", "版本需确认"
	}
	if cmp > 0 {
		return "update_available", "可更新"
	}
	if cmp < 0 {
		return "ahead", "本地版本高于云端"
	}
	return "latest", "已是最新"
}

func currentBinaryVersions() map[string]map[string]string {
	return binary.CurrentClaudeVersionMap()
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

// detectUncommittedChanges 轻量级检测本地是否有未提交的变更
// 比较文件 metadata（size + modtime）与快照，不计算 hash
func detectUncommittedChanges(excludePatterns []string, localSnap *snapshot.Snapshot) bool {
	root := config.ClaudeDir()
	snapFiles := localSnap.Files
	seen := make(map[string]bool)
	changed := false

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || changed {
			return nil
		}
		if info.IsDir() {
			relPath := normalize.RelativePath(root, path)
			if relPath == "." {
				return nil
			}
			if isDashboardExcluded(relPath, true, excludePatterns) {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		relPath := normalize.RelativePath(root, path)
		if isDashboardExcluded(relPath, false, excludePatterns) {
			return nil
		}

		seen[relPath] = true
		entry, exists := snapFiles[relPath]
		if !exists || info.Size() != entry.Size || !info.ModTime().UTC().Equal(entry.Modified) {
			changed = true
		}
		return nil
	})

	if changed {
		return true
	}

	// 检查外部 ~/.claude.json
	jsonPath := config.ClaudeJSONPath()
	if info, err := os.Stat(jsonPath); err == nil && info.Mode().IsRegular() {
		seen[".claude.json"] = true
		if entry, exists := snapFiles[".claude.json"]; !exists {
			return true
		} else if info.Size() != entry.Size || !info.ModTime().UTC().Equal(entry.Modified) {
			return true
		}
	}

	// 检查快照中有但本地已删除的文件
	for path := range snapFiles {
		if !seen[path] {
			return true
		}
	}

	return false
}

// isDashboardExcluded 与 files.go 中的 isFileTreeExcluded 逻辑一致
func isDashboardExcluded(relPath string, isDir bool, patterns []string) bool {
	for _, p := range patterns {
		if strings.HasSuffix(p, "/") {
			dirName := strings.TrimSuffix(p, "/")
			for _, part := range strings.Split(relPath, "/") {
				if matchDashboardGlob(part, dirName) {
					return true
				}
			}
			continue
		}
		if strings.Contains(p, "*") {
			if matchDashboardGlob(filepath.Base(relPath), p) {
				return true
			}
			continue
		}
		if relPath == p || strings.HasPrefix(relPath, p+"/") {
			return true
		}
	}
	return false
}

func matchDashboardGlob(name, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(name, pattern[1:])
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(name, pattern[:len(pattern)-1])
	}
	return name == pattern
}
