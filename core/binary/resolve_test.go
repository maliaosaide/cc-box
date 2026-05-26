package binary

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/user/cc-box/core/config"
)

func withResolveTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(ClaudePathEnv, "")
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

func TestResolveClaudeManagedPathUsesConfiguredPathEvenWhenMissing(t *testing.T) {
	home := withResolveTempHome(t)
	configured := filepath.Join(home, "npm", "claude.cmd")
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

func TestDefaultClaudeManagedPathUsesLocalBinEvenWhenExistingFileIsShim(t *testing.T) {
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

	want := filepath.Join(config.LocalBinDir(), managedBinaryName("claude"))
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

func TestResolveClaudeBinaryCachedUsesStoredVersionWithoutExecutingShim(t *testing.T) {
	home := withResolveTempHome(t)
	dir := filepath.Join(home, "npm")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "executed")
	launcherName := "claude"
	script := []byte("#!/bin/sh\n: > '" + strings.ReplaceAll(marker, "'", "'\\''") + "'\necho Claude Code 0.0.0\n")
	if runtime.GOOS == "windows" {
		launcherName = "claude.cmd"
		script = []byte("@echo off\r\ntype nul > \"" + marker + "\"\r\necho Claude Code 0.0.0\r\n")
	}
	launcher := filepath.Join(dir, launcherName)
	if err := os.WriteFile(launcher, script, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.Binary.BinDir = filepath.Join(home, "managed")
	cfg.Binary.ClaudePath = launcher
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	managed := ResolveClaudeManagedPath()
	if err := saveClaudeCache(ClaudeResolution{CurrentPath: launcher, ManagedPath: managed, Version: "9.9.9", Valid: true, ReadOnly: true, IsShim: true}, "configured"); err != nil {
		t.Fatal(err)
	}

	got := ResolveClaudeBinaryCached()
	if !got.Valid || got.Version != "9.9.9" || !got.IsShim || got.Stale {
		t.Fatalf("ResolveClaudeBinaryCached() = %+v, want cached shim version", got)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("cached resolve executed shim; marker stat err = %v", err)
	}
}

func TestResolveClaudeBinaryFastMarksExpiredCacheStale(t *testing.T) {
	home := withResolveTempHome(t)
	bin := filepath.Join(home, "bin", managedBinaryName("claude"))
	if err := os.MkdirAll(filepath.Dir(bin), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte{'M', 'Z', 0, 1, 2, 3}, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.Binary.BinDir = filepath.Join(home, "managed")
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	managed := ResolveClaudeManagedPath()
	if err := saveClaudeCache(ClaudeResolution{CurrentPath: bin, ManagedPath: managed, Version: "1.2.3", Valid: true}, "bin_dir"); err != nil {
		t.Fatal(err)
	}

	got := resolveClaudeBinaryFast(0)
	if !got.Valid || got.Version != "1.2.3" || !got.Stale {
		t.Fatalf("resolveClaudeBinaryFast(0) = %+v, want stale cached version", got)
	}
}

func TestPlatformExecutableAndClaudeCandidates(t *testing.T) {
	name := executableName("claude")
	candidates := claudeCandidateNames()
	if runtime.GOOS == "windows" {
		if name != "claude.exe" {
			t.Fatalf("executableName = %q, want claude.exe", name)
		}
		for _, want := range []string{"claude.exe", "claude.cmd", "claude.bat", "claude.ps1"} {
			if !containsString(candidates, want) {
				t.Fatalf("claudeCandidateNames missing %q in %v", want, candidates)
			}
		}
		return
	}
	if name != "claude" {
		t.Fatalf("executableName = %q, want claude", name)
	}
	if len(candidates) != 1 || candidates[0] != "claude" {
		t.Fatalf("claudeCandidateNames = %v, want [claude]", candidates)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
