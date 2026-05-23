package binary

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/webdav"
)

type binaryTestDAV struct {
	server  *httptest.Server
	mu      sync.Mutex
	files   map[string][]byte
	etags   map[string]string
	counter int
}

func newBinaryTestDAV(t *testing.T) (*webdav.Client, *binaryTestDAV) {
	t.Helper()
	dav := &binaryTestDAV{files: make(map[string][]byte), etags: make(map[string]string)}
	dav.server = httptest.NewServer(http.HandlerFunc(dav.serveHTTP))
	t.Cleanup(dav.server.Close)
	return webdav.NewClient(dav.server.URL, "", ""), dav
}

func (d *binaryTestDAV) serveHTTP(w http.ResponseWriter, r *http.Request) {
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
		_, exists := d.files[remotePath]
		if r.Header.Get("If-None-Match") == "*" && exists {
			d.mu.Unlock()
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		if ifMatch := r.Header.Get("If-Match"); ifMatch != "" && (!exists || d.etags[remotePath] != ifMatch) {
			d.mu.Unlock()
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		d.files[remotePath] = append([]byte(nil), data...)
		etag := d.nextETag()
		d.etags[remotePath] = etag
		d.mu.Unlock()
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusCreated)
	case "GET", "HEAD":
		d.mu.Lock()
		data, ok := d.files[remotePath]
		etag := d.etags[remotePath]
		d.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		if r.Method == "GET" {
			_, _ = w.Write(data)
		}
	case "DELETE":
		d.mu.Lock()
		_, ok := d.files[remotePath]
		delete(d.files, remotePath)
		delete(d.etags, remotePath)
		d.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
	}
}

func (d *binaryTestDAV) get(remotePath string) ([]byte, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	data, ok := d.files[remotePath]
	return append([]byte(nil), data...), ok
}

func (d *binaryTestDAV) nextETag() string {
	d.counter++
	return fmt.Sprintf("%q", fmt.Sprintf("etag-%d", d.counter))
}

func configureBinaryTest(t *testing.T, binaryConfig config.BinaryConfig) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	cfg := config.DefaultConfig()
	cfg.Device.ID = "test-device"
	cfg.Binary = binaryConfig
	if err := config.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
}

func TestEncryptedWholeUploadDownloadRoundTrip(t *testing.T) {
	configureBinaryTest(t, config.BinaryConfig{
		Encrypt:          true,
		ChunkMode:        "never",
		ChunkSizeMB:      1,
		ChunkThresholdMB: 1,
	})
	client, dav := newBinaryTestDAV(t)
	key := bytes.Repeat([]byte{0x42}, 32)
	data := []byte("fake claude binary data")

	if err := Upload(client, key, "claude", data, "1.2.3", nil); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	idx, err := LoadIndex(client)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	version := idx.GetBinaryInfo(config.Platform(), "claude").Versions["1.2.3"]
	if !version.Encrypted || version.Chunked {
		t.Fatalf("version metadata = %+v, want encrypted whole upload", version)
	}

	remotePath := fmt.Sprintf("binaries/%s/claude-1.2.3.enc", config.Platform())
	payload, ok := dav.get(remotePath)
	if !ok {
		t.Fatalf("missing encrypted payload at %s", remotePath)
	}
	if bytes.Equal(payload, data) {
		t.Fatalf("remote encrypted payload equals plaintext")
	}
	if _, ok := dav.get(fmt.Sprintf("binaries/%s/claude-1.2.3.bin", config.Platform())); ok {
		t.Fatalf("unexpected plaintext .bin payload exists")
	}

	target := filepath.Join(t.TempDir(), "claude")
	if err := Download(client, key, "claude", "1.2.3", target, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("downloaded data mismatch")
	}

	wrongKey := bytes.Repeat([]byte{0x24}, 32)
	if err := Download(client, wrongKey, "claude", "1.2.3", filepath.Join(t.TempDir(), "wrong"), nil); err == nil {
		t.Fatalf("Download with wrong key succeeded")
	}
}

func TestEncryptedChunkedUploadDownloadRoundTrip(t *testing.T) {
	configureBinaryTest(t, config.BinaryConfig{
		Encrypt:          true,
		ChunkMode:        "always",
		ChunkSizeMB:      1,
		ChunkThresholdMB: 1,
	})
	client, dav := newBinaryTestDAV(t)
	key := bytes.Repeat([]byte{0x37}, 32)
	data := bytes.Repeat([]byte("0123456789abcdef"), 80*1024)

	if err := Upload(client, key, "claude", data, "2.0.0", nil); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	idx, err := LoadIndex(client)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	version := idx.GetBinaryInfo(config.Platform(), "claude").Versions["2.0.0"]
	if !version.Encrypted || !version.Chunked {
		t.Fatalf("version metadata = %+v, want encrypted chunked upload", version)
	}
	if _, ok := dav.get("binaries/parts/" + version.Hash + "/manifest.json"); !ok {
		t.Fatalf("missing chunk manifest")
	}
	partPath := "binaries/parts/" + version.Hash + "/part-000.enc"
	payload, ok := dav.get(partPath)
	if !ok {
		t.Fatalf("missing encrypted chunk at %s", partPath)
	}
	if bytes.Equal(payload, data[:len(payload)]) {
		t.Fatalf("remote encrypted chunk equals plaintext")
	}
	if _, ok := dav.get("binaries/parts/" + version.Hash + "/part-000.bin"); ok {
		t.Fatalf("unexpected plaintext chunk exists")
	}

	target := filepath.Join(t.TempDir(), "claude")
	if err := Download(client, key, "claude", "2.0.0", target, nil); err != nil {
		t.Fatalf("Download: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("downloaded data mismatch")
	}
}
