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

func TestLoadBinaryAutoUploadAsLegacySyncEnabled(t *testing.T) {
	home := withTempHome(t)
	cfgDir := filepath.Join(home, ".cc-box")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	configData := []byte(`
[webdav]
url = "https://example.test/dav/"
username = "u"

[binary]
auto_upload = true
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), configData, 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Binary.SyncEnabled || !cfg.Binary.AutoUpload {
		t.Fatalf("binary sync flags = sync:%v auto:%v, want both true", cfg.Binary.SyncEnabled, cfg.Binary.AutoUpload)
	}
}

func TestClaudeJSONPathDefaultAndCustom(t *testing.T) {
	home := withTempHome(t)
	if got, want := ClaudeJSONPath(), filepath.Join(home, ".claude.json"); got != want {
		t.Fatalf("ClaudeJSONPath default = %q, want %q", got, want)
	}

	cfgDir := filepath.Join(home, ".cc-box")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	configData := []byte(`
[claude]
json_path = "~/custom-claude.json"
`)
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), configData, 0600); err != nil {
		t.Fatal(err)
	}
	if got, want := ClaudeJSONPath(), filepath.Join(home, "custom-claude.json"); got != want {
		t.Fatalf("ClaudeJSONPath custom = %q, want %q", got, want)
	}
}

func TestDefaultConfigDoesNotExcludeDirectories(t *testing.T) {
	cfg := DefaultConfig()
	if len(cfg.Exclude.Patterns) != 0 {
		t.Fatalf("default exclude patterns = %+v, want none", cfg.Exclude.Patterns)
	}
}

func TestSaveWritesSnakeCaseKeys(t *testing.T) {
	home := withTempHome(t)
	cfg := DefaultConfig()
	cfg.WebDAV.URL = "https://example.test/dav/"
	cfg.WebDAV.Username = "u"
	cfg.Sync.AutoSyncInterval = "15m"
	cfg.Claude.JSONPath = "~/.claude-custom.json"
	cfg.Binary.BinDir = "~/bin-new"
	cfg.Binary.VersionsDir = "~/versions-new"
	cfg.Binary.ClaudePath = "~/bin-new/claude"

	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".cc-box", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{"json_path", "bin_dir", "versions_dir", "claude_path", "auto_sync_interval", "merge_retry_max", "chunk_size_mb", "sync_enabled", "auto_configure_path"} {
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

func TestNormalizeWebDAVRoot(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "spaces", in: "   ", want: ""},
		{name: "plain", in: "cc-box", want: "cc-box"},
		{name: "leading slash", in: "/cc-box", want: "cc-box"},
		{name: "wrapped slash", in: "/cc-box/", want: "cc-box"},
		{name: "duplicate slash", in: "//cc-box//snapshots//", want: "cc-box/snapshots"},
		{name: "segment spaces", in: " /cc-box / snapshots/ ", want: "cc-box/snapshots"},
		{name: "backslash", in: `cc-box\\snapshots`, want: "cc-box/snapshots"},
		{name: "unicode segment", in: "前端/cc-box", want: "前端/cc-box"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeWebDAVRoot(tt.in); got != tt.want {
				t.Fatalf("NormalizeWebDAVRoot(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestWebDAVBaseURLNormalizesRoot(t *testing.T) {
	tests := []struct {
		name string
		url  string
		root string
		want string
	}{
		{name: "empty root", url: "https://example.test/dav/", root: "", want: "https://example.test/dav/"},
		{name: "plain root", url: "https://example.test/dav", root: "cc-box", want: "https://example.test/dav/cc-box/"},
		{name: "wrapped slash root", url: "https://example.test/dav/", root: "/cc-box/", want: "https://example.test/dav/cc-box/"},
		{name: "nested root", url: "https://example.test/alist/webdav-test", root: "cc-box/snapshots", want: "https://example.test/alist/webdav-test/cc-box/snapshots/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WebDAVBaseURL(tt.url, tt.root); got != tt.want {
				t.Fatalf("WebDAVBaseURL(%q, %q) = %q, want %q", tt.url, tt.root, got, tt.want)
			}
		})
	}
}

func TestSaveAndLoadNormalizeWebDAVRoot(t *testing.T) {
	withTempHome(t)
	cfg := DefaultConfig()
	cfg.WebDAV.URL = "https://example.test/dav/"
	cfg.WebDAV.Root = "/cc-box/"

	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.WebDAV.Root != "cc-box" {
		t.Fatalf("loaded root = %q, want %q", loaded.WebDAV.Root, "cc-box")
	}
	if got, want := ConfiguredWebDAVURL(loaded), "https://example.test/dav/cc-box/"; got != want {
		t.Fatalf("ConfiguredWebDAVURL = %q, want %q", got, want)
	}
}

func TestWebDAVPasswordPrefersEnvironment(t *testing.T) {
	withTempHome(t)
	t.Setenv("CC_BOX_WEBDAV_PASSWORD", "from-env")
	if err := SaveWebDAVPassword("from-store"); err != nil {
		t.Fatal(err)
	}
	got, err := LoadWebDAVPassword()
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-env" {
		t.Fatalf("LoadWebDAVPassword = %q, want from-env", got)
	}
}

func TestWebDAVPasswordFileStoreFallback(t *testing.T) {
	home := withTempHome(t)
	t.Setenv("CC_BOX_WEBDAV_PASSWORD", "")
	if err := SaveWebDAVPassword("stored-pass"); err != nil {
		t.Fatal(err)
	}
	got, err := LoadWebDAVPassword()
	if err != nil {
		t.Fatal(err)
	}
	if got != "stored-pass" {
		t.Fatalf("LoadWebDAVPassword = %q, want stored-pass", got)
	}
	if _, err := os.Stat(filepath.Join(home, ".cc-box", "secrets.json")); err != nil {
		t.Fatalf("secrets.json missing: %v", err)
	}
}
