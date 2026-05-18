package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func TestLoadBinaryLegacyPathKeys(t *testing.T) {
	home := withTempHome(t)
	cfgDir := filepath.Join(home, ".cc-box")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	configData := []byte(`
[webdav]
url = "https://example.test/dav/"
username = "u"
root = "/cc-box/"

[device]
id = "dev"
name = "dev"

[binary]
bindir = "~/bin-old"
versionsdir = "~/versions-old"
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), configData, 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Binary.BinDir != "~/bin-old" {
		t.Fatalf("BinDir = %q", cfg.Binary.BinDir)
	}
	if cfg.Binary.VersionsDir != "~/versions-old" {
		t.Fatalf("VersionsDir = %q", cfg.Binary.VersionsDir)
	}
}

func TestSaveWritesSnakeCaseKeys(t *testing.T) {
	home := withTempHome(t)
	cfg := DefaultConfig()
	cfg.WebDAV.URL = "https://example.test/dav/"
	cfg.WebDAV.Username = "u"
	cfg.Sync.AutoSyncInterval = "15m"
	cfg.Binary.BinDir = "~/bin-new"
	cfg.Binary.VersionsDir = "~/versions-new"

	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".cc-box", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{"bin_dir", "versions_dir", "auto_sync_interval", "merge_retry_max", "chunk_size_mb"} {
		if !strings.Contains(content, want) {
			t.Fatalf("saved config missing %q:\n%s", want, content)
		}
	}
	for _, old := range []string{"bindir", "versionsdir", "autosyncinterval", "mergeretrymax", "chunksizemb"} {
		if strings.Contains(content, old) {
			t.Fatalf("saved config contains legacy key %q:\n%s", old, content)
		}
	}
}
