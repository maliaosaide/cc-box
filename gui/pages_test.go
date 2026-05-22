// GUI 页面后端测试
// 覆盖快照列表本地优先与本地快照读取逻辑
package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/snapshot"
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

func TestSnapshotDisplayBinaryVersionsOnlyIncludesCurrentPlatformClaude(t *testing.T) {
	platform := config.Platform()
	got := snapshotDisplayBinaryVersions(map[string]map[string]string{
		platform: {
			"claude": "2.1.126",
			"uv":     "0.11.7",
			"uvx":    "0.11.7",
		},
		"linux-amd64": {
			"claude": "1.0.0",
		},
	})

	if len(got) != 1 || got[platform]["claude"] != "2.1.126" || len(got[platform]) != 1 {
		t.Fatalf("unexpected display binary versions: %+v", got)
	}
}

func TestSnapshotDisplayBinaryVersionsReturnsNilWithoutCurrentClaude(t *testing.T) {
	platform := config.Platform()
	got := snapshotDisplayBinaryVersions(map[string]map[string]string{
		platform:      {"uv": "0.11.7"},
		"linux-amd64": {"claude": "1.0.0"},
	})
	if got != nil {
		t.Fatalf("expected nil display binary versions, got %+v", got)
	}
}

func TestGetClaudeExcludeFilesReturnsSettingsJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("create claude dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{}"), 0600); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Claude.Path = claudeDir
	if err := config.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	files, err := (&App{}).GetClaudeExcludeFiles()
	if err != nil {
		t.Fatalf("GetClaudeExcludeFiles returned error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %+v", files)
	}
	if files[0].Name != "settings.json" || files[0].Pattern != "settings.json" || files[0].Path != filepath.Join(claudeDir, "settings.json") || files[0].Excluded {
		t.Fatalf("unexpected settings.json item: %+v", files[0])
	}

	cfg.Exclude.Patterns = []string{"settings.json"}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("save excluded config: %v", err)
	}
	files, err = (&App{}).GetClaudeExcludeFiles()
	if err != nil {
		t.Fatalf("GetClaudeExcludeFiles returned error after exclude: %v", err)
	}
	if !files[0].Excluded {
		t.Fatalf("settings.json should be marked excluded: %+v", files[0])
	}
}

func TestGetClaudeDirectoriesReturnsTopLevelDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	claudeDir := filepath.Join(home, ".claude")
	for _, dir := range []string{"sessions", "projects", filepath.Join("plugins", "data")} {
		if err := os.MkdirAll(filepath.Join(claudeDir, dir), 0755); err != nil {
			t.Fatalf("create claude dir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{}"), 0600); err != nil {
		t.Fatalf("write claude file: %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Claude.Path = claudeDir
	cfg.Exclude.Patterns = []string{"sessions/", "plugins/data/", "*.lock"}
	if err := config.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	dirs, err := (&App{}).GetClaudeDirectories()
	if err != nil {
		t.Fatalf("GetClaudeDirectories returned error: %v", err)
	}
	if len(dirs) != 3 {
		t.Fatalf("expected 3 directories, got %+v", dirs)
	}
	if dirs[0].Name != "plugins" || dirs[1].Name != "projects" || dirs[2].Name != "sessions" {
		t.Fatalf("unexpected directory order: %+v", dirs)
	}
	if dirs[0].Excluded || dirs[1].Excluded || !dirs[2].Excluded {
		t.Fatalf("unexpected excluded flags: %+v", dirs)
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
