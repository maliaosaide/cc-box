package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func withInput(t *testing.T, input string) {
	t.Helper()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = r.Close()
	})
}

func TestRunResolveNestedConflictWritesTargetFile(t *testing.T) {
	home := withTempHome(t)
	conflictPath := filepath.Join(home, ".cc-box", "conflicts", "nested")
	if err := os.MkdirAll(conflictPath, 0700); err != nil {
		t.Fatal(err)
	}
	localFile := filepath.Join(conflictPath, "file.json.local")
	remoteFile := filepath.Join(conflictPath, "file.json.remote")
	if err := os.WriteFile(localFile, []byte("local"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(remoteFile, []byte("remote"), 0600); err != nil {
		t.Fatal(err)
	}

	withInput(t, "2\n")
	if err := runResolve(nil, []string{"nested/file.json"}); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(home, ".claude", "nested", "file.json")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "remote" {
		t.Fatalf("resolved content = %q, want remote", string(data))
	}
	if _, err := os.Stat(localFile); !os.IsNotExist(err) {
		t.Fatalf("local conflict file still exists: %v", err)
	}
	if _, err := os.Stat(remoteFile); !os.IsNotExist(err) {
		t.Fatalf("remote conflict file still exists: %v", err)
	}
}

func TestRunResolveRejectsTraversal(t *testing.T) {
	withTempHome(t)
	if err := runResolve(nil, []string{"../escape"}); err == nil {
		t.Fatal("runResolve should reject traversal path")
	}
}
