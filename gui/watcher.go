// 文件监听与自动同步
// fsnotify 监听 ~/.claude/ 变更 + 定时同步
package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/user/cc-box/core/config"
)

const (
	debounceDelay  = 3 * time.Second
	minAutoSyncGap = 2 * time.Minute
)

// Watcher 文件变更监听器
type Watcher struct {
	fsw     *fsnotify.Watcher
	app     *App
	dir     string
	changed bool
	mu      sync.Mutex
	cancel  context.CancelFunc
	syncing bool
	stopped bool

	interval time.Duration
	lastSync time.Time
	timer    *time.Timer

	watchErrors []error
	updateTray  func(TrayState)
}

// NewWatcher 创建监听器
func NewWatcher(app *App) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		fsw:        fsw,
		app:        app,
		dir:        config.ClaudeDir(),
		interval:   autoSyncInterval(),
		updateTray: UpdateTrayState,
	}, nil
}

// Start 开始监听
func (w *Watcher) Start(ctx context.Context) {
	ctx, w.cancel = context.WithCancel(ctx)

	w.addWatch(w.dir)
	filepath.Walk(w.dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			w.addWatch(path)
		}
		return nil
	})

	go w.watchLoop(ctx)

	if w.interval > 0 {
		w.timer = time.AfterFunc(w.interval, func() { w.triggerAutoSync(ctx) })
	}
}

// Stop 停止监听
func (w *Watcher) Stop() {
	w.mu.Lock()
	w.stopped = true
	timer := w.timer
	cancel := w.cancel
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if timer != nil {
		timer.Stop()
	}
	w.fsw.Close()
}

func (w *Watcher) WatchErrors() []error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]error{}, w.watchErrors...)
}

func (w *Watcher) addWatch(path string) {
	if err := w.fsw.Add(path); err != nil {
		w.mu.Lock()
		w.watchErrors = append(w.watchErrors, err)
		w.mu.Unlock()
	}
}

func (w *Watcher) setTrayState(state TrayState) {
	if w.updateTray != nil {
		w.updateTray(state)
	}
}

func (w *Watcher) watchLoop(ctx context.Context) {
	debounce := time.NewTimer(debounceDelay)
	debounce.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) ||
				event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				if event.Has(fsnotify.Create) {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						w.addWatch(event.Name)
					}
				}
				w.mu.Lock()
				w.changed = true
				w.mu.Unlock()
				debounce.Reset(debounceDelay)
			}
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			if err != nil {
				w.mu.Lock()
				w.watchErrors = append(w.watchErrors, err)
				w.mu.Unlock()
				w.setTrayState(TrayConflict)
			}
		case <-debounce.C:
			w.mu.Lock()
			changed := w.changed
			w.changed = false
			w.mu.Unlock()
			if changed {
				w.setTrayState(TrayPending)
			}
		}
	}
}

// triggerAutoSync 定时自动同步
func (w *Watcher) triggerAutoSync(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	w.mu.Lock()
	if w.stopped || w.syncing || time.Since(w.lastSync) < minAutoSyncGap {
		shouldReset := !w.stopped
		w.mu.Unlock()
		if shouldReset {
			w.resetTimer()
		}
		return
	}
	w.syncing = true
	w.mu.Unlock()

	w.setTrayState(TraySyncing)
	opID := w.app.QuickSync()

	go func() {
		for {
			time.Sleep(300 * time.Millisecond)
			opCancelMu.Lock()
			_, running := opCancels[opID]
			opCancelMu.Unlock()
			if !running {
				break
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
		syncErr, _ := takeOpResult(opID)

		w.mu.Lock()
		w.syncing = false
		w.lastSync = time.Now()
		if syncErr == nil {
			w.changed = false
		}
		w.mu.Unlock()

		if syncErr != nil {
			w.setTrayState(TrayConflict)
			w.resetTimer()
			return
		}
		w.setTrayState(TraySynced)
		w.resetTimer()
	}()
}

func (w *Watcher) resetTimer() {
	if w.interval <= 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.stopped && w.timer != nil {
		w.timer.Reset(w.interval)
	}
}

// autoSyncInterval 从配置读取自动同步间隔
func autoSyncInterval() time.Duration {
	v := config.LoadRaw()
	s := v.GetString("sync.auto_sync_interval")
	switch s {
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "60m":
		return 60 * time.Minute
	default:
		return 0
	}
}
