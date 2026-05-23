//go:build linux

package desktop

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeExecutable(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
}

func TestOpenLinuxPathFallsBackToGio(t *testing.T) {
	dir := t.TempDir()
	writeExecutable(t, dir, "xdg-open")
	writeExecutable(t, dir, "gio")
	t.Setenv("PATH", dir)

	oldRunner := runLinuxOpenCommand
	t.Cleanup(func() { runLinuxOpenCommand = oldRunner })
	var calls []string
	runLinuxOpenCommand = func(cmdPath string, args ...string) (string, error) {
		calls = append(calls, filepath.Base(cmdPath)+" "+strings.Join(args, " "))
		if filepath.Base(cmdPath) == "xdg-open" {
			return "xdg failed", errors.New("exit status 1")
		}
		return "", nil
	}

	if err := openLinuxPath("/tmp/demo"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0] != "xdg-open /tmp/demo" || calls[1] != "gio open /tmp/demo" {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestOpenLinuxPathReturnsFailures(t *testing.T) {
	dir := t.TempDir()
	writeExecutable(t, dir, "xdg-open")
	writeExecutable(t, dir, "gio")
	t.Setenv("PATH", dir)

	oldRunner := runLinuxOpenCommand
	t.Cleanup(func() { runLinuxOpenCommand = oldRunner })
	runLinuxOpenCommand = func(cmdPath string, args ...string) (string, error) {
		return filepath.Base(cmdPath) + " failed", errors.New("exit status 1")
	}

	err := openLinuxPath("/tmp/demo")
	if err == nil {
		t.Fatal("openLinuxPath should return an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "xdg-open failed") || !strings.Contains(msg, "gio failed") {
		t.Fatalf("error = %q", msg)
	}
}
