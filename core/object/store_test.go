package object

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/webdav"
)

type objectTestDAV struct {
	server *httptest.Server
	mu     sync.Mutex
	files  map[string][]byte
}

func newObjectTestDAV(t *testing.T) (*webdav.Client, *objectTestDAV) {
	t.Helper()
	dav := &objectTestDAV{files: make(map[string][]byte)}
	dav.server = httptest.NewServer(http.HandlerFunc(dav.serveHTTP))
	t.Cleanup(dav.server.Close)
	return webdav.NewClient(dav.server.URL, "", ""), dav
}

func (d *objectTestDAV) serveHTTP(w http.ResponseWriter, r *http.Request) {
	remotePath := strings.TrimPrefix(path.Clean("/"+strings.TrimPrefix(r.URL.Path, "/")), "/")
	switch r.Method {
	case "MKCOL":
		w.WriteHeader(http.StatusCreated)
	case "PUT":
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		d.mu.Lock()
		d.files[remotePath] = append([]byte(nil), data...)
		d.mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	case "GET", "HEAD":
		d.mu.Lock()
		data, ok := d.files[remotePath]
		d.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("ETag", fmt.Sprintf("%q", remotePath))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		if r.Method == "GET" {
			_, _ = w.Write(data)
		}
	default:
		http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
	}
}

func (d *objectTestDAV) get(remotePath string) ([]byte, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	data, ok := d.files[remotePath]
	return append([]byte(nil), data...), ok
}

func (d *objectTestDAV) delete(remotePath string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.files, remotePath)
}

func configureObjectTest(t *testing.T, encryptionEnabled bool, chunkMode string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	cfg := config.DefaultConfig()
	cfg.Device.ID = "test-device"
	cfg.Encryption.Enabled = encryptionEnabled
	cfg.Binary.ChunkMode = chunkMode
	cfg.Binary.ChunkSizeMB = 1
	cfg.Binary.ChunkThresholdMB = 1
	if err := config.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
}

func TestChunkedObjectUploadDownloadRoundTripEncrypted(t *testing.T) {
	configureObjectTest(t, true, "always")
	client, dav := newObjectTestDAV(t)
	key := bytes.Repeat([]byte{0x37}, 32)
	data := bytes.Repeat([]byte("0123456789abcdef"), 80*1024)
	store := NewStore(client, key, "")

	hash, err := store.Upload(data)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if _, ok := dav.get(ObjectPath(hash)); ok {
		t.Fatalf("unexpected whole object upload")
	}
	if _, ok := dav.get(objectManifestPath(hash)); !ok {
		t.Fatalf("missing object manifest")
	}
	payload, ok := dav.get(objectPartPath(hash, 0, true))
	if !ok {
		t.Fatalf("missing encrypted object part")
	}
	if bytes.Equal(payload, data[:len(payload)]) {
		t.Fatalf("encrypted object part equals plaintext")
	}
	if _, ok := dav.get(objectPartPath(hash, 0, false)); ok {
		t.Fatalf("unexpected plaintext object part")
	}

	got, err := store.Download(hash)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("downloaded data mismatch")
	}
}

func TestChunkedObjectUploadDownloadRoundTripPlaintext(t *testing.T) {
	configureObjectTest(t, false, "always")
	client, dav := newObjectTestDAV(t)
	key := bytes.Repeat([]byte{0x24}, 32)
	data := bytes.Repeat([]byte("abcdefghij"), 140*1024)
	store := NewStore(client, key, "")

	hash, err := store.Upload(data)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	payload, ok := dav.get(objectPartPath(hash, 0, false))
	if !ok {
		t.Fatalf("missing plaintext object part")
	}
	if !bytes.Equal(payload, data[:len(payload)]) {
		t.Fatalf("plaintext object part does not match source data")
	}
	if _, ok := dav.get(objectPartPath(hash, 0, true)); ok {
		t.Fatalf("unexpected encrypted object part")
	}

	got, err := store.Download(hash)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("downloaded data mismatch")
	}
}

func TestChunkedObjectExistsRequiresAllParts(t *testing.T) {
	configureObjectTest(t, true, "always")
	client, dav := newObjectTestDAV(t)
	key := bytes.Repeat([]byte{0x52}, 32)
	data := bytes.Repeat([]byte("0123456789abcdef"), 80*1024)
	store := NewStore(client, key, "")

	hash, err := store.Upload(data)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	exists, err := store.Exists(hash)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Fatalf("chunked object should exist")
	}

	dav.delete(objectPartPath(hash, 0, true))
	exists, err = NewStore(client, key, "").Exists(hash)
	if err != nil {
		t.Fatalf("Exists after deleting part: %v", err)
	}
	if exists {
		t.Fatalf("chunked object with missing part should not exist")
	}
}
