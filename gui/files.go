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

	"github.com/user/cc-box/internal/config"
	"github.com/user/cc-box/internal/crypto"
	"github.com/user/cc-box/internal/object"
	"github.com/user/cc-box/internal/snapshot"
	"github.com/user/cc-box/internal/webdav"
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
	Path   string `json:"path"`
	Local  string `json:"local"`
	Remote string `json:"remote"`
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

	client := webdav.NewClient(cfg.WebDAV.URL, cfg.WebDAV.Username, pass)
	return cfg, client, key, nil
}

// loadLocalSnap 加载本地 HEAD 指向的快照（先本地缓存，再远程）
func (a *App) loadLocalSnap(client *webdav.Client, key []byte) (*snapshot.Snapshot, error) {
	head, err := os.ReadFile(config.CCBoxDir() + "/HEAD")
	if err != nil || string(head) == "" {
		return nil, nil
	}

	// 先尝试本地缓存
	snapDir := config.CCBoxDir() + "/snapshots/"
	data, err := os.ReadFile(snapDir + string(head) + ".json")
	if err == nil {
		return snapshot.Deserialize(data)
	}

	// 从远程下载
	snapPath := "snapshots/" + string(head) + ".json.enc"
	encrypted, _, err := client.GET(snapPath)
	if err != nil {
		return nil, err
	}
	decrypted, err := crypto.Decrypt(encrypted, key)
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

	// 先尝试本地缓存
	snapDir := config.CCBoxDir() + "/snapshots/"
	local, err := os.ReadFile(snapDir + head + ".json")
	if err == nil {
		return snapshot.Deserialize(local)
	}

	snapPath := "snapshots/" + head + ".json.enc"
	encrypted, _, err := client.GET(snapPath)
	if err != nil {
		return nil, err
	}
	decrypted, err := crypto.Decrypt(encrypted, key)
	if err != nil {
		return nil, err
	}
	return snapshot.Deserialize(decrypted)
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

		if string(localHead) != remoteHeadStr {
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
	dir := config.CCBoxDir() + "/conflicts"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return result
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		base := strings.TrimSuffix(name, ".local")
		base = strings.TrimSuffix(base, ".remote")
		if base != name {
			result[base] = true
		}
	}
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

	fullPath := filepath.Join(config.ClaudeDir(), relPath)
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
	fullPath := filepath.Join(config.ClaudeDir(), relPath)
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
	dir := config.CCBoxDir() + "/conflicts"
	localFile := dir + "/" + relPath + ".local"
	remoteFile := dir + "/" + relPath + ".remote"

	localData, err := os.ReadFile(localFile)
	if err != nil {
		return nil, fmt.Errorf("本地冲突版本不存在")
	}
	remoteData, err := os.ReadFile(remoteFile)
	if err != nil {
		return nil, fmt.Errorf("远程冲突版本不存在")
	}

	return &ConflictDetail{
		Path:   relPath,
		Local:  string(localData),
		Remote: string(remoteData),
	}, nil
}

// ResolveConflict 解决冲突
func (a *App) ResolveConflict(relPath, choice string) error {
	dir := config.CCBoxDir() + "/conflicts"
	localFile := dir + "/" + relPath + ".local"
	remoteFile := dir + "/" + relPath + ".remote"

	var data []byte
	switch choice {
	case "local":
		d, err := os.ReadFile(localFile)
		if err != nil {
			return err
		}
		data = d
	case "remote":
		d, err := os.ReadFile(remoteFile)
		if err != nil {
			return err
		}
		data = d
	case "merged":
		// 合并结果需要前端提供，暂时跳过
		return fmt.Errorf("合并模式暂未实现")
	default:
		return fmt.Errorf("无效选择: %s", choice)
	}

	targetPath := filepath.Join(config.ClaudeDir(), filepath.FromSlash(relPath))
	os.MkdirAll(filepath.Dir(targetPath), 0755)
	if err := os.WriteFile(targetPath, data, 0600); err != nil {
		return fmt.Errorf("写入失败: %w", err)
	}

	os.Remove(localFile)
	os.Remove(remoteFile)
	return nil
}

// ExcludeFile 将文件/目录添加到排除规则
func (a *App) ExcludeFile(relPath string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pattern := relPath
	info, err := os.Stat(filepath.Join(config.ClaudeDir(), relPath))
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
	var localSnap *snapshot.Snapshot
	if string(localHead) != "" {
		localSnap, _ = a.loadLocalSnap(client, key)
	}

	// 计算变更
	var changes []snapshot.Change
	if localSnap != nil {
		currentSnap := snapshot.CreateSnapshot("", cfg.Device.ID, "", scanResult.Files)
		changes = localSnap.Diff(currentSnap)
	} else {
		for path, entry := range scanResult.Files {
			changes = append(changes, snapshot.Change{Path: path, Type: snapshot.Added, NewHash: entry.Hash, NewSize: entry.Size})
		}
	}

	if len(changes) == 0 {
		a.emitProgress(opID, "bulk-push", 1, 1, 1, 1, "没有变更需要推送")
		return nil
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

		fullPath := filepath.Join(config.ClaudeDir(), filepath.FromSlash(c.Path))
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		if _, err := store.Upload(data); err != nil {
			continue
		}
		a.emitProgress(opID, "bulk-push", int64(i+1), total, int(i+1), int(total), fmt.Sprintf("推送 %s", c.Path))
	}

	newSnap := snapshot.CreateSnapshot(string(localHead), cfg.Device.ID, "gui push", scanResult.Files)
	snapData, _ := newSnap.Serialize()
	encrypted, err := crypto.Encrypt(snapData, key)
	if err != nil {
		return fmt.Errorf("encrypt snap: %w", err)
	}
	client.EnsureDir("snapshots/")
	if _, err := client.PUT("snapshots/"+newSnap.ID+".json.enc", encrypted, ""); err != nil {
		return fmt.Errorf("upload snap: %w", err)
	}
	for attempt := 0; attempt < cfg.Sync.MergeRetryMax; attempt++ {
		_, etag, err := client.GET("HEAD")
		if err != nil && err != webdav.ErrNotFound {
			return fmt.Errorf("read HEAD: %w", err)
		}
		res, err := client.CompareAndSwapHEAD("HEAD", newSnap.ID, etag)
		if err != nil {
			return fmt.Errorf("cas HEAD: %w", err)
		}
		if res.Success {
			break
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
		if attempt == cfg.Sync.MergeRetryMax-1 {
			return fmt.Errorf("conflict, pull first")
		}
	}
	os.WriteFile(config.CCBoxDir()+"/HEAD", []byte(newSnap.ID), 0600)
	os.WriteFile(config.CCBoxDir()+"/snapshots/"+newSnap.ID+".json", snapData, 0600)
	return nil
}

func (a *App) doBulkPull(ctx context.Context, opID int64, cfg *config.Config, client *webdav.Client, key []byte) error {
	remoteSnap, err := a.loadRemoteSnap(client, key)
	if err != nil {
		return fmt.Errorf("加载远程快照失败: %w", err)
	}
	if remoteSnap == nil {
		return fmt.Errorf("远程没有数据")
	}

	// 只下载有差异的文件
	scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
	scanResult, err := scanner.Scan()
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}
	var toDownload []string
	for path, remoteEntry := range remoteSnap.Files {
		localEntry, exists := scanResult.Files[path]
		if !exists || localEntry.Hash != remoteEntry.Hash {
			toDownload = append(toDownload, path)
		}
	}
	if len(toDownload) == 0 {
		os.WriteFile(config.CCBoxDir()+"/HEAD", []byte(remoteSnap.ID), 0600)
		a.emitProgress(opID, "bulk-pull", 1, 1, 1, 1, "already up to date")
		return nil
	}
	store := object.NewStore(client, key, config.CCBoxDir()+"/cache/objects")
	total := int64(len(toDownload))
	for i, path := range toDownload {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		remoteEntry, ok := remoteSnap.Files[path]
		if !ok {
			continue
		}
		data, err := store.Download(remoteEntry.Hash)
		if err != nil {
			continue
		}
		fullPath := filepath.Join(config.ClaudeDir(), filepath.FromSlash(path))
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, data, 0600)
		a.emitProgress(opID, "bulk-pull", int64(i+1), total, int(i+1), int(total), fmt.Sprintf("pull %s", path))
	}


	// 更新本地 HEAD
	os.WriteFile(config.CCBoxDir()+"/HEAD", []byte(remoteSnap.ID), 0600)
	snapData, _ := remoteSnap.Serialize()
	os.WriteFile(config.CCBoxDir()+"/snapshots/"+remoteSnap.ID+".json", snapData, 0600)

	return nil
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
	dir := config.CCBoxDir() + "/conflicts"
	os.Remove(dir + "/" + relPath + ".local")
	os.Remove(dir + "/" + relPath + ".remote")

	targetPath := filepath.Join(config.ClaudeDir(), filepath.FromSlash(relPath))
	os.MkdirAll(filepath.Dir(targetPath), 0755)
	return os.WriteFile(targetPath, []byte(content), 0600)
}

// 导出 JSON 给前端使用（辅助）
func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
