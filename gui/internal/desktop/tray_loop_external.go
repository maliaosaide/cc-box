//go:build !windows && (!darwin || cgo)

package desktop

import "fyne.io/systray"

func startSystray(onReady, onExit func()) func() {
	start, stop := systray.RunWithExternalLoop(onReady, onExit)
	start()
	return stop
}
