// 二进制上传下载单元测试
// 测试四种组合模式 + 分块判断逻辑
package binary

import (
	"bytes"
	"testing"
	"time"

	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/webdav"
)

func TestShouldChunk(t *testing.T) {
	tests := []struct {
		name      string
		size      int64
		chunkMode string
		threshold int64
		want      bool
	}{
		{"auto小文件不切块", 100, "auto", 50 * 1024 * 1024, false},
		{"auto大文件切块", 100 * 1024 * 1024, "auto", 50 * 1024 * 1024, true},
		{"auto等于阈值不切块", 50 * 1024 * 1024, "auto", 50 * 1024 * 1024, false},
		{"always小文件也切块", 100, "always", 50 * 1024 * 1024, true},
		{"always大文件切块", 100 * 1024 * 1024, "always", 50 * 1024 * 1024, true},
		{"auto零阈值全部切块", 100, "auto", 0, true},
		{"never大文件也不切块", 100 * 1024 * 1024, "never", 50 * 1024 * 1024, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldChunk(tt.size, tt.chunkMode, tt.threshold)
			if got != tt.want {
				t.Errorf("ShouldChunk(%d, %q, %d) = %v, want %v", tt.size, tt.chunkMode, tt.threshold, got, tt.want)
			}
		})
	}
}

func TestSplitAndReassemble(t *testing.T) {
	data := bytes.Repeat([]byte("hello world, this is a test of chunking!"), 100000) // ~3.8MB
	chunkSize := 1024 * 1024                                                         // 1MB

	result := Split(data, chunkSize)

	if result.Manifest.TotalSize != int64(len(data)) {
		t.Errorf("TotalSize = %d, want %d", result.Manifest.TotalSize, int64(len(data)))
	}

	expectedParts := (len(data) + chunkSize - 1) / chunkSize
	if result.Manifest.TotalParts != expectedParts {
		t.Errorf("TotalParts = %d, want %d", result.Manifest.TotalParts, expectedParts)
	}

	// 验证分块能重新拼回原始数据
	var reassembled []byte
	for _, chunk := range result.Chunks {
		reassembled = append(reassembled, chunk...)
	}

	if !bytes.Equal(reassembled, data) {
		t.Error("reassembled data doesn't match original")
	}
}

func TestSplitSmallData(t *testing.T) {
	data := []byte("tiny")
	result := Split(data, 10*1024*1024)

	if result.Manifest.TotalParts != 1 {
		t.Errorf("TotalParts = %d, want 1", result.Manifest.TotalParts)
	}
	if len(result.Chunks) != 1 {
		t.Fatalf("len(Chunks) = %d, want 1", len(result.Chunks))
	}
	if !bytes.Equal(result.Chunks[0], data) {
		t.Error("single chunk doesn't match data")
	}
}

func TestChunkProgressDoesNotExceedTotal(t *testing.T) {
	configureBinaryTest(t, config.BinaryConfig{
		Encrypt:          true,
		ChunkMode:        "always",
		ChunkSizeMB:      1,
		ChunkThresholdMB: 1,
	})
	client, _ := newBinaryTestDAV(t)
	key := bytes.Repeat([]byte{0x58}, 32)
	data := bytes.Repeat([]byte("x"), 1024*1024+1)
	var lastTotal, lastUploaded int64

	if err := Upload(client, key, "claude", data, "progress-test", func(total, uploaded int64, _, _ int) {
		lastTotal = total
		lastUploaded = uploaded
	}); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if lastTotal != int64(len(data)) || lastUploaded != int64(len(data)) {
		t.Fatalf("progress total=%d uploaded=%d, want %d", lastTotal, lastUploaded, len(data))
	}
}

func TestBinaryVersionLockRejectsConcurrentHolder(t *testing.T) {
	client, _ := newBinaryTestDAV(t)
	platform := config.Platform()
	lockPath := "binaries/locks/version/" + encodeLockPart(platform) + "/" + encodeLockPart("claude") + "/" + encodeLockPart("1.0.0") + ".lock"
	if err := client.EnsureDir(lockPath); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	if _, err := client.PUTIfAbsent(lockPath, []byte("locked")); err != nil {
		t.Fatalf("PUTIfAbsent lock: %v", err)
	}
	if err := withBinaryVersionLock(client, platform, "claude", "1.0.0", func() error { return nil }); err == nil {
		t.Fatalf("second lock acquisition succeeded")
	}
	if err := client.DELETE(lockPath); err != nil {
		t.Fatalf("DELETE lock: %v", err)
	}
	if err := withBinaryVersionLock(client, platform, "claude", "1.0.0", func() error { return nil }); err != nil {
		t.Fatalf("lock after release: %v", err)
	}
}

func TestSaveIndexRejectsStaleCreate(t *testing.T) {
	client, _ := newBinaryTestDAV(t)
	rev, err := LoadIndexRevision(client)
	if err != nil {
		t.Fatalf("LoadIndexRevision: %v", err)
	}

	first := NewIndex()
	first.EnsureBinaryInfo(config.Platform(), "claude").Versions["1.0.0"] = Version{Hash: "sha256:first"}
	if err := SaveIndex(client, first, rev.ETag, rev.Exists); err != nil {
		t.Fatalf("SaveIndex first: %v", err)
	}

	stale := NewIndex()
	stale.EnsureBinaryInfo(config.Platform(), "claude").Versions["2.0.0"] = Version{Hash: "sha256:stale"}
	if err := SaveIndex(client, stale, rev.ETag, rev.Exists); err != webdav.ErrConflict {
		t.Fatalf("SaveIndex stale error = %v, want ErrConflict", err)
	}
}

func TestUpdateIndexRetriesAndMergesConcurrentChange(t *testing.T) {
	client, _ := newBinaryTestDAV(t)
	platform := config.Platform()
	injected := false

	if err := UpdateIndex(client, func(idx *Index) error {
		if !injected {
			injected = true
			other := NewIndex()
			other.EnsureBinaryInfo(platform, "claude").Versions["other"] = Version{Hash: "sha256:other"}
			rev, err := LoadIndexRevision(client)
			if err != nil {
				return err
			}
			if err := SaveIndex(client, other, rev.ETag, rev.Exists); err != nil {
				return err
			}
		}
		idx.EnsureBinaryInfo(platform, "claude").Versions["mine"] = Version{Hash: "sha256:mine"}
		return nil
	}); err != nil {
		t.Fatalf("UpdateIndex: %v", err)
	}

	idx, err := LoadIndex(client)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	versions := idx.GetBinaryInfo(platform, "claude").Versions
	if _, ok := versions["other"]; !ok {
		t.Fatalf("missing concurrently added version")
	}
	if _, ok := versions["mine"]; !ok {
		t.Fatalf("missing retried version")
	}
}

func TestDeleteRemoteVersionMovesCurrentToRemainingVersion(t *testing.T) {
	client, _ := newBinaryTestDAV(t)
	platform := config.Platform()
	idx := NewIndex()
	info := idx.EnsureBinaryInfo(platform, "claude")
	info.Current = "1.0.0"
	info.Versions["1.0.0"] = Version{Hash: "hash-old", Uploaded: time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)}
	info.Versions["2.0.0"] = Version{Hash: "hash-new", Uploaded: time.Date(2026, 5, 24, 11, 0, 0, 0, time.UTC)}
	if err := SaveIndex(client, idx, "", false); err != nil {
		t.Fatalf("SaveIndex: %v", err)
	}

	if err := DeleteRemoteVersion(client, nil, "claude", "1.0.0", platform); err != nil {
		t.Fatalf("DeleteRemoteVersion: %v", err)
	}
	updated, err := LoadIndex(client)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}
	updatedInfo := updated.GetBinaryInfo(platform, "claude")
	if updatedInfo.Current != "2.0.0" {
		t.Fatalf("Current = %q, want 2.0.0", updatedInfo.Current)
	}
	if _, exists := updatedInfo.Versions["1.0.0"]; exists {
		t.Fatalf("deleted version still exists: %+v", updatedInfo.Versions)
	}
}

func TestExtForEncrypted(t *testing.T) {
	if ext := extForEncrypted(true); ext != ".enc" {
		t.Errorf("encrypted ext = %q, want .enc", ext)
	}
	if ext := extForEncrypted(false); ext != ".bin" {
		t.Errorf("unencrypted ext = %q, want .bin", ext)
	}
}

func TestManifestSerialization(t *testing.T) {
	m := &Manifest{
		Hash:       "sha256:abc123",
		TotalParts: 5,
		PartHashes: []string{"h0", "h1", "h2", "h3", "h4"},
		TotalSize:  50000000,
	}

	data, err := SerializeManifest(m)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := DeserializeManifest(data)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.Hash != m.Hash {
		t.Errorf("Hash = %q, want %q", parsed.Hash, m.Hash)
	}
	if parsed.TotalParts != m.TotalParts {
		t.Errorf("TotalParts = %d, want %d", parsed.TotalParts, m.TotalParts)
	}
	if parsed.TotalSize != m.TotalSize {
		t.Errorf("TotalSize = %d, want %d", parsed.TotalSize, m.TotalSize)
	}
}

func TestSplitConsistency(t *testing.T) {
	data := bytes.Repeat([]byte("consistent hash test"), 50000)

	r1 := Split(data, 1024*1024)
	r2 := Split(data, 1024*1024)

	if r1.Manifest.Hash != r2.Manifest.Hash {
		t.Errorf("same data produced different hashes: %s vs %s", r1.Manifest.Hash, r2.Manifest.Hash)
	}

	// 不同数据应产生不同哈希
	data2 := bytes.Repeat([]byte("different hash test"), 50000)
	r3 := Split(data2, 1024*1024)

	if r1.Manifest.Hash == r3.Manifest.Hash {
		t.Error("different data produced same hash")
	}
}
