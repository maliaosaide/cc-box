// Dashboard 后端绑定
// 概览页数据获取 + 快捷操作
package main

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/user/cc-box/internal/binary"
	"github.com/user/cc-box/internal/config"
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

	// TODO: 从 internal/binary 和 internal/snapshot 获取真实数据
	// 占位数据，Phase 3c 对接后替换
	binPath := binary.GetBinaryPath("claude")
	data, _ = fillBinaryVersion(data, binPath)
	data.Binaries = []BinaryInfo{
		{Name: "uv", Version: "-", Latest: true},
	}
	data.Backups = []BackupInfo{
		{ID: "latest", Message: "auto sync", Device: cfg.Device.Name, Time: "刚刚"},
	}
	data.RecentChanges = []ChangeInfo{
		{Status: "M", Path: "settings.json", Time: "10 分钟前"},
		{Status: "A", Path: "skills/new-skill/", Time: "1 小时前"},
		{Status: "M", Path: "CLAUDE.md", Time: "3 小时前"},
	}

	_ = time.Now()
	return data, nil
}

// QuickPush 快捷推送，返回 opId
func (a *App) QuickPush() int64 {
	return a.StartAsync("quick-push", func(ctx context.Context, opID int64) error {
		a.emitProgress(opID, "quick-push", 0, 1, 0, 1, "正在扫描变更...")
		time.Sleep(500 * time.Millisecond)
		a.emitProgress(opID, "quick-push", 1, 1, 1, 1, "推送完成")
		return nil
	})
}

// QuickPull 快捷拉取，返回 opId
func (a *App) QuickPull() int64 {
	return a.StartAsync("quick-pull", func(ctx context.Context, opID int64) error {
		a.emitProgress(opID, "quick-pull", 0, 1, 0, 1, "正在拉取...")
		time.Sleep(500 * time.Millisecond)
		a.emitProgress(opID, "quick-pull", 1, 1, 1, 1, "拉取完成")
		return nil
	})
}

// QuickSync 快捷同步（pull + push），返回 opId
func (a *App) QuickSync() int64 {
	return a.StartAsync("quick-sync", func(ctx context.Context, opID int64) error {
		a.emitProgress(opID, "quick-sync", 0, 2, 0, 2, "正在拉取...")
		time.Sleep(500 * time.Millisecond)
		a.emitProgress(opID, "quick-sync", 1, 2, 1, 2, "正在推送...")
		time.Sleep(500 * time.Millisecond)
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
