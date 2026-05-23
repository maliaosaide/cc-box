package main

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/user/cc-box/gui/internal/desktop"
)

type recordingTray struct {
	mu     sync.Mutex
	states []TrayState
}

func (r *recordingTray) Start(desktop.TrayActions) error { return nil }
func (r *recordingTray) Stop()                           {}
func (r *recordingTray) IsReady() bool                   { return true }

func (r *recordingTray) SetState(state TrayState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states = append(r.states, state)
}

func (r *recordingTray) recordedStates() []TrayState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]TrayState(nil), r.states...)
}

func TestStartAsyncUpdatesTrayForSyncOperations(t *testing.T) {
	fakeTray := useRecordingTray(t)
	app := &App{}

	opID := app.StartAsync("quick-push", func(context.Context, int64) error {
		return nil
	})
	if err := waitAsyncResult(t, opID); err != nil {
		t.Fatalf("async op returned error: %v", err)
	}

	want := []TrayState{TraySyncing, TraySynced}
	if got := fakeTray.recordedStates(); !reflect.DeepEqual(got, want) {
		t.Fatalf("tray states = %v, want %v", got, want)
	}
}

func TestStartAsyncMarksTrayConflictOnSyncError(t *testing.T) {
	fakeTray := useRecordingTray(t)
	app := &App{}
	boom := errors.New("boom")

	opID := app.StartAsync("bulk-pull", func(context.Context, int64) error {
		return boom
	})
	if err := waitAsyncResult(t, opID); !errors.Is(err, boom) {
		t.Fatalf("async op error = %v, want %v", err, boom)
	}

	want := []TrayState{TraySyncing, TrayConflict}
	if got := fakeTray.recordedStates(); !reflect.DeepEqual(got, want) {
		t.Fatalf("tray states = %v, want %v", got, want)
	}
}

func TestStartAsyncLeavesTrayForNonSyncOperations(t *testing.T) {
	fakeTray := useRecordingTray(t)
	app := &App{}

	opID := app.StartAsync("binary-upload", func(context.Context, int64) error {
		return nil
	})
	if err := waitAsyncResult(t, opID); err != nil {
		t.Fatalf("async op returned error: %v", err)
	}

	if got := fakeTray.recordedStates(); len(got) != 0 {
		t.Fatalf("tray states = %v, want none", got)
	}
}

func TestStartAsyncKeepsTraySyncingDuringNestedSyncOperations(t *testing.T) {
	fakeTray := useRecordingTray(t)
	app := &App{}

	opID := app.StartAsync("quick-sync", func(context.Context, int64) error {
		childID := app.StartAsync("quick-pull", func(context.Context, int64) error {
			return nil
		})
		err, ok := waitAsyncResultFor(childID, time.Second)
		if !ok {
			return errors.New("timeout waiting for child operation")
		}
		return err
	})
	if err := waitAsyncResult(t, opID); err != nil {
		t.Fatalf("async op returned error: %v", err)
	}

	want := []TrayState{TraySyncing, TraySynced}
	if got := fakeTray.recordedStates(); !reflect.DeepEqual(got, want) {
		t.Fatalf("tray states = %v, want %v", got, want)
	}
}

func TestStartAsyncPrunesStoredResults(t *testing.T) {
	resetOpResultsForTest(t)
	app := &App{}

	for i := 0; i < maxStoredOpResults+5; i++ {
		opID := app.StartAsync("binary-upload", func(context.Context, int64) error { return nil })
		if err := waitAsyncResult(t, opID); err != nil {
			t.Fatalf("async op returned error: %v", err)
		}
	}

	opCancelMu.Lock()
	defer opCancelMu.Unlock()
	if len(opResults) > maxStoredOpResults {
		t.Fatalf("stored op results = %d, want <= %d", len(opResults), maxStoredOpResults)
	}
}

func TestTakeOpResultRemovesStoredResult(t *testing.T) {
	resetOpResultsForTest(t)
	app := &App{}
	opID := app.StartAsync("binary-upload", func(context.Context, int64) error { return nil })
	if err := waitAsyncResult(t, opID); err != nil {
		t.Fatalf("async op returned error: %v", err)
	}

	if _, ok := takeOpResult(opID); !ok {
		t.Fatalf("takeOpResult did not find op result")
	}
	opCancelMu.Lock()
	_, exists := opResults[opID]
	opCancelMu.Unlock()
	if exists {
		t.Fatalf("op result %d was not removed", opID)
	}
}

func useRecordingTray(t *testing.T) *recordingTray {
	t.Helper()
	fakeTray := &recordingTray{}

	trayMu.Lock()
	oldTray := tray
	tray = fakeTray
	trayMu.Unlock()

	trayOpMu.Lock()
	oldActiveOps := trayActiveOps
	oldHadError := trayHadError
	trayActiveOps = 0
	trayHadError = false
	trayOpMu.Unlock()

	t.Cleanup(func() {
		trayMu.Lock()
		tray = oldTray
		trayMu.Unlock()

		trayOpMu.Lock()
		trayActiveOps = oldActiveOps
		trayHadError = oldHadError
		trayOpMu.Unlock()
	})

	return fakeTray
}

func resetOpResultsForTest(t *testing.T) {
	t.Helper()
	opCancelMu.Lock()
	oldResults := opResults
	oldOrder := opResultOrder
	opResults = make(map[int64]error)
	opResultOrder = nil
	opCancelMu.Unlock()

	t.Cleanup(func() {
		opCancelMu.Lock()
		opResults = oldResults
		opResultOrder = oldOrder
		opCancelMu.Unlock()
	})
}

func waitAsyncResult(t *testing.T, opID int64) error {
	t.Helper()
	err, ok := waitAsyncResultFor(opID, 2*time.Second)
	if !ok {
		t.Fatalf("timeout waiting for async op %d", opID)
	}
	return err
}

func waitAsyncResultFor(opID int64, timeout time.Duration) (error, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		opCancelMu.Lock()
		_, running := opCancels[opID]
		err, done := opResults[opID]
		opCancelMu.Unlock()
		if done && !running {
			return err, true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil, false
}
