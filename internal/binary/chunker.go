// 大文件分块和合并
// 用于二进制文件的上传/下载
package binary

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const DefaultChunkSize = 10 * 1024 * 1024 // 10MB

// Manifest 分块清单
type Manifest struct {
	Hash       string   `json:"hash"`
	TotalParts int      `json:"total_parts"`
	PartHashes []string `json:"part_hashes"`
	TotalSize  int64    `json:"total_size"`
}

// ChunkResult 分块结果
type ChunkResult struct {
	Manifest *Manifest
	Chunks   [][]byte
}

// Split 将数据分块
func Split(data []byte, chunkSize int) *ChunkResult {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}

	totalSize := len(data)
	totalParts := (totalSize + chunkSize - 1) / chunkSize
	if totalParts == 0 {
		totalParts = 1
	}

	chunks := make([][]byte, totalParts)
	partHashes := make([]string, totalParts)

	for i := 0; i < totalParts; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > totalSize {
			end = totalSize
		}
		chunks[i] = data[start:end]

		h := sha256.Sum256(chunks[i])
		partHashes[i] = hex.EncodeToString(h[:])
	}

	// 整体哈希
	wholeHash := sha256.Sum256(data)

	manifest := &Manifest{
		Hash:       "sha256:" + hex.EncodeToString(wholeHash[:]),
		TotalParts: totalParts,
		PartHashes: partHashes,
		TotalSize:  int64(totalSize),
	}

	return &ChunkResult{Manifest: manifest, Chunks: chunks}
}

// ShouldChunk 判断是否需要分块
// chunkMode: "always" 始终分块, "auto" 超过阈值时分块
func ShouldChunk(size int64, chunkMode string, thresholdBytes int64) bool {
	if chunkMode == "always" {
		return true
	}
	// "auto" 模式
	return size > thresholdBytes
}

// SerializeManifest 序列化 manifest
func SerializeManifest(m *Manifest) ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// DeserializeManifest 反序列化 manifest
func DeserializeManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("解析 manifest 失败: %w", err)
	}
	return &m, nil
}
