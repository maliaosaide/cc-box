// 系统托盘
// 托盘图标状态管理 + 右键菜单 + 自动启动
package main

import (
	"embed"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"

	"fyne.io/systray"
)

//go:embed icon_synced.ico icon_pending.ico icon_conflict.ico icon_syncing.ico
var trayIcons embed.FS

// TrayState 托盘同步状态
type TrayState string

const (
	TraySynced   TrayState = "synced"
	TrayPending  TrayState = "pending"
	TrayConflict TrayState = "conflict"
	TraySyncing  TrayState = "syncing"
)

var (
	trayState  = TraySynced
	mPush      *systray.MenuItem
	mPull      *systray.MenuItem
	mSync      *systray.MenuItem
	mOpen      *systray.MenuItem
	mAutoStart *systray.MenuItem
	mQuit      *systray.MenuItem
	shouldQuit atomic.Bool
	appRef     *App
	trayReady  atomic.Bool
	stopTray   func()
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
	appRef = app
	start, stop := systray.RunWithExternalLoop(trayOnReady, trayOnExit)
	stopTray = stop
	start()
}

func StopTray() {
	if stopTray != nil {
		stopTray()
		stopTray = nil
	}
}

func trayOnReady() {
	systray.SetOnTapped(func() {
		if appRef != nil {
			appRef.showWindow()
		}
	})

	mPush = systray.AddMenuItem("↑ 推送配置", "推送本地变更到云端")
	mPull = systray.AddMenuItem("↓ 拉取配置", "拉取远程变更到本地")
	mSync = systray.AddMenuItem("⟷ 同步", "拉取并推送")
	systray.AddSeparator()
	mOpen = systray.AddMenuItem("打开主窗口", "显示 CC-Box 主界面")
	mAutoStart = systray.AddMenuItemCheckbox("开机自启动", "系统启动时自动运行", isAutoStartEnabled())
	systray.AddSeparator()
	mQuit = systray.AddMenuItem("退出", "关闭 CC-Box")

	go trayMenuLoop()
	trayReady.Store(true)
	UpdateTrayState(TraySynced)
}

func trayOnExit() {
	trayReady.Store(false)
}

func trayMenuLoop() {
	for {
		select {
		case <-mPush.ClickedCh:
			appRef.QuickPush()
		case <-mPull.ClickedCh:
			appRef.QuickPull()
		case <-mSync.ClickedCh:
			appRef.QuickSync()
		case <-mOpen.ClickedCh:
			appRef.showWindow()
		case <-mAutoStart.ClickedCh:
			enable := !mAutoStart.Checked()
			if setAutoStart(enable) {
				if enable {
					mAutoStart.Check()
				} else {
					mAutoStart.Uncheck()
				}
			}
		case <-mQuit.ClickedCh:
			RequestQuit()
			systray.Quit()
			appRef.quitApp()
		}
	}
}

// UpdateTrayState 更新托盘图标状态
func UpdateTrayState(state TrayState) {
	if !trayReady.Load() {
		return
	}
	trayState = state
	iconData, _ := trayIcons.ReadFile(stateIconFiles[state])
	systray.SetIcon(iconData)
	systray.SetTooltip("CC-Box - " + stateLabels[state])
	if state != TraySyncing {
		mPush.Enable()
		mPull.Enable()
		mSync.Enable()
	} else {
		mPush.Disable()
		mPull.Disable()
		mSync.Disable()
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
	return trayReady.Load()
}

// Windows 开机自启动

func shortcutPath() string {
	startup := filepath.Join(os.Getenv("APPDATA"),
		"Microsoft", "Windows", "Start Menu", "Programs", "Startup")
	return filepath.Join(startup, "CC-Box.lnk")
}

func isAutoStartEnabled() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	_, err := os.Stat(shortcutPath())
	return err == nil
}

func setAutoStart(enable bool) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	path := shortcutPath()
	if !enable {
		os.Remove(path)
		return true
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	script := `$ws = New-Object -ComObject WScript.Shell; $s = $ws.CreateShortcut('` + powerShellSingleQuote(path) + `'); $s.TargetPath = '` + powerShellSingleQuote(exe) + `'; $s.Save()`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	hideCommandWindow(cmd)
	return cmd.Run() == nil
}

func powerShellSingleQuote(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
