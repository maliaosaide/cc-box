// 文件扫描器
// 扫描 ~/.claude/ 目录，应用排除规则，计算规范化哈希
package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/cc-box/core/normalize"
	"github.com/user/cc-box/core/object"
)

// Scanner 文件扫描器
type Scanner struct {
	root    string
	exclude []string
	maxSize int64 // 单文件最大大小
}

// NewScanner 创建扫描器
func NewScanner(root string, excludePatterns []string) *Scanner {
	return &Scanner{
		root:    root,
		exclude: excludePatterns,
		maxSize: 50 * 1024 * 1024, // 50MB
	}
}

// ScanResult 扫描结果
type ScanResult struct {
	Files map[string]FileEntry
	Stats ScanStats
}

// ScanStats 扫描统计
type ScanStats struct {
	TotalFiles  int
	TotalSize   int64
	Skipped     int
	SkippedSize int64
}

// Scan 扫描目录，返回文件条目映射
func (s *Scanner) Scan() (*ScanResult, error) {
	result := &ScanResult{
		Files: make(map[string]FileEntry),
	}

	err := filepath.Walk(s.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过不可访问的文件
		}

		// 跳过目录本身
		if info.IsDir() {
			relPath := normalize.RelativePath(s.root, path)
			if s.isExcluded(relPath, true) {
				return filepath.SkipDir
			}
			return nil
		}

		relPath := normalize.RelativePath(s.root, path)

		// 排除规则
		if s.isExcluded(relPath, false) {
			result.Stats.Skipped++
			result.Stats.SkippedSize += info.Size()
			return nil
		}

		// 永远排除的文件
		if isAlwaysExcluded(relPath) {
			result.Stats.Skipped++
			return nil
		}

		// 大小限制
		if info.Size() > s.maxSize {
			result.Stats.Skipped++
			result.Stats.SkippedSize += info.Size()
			return nil
		}

		// 读取文件内容并计算哈希
		data, err := os.ReadFile(path)
		if err != nil {
			result.Stats.Skipped++
			return nil
		}

		// 规范化后计算哈希
		hashData := normalize.HashContent(data)
		hash := object.ComputeHash(hashData)

		result.Files[relPath] = FileEntry{
			Hash:     hash,
			Size:     info.Size(),
			Modified: info.ModTime().UTC(),
		}

		result.Stats.TotalFiles++
		result.Stats.TotalSize += info.Size()

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("扫描目录失败: %w", err)
	}

	return result, nil
}

// isExcluded 检查路径是否匹配排除规则
func (s *Scanner) isExcluded(relPath string, isDir bool) bool {
	for _, pattern := range s.exclude {
		if matchExclude(relPath, pattern, isDir) {
			return true
		}
	}
	return false
}

// matchExclude 匹配排除规则
// 支持目录后缀（如 "cache/"）和通配符（如 "*.lock"）
func matchExclude(relPath, pattern string, isDir bool) bool {
	// 目录后缀匹配
	if strings.HasSuffix(pattern, "/") {
		dirName := strings.TrimSuffix(pattern, "/")
		// 匹配路径中的任意目录段
		parts := strings.Split(relPath, "/")
		for _, part := range parts {
			if matchGlob(part, dirName) {
				return true
			}
		}
		return false
	}

	// 通配符匹配文件名
	if strings.Contains(pattern, "*") {
		fileName := filepath.Base(relPath)
		return matchGlob(fileName, pattern)
	}

	// 精确匹配
	return relPath == pattern || strings.HasPrefix(relPath, pattern+"/")
}

// matchGlob 简单通配符匹配
func matchGlob(name, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(name, pattern[1:])
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(name, pattern[:len(pattern)-1])
	}
	return name == pattern
}

// isAlwaysExcluded 永远排除的文件
func isAlwaysExcluded(relPath string) bool {
	alwaysExclude := []string{
		".credentials.json",
		"settings.local.json",
		"stats-cache.json",
	}
	for _, name := range alwaysExclude {
		if filepath.Base(relPath) == name {
			return true
		}
	}
	return false
}

// EmptySnapshot 创建空快照（用于首次 init）
func EmptySnapshot(deviceID, message string) *Snapshot {
	snap := New("", deviceID, message)
	snap.Files = make(map[string]FileEntry)
	return snap
}

// CreateSnapshot 从扫描结果创建快照
func CreateSnapshot(parent, deviceID, message string, files map[string]FileEntry) *Snapshot {
	snap := New(parent, deviceID, message)
	snap.Files = files
	return snap
}
