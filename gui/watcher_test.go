package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestWatcherRecordsFsnotifyErrors(t *testing.T) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer fsw.Close()

	trayStates := make(chan TrayState, 1)
	w := &Watcher{
		fsw: fsw,
		updateTray: func(state TrayState) {
			trayStates <- state
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.watchLoop(ctx)

	expected := errors.New("watcher failed")
	go func() { fsw.Errors <- expected }()

	select {
	case state := <-trayStates:
		if state != TrayConflict {
			t.Fatalf("tray state = %v, want %v", state, TrayConflict)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for tray state")
	}

	errs := w.WatchErrors()
	if len(errs) != 1 || errs[0].Error() != expected.Error() {
		t.Fatalf("watch errors = %+v", errs)
	}
}
