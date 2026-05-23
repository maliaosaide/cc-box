// GUI 入口
// Wails v2 应用启动，加载前端资源
package main

import (
	"embed"
	goruntime "runtime"

	"github.com/user/cc-box/core/config"
	wails "github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

const singleInstanceUniqueID = "b7c80d0f-7d2a-4f3c-8b60-64a0d6e96f6d"

func singleInstanceLock(app *App) *options.SingleInstanceLock {
	return &options.SingleInstanceLock{
		UniqueId:               singleInstanceUniqueID,
		OnSecondInstanceLaunch: app.onSecondInstanceLaunch,
	}
}

func (a *App) onSecondInstanceLaunch(_ options.SecondInstanceData) {
	a.showWindow()
}

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "CC-Box",
		Width:     1120,
		Height:    720,
		Menu:      buildAppMenu(app),
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:   &options.RGBA{R: 15, G: 15, B: 20, A: 1},
		OnStartup:          app.startup,
		OnShutdown:         app.shutdown,
		OnBeforeClose:      app.OnBeforeClose,
		SingleInstanceLock: singleInstanceLock(app),
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

func buildAppMenu(app *App) *menu.Menu {
	appMenu := menu.NewMenu()
	if goruntime.GOOS == "darwin" {
		appMenu.Append(menu.AppMenu())
	}

	fileMenu := appMenu.AddSubmenu("文件")
	fileMenu.AddText("打开主窗口", keys.CmdOrCtrl("o"), func(_ *menu.CallbackData) {
		app.showWindow()
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("退出", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		app.quitApp()
	})

	if goruntime.GOOS == "darwin" {
		appMenu.Append(menu.EditMenu())
	}
	return appMenu
}

func init() {
	_ = config.Platform()
}
