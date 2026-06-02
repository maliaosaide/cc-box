// 配置文件页面后端绑定
// 文件树、内容预览、diff、冲突解决、排除、批量同步
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/user/cc-box/core/binary"
	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/crypto"
	"github.com/user/cc-box/core/normalize"
	"github.com/user/cc-box/core/object"
	"github.com/user/cc-box/core/pathutil"
	"github.com/user/cc-box/core/snapshot"
	"github.com/user/cc-box/core/webdav"
)

// FileNode 文件树节点
type FileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	FullPath string      `json:"fullPath,omitempty"`
	IsDir    bool        `json:"isDir"`
	Status   string      `json:"status"` // synced, modified, added, deleted, conflict, failed
	Size     int64       `json:"size"`
	Modified string      `json:"modified"`
	Error    string      `json:"error,omitempty"`
	Children []*FileNode `json:"children,omitempty"`
	Expanded bool        `json:"expanded,omitempty"`
}

// FileDetail 文件详情
type FileDetail struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
	Status   string `json:"status"`
	Content  string `json:"content,omitempty"`
	Hash     string `json:"hash,omitempty"`
}

// DiffResult diff 结果
type DiffResult struct {
	Path      string     `json:"path"`
	Local     string     `json:"local,omitempty"`
	Remote    string     `json:"remote,omitempty"`
	Hunks     []DiffHunk `json:"hunks,omitempty"`
	Status    string     `json:"status"`
	LocalNew  bool       `json:"localNew,omitempty"`
	RemoteNew bool       `json:"remoteNew,omitempty"`
}

// DiffHunk 差异块
type DiffHunk struct {
	OldStart int      `json:"oldStart"`
	OldCount int      `json:"oldCount"`
	NewStart int      `json:"newStart"`
	NewCount int      `json:"newCount"`
	Lines    []string `json:"lines"` // " " / "+" / "-" 前缀
}

// ConflictDetail 冲突详情
type ConflictDetail struct {
	Path           string `json:"path"`
	Local          string `json:"local"`
	Remote         string `json:"remote"`
	LocalModified  string `json:"localModified"`
	RemoteModified string `json:"remoteModified"`
	Recommended    string `json:"recommended"`
	LocalExists    bool   `json:"localExists"`
	RemoteExists   bool   `json:"remoteExists"`
}

type conflictMetadata struct {
	LocalModified  time.Time `json:"local_modified"`
	RemoteModified time.Time `json:"remote_modified"`
	Recommended    string    `json:"recommended"`
	LocalExists    bool      `json:"local_exists"`
	RemoteExists   bool      `json:"remote_exists"`
}

type FileFailure struct {
	Path     string `json:"path"`
	FullPath string `json:"fullPath"`
	Error    string `json:"error"`
}

// FileTreeResult 文件树返回结果
type FileTreeResult struct {
	Root      *FileNode     `json:"root"`
	Total     int           `json:"total"`
	Changed   int           `json:"changed"`
	Conflicts int           `json:"conflicts"`
	Failed    int           `json:"failed"`
	Failures  []FileFailure `json:"failures,omitempty"`
	Checking  bool          `json:"checking,omitempty"`
}

// loadClients 加载配置、WebDAV 客户端、密钥
func (a *App) loadClients() (*config.Config, *webdav.Client, []byte, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("请先运行初始化: %w", err)
	}

	key, err := crypto.LoadKey(config.KeyPath())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("加载密钥失败")
	}

	pass, err := config.LoadWebDAVPassword()
	if err != nil {
		return nil, nil, nil, err
	}

	client := newConfiguredWebDAVClient(cfg, pass)
	return cfg, client, key, nil
}

func readObjectData(fullPath string) ([]byte, error) {
	return os.ReadFile(fullPath)
}

func safeJoin(root, relPath string) (string, error) {
	return pathutil.SafeJoin(root, relPath)
}

func safeClaudePath(relPath string) (string, error) {
	if relPath == ".claude.json" {
		return config.ClaudeJSONPath(), nil
	}
	return pathutil.SafeJoin(config.ClaudeDir(), relPath)
}

func claudeExtraFiles() []snapshot.ExtraFile {
	jsonPath := config.ClaudeJSONPath()
	if _, err := os.Stat(jsonPath); err == nil {
		return []snapshot.ExtraFile{{RelPath: ".claude.json", RealPath: jsonPath}}
	}
	return nil
}

func newClaudeScanner(excludePatterns []string) *snapshot.Scanner {
	s := snapshot.NewScanner(config.ClaudeDir(), excludePatterns)
	s.SetExtraFiles(claudeExtraFiles())
	return s
}

func addClaudeJSONToFiles(files map[string]snapshot.FileEntry) {
	jsonPath := config.ClaudeJSONPath()
	info, err := os.Stat(jsonPath)
	if err != nil {
		return
	}
	if !info.Mode().IsRegular() {
		return
	}
	if _, exists := files[".claude.json"]; exists {
		return
	}
	files[".claude.json"] = snapshot.FileEntry{
		Size:     info.Size(),
		Modified: info.ModTime().UTC(),
	}
}

func validateSnapshotID(id string) error {
	if id == "" || strings.ContainsRune(id, 0) || strings.Contains(id, "..") || strings.ContainsAny(id, `/\\`) {
		return fmt.Errorf("无效快照 ID: %s", id)
	}
	return nil
}

func remoteHeadETagForUpdate(client *webdav.Client, localHead string) (string, string, error) {
	if localHead != "" {
		if err := validateSnapshotID(localHead); err != nil {
			return "", "", err
		}
	}
	data, etag, err := client.GET("HEAD")
	if err == webdav.ErrNotFound {
		if localHead != "" {
			return "", "", fmt.Errorf("远程 HEAD 不存在，请先拉取或重新初始化")
		}
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("读取远程 HEAD 失败: %w", err)
	}
	remoteHead := strings.TrimSpace(string(data))
	if remoteHead != localHead {
		return remoteHead, etag, fmt.Errorf("远程已更新，请先拉取并解决冲突")
	}
	if remoteHead != "" && etag == "" {
		return remoteHead, "", fmt.Errorf("远程服务未返回 HEAD ETag，无法安全更新")
	}
	return remoteHead, etag, nil
}

// loadLocalSnap 加载本地 HEAD 指向的快照（先本地缓存，再远程）
func (a *App) loadLocalSnap(client *webdav.Client, key []byte) (*snapshot.Snapshot, error) {
	head, err := os.ReadFile(config.CCBoxDir() + "/HEAD")
	if err != nil {
		return nil, nil
	}
	headID := strings.TrimSpace(string(head))
	if headID == "" {
		return nil, nil
	}
	if err := validateSnapshotID(headID); err != nil {
		return nil, err
	}

	// 先尝试本地缓存
	snapDir := config.CCBoxDir() + "/snapshots/"
	data, err := os.ReadFile(snapDir + headID + ".json")
	if err == nil {
		return snapshot.Deserialize(data)
	}

	// 从远程下载
	snapPath := "snapshots/" + headID + ".json.enc"
	encrypted, _, err := client.GET(snapPath)
	if err != nil {
		return nil, err
	}
	decrypted, err := decryptRemoteData(encrypted, key)
	if err != nil {
		return nil, err
	}
	return snapshot.Deserialize(decrypted)
}

// loadRemoteSnap 加载远程 HEAD 指向的快照
func (a *App) loadRemoteSnap(client *webdav.Client, key []byte) (*snapshot.Snapshot, error) {
	data, _, err := client.GET("HEAD")
	if err != nil {
		return nil, err
	}
	head := strings.TrimSpace(string(data))
	if head == "" {
		return nil, nil
	}
	if err := validateSnapshotID(head); err != nil {
		return nil, err
	}

	// 先尝试本地缓存
	snapDir := config.CCBoxDir() + "/snapshots/"
	local, err := os.ReadFile(snapDir + head + ".json")
	if err == nil {
		snap, err := snapshot.Deserialize(local)
		if err != nil {
			return nil, err
		}
		if snap.ID != head {
			return nil, fmt.Errorf("本地缓存快照 ID 与远程 HEAD 不一致")
		}
		return snap, nil
	}

	snapPath := "snapshots/" + head + ".json.enc"
	encrypted, _, err := client.GET(snapPath)
	if err != nil {
		return nil, err
	}
	decrypted, err := decryptRemoteData(encrypted, key)
	if err != nil {
		return nil, err
	}
	snap, err := snapshot.Deserialize(decrypted)
	if err != nil {
		return nil, err
	}
	if snap.ID != head {
		return nil, fmt.Errorf("远程快照 ID 与 HEAD 不一致")
	}
	return snap, nil
}

// GetFileTreeLocal 返回不依赖远程请求和内容 hash 的本地文件树
func (a *App) GetFileTreeLocal() (*FileTreeResult, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	files, failures, err := scanFileTreeMetadata(config.ClaudeDir(), cfg.Exclude.Patterns)
	if err != nil {
		return nil, fmt.Errorf("扫描失败: %w", err)
	}
	addClaudeJSONToFiles(files)
	var localSnap *snapshot.Snapshot
	if headID, err := readLocalHeadID(); err == nil && headID != "" {
		localSnap, _ = a.loadLocalSnapByID(headID)
	}
	return buildFileTreePreviewResult(files, failures, localSnap), nil
}

func scanFileTreeMetadata(root string, exclude []string) (map[string]snapshot.FileEntry, []FileFailure, error) {
	files := make(map[string]snapshot.FileEntry)
	failures := []FileFailure{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			failures = append(failures, fileFailureFromPath(root, path, err))
			return nil
		}
		if info == nil {
			return nil
		}
		if path == root {
			return nil
		}
		relPath := normalize.RelativePath(root, path)
		if info.IsDir() {
			if isFileTreeExcluded(relPath, true, exclude) {
				return filepath.SkipDir
			}
			return nil
		}
		if isFileTreeExcluded(relPath, false, exclude) {
			return nil
		}
		fileInfo, ok, statErr := readableFileTreeInfo(path, info)
		if statErr != nil {
			failures = append(failures, fileFailureFromPath(root, path, statErr))
			return nil
		}
		if !ok {
			return nil
		}
		files[relPath] = snapshot.FileEntry{Size: fileInfo.Size(), Modified: fileInfo.ModTime().UTC()}
		return nil
	})
	return files, failures, err
}

func readableFileTreeInfo(path string, info os.FileInfo) (os.FileInfo, bool, error) {
	if info.Mode().IsRegular() {
		return info, true, nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		targetInfo, err := os.Stat(path)
		if err != nil {
			return nil, false, err
		}
		if targetInfo.Mode().IsRegular() {
			return targetInfo, true, nil
		}
		return nil, false, fmt.Errorf("符号链接目标不是普通文件: %s", targetInfo.Mode().String())
	}
	return nil, false, fmt.Errorf("不是普通文件: %s", info.Mode().String())
}

func fileFailureFromPath(root, path string, err error) FileFailure {
	if path == "" {
		path = root
	}
	relPath := normalize.RelativePath(root, path)
	if relPath == "." {
		relPath = ""
	}
	return FileFailure{Path: relPath, FullPath: path, Error: err.Error()}
}

func fileFailuresFromScan(failures []snapshot.ScanFailure) []FileFailure {
	result := make([]FileFailure, 0, len(failures))
	for _, failure := range failures {
		result = append(result, FileFailure{Path: failure.Path, FullPath: failure.FullPath, Error: failure.Error})
	}
	return result
}

func requireCompleteScan(scanResult *snapshot.ScanResult) error {
	if err := scanResult.FailureError(); err != nil {
		return fmt.Errorf("存在失败文件，请在配置文件页查看并修复后重试: %w", err)
	}
	return nil
}

func (a *App) ensureRemoteSnapshotObjects(ctx context.Context, opID int64, operation string, client *webdav.Client, key []byte, files map[string]snapshot.FileEntry) (int, error) {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	store := object.NewStore(client, key, config.CCBoxDir()+"/cache/objects")
	total := int64(len(paths))
	uploaded := 0
	for i, path := range paths {
		select {
		case <-ctx.Done():
			return uploaded, ctx.Err()
		default:
		}

		entry := files[path]
		exists, err := store.Exists(entry.Hash)
		if err != nil {
			return uploaded, fmt.Errorf("检查远程 object %s 失败: %w", path, err)
		}
		if exists {
			if opID != 0 {
				a.emitProgress(opID, operation, int64(i+1), total, i+1, int(total), "检查已有文件")
			}
			continue
		}

		fullPath, err := safeClaudePath(path)
		if err != nil {
			return uploaded, err
		}
		data, err := readObjectData(fullPath)
		if err != nil {
			return uploaded, fmt.Errorf("读取文件 %s 失败: %w", path, err)
		}
		if hash := object.ComputeHash(data); hash != entry.Hash {
			return uploaded, fmt.Errorf("文件 %s hash 不一致", path)
		}
		if hash, err := store.Upload(data); err != nil {
			return uploaded, fmt.Errorf("补传文件 %s 失败: %w", path, err)
		} else if hash != entry.Hash {
			return uploaded, fmt.Errorf("文件 %s hash 不一致", path)
		}
		uploaded++
		if opID != 0 {
			a.emitProgress(opID, operation, int64(i+1), total, i+1, int(total), fmt.Sprintf("补传 %s", path))
		}
	}
	return uploaded, nil
}

func isFileTreeExcluded(relPath string, isDir bool, patterns []string) bool {
	for _, pattern := range patterns {
		if matchFileTreeExclude(relPath, pattern, isDir) {
			return true
		}
	}
	return false
}

func matchFileTreeExclude(relPath, pattern string, isDir bool) bool {
	if strings.HasSuffix(pattern, "/") {
		dirName := strings.TrimSuffix(pattern, "/")
		parts := strings.Split(relPath, "/")
		for _, part := range parts {
			if matchFileTreeGlob(part, dirName) {
				return true
			}
		}
		return false
	}
	if strings.Contains(pattern, "*") {
		return matchFileTreeGlob(filepath.Base(relPath), pattern)
	}
	return relPath == pattern || strings.HasPrefix(relPath, pattern+"/")
}

func matchFileTreeGlob(name, pattern string) bool {
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

// GetFileTree 返回配置文件树及同步状态
func (a *App) GetFileTree() (*FileTreeResult, error) {
	cfg, client, key, err := a.loadClients()
	if err != nil {
		return nil, err
	}
	client.SetTimeout(8 * time.Second)

	scanner := newClaudeScanner(cfg.Exclude.Patterns)
	scanResult, err := scanner.ScanPartial()
	if err != nil {
		return nil, fmt.Errorf("扫描失败: %w", err)
	}

	localSnap, _ := a.loadLocalSnap(client, key)
	remoteHeadData, _, _ := client.GET("HEAD")
	remoteHeadStr := strings.TrimSpace(string(remoteHeadData))
	var remoteSnap *snapshot.Snapshot
	if remoteHeadStr != "" {
		remoteSnap, _ = a.loadRemoteSnapByID(client, key, remoteHeadStr)
	}

	return buildFileTreeResult(scanResult.Files, fileFailuresFromScan(scanResult.Failures), localSnap, remoteSnap, remoteHeadStr), nil
}

func buildFileTreePreviewResult(files map[string]snapshot.FileEntry, failures []FileFailure, localSnap *snapshot.Snapshot) *FileTreeResult {
	return buildFileTreeNodes(files, failures, localSnap, true, func(string) string { return "checking" })
}

func buildFileTreeResult(files map[string]snapshot.FileEntry, failures []FileFailure, localSnap, remoteSnap *snapshot.Snapshot, remoteHeadStr string) *FileTreeResult {
	localHeadStr, _ := readLocalHeadID()
	return buildFileTreeNodes(files, failures, localSnap, false, func(path string) string {
		return computeFileStatus(path, files, localSnap, remoteSnap, localHeadStr, remoteHeadStr)
	})
}

func buildFileTreeNodes(files map[string]snapshot.FileEntry, failures []FileFailure, localSnap *snapshot.Snapshot, checking bool, statusFor func(string) string) *FileTreeResult {
	statusMap := make(map[string]string)
	for path := range files {
		statusMap[path] = statusFor(path)
	}
	if localSnap != nil {
		for path := range localSnap.Files {
			if _, exists := files[path]; !exists {
				statusMap[path] = "deleted"
			}
		}
	}

	failureMap := make(map[string]FileFailure, len(failures))
	for _, failure := range failures {
		path := failure.Path
		if path == "" {
			path = ".claude"
			failure.Path = path
		}
		failureMap[path] = failure
		statusMap[path] = "failed"
	}

	conflictFiles := listConflicts()
	wrapperRoot := &FileNode{Name: "", Path: "", IsDir: true, Expanded: true}
	claudeDir := &FileNode{Name: ".claude", Path: "", IsDir: true, Expanded: true}
	wrapperRoot.Children = append(wrapperRoot.Children, claudeDir)
	changed := 0
	conflicts := 0
	failed := 0

	for path, status := range statusMap {
		if status != "failed" {
			if _, isConflict := conflictFiles[path]; isConflict {
				status = "conflict"
			}
		}
		if isChangedFileStatus(status) {
			changed++
		}
		if status == "conflict" {
			conflicts++
		}
		if status == "failed" {
			failed++
		}

		failure := failureMap[path]
		entry, hasEntry := files[path]
		if hasEntry {
			insertNodeWithMeta(claudeDir, path, status, entry.Size, entry.Modified, failure.Error, failure.FullPath)
			continue
		}
		if localSnap != nil {
			if old, ok := localSnap.Files[path]; ok {
				insertNodeWithMeta(claudeDir, path, status, old.Size, old.Modified, failure.Error, failure.FullPath)
				continue
			}
		}
		if status == "failed" {
			insertNodeWithMeta(claudeDir, path, status, 0, time.Time{}, failure.Error, failure.FullPath)
		}
	}

	// 从目录内移除 walk 找到的 .claude.json，统一作为同级外部文件展示
	removeChild(claudeDir, ".claude.json")
	if entry, ok := files[".claude.json"]; ok {
		jsonStatus := statusFor(".claude.json")
		if _, isConflict := conflictFiles[".claude.json"]; isConflict {
			jsonStatus = "conflict"
		}
		jsonNode := &FileNode{
			Name:     ".claude.json",
			Path:     ".claude.json",
			IsDir:    false,
			Status:   jsonStatus,
			Size:     entry.Size,
			Modified: formatTime(entry.Modified),
		}
		wrapperRoot.Children = append(wrapperRoot.Children, jsonNode)
		total := len(statusMap) + 1
		sortNodes(claudeDir)
		return &FileTreeResult{Root: wrapperRoot, Total: total, Changed: changed, Conflicts: conflicts, Failed: failed, Failures: failures, Checking: checking}
	}

	sortNodes(claudeDir)
	return &FileTreeResult{Root: wrapperRoot, Total: len(statusMap), Changed: changed, Conflicts: conflicts, Failed: failed, Failures: failures, Checking: checking}
}

func isChangedFileStatus(status string) bool {
	switch status {
	case "modified", "added", "deleted", "conflict":
		return true
	default:
		return false
	}
}

// computeFileStatus 计算单个文件的同步状态
func computeFileStatus(path string, current map[string]snapshot.FileEntry, localSnap, remoteSnap *snapshot.Snapshot, localHeadStr, remoteHeadStr string) string {
	cur, ok := current[path]
	if !ok {
		return "deleted"
	}

	// 没有快照 = 全是新文件
	if localSnap == nil && remoteSnap == nil {
		return "added"
	}

	// 与本地 HEAD 比较
	if localSnap != nil {
		if old, exists := localSnap.Files[path]; exists {
			if old.Hash != cur.Hash {
				return "modified"
			}
		} else {
			return "added"
		}
	}

	// 检查远程是否有更新
	if remoteSnap != nil && localSnap != nil {
		if localHeadStr != remoteHeadStr {
			if remoteEntry, exists := remoteSnap.Files[path]; exists {
				if localSnap != nil {
					if localEntry, ok := localSnap.Files[path]; ok {
						if localEntry.Hash != remoteEntry.Hash && localEntry.Hash != cur.Hash {
							return "conflict"
						}
						if localEntry.Hash != remoteEntry.Hash {
							return "modified" // 远程有更新待拉取
						}
					}
				}
			}
		}
	}

	return "synced"
}

// listConflicts 列出冲突目录中的文件
func listConflicts() map[string]bool {
	result := make(map[string]bool)
	dir := filepath.Join(config.CCBoxDir(), "conflicts")
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		base := strings.TrimSuffix(rel, ".local")
		base = strings.TrimSuffix(base, ".remote")
		if base != rel {
			result[base] = true
		}
		return nil
	})
	return result
}

// insertNode 向文件树插入路径节点
func insertNode(root *FileNode, relPath, status string, size int64, modified time.Time) {
	insertNodeWithMeta(root, relPath, status, size, modified, "", "")
}

func insertNodeWithMeta(root *FileNode, relPath, status string, size int64, modified time.Time, message, fullPath string) {
	parts := strings.Split(relPath, "/")
	current := root

	for i, part := range parts {
		isFile := (i == len(parts)-1)

		if isFile {
			current.Children = append(current.Children, &FileNode{
				Name:     part,
				Path:     relPath,
				FullPath: fullPath,
				IsDir:    false,
				Status:   status,
				Size:     size,
				Modified: formatTime(modified),
				Error:    message,
			})
		} else {
			found := findChild(current, part)
			if found == nil {
				dirPath := strings.Join(parts[:i+1], "/")
				found = &FileNode{
					Name:     part,
					Path:     dirPath,
					IsDir:    true,
					Status:   "synced",
					Children: []*FileNode{},
					Expanded: i == 0,
				}
				current.Children = append(current.Children, found)
			}
			current = found
		}
	}
}

func findChild(node *FileNode, name string) *FileNode {
	for _, child := range node.Children {
		if child.Name == name {
			return child
		}
	}
	return nil
}

func removeChild(node *FileNode, name string) {
	filtered := node.Children[:0]
	for _, child := range node.Children {
		if child.Name != name {
			filtered = append(filtered, child)
		}
	}
	node.Children = filtered
}

func sortNodes(node *FileNode) {
	if node.Children == nil {
		return
	}
	sort.SliceStable(node.Children, func(i, j int) bool {
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		return node.Children[i].Name < node.Children[j].Name
	})
	for _, child := range node.Children {
		sortNodes(child)
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "刚刚"
	case diff < time.Hour:
		return fmt.Sprintf("%d 分钟前", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%d 小时前", int(diff.Hours()))
	default:
		return t.Format("2006-01-02 15:04")
	}
}

// GetFileContent 读取文件内容
func (a *App) GetFileContent(relPath string) (*FileDetail, error) {
	_, err := config.Load()
	if err != nil {
		return nil, err
	}

	fullPath, err := safeClaudePath(relPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %s", relPath)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("读取失败: %w", err)
	}

	detail := &FileDetail{
		Path:     relPath,
		Size:     info.Size(),
		Modified: formatTime(info.ModTime()),
	}

	// 只对文本文件提供内容预览
	if isTextFile(relPath, data) {
		maxSize := 500 * 1024 // 500KB 预览上限
		if len(data) > maxSize {
			detail.Content = string(data[:maxSize]) + "\n... (已截断)"
		} else {
			detail.Content = string(data)
		}
	}

	return detail, nil
}

// GetFileDiff 返回本地与远程的 diff
func (a *App) GetFileDiff(relPath string) (*DiffResult, error) {
	_, client, key, err := a.loadClients()
	if err != nil {
		return nil, err
	}

	// 加载本地快照作为远程参照
	snap, _ := a.loadLocalSnap(client, key)
	if snap == nil {
		return nil, fmt.Errorf("没有快照数据，请先推送")
	}

	result := &DiffResult{Path: relPath}

	// 读取本地文件
	fullPath, err := safeClaudePath(relPath)
	if err != nil {
		return nil, err
	}
	localData, err := os.ReadFile(fullPath)
	if err != nil {
		// 本地文件不存在，可能已删除
		if entry, ok := snap.Files[relPath]; ok {
			store := object.NewStore(client, key, config.CCBoxDir()+"/cache/objects")
			remoteData, err := store.Download(entry.Hash)
			if err != nil {
				return nil, err
			}
			result.Remote = string(remoteData)
			result.Status = "deleted"
			result.RemoteNew = true
			return result, nil
		}
		return nil, fmt.Errorf("文件不存在")
	}
	result.Local = string(localData)

	// 查找远程版本
	entry, exists := snap.Files[relPath]
	if !exists {
		result.Status = "added"
		result.LocalNew = true
		return result, nil
	}

	store := object.NewStore(client, key, config.CCBoxDir()+"/cache/objects")
	remoteData, err := store.Download(entry.Hash)
	if err != nil {
		return nil, fmt.Errorf("下载远程版本失败: %w", err)
	}
	result.Remote = string(remoteData)

	// 比较
	if result.Local == result.Remote {
		result.Status = "synced"
		return result, nil
	}

	result.Status = "modified"
	result.Hunks = computeHunks(result.Remote, result.Local)
	return result, nil
}

// GetConflictDetail 返回冲突文件的三方信息
func (a *App) GetConflictDetail(relPath string) (*ConflictDetail, error) {
	localFile, remoteFile, metaFile, err := conflictFilePaths(relPath)
	if err != nil {
		return nil, err
	}

	localData, err := os.ReadFile(localFile)
	if err != nil {
		return nil, fmt.Errorf("本地冲突版本不存在")
	}
	remoteData, err := os.ReadFile(remoteFile)
	if err != nil {
		return nil, fmt.Errorf("远程冲突版本不存在")
	}

	meta := conflictMetadata{LocalExists: true, RemoteExists: true}
	if metaData, err := os.ReadFile(metaFile); err == nil {
		_ = json.Unmarshal(metaData, &meta)
	}

	return &ConflictDetail{
		Path:           relPath,
		Local:          string(localData),
		Remote:         string(remoteData),
		LocalModified:  formatConflictTime(meta.LocalModified),
		RemoteModified: formatConflictTime(meta.RemoteModified),
		Recommended:    meta.Recommended,
		LocalExists:    meta.LocalExists,
		RemoteExists:   meta.RemoteExists,
	}, nil
}

// ResolveConflict 解决冲突
func (a *App) ResolveConflict(relPath, choice string) error {
	localFile, remoteFile, metaFile, err := conflictFilePaths(relPath)
	if err != nil {
		return err
	}
	meta := conflictMetadata{LocalExists: true, RemoteExists: true}
	if metaData, err := os.ReadFile(metaFile); err == nil {
		_ = json.Unmarshal(metaData, &meta)
	}

	var data []byte
	shouldWrite := true
	switch choice {
	case "local":
		if !meta.LocalExists {
			shouldWrite = false
			break
		}
		d, err := os.ReadFile(localFile)
		if err != nil {
			return err
		}
		data = d
	case "remote":
		if !meta.RemoteExists {
			shouldWrite = false
			break
		}
		d, err := os.ReadFile(remoteFile)
		if err != nil {
			return err
		}
		data = d
	case "merged":
		return fmt.Errorf("合并模式暂未实现")
	default:
		return fmt.Errorf("无效选择: %s", choice)
	}

	targetPath, err := safeClaudePath(relPath)
	if err != nil {
		return err
	}
	if shouldWrite {
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
		if err := os.WriteFile(targetPath, data, 0600); err != nil {
			return fmt.Errorf("写入失败: %w", err)
		}
	} else if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除失败: %w", err)
	}

	removeConflictFiles(relPath)
	a.emitDataChanged("files", "resolve-conflict")
	return nil
}

// ExcludeFile 将文件/目录添加到排除规则
func (a *App) ExcludeFile(relPath string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pattern := relPath
	fullPath, pathErr := safeClaudePath(relPath)
	if pathErr != nil {
		return pathErr
	}
	info, err := os.Stat(fullPath)
	if err == nil && info.IsDir() {
		pattern = relPath + "/"
	}

	for _, p := range cfg.Exclude.Patterns {
		if p == pattern {
			return nil
		}
	}

	cfg.Exclude.Patterns = append(cfg.Exclude.Patterns, pattern)
	if err := config.Save(cfg); err != nil {
		return err
	}
	a.emitDataChanged("config", "exclude-file")
	return nil
}

// BulkSync 批量同步（push 或 pull），返回 opId
func (a *App) BulkSync(action string) int64 {
	return a.StartAsync("bulk-"+action, func(ctx context.Context, opID int64) error {
		cfg, client, key, err := a.loadClients()
		if err != nil {
			return err
		}

		if action == "push" {
			return a.doBulkPush(ctx, opID, cfg, client, key)
		}
		return a.doBulkPull(ctx, opID, cfg, client, key)
	})
}

func (a *App) doBulkPush(ctx context.Context, opID int64, cfg *config.Config, client *webdav.Client, key []byte) error {
	scanner := newClaudeScanner(cfg.Exclude.Patterns)
	scanResult, err := scanner.ScanPartial()
	if err != nil {
		return fmt.Errorf("扫描失败: %w", err)
	}
	if err := requireCompleteScan(scanResult); err != nil {
		return err
	}

	localHead, _ := os.ReadFile(config.CCBoxDir() + "/HEAD")
	localHeadStr := strings.TrimSpace(string(localHead))
	var localSnap *snapshot.Snapshot
	if localHeadStr != "" {
		localSnap, _ = a.loadLocalSnap(client, key)
	}

	// 计算变更
	var currentBins map[string]map[string]string
	var changes []snapshot.Change
	binaryChanged := false
	if cfg.Binary.SyncEnabled {
		version, err := binary.CurrentClaudeVersion()
		if err != nil {
			return err
		}
		currentBins = map[string]map[string]string{config.Platform(): {"claude": version}}
	}
	if localSnap != nil {
		currentSnap := snapshot.CreateSnapshot("", cfg.Device.ID, "", scanResult.Files)
		changes = localSnap.Diff(currentSnap)
		binaryChanged = cfg.Binary.SyncEnabled && !binaryVersionsEqual(localSnap.Binary, currentBins)
	} else {
		for path, entry := range scanResult.Files {
			changes = append(changes, snapshot.Change{Path: path, Type: snapshot.Added, NewHash: entry.Hash, NewSize: entry.Size})
		}
		binaryChanged = cfg.Binary.SyncEnabled
	}

	if len(changes) == 0 && !binaryChanged {
		repaired, err := a.ensureRemoteSnapshotObjects(ctx, opID, "bulk-push", client, key, scanResult.Files)
		if err != nil {
			return err
		}
		if repaired > 0 {
			a.emitProgress(opID, "bulk-push", 1, 1, 1, 1, fmt.Sprintf("已补传 %d 个缺失文件", repaired))
			return nil
		}
		a.emitProgress(opID, "bulk-push", 1, 1, 1, 1, "没有变更需要推送")
		return nil
	}

	_, headETag, err := remoteHeadETagForUpdate(client, localHeadStr)
	if err != nil {
		return err
	}

	total := int64(len(changes))
	store := object.NewStore(client, key, config.CCBoxDir()+"/cache/objects")

	for i, c := range changes {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if c.Type == snapshot.Deleted {
			continue
		}

		fullPath, err := safeClaudePath(c.Path)
		if err != nil {
			return err
		}
		data, err := readObjectData(fullPath)
		if err != nil {
			return fmt.Errorf("读取文件 %s 失败: %w", c.Path, err)
		}

		if hash, err := store.Upload(data); err != nil {
			return fmt.Errorf("上传文件 %s 失败: %w", c.Path, err)
		} else if hash != c.NewHash {
			return fmt.Errorf("文件 %s hash 不一致", c.Path)
		}
		a.emitProgress(opID, "bulk-push", int64(i+1), total, int(i+1), int(total), fmt.Sprintf("推送 %s", c.Path))
	}

	if cfg.Binary.SyncEnabled {
		version, uploadedBinary, err := binary.EnsureCurrentClaudeUploaded(client, key, a.progressCallback(opID, "bulk-push"))
		if err != nil {
			return err
		}
		currentBins = map[string]map[string]string{config.Platform(): {"claude": version}}
		if uploadedBinary {
			a.clearBinaryIndexCache()
		}
	}

	newSnap := snapshot.CreateSnapshot(localHeadStr, cfg.Device.ID, "gui push", scanResult.Files)
	newSnap.Binary = currentBins
	snapData, _ := newSnap.Serialize()
	encrypted, err := encryptRemoteData(snapData, key)
	if err != nil {
		return fmt.Errorf("encrypt snap: %w", err)
	}
	if err := client.EnsureDir("snapshots/"); err != nil {
		return fmt.Errorf("create snapshots dir: %w", err)
	}
	if _, err := client.PUT("snapshots/"+newSnap.ID+".json.enc", encrypted, ""); err != nil {
		return fmt.Errorf("upload snap: %w", err)
	}
	res, err := client.CompareAndSwapHEAD("HEAD", newSnap.ID, headETag)
	if err != nil {
		return fmt.Errorf("cas HEAD: %w", err)
	}
	if !res.Success {
		return fmt.Errorf("远程 HEAD 已变化，请先拉取")
	}
	if err := config.WriteFileEnsureDir(config.CCBoxDir()+"/HEAD", []byte(newSnap.ID), 0600); err != nil {
		return fmt.Errorf("更新本地 HEAD 失败: %w", err)
	}
	if err := config.WriteFileEnsureDir(config.CCBoxDir()+"/snapshots/"+newSnap.ID+".json", snapData, 0600); err != nil {
		return fmt.Errorf("缓存快照失败: %w", err)
	}
	return nil
}

func (a *App) doBulkPull(ctx context.Context, opID int64, cfg *config.Config, client *webdav.Client, key []byte) error {
	remoteHeadData, _, err := client.GET("HEAD")
	if err != nil {
		return fmt.Errorf("读取远程 HEAD 失败: %w", err)
	}
	remoteHead := strings.TrimSpace(string(remoteHeadData))
	if remoteHead == "" {
		return fmt.Errorf("远程没有数据")
	}
	if err := validateSnapshotID(remoteHead); err != nil {
		return err
	}
	remoteSnap, err := a.loadSnapByID(client, key, remoteHead)
	if err != nil {
		return fmt.Errorf("加载远程快照失败: %w", err)
	}

	result, err := a.applyRemoteSnapshot(ctx, opID, "bulk-pull", cfg, client, key, remoteHead, remoteSnap)
	if err != nil {
		return err
	}
	if result.Conflicts > 0 {
		UpdateTrayState(TrayConflict)
		return fmt.Errorf("发现 %d 个冲突，请在文件页选择以本地或远程为准", result.Conflicts)
	}
	if result.Applied == 0 {
		if result.BinaryApplied {
			a.emitProgress(opID, "bulk-pull", 1, 1, 1, 1, "已恢复 Claude binary")
		} else {
			a.emitProgress(opID, "bulk-pull", 1, 1, 1, 1, "已是最新")
		}
		return nil
	}
	if result.BinaryApplied {
		a.emitProgress(opID, "bulk-pull", int64(result.Applied), int64(result.Total), result.Applied, result.Total, fmt.Sprintf("已拉取 %d 个文件并恢复 Claude binary", result.Applied))
	}
	return nil
}

type pullMergeResult struct {
	Applied       int
	Conflicts     int
	Total         int
	BinaryApplied bool
}

func (a *App) applyRemoteSnapshot(ctx context.Context, opID int64, operation string, cfg *config.Config, client *webdav.Client, key []byte, remoteHead string, remoteSnap *snapshot.Snapshot) (*pullMergeResult, error) {
	scanner := newClaudeScanner(cfg.Exclude.Patterns)
	scanResult, err := scanner.ScanPartial()
	if err != nil {
		return nil, fmt.Errorf("扫描失败: %w", err)
	}
	if err := requireCompleteScan(scanResult); err != nil {
		return nil, err
	}

	localHead, _ := os.ReadFile(config.CCBoxDir() + "/HEAD")
	localHeadStr := strings.TrimSpace(string(localHead))
	var baseSnap *snapshot.Snapshot
	if localHeadStr != "" {
		baseSnap, _ = a.loadSnapByID(client, key, localHeadStr)
	}

	paths := mergePathSet(baseSnap, scanResult.Files, remoteSnap)
	result := &pullMergeResult{Total: len(paths)}
	if len(paths) == 0 {
		if cfg.Binary.SyncEnabled {
			result.BinaryApplied, err = a.applyRemoteClaudeRestore(ctx, opID, operation, client, key, remoteSnap)
			if err != nil {
				return nil, err
			}
		}
		if err := cachePulledSnapshot(remoteHead, remoteSnap); err != nil {
			return nil, err
		}
		if err := config.WriteFileEnsureDir(config.CCBoxDir()+"/HEAD", []byte(remoteHead), 0600); err != nil {
			return nil, err
		}
		return result, nil
	}

	store := object.NewStore(client, key, config.CCBoxDir()+"/cache/objects")
	for i, path := range paths {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		baseEntry, baseExists := fileEntryFromSnapshot(baseSnap, path)
		localEntry, localExists := scanResult.Files[path]
		remoteEntry, remoteExists := remoteSnap.Files[path]
		localChanged := !sameFileState(baseExists, baseEntry, localExists, localEntry)
		remoteChanged := !sameFileState(baseExists, baseEntry, remoteExists, remoteEntry)

		if !localChanged && !remoteChanged {
			continue
		}
		if sameFileState(localExists, localEntry, remoteExists, remoteEntry) {
			continue
		}
		if localChanged && remoteChanged {
			if err := a.savePullConflict(path, localExists, localEntry, remoteExists, remoteEntry, store); err != nil {
				return nil, err
			}
			result.Conflicts++
			a.emitProgress(opID, operation, int64(i+1), int64(len(paths)), int(i+1), len(paths), fmt.Sprintf("冲突 %s", path))
			continue
		}
		if remoteChanged {
			if err := applyRemoteFile(path, remoteExists, remoteEntry, store); err != nil {
				return nil, err
			}
			result.Applied++
			a.emitProgress(opID, operation, int64(i+1), int64(len(paths)), int(i+1), len(paths), fmt.Sprintf("拉取 %s", path))
		}
	}

	if cfg.Binary.SyncEnabled && result.Conflicts == 0 {
		result.BinaryApplied, err = a.applyRemoteClaudeRestore(ctx, opID, operation, client, key, remoteSnap)
		if err != nil {
			return nil, err
		}
	}
	if err := cachePulledSnapshot(remoteHead, remoteSnap); err != nil {
		return nil, err
	}
	if result.Conflicts == 0 {
		if err := config.WriteFileEnsureDir(config.CCBoxDir()+"/HEAD", []byte(remoteHead), 0600); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (a *App) applyRemoteClaudeRestore(ctx context.Context, opID int64, operation string, client *webdav.Client, key []byte, snap *snapshot.Snapshot) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	plan, err := binary.PlanClaudeRestore(client, key, snap, binary.ClaudeRestoreExact)
	if err != nil {
		return false, err
	}
	switch plan.Action {
	case binary.ClaudeActionSkipAlreadyInstalled, binary.ClaudeActionSkipNoSnapshot, binary.ClaudeActionNeedUserChoice:
		return false, nil
	case binary.ClaudeActionUnavailable:
		return false, fmt.Errorf("快照需要 Claude %s，但云端没有当前平台可用版本", plan.TargetVersion)
	case binary.ClaudeActionDownload:
		a.emitProgress(opID, operation, 0, 1, 0, 1, "恢复 Claude binary "+plan.TargetVersion)
		if err := binary.ApplyClaudeRestore(client, key, plan, a.progressCallback(opID, operation)); err != nil {
			return false, err
		}
		if plan.PathConfig != nil && plan.PathConfig.Error != "" {
			a.emitProgress(opID, operation, 1, 1, 1, 1, plan.PathConfig.Message)
		} else {
			a.emitProgress(opID, operation, 1, 1, 1, 1, "已恢复 Claude binary "+plan.TargetVersion)
		}
		return true, nil
	default:
		return false, fmt.Errorf("未知 Claude binary 恢复动作: %s", plan.Action)
	}
}

func mergePathSet(baseSnap *snapshot.Snapshot, current map[string]snapshot.FileEntry, remoteSnap *snapshot.Snapshot) []string {
	seen := make(map[string]bool)
	for path := range current {
		seen[path] = true
	}
	if baseSnap != nil {
		for path := range baseSnap.Files {
			seen[path] = true
		}
	}
	if remoteSnap != nil {
		for path := range remoteSnap.Files {
			seen[path] = true
		}
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func fileEntryFromSnapshot(snap *snapshot.Snapshot, path string) (snapshot.FileEntry, bool) {
	if snap == nil {
		return snapshot.FileEntry{}, false
	}
	entry, ok := snap.Files[path]
	return entry, ok
}

func sameFileState(aExists bool, a snapshot.FileEntry, bExists bool, b snapshot.FileEntry) bool {
	if aExists != bExists {
		return false
	}
	if !aExists {
		return true
	}
	return a.Hash == b.Hash
}

func applyRemoteFile(path string, remoteExists bool, remoteEntry snapshot.FileEntry, store *object.Store) error {
	fullPath, err := safeClaudePath(path)
	if err != nil {
		return err
	}
	if !remoteExists {
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("删除本地文件 %s 失败: %w", path, err)
		}
		return nil
	}
	data, err := store.Download(remoteEntry.Hash)
	if err != nil {
		return fmt.Errorf("下载文件 %s 失败: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	if err := os.WriteFile(fullPath, data, 0600); err != nil {
		return fmt.Errorf("写入文件 %s 失败: %w", path, err)
	}
	return nil
}

func (a *App) savePullConflict(path string, localExists bool, localEntry snapshot.FileEntry, remoteExists bool, remoteEntry snapshot.FileEntry, store *object.Store) error {
	var localData []byte
	if localExists {
		fullPath, err := safeClaudePath(path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("读取本地冲突文件 %s 失败: %w", path, err)
		}
		localData = data
	}
	var remoteData []byte
	if remoteExists {
		data, err := store.Download(remoteEntry.Hash)
		if err != nil {
			return fmt.Errorf("下载远程冲突文件 %s 失败: %w", path, err)
		}
		remoteData = data
	}
	meta := conflictMetadata{
		LocalModified:  localEntry.Modified,
		RemoteModified: remoteEntry.Modified,
		Recommended:    recommendedConflictChoice(localExists, localEntry.Modified, remoteExists, remoteEntry.Modified),
		LocalExists:    localExists,
		RemoteExists:   remoteExists,
	}
	return saveConflictFiles(path, localData, remoteData, meta)
}

func recommendedConflictChoice(localExists bool, localModified time.Time, remoteExists bool, remoteModified time.Time) string {
	if localExists != remoteExists {
		return ""
	}
	if !localExists {
		return ""
	}
	if localModified.After(remoteModified) {
		return "local"
	}
	if remoteModified.After(localModified) {
		return "remote"
	}
	return ""
}

func saveConflictFiles(relPath string, localData, remoteData []byte, meta conflictMetadata) error {
	localFile, remoteFile, metaFile, err := conflictFilePaths(relPath)
	if err != nil {
		return err
	}
		// WriteFileEnsureDir 内部已包含 MkdirAll，无需显式创建目录
	if err := config.WriteFileEnsureDir(localFile, localData, 0600); err != nil {
		return err
	}
	if err := config.WriteFileEnsureDir(remoteFile, remoteData, 0600); err != nil {
		return err
	}
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return config.WriteFileEnsureDir(metaFile, metaData, 0600)
}

func conflictFilePaths(relPath string) (string, string, string, error) {
	base, err := safeJoin(filepath.Join(config.CCBoxDir(), "conflicts"), relPath)
	if err != nil {
		return "", "", "", err
	}
	return base + ".local", base + ".remote", base + ".meta", nil
}

func removeConflictFiles(relPath string) {
	localFile, remoteFile, metaFile, err := conflictFilePaths(relPath)
	if err != nil {
		return
	}
	os.Remove(localFile)
	os.Remove(remoteFile)
	os.Remove(metaFile)
}

func formatConflictTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04")
}

func cachePulledSnapshot(remoteHead string, remoteSnap *snapshot.Snapshot) error {
	if err := validateSnapshotID(remoteHead); err != nil {
		return err
	}
	snapData, err := remoteSnap.Serialize()
	if err != nil {
		return err
	}
	return config.WriteFileEnsureDir(config.CCBoxDir()+"/snapshots/"+remoteHead+".json", snapData, 0600)
}

// isTextFile 判断是否为文本文件
func isTextFile(path string, data []byte) bool {
	ext := strings.ToLower(filepath.Ext(path))
	textExts := map[string]bool{
		".json": true, ".md": true, ".toml": true, ".yaml": true, ".yml": true,
		".txt": true, ".js": true, ".ts": true, ".svelte": true, ".css": true,
		".html": true, ".xml": true, ".jsonl": true, ".sh": true, ".bat": true,
		".ps1": true, ".py": true, ".go": true, ".rs": true, ".cfg": true,
		".conf": true, ".ini": true, ".env": true, ".gitignore": true,
		".lock": true, ".log": true,
	}
	if textExts[ext] {
		return true
	}
	// 检测是否包含大量不可打印字符
	if len(data) == 0 {
		return true
	}
	nonPrintable := 0
	sample := data
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	for _, b := range sample {
		if b < 32 && b != '\n' && b != '\r' && b != '\t' {
			nonPrintable++
		}
	}
	return float64(nonPrintable)/float64(len(sample)) < 0.1
}

// computeHunks 计算差异块
func computeHunks(oldStr, newStr string) []DiffHunk {
	oldLines := strings.Split(oldStr, "\n")
	newLines := strings.Split(newStr, "\n")
	if len(oldLines) > 0 && oldLines[len(oldLines)-1] == "" {
		oldLines = oldLines[:len(oldLines)-1]
	}
	if len(newLines) > 0 && newLines[len(newLines)-1] == "" {
		newLines = newLines[:len(newLines)-1]
	}

	var hunks []DiffHunk
	oIdx, nIdx := 0, 0

	for oIdx < len(oldLines) || nIdx < len(newLines) {
		// 跳过相同行
		for oIdx < len(oldLines) && nIdx < len(newLines) && oldLines[oIdx] == newLines[nIdx] {
			oIdx++
			nIdx++
		}
		if oIdx >= len(oldLines) && nIdx >= len(newLines) {
			break
		}

		// 找到下一个匹配点
		oEnd, nEnd := findNextMatch(oldLines, newLines, oIdx, nIdx)

		var lines []string
		for i := oIdx; i < oEnd; i++ {
			lines = append(lines, "-"+oldLines[i])
		}
		for i := nIdx; i < nEnd; i++ {
			lines = append(lines, "+"+newLines[i])
		}

		if len(lines) > 0 {
			hunks = append(hunks, DiffHunk{
				OldStart: oIdx + 1,
				OldCount: oEnd - oIdx,
				NewStart: nIdx + 1,
				NewCount: nEnd - nIdx,
				Lines:    lines,
			})
		}

		// 添加上下文
		ctxLines := 3
		for i := 0; i < ctxLines && oEnd+i < len(oldLines) && nEnd+i < len(newLines); i++ {
			if oldLines[oEnd+i] == newLines[nEnd+i] {
				oEnd++
				nEnd++
			} else {
				break
			}
		}

		oIdx = oEnd
		nIdx = nEnd
	}

	return hunks
}

func findNextMatch(old, new_ []string, oStart, nStart int) (int, int) {
	// 找到下一个匹配位置
	for o := oStart; o < len(old); o++ {
		for n := nStart; n < len(new_); n++ {
			if old[o] == new_[n] {
				return o, n
			}
		}
	}
	return len(old), len(new_)
}

// SaveMergedConflict 保存合并后的冲突解决结果
func (a *App) SaveMergedConflict(relPath, content string) error {
	targetPath, err := safeClaudePath(relPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	if err := os.WriteFile(targetPath, []byte(content), 0600); err != nil {
		return err
	}
	removeConflictFiles(relPath)
	a.emitDataChanged("files", "save-merged-conflict")
	return nil
}

// 导出 JSON 给前端使用（辅助）
func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
