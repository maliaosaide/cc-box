package main

import (
	"testing"

	"github.com/user/cc-box/core/webdav"
)

func TestAcquireRemoteInitLockPreventsConcurrentInit(t *testing.T) {
	server := newVirtualWebDAVServer(t)
	client := webdav.NewClient(server.server.URL, "", "")

	release, err := acquireRemoteInitLock(client, "device-a")
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}
	if _, err := acquireRemoteInitLock(client, "device-b"); err == nil {
		t.Fatalf("second lock acquisition succeeded")
	}
	release()
	if release, err := acquireRemoteInitLock(client, "device-b"); err != nil {
		t.Fatalf("acquire after release: %v", err)
	} else {
		release()
	}
}

func TestCleanupRemoteFilesRemovesPartialInitFiles(t *testing.T) {
	server := newVirtualWebDAVServer(t)
	client := webdav.NewClient(server.server.URL, "", "")

	if _, err := client.PUT("salt.bin", []byte("salt"), ""); err != nil {
		t.Fatalf("put salt: %v", err)
	}
	if _, err := client.PUT("snapshots/test.json.enc", []byte("snapshot"), ""); err != nil {
		t.Fatalf("put snapshot: %v", err)
	}
	cleanupRemoteFiles(client, []string{"salt.bin", "snapshots/test.json.enc"})
	if _, _, err := client.GET("salt.bin"); err != webdav.ErrNotFound {
		t.Fatalf("salt after cleanup error = %v, want ErrNotFound", err)
	}
	if _, _, err := client.GET("snapshots/test.json.enc"); err != webdav.ErrNotFound {
		t.Fatalf("snapshot after cleanup error = %v, want ErrNotFound", err)
	}
}
