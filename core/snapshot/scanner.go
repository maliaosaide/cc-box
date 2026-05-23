// 文件扫描器
// 扫描 ~/.claude/ 目录，应用排除规则，计算内容哈希
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
}

// NewScanner 创建扫描器
func NewScanner(root string, excludePatterns []string) *Scanner {
	return &Scanner{
		root:    root,
		exclude: excludePatterns,
	}
}

// ScanResult 扫描结果
type ScanResult struct {
	Files    map[string]FileEntry
	Stats    ScanStats
	Failures []ScanFailure
}

// ScanFailure 记录扫描中无法读取的文件或目录
type ScanFailure struct {
	Path     string `json:"path"`
	FullPath string `json:"fullPath"`
	Error    string `json:"error"`
}

func (r *ScanResult) HasFailures() bool {
	return r != nil && len(r.Failures) > 0
}

func (r *ScanResult) FailureError() error {
	if !r.HasFailures() {
		return nil
	}
	first := r.Failures[0]
	if len(r.Failures) == 1 {
		return fmt.Errorf("%s: %s", first.Path, first.Error)
	}
	return fmt.Errorf("%d 个文件扫描失败，首个失败: %s: %s", len(r.Failures), first.Path, first.Error)
}

func (r *ScanResult) addFailure(root, path string, err error) {
	if path == "" {
		path = root
	}
	relPath := normalize.RelativePath(root, path)
	if relPath == "." {
		relPath = ""
	}
	r.Failures = append(r.Failures, ScanFailure{Path: relPath, FullPath: path, Error: err.Error()})
}

// ScanStats 扫描统计
type ScanStats struct {
	TotalFiles  int
	TotalSize   int64
	Skipped     int
	SkippedSize int64
}

// Scan 扫描目录，返回文件条目映射；扫描失败会作为错误返回，避免静默漏同步
func (s *Scanner) Scan() (*ScanResult, error) {
	result, err := s.ScanPartial()
	if err != nil {
		return result, err
	}
	if err := result.FailureError(); err != nil {
		return result, err
	}
	return result, nil
}

// ScanPartial 扫描目录并保留失败列表，用于 GUI 展示失败文件
func (s *Scanner) ScanPartial() (*ScanResult, error) {
	result := &ScanResult{
		Files: make(map[string]FileEntry),
	}
	pathSources := make(map[string]string)

	err := filepath.Walk(s.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			result.addFailure(s.root, path, err)
			return nil
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

		fileInfo, ok, statErr := readableRegularFileInfo(path, info)
		if statErr != nil {
			result.Stats.Skipped++
			result.Stats.SkippedSize += info.Size()
			result.addFailure(s.root, path, statErr)
			return nil
		}
		if !ok {
			return nil
		}
		caseKey := strings.ToLower(relPath)
		if previousPath, exists := pathSources[caseKey]; exists {
			result.Stats.Skipped++
			result.Stats.SkippedSize += fileInfo.Size()
			result.addFailure(s.root, path, fmt.Errorf("路径大小写冲突: %s 与 %s 规范化为 %s", previousPath, path, caseKey))
			return nil
		}
		pathSources[caseKey] = path

		// 读取文件内容并计算哈希
		data, err := os.ReadFile(path)
		if err != nil {
			result.Stats.Skipped++
			result.Stats.SkippedSize += fileInfo.Size()
			result.addFailure(s.root, path, err)
			return nil
		}

		hash := object.ComputeHash(data)

		result.Files[relPath] = FileEntry{
			Hash:     hash,
			Size:     fileInfo.Size(),
			Modified: fileInfo.ModTime().UTC(),
		}

		result.Stats.TotalFiles++
		result.Stats.TotalSize += fileInfo.Size()

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("扫描目录失败: %w", err)
	}

	return result, nil
}

func readableRegularFileInfo(path string, info os.FileInfo) (os.FileInfo, bool, error) {
	if info.Mode().IsRegular() {
		return info, true, nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, fmt.Errorf("符号链接不同步")
	}
	return nil, false, fmt.Errorf("不是普通文件: %s", info.Mode().String())
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
