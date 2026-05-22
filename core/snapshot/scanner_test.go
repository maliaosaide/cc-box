package snapshot

import (
	"os"
	"path/filepath"
	"runtime"
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
