//go:build windows

package desktop

import (
	"runtime"

	"fyne.io/systray"
)

func startSystray(onReady, onExit func()) func() {
	go func() {
		runtime.LockOSThread()
		systray.Run(onReady, onExit)
	}()
	return systray.Quit
}
