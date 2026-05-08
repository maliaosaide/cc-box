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
	"github.com/user/cc-box/internal/config"
)

const (
	debounceDelay   = 3 * time.Second
	minAutoSyncGap  = 2 * time.Minute
)

// Watcher 文件变更监听器
type Watcher struct {
	fsw       *fsnotify.Watcher
	dir       string
	changed   bool
	mu        sync.Mutex
	cancel    context.CancelFunc
	syncing   bool

	// 自动同步
	interval   time.Duration
	lastSync   time.Time
	timer      *time.Timer
}

// NewWatcher 创建监听器
func NewWatcher() (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	dir := config.ClaudeDir()
	return &Watcher{
		fsw:     fsw,
		dir:     dir,
		interval: autoSyncInterval(),
	}, nil
}

// Start 开始监听
func (w *Watcher) Start(ctx context.Context) {
	ctx, w.cancel = context.WithCancel(ctx)

	// 监听主目录
	_ = w.fsw.Add(w.dir)

	// 监听子目录
	filepath.Walk(w.dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			_ = w.fsw.Add(path)
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
	if w.cancel != nil {
		w.cancel()
	}
	if w.timer != nil {
		w.timer.Stop()
	}
	w.fsw.Close()
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
				// 新建目录时加入监听
				if event.Has(fsnotify.Create) {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						_ = w.fsw.Add(event.Name)
					}
				}
				w.mu.Lock()
				w.changed = true
				w.mu.Unlock()
				debounce.Reset(debounceDelay)
			}
		case <-debounce.C:
			w.mu.Lock()
			if w.changed {
				w.changed = false
				UpdateTrayState(TrayPending)
			}
			w.mu.Unlock()
		}
	}
}

// triggerAutoSync 定时自动同步
func (w *Watcher) triggerAutoSync(ctx context.Context) {
	w.mu.Lock()
	if w.syncing {
		w.mu.Unlock()
		return
	}
	w.syncing = true
	w.mu.Unlock()

	UpdateTrayState(TraySyncing)
	opID := appRef.QuickSync()

	// 等待同步操作完成
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
		w.mu.Lock()
		w.syncing = false
		w.lastSync = time.Now()
		w.changed = false
		w.mu.Unlock()
		UpdateTrayState(TraySynced)
	}()

	// 重置定时器
	if w.interval > 0 {
		w.mu.Lock()
		if w.timer != nil {
			w.timer.Reset(w.interval)
		}
		w.mu.Unlock()
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
