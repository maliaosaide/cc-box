package snapshot

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestScannerIncludesPreviouslyHardcodedAndLargeFiles(t *testing.T) {
	root := t.TempDir()
	files := map[string][]byte{
		".credentials.json":    []byte("credentials"),
		"settings.local.json":  []byte("local settings"),
		"stats-cache.json":     []byte("stats"),
		"large.bin":            make([]byte, 50*1024*1024+1),
		"nested/settings.json": []byte("settings"),
	}
	for relPath, data := range files {
		path := filepath.Join(root, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}

	result, err := NewScanner(root, nil).Scan()
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	for relPath := range files {
		if _, ok := result.Files[relPath]; !ok {
			t.Fatalf("%s was not scanned", relPath)
		}
	}
}

func TestScannerRejectsCaseInsensitivePathCollision(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 文件系统通常不能同时创建仅大小写不同的同目录文件")
	}
	root := t.TempDir()
	upper := filepath.Join(root, "Settings.JSON")
	lower := filepath.Join(root, "settings.json")
	if err := os.WriteFile(upper, []byte("upper"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lower, []byte("lower"), 0600); err != nil {
		t.Fatal(err)
	}
	upperInfo, err := os.Stat(upper)
	if err != nil {
		t.Fatal(err)
	}
	lowerInfo, err := os.Stat(lower)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(upperInfo, lowerInfo) {
		t.Skip("当前文件系统大小写不敏感，无法创建大小写碰撞样本")
	}

	result, err := NewScanner(root, nil).ScanPartial()
	if err != nil {
		t.Fatalf("ScanPartial returned error: %v", err)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("failures = %d, want 1", len(result.Failures))
	}
	if !strings.Contains(result.Failures[0].Error, "路径大小写冲突") {
		t.Fatalf("failure error = %q, want case collision", result.Failures[0].Error)
	}
	if _, err := NewScanner(root, nil).Scan(); err == nil {
		t.Fatal("Scan returned nil error for case-insensitive path collision")
	}
}

func TestScannerRejectsSymlinkTargets(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	result, err := NewScanner(root, nil).ScanPartial()
	if err != nil {
		t.Fatalf("ScanPartial returned error: %v", err)
	}
	if _, ok := result.Files["link.txt"]; ok {
		t.Fatal("symlink should not be scanned as a file")
	}
	if len(result.Failures) != 1 {
		t.Fatalf("failures = %d, want 1", len(result.Failures))
	}
	if !strings.Contains(result.Failures[0].Error, "符号链接") {
		t.Fatalf("failure error = %q, want symlink rejection", result.Failures[0].Error)
	}
}

func TestScannerReportsUnreadableSpecialFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no portable FIFO for this test")
	}
	root := t.TempDir()
	fifo := filepath.Join(root, "pipe")
	if err := syscallMkfifo(fifo); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}

	result, err := NewScanner(root, nil).ScanPartial()
	if err != nil {
		t.Fatalf("ScanPartial returned error: %v", err)
	}
	if len(result.Failures) != 1 {
		t.Fatalf("failures = %d, want 1", len(result.Failures))
	}
	if result.Failures[0].Path != "pipe" {
		t.Fatalf("failure path = %q, want pipe", result.Failures[0].Path)
	}
	if _, err := NewScanner(root, nil).Scan(); err == nil {
		t.Fatal("Scan returned nil error for failed file")
	}
}
