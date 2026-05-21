// 二进制上传下载单元测试
// 测试四种组合模式 + 分块判断逻辑
package binary

import (
	"bytes"
	"testing"
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
