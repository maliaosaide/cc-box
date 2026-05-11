// GUI 页面后端测试
// 覆盖快照列表本地优先与本地快照读取逻辑
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/cc-box/internal/snapshot"
)

func TestGetSnapshotListPrefersLocalCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	ccBoxDir := filepath.Join(home, ".cc-box")
	if err := os.MkdirAll(filepath.Join(ccBoxDir, "snapshots"), 0755); err != nil {
		t.Fatalf("create cc-box dir: %v", err)
	}

	writeSnapshotFixture(t, ccBoxDir, "snap-1", "", "device-a", "first", time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC))
	writeSnapshotFixture(t, ccBoxDir, "snap-2", "snap-1", "device-a", "second", time.Date(2026, 5, 7, 11, 0, 0, 0, time.UTC))
	writeSnapshotFixture(t, ccBoxDir, "snap-3", "snap-2", "device-a", "third", time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC))

	if err := os.WriteFile(filepath.Join(ccBoxDir, "HEAD"), []byte("snap-3"), 0600); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}

	entries, err := (&App{}).GetSnapshotList(10)
	if err != nil {
		t.Fatalf("GetSnapshotList returned error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].ID != "snap-3" || entries[1].ID != "snap-2" || entries[2].ID != "snap-1" {
		t.Fatalf("unexpected entry order: %+v", entries)
	}
}

func TestGetLocalSnapshotList(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	ccBoxDir := filepath.Join(home, ".cc-box")
	if err := os.MkdirAll(filepath.Join(ccBoxDir, "snapshots"), 0755); err != nil {
		t.Fatalf("create cc-box dir: %v", err)
	}

	writeSnapshotFixture(t, ccBoxDir, "snap-1", "", "device-a", "first", time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC))
	writeSnapshotFixture(t, ccBoxDir, "snap-2", "snap-1", "device-a", "second", time.Date(2026, 5, 7, 11, 0, 0, 0, time.UTC))
	writeSnapshotFixture(t, ccBoxDir, "snap-3", "snap-2", "device-a", "third", time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC))

	if err := os.WriteFile(filepath.Join(ccBoxDir, "HEAD"), []byte("snap-3"), 0600); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}

	entries, err := (&App{}).GetLocalSnapshotList(2)
	if err != nil {
		t.Fatalf("GetLocalSnapshotList returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].ID != "snap-3" || entries[1].ID != "snap-2" {
		t.Fatalf("unexpected entry order: %+v", entries)
	}
}

func writeSnapshotFixture(t *testing.T, ccBoxDir, id, parent, device, message string, timestamp time.Time) {
	t.Helper()

	snap := &snapshot.Snapshot{
		ID:        id,
		Parent:    parent,
		Timestamp: timestamp,
		Device:    device,
		Message:   message,
		Files:     map[string]snapshot.FileEntry{},
	}
	data, err := snap.Serialize()
	if err != nil {
		t.Fatalf("serialize snapshot %s: %v", id, err)
	}
	if err := os.WriteFile(filepath.Join(ccBoxDir, "snapshots", id+".json"), data, 0600); err != nil {
		t.Fatalf("write snapshot %s: %v", id, err)
	}
}
