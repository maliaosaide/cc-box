// 系统托盘
// 托盘图标状态管理 + 右键菜单
package main

import (
	"embed"
	"sync"
	"sync/atomic"

	"github.com/user/cc-box/gui/internal/desktop"
)

//go:embed icon_synced.ico icon_pending.ico icon_conflict.ico icon_syncing.ico
var trayIcons embed.FS

// TrayState 托盘同步状态
type TrayState = desktop.TrayState

const (
	TraySynced   = desktop.TraySynced
	TrayPending  = desktop.TrayPending
	TrayConflict = desktop.TrayConflict
	TraySyncing  = desktop.TraySyncing
)

var (
	shouldQuit atomic.Bool
	trayMu     sync.RWMutex
	tray       desktop.TrayAdapter
)

var stateIconFiles = map[TrayState]string{
	TraySynced:   "icon_synced.ico",
	TrayPending:  "icon_pending.ico",
	TrayConflict: "icon_conflict.ico",
	TraySyncing:  "icon_syncing.ico",
}

var stateLabels = map[TrayState]string{
	TraySynced:   "已同步",
	TrayPending:  "待同步",
	TrayConflict: "冲突或连接错误",
	TraySyncing:  "同步中",
}

// StartTray 启动系统托盘
func StartTray(app *App) {
	adapter := desktop.NewTrayAdapter(loadTrayIcons(), stateLabels)
	_ = adapter.Start(desktop.TrayActions{
		OnPush: func() {
			app.QuickPush()
		},
		OnPull: func() {
			app.QuickPull()
		},
		OnSync: func() {
			app.QuickSync()
		},
		OnOpen: func() {
			app.showWindow()
		},
		OnQuit: func() {
			RequestQuit()
			app.quitApp()
		},
	})
	trayMu.Lock()
	tray = adapter
	trayMu.Unlock()
}

func StopTray() {
	trayMu.Lock()
	adapter := tray
	tray = nil
	trayMu.Unlock()
	if adapter != nil {
		adapter.Stop()
	}
}

// UpdateTrayState 更新托盘图标状态
func UpdateTrayState(state TrayState) {
	trayMu.RLock()
	adapter := tray
	trayMu.RUnlock()
	if adapter != nil {
		adapter.SetState(state)
	}
}

// RequestQuit 标记下一次关闭为真正退出。
func RequestQuit() {
	shouldQuit.Store(true)
}

// ShouldQuit 关闭窗口时判断是否真正退出
func ShouldQuit() bool {
	return shouldQuit.Load()
}

func IsTrayReady() bool {
	trayMu.RLock()
	adapter := tray
	trayMu.RUnlock()
	return adapter != nil && adapter.IsReady()
}

func loadTrayIcons() map[TrayState][]byte {
	icons := make(map[TrayState][]byte, len(stateIconFiles))
	for state, file := range stateIconFiles {
		data, err := trayIcons.ReadFile(file)
		if err == nil {
			icons[state] = data
		}
	}
	return icons
}
