package binary

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/cc-box/cli/internal/config"
)

func withResolveTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func TestParseVersionOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{name: "plain", output: "Claude Code 1.2.3\n", want: "1.2.3"},
		{name: "v prefix", output: "claude-code v1.2.3\n", want: "1.2.3"},
		{name: "no version", output: "claude-code dev\n", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseVersionOutput(tt.output); got != tt.want {
				t.Fatalf("ParseVersionOutput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExpandBinaryPathSupportsPercentEnv(t *testing.T) {
	t.Setenv("CC_BOX_TEST_BIN", filepath.Join("tmp", "cc-box-bin"))
	got := expandBinaryPath("%CC_BOX_TEST_BIN%/claude")
	wantSuffix := filepath.Join("tmp", "cc-box-bin", "claude")
	if !strings.HasSuffix(got, wantSuffix) {
		t.Fatalf("expandBinaryPath() = %q, want suffix %q", got, wantSuffix)
	}
}

func TestResolveClaudeManagedPathUsesConfiguredBinary(t *testing.T) {
	home := withResolveTempHome(t)
	configured := filepath.Join(home, "tools", managedBinaryName("claude"))
	if err := os.MkdirAll(filepath.Dir(configured), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configured, []byte{'M', 'Z', 0, 1, 2, 3}, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.Binary.BinDir = filepath.Join(home, "managed")
	cfg.Binary.ClaudePath = configured
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	if got := ResolveClaudeManagedPath(); got != configured {
		t.Fatalf("ResolveClaudeManagedPath() = %q, want %q", got, configured)
	}
}

func TestResolveClaudeManagedPathDoesNotUseScriptShim(t *testing.T) {
	home := withResolveTempHome(t)
	cfg := config.DefaultConfig()
	cfg.Binary.BinDir = filepath.Join(home, "managed")
	cfg.Binary.ClaudePath = filepath.Join(home, "npm", "claude.cmd")
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(config.LocalBinDir(), managedBinaryName("claude"))
	if got := ResolveClaudeManagedPath(); got != want {
		t.Fatalf("ResolveClaudeManagedPath() = %q, want %q", got, want)
	}
}

func TestDefaultClaudeManagedPathAvoidsExistingShim(t *testing.T) {
	home := withResolveTempHome(t)
	cfg := config.DefaultConfig()
	cfg.Binary.BinDir = filepath.Join(home, "managed")
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(config.LocalBinDir(), 0755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(config.LocalBinDir(), managedBinaryName("claude"))
	if err := os.WriteFile(shim, []byte("#!/usr/bin/env node\nrequire('./cli.js')\n"), 0755); err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(config.CCBoxDir(), "bin", managedBinaryName("claude"))
	if got := ResolveClaudeManagedPath(); got != want {
		t.Fatalf("ResolveClaudeManagedPath() = %q, want %q", got, want)
	}
}

func TestIsScriptShimDetectsTextLaunchers(t *testing.T) {
	dir := t.TempDir()
	launcher := filepath.Join(dir, "claude")
	if err := os.WriteFile(launcher, []byte("#!/usr/bin/env node\nrequire('./cli.js')\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if !isScriptShim(launcher) {
		t.Fatal("expected shebang launcher to be detected as shim")
	}
}

func TestIsScriptShimAllowsBinaryMagic(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, managedBinaryName("claude"))
	if err := os.WriteFile(bin, []byte{'M', 'Z', 0, 1, 2, 3}, 0755); err != nil {
		t.Fatal(err)
	}
	if isScriptShim(bin) {
		t.Fatal("expected binary magic file not to be detected as shim")
	}
}

func TestClearClaudeResolutionCacheIgnoresMissingFile(t *testing.T) {
	withResolveTempHome(t)
	if err := ClearClaudeResolutionCache(); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
