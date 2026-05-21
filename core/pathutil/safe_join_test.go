package pathutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeJoinAcceptsRelativePath(t *testing.T) {
	root := t.TempDir()
	got, err := SafeJoin(root, "dir/file.txt")
	if err != nil {
		t.Fatalf("SafeJoin returned error: %v", err)
	}
	want := filepath.Join(root, "dir", "file.txt")
	if got != want {
		t.Fatalf("SafeJoin = %q, want %q", got, want)
	}
}

func TestSafeJoinRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	cases := []string{
		"",
		"../outside",
		`..\\outside`,
		"/absolute",
		".",
		"..",
		"safe\x00bad",
	}
	for _, tc := range cases {
		t.Run(strings.ReplaceAll(tc, "\x00", "nul"), func(t *testing.T) {
			if _, err := SafeJoin(root, tc); err == nil {
				t.Fatalf("SafeJoin(%q) succeeded, want error", tc)
			}
		})
	}
}

func TestSafeJoinRejectsWindowsVolumePath(t *testing.T) {
	root := t.TempDir()
	if _, err := SafeJoin(root, `C:\\Users\\a`); err == nil {
		t.Fatalf("SafeJoin accepted Windows volume path")
	}
}

func TestSafeJoinRejectsSymlinkComponent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := SafeJoin(root, "link/file.txt"); err == nil {
		t.Fatalf("SafeJoin accepted symlink escape")
	}
}
