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

	"github.com/user/cc-box/gui/internal/config"
	"github.com/user/cc-box/gui/internal/crypto"
	"github.com/user/cc-box/gui/internal/normalize"
	"github.com/user/cc-box/gui/internal/object"
	"github.com/user/cc-box/gui/internal/snapshot"
	"github.com/user/cc-box/gui/internal/webdav"
)

// FileNode 文件树节点
type FileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"isDir"`
	Status   string      `json:"status"` // synced, modified, added, deleted, conflict, excluded
	Size     int64       `json:"size"`
	Modified string      `json:"modified"`
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

// FileTreeResult 文件树返回结果
type FileTreeResult struct {
	Root      *FileNode `json:"root"`
	Total     int       `json:"total"`
	Changed   int       `json:"changed"`
	Conflicts int       `json:"conflicts"`
}

// loadClients 加载配置、WebDAV 客户端、密钥
func (a *App) loadClients() (*config.Config, *webdav.Client, []byte, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("请先运行初始化")
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
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	return normalize.HashContent(data), nil
}

func safeJoin(root, relPath string) (string, error) {
	if relPath == "" || strings.ContainsRune(relPath, 0) {
		return "", fmt.Errorf("无效路径: %s", relPath)
	}
	clean := filepath.Clean(filepath.FromSlash(relPath))
	if filepath.IsAbs(clean) || filepath.VolumeName(clean) != "" || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("路径越界: %s", relPath)
	}
	fullPath := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, fullPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("路径越界: %s", relPath)
	}
	return fullPath, nil
}

func safeClaudePath(relPath string) (string, error) {
	return safeJoin(config.ClaudeDir(), relPath)
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

// GetFileTree 返回配置文件树及同步状态
func (a *App) GetFileTree() (*FileTreeResult, error) {
	cfg, client, key, err := a.loadClients()
	if err != nil {
		return nil, err
	}

	// 扫描本地文件
	scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
	scanResult, err := scanner.Scan()
	if err != nil {
		return nil, fmt.Errorf("扫描失败: %w", err)
	}

	// 加载本地快照
	localSnap, _ := a.loadLocalSnap(client, key)

	// 加载远程快照
	remoteSnap, _ := a.loadRemoteSnap(client, key)

	// 读取远程 HEAD 用于比对
	remoteHeadData, _, _ := client.GET("HEAD")
	remoteHeadStr := strings.TrimSpace(string(remoteHeadData))

	// 计算每个文件的同步状态
	statusMap := make(map[string]string)
	for path := range scanResult.Files {
		statusMap[path] = computeFileStatus(path, scanResult.Files, localSnap, remoteSnap, remoteHeadStr)
	}

	// 检查快照中有但本地没有的（已删除）
	if localSnap != nil {
		for path := range localSnap.Files {
			if _, exists := scanResult.Files[path]; !exists {
				statusMap[path] = "deleted"
			}
		}
	}

	// 检查冲突目录
	conflictFiles := listConflicts()

	// 构建文件树
	root := &FileNode{Name: ".claude", Path: "", IsDir: true, Expanded: true}
	changed := 0
	conflicts := 0

	for path, status := range statusMap {
		if _, isConflict := conflictFiles[path]; isConflict {
			status = "conflict"
		}
		if status != "synced" {
			changed++
		}
		if status == "conflict" {
			conflicts++
		}

		entry, hasEntry := scanResult.Files[path]
		if !hasEntry {
			// 已删除文件
			if old, ok := localSnap.Files[path]; ok {
				insertNode(root, path, status, old.Size, old.Modified)
			}
			continue
		}
		insertNode(root, path, status, entry.Size, entry.Modified)
	}

	sortNodes(root)

	return &FileTreeResult{
		Root:      root,
		Total:     len(statusMap),
		Changed:   changed,
		Conflicts: conflicts,
	}, nil
}

// computeFileStatus 计算单个文件的同步状态
func computeFileStatus(path string, current map[string]snapshot.FileEntry, localSnap, remoteSnap *snapshot.Snapshot, remoteHeadStr string) string {
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
		localHead, _ := os.ReadFile(config.CCBoxDir() + "/HEAD")

		if strings.TrimSpace(string(localHead)) != remoteHeadStr {
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
	parts := strings.Split(relPath, "/")
	current := root

	for i, part := range parts {
		isFile := (i == len(parts)-1)

		if isFile {
			current.Children = append(current.Children, &FileNode{
				Name:     part,
				Path:     relPath,
				IsDir:    false,
				Status:   status,
				Size:     size,
				Modified: formatTime(modified),
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
	return config.Save(cfg)
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
	scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
	scanResult, err := scanner.Scan()
	if err != nil {
		return fmt.Errorf("扫描失败: %w", err)
	}

	localHead, _ := os.ReadFile(config.CCBoxDir() + "/HEAD")
	localHeadStr := strings.TrimSpace(string(localHead))
	var localSnap *snapshot.Snapshot
	if localHeadStr != "" {
		localSnap, _ = a.loadLocalSnap(client, key)
	}

	// 计算变更
	currentBins := currentBinaryVersions()
	var changes []snapshot.Change
	binaryChanged := localSnap == nil || !binaryVersionsEqual(localSnap.Binary, currentBins)
	if localSnap != nil {
		currentSnap := snapshot.CreateSnapshot("", cfg.Device.ID, "", scanResult.Files)
		changes = localSnap.Diff(currentSnap)
	} else {
		for path, entry := range scanResult.Files {
			changes = append(changes, snapshot.Change{Path: path, Type: snapshot.Added, NewHash: entry.Hash, NewSize: entry.Size})
		}
	}

	if len(changes) == 0 && !binaryChanged {
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
	if err := os.WriteFile(config.CCBoxDir()+"/HEAD", []byte(newSnap.ID), 0600); err != nil {
		return fmt.Errorf("更新本地 HEAD 失败: %w", err)
	}
	if err := os.WriteFile(config.CCBoxDir()+"/snapshots/"+newSnap.ID+".json", snapData, 0600); err != nil {
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
		a.emitProgress(opID, "bulk-pull", 1, 1, 1, 1, "已是最新")
		return nil
	}
	return nil
}

type pullMergeResult struct {
	Applied   int
	Conflicts int
	Total     int
}

func (a *App) applyRemoteSnapshot(ctx context.Context, opID int64, operation string, cfg *config.Config, client *webdav.Client, key []byte, remoteHead string, remoteSnap *snapshot.Snapshot) (*pullMergeResult, error) {
	scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
	scanResult, err := scanner.Scan()
	if err != nil {
		return nil, fmt.Errorf("扫描失败: %w", err)
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
		if err := cachePulledSnapshot(remoteHead, remoteSnap); err != nil {
			return nil, err
		}
		if err := os.WriteFile(config.CCBoxDir()+"/HEAD", []byte(remoteHead), 0600); err != nil {
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

	if err := cachePulledSnapshot(remoteHead, remoteSnap); err != nil {
		return nil, err
	}
	if result.Conflicts == 0 {
		if err := os.WriteFile(config.CCBoxDir()+"/HEAD", []byte(remoteHead), 0600); err != nil {
			return nil, err
		}
	}
	return result, nil
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
	if err := os.MkdirAll(filepath.Dir(localFile), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(localFile, localData, 0600); err != nil {
		return err
	}
	if err := os.WriteFile(remoteFile, remoteData, 0600); err != nil {
		return err
	}
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaFile, metaData, 0600)
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
	return os.WriteFile(config.CCBoxDir()+"/snapshots/"+remoteHead+".json", snapData, 0600)
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
	return nil
}

// 导出 JSON 给前端使用（辅助）
func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
