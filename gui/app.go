// Wails 应用核心
// App 结构体、生命周期管理、状态查询
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/user/cc-box/internal/config"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App Wails 绑定结构体
type App struct {
	ctx     context.Context
	watcher *Watcher
}

// NewApp 创建 App 实例
func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 启动系统托盘
	StartTray(a)

	// 启动文件监听
	if config.IsInitialized() {
		if w, err := NewWatcher(); err == nil {
			w.Start(ctx)
			a.watcher = w
		}
	}
}

func (a *App) shutdown(_ context.Context) {
	if a.watcher != nil {
		a.watcher.Stop()
	}
}

// OnBeforeClose 窗口关闭拦截：最小化到托盘而非退出
func (a *App) OnBeforeClose(ctx context.Context) bool {
	if ShouldQuit() {
		return false // 允许关闭
	}
	runtime.WindowHide(ctx)
	return true // 阻止关闭
}

func (a *App) showWindow() {
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

func (a *App) quitApp() {
	runtime.Quit(a.ctx)
}

// IsInitialized 检查是否已完成初始化
func (a *App) IsInitialized() bool {
	return config.IsInitialized()
}

// GetAppInfo 返回应用基本信息
func (a *App) GetAppInfo() map[string]interface{} {
	return map[string]interface{}{
		"name":     "CC-Box",
		"version":  "0.1.0",
		"platform": config.Platform(),
	}
}

// BrowseFolder 打开系统文件夹选择对话框
func (a *App) BrowseFolder(title string) (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: title,
	})
	if err != nil {
		return "", fmt.Errorf("选择目录失败: %w", err)
	}
	return dir, nil
}

// BrowseFile 打开系统文件选择对话框
func (a *App) BrowseFile(title string) (string, error) {
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: title,
	})
	if err != nil {
		return "", fmt.Errorf("选择文件失败: %w", err)
	}
	return file, nil
}

// OpenInExplorer 在系统文件管理器中打开路径
func (a *App) OpenInExplorer(path string) error {
	expanded := expandHome(path)
	_, err := os.Stat(expanded)
	if err != nil {
		return fmt.Errorf("路径不存在: %s", path)
	}
	runtime.BrowserOpenURL(a.ctx, "file:///"+filepath.ToSlash(expanded))
	return nil
}

func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
