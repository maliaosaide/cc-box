//go:build !darwin || cgo

package desktop

import (
	"sync/atomic"

	"fyne.io/systray"
)

type SystrayAdapter struct {
	icons   map[TrayState][]byte
	labels  map[TrayState]string
	actions TrayActions

	mPush *systray.MenuItem
	mPull *systray.MenuItem
	mSync *systray.MenuItem
	mOpen *systray.MenuItem
	mQuit *systray.MenuItem

	ready atomic.Bool
	stop  func()
}

func NewTrayAdapter(icons map[TrayState][]byte, labels map[TrayState]string) TrayAdapter {
	return &SystrayAdapter{
		icons:  icons,
		labels: labels,
	}
}

func (t *SystrayAdapter) Start(actions TrayActions) error {
	t.actions = actions
	t.stop = startSystray(t.onReady, t.onExit)
	return nil
}

func (t *SystrayAdapter) Stop() {
	if t.stop != nil {
		t.stop()
		t.stop = nil
	}
}

func (t *SystrayAdapter) SetState(state TrayState) {
	if !t.ready.Load() {
		return
	}
	if iconData, ok := t.icons[state]; ok {
		systray.SetIcon(iconData)
	}
	if label, ok := t.labels[state]; ok {
		systray.SetTooltip("CC-Box - " + label)
	}
	if t.mPush == nil || t.mPull == nil || t.mSync == nil {
		return
	}
	if state == TraySyncing {
		t.mPush.Disable()
		t.mPull.Disable()
		t.mSync.Disable()
		return
	}
	t.mPush.Enable()
	t.mPull.Enable()
	t.mSync.Enable()
}

func (t *SystrayAdapter) IsReady() bool {
	return t.ready.Load()
}

func (t *SystrayAdapter) onReady() {
	systray.SetOnTapped(func() {
		if t.actions.OnOpen != nil {
			t.actions.OnOpen()
		}
	})

	t.mPush = systray.AddMenuItem("↑ 推送配置", "推送本地变更到云端")
	t.mPull = systray.AddMenuItem("↓ 拉取配置", "拉取远程变更到本地")
	t.mSync = systray.AddMenuItem("⟷ 同步", "拉取并推送")
	systray.AddSeparator()
	t.mOpen = systray.AddMenuItem("打开主窗口", "显示 CC-Box 主界面")
	systray.AddSeparator()
	t.mQuit = systray.AddMenuItem("退出", "关闭 CC-Box")

	go t.menuLoop()
	t.ready.Store(true)
	t.SetState(TraySynced)
}

func (t *SystrayAdapter) onExit() {
	t.ready.Store(false)
}

func (t *SystrayAdapter) menuLoop() {
	for {
		select {
		case <-t.mPush.ClickedCh:
			if t.actions.OnPush != nil {
				t.actions.OnPush()
			}
		case <-t.mPull.ClickedCh:
			if t.actions.OnPull != nil {
				t.actions.OnPull()
			}
		case <-t.mSync.ClickedCh:
			if t.actions.OnSync != nil {
				t.actions.OnSync()
			}
		case <-t.mOpen.ClickedCh:
			if t.actions.OnOpen != nil {
				t.actions.OnOpen()
			}
		case <-t.mQuit.ClickedCh:
			systray.Quit()
			if t.actions.OnQuit != nil {
				t.actions.OnQuit()
			}
			return
		}
	}
}
