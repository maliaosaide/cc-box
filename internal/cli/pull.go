// pull 命令
// 三方合并拉取：尝试找共同祖先 → 三方合并 → 降级为文件级比较
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/cc-box/internal/config"
	"github.com/user/cc-box/internal/crypto"
	"github.com/user/cc-box/internal/object"
	"github.com/user/cc-box/internal/snapshot"
	"github.com/user/cc-box/internal/sync"
	"github.com/user/cc-box/internal/webdav"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "拉取远程配置变更",
	RunE:  runPull,
}

func init() {
	rootCmd.AddCommand(pullCmd)
	pullCmd.Flags().Bool("dry-run", false, "仅显示将要拉取的变更，不实际应用")
	pullCmd.Flags().Bool("force", false, "强制覆盖本地变更")
}

func runPull(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("请先运行 cc-box init")
	}

	key, err := crypto.LoadKey(config.KeyPath())
	if err != nil {
		return fmt.Errorf("加载密钥失败")
	}

	pass, err := config.LoadWebDAVPassword()
	if err != nil {
		return err
	}
	client := webdav.NewClient(cfg.WebDAV.URL, cfg.WebDAV.Username, pass)

	// 坚果云优化：先 HEAD 检查 ETag，没变就跳过
	if cachedETag := readCachedRemoteETag(); cachedETag != "" {
		info, err := client.HEAD("HEAD")
		if err == nil && info.ETag == cachedETag {
			fmt.Println("已是最新（ETag 未变）")
			return nil
		}
	}

	// 读取远程 HEAD
	remoteHeadData, _, err := client.GET("HEAD")
	if err != nil {
		if err == webdav.ErrNotFound {
			return fmt.Errorf("远程没有数据")
		}
		return fmt.Errorf("读取远程 HEAD 失败: %w", err)
	}
	remoteHead := string(remoteHeadData)

	localHead, err := loadLocalHEAD()
	if err != nil {
		localHead = ""
	}

	if localHead == remoteHead {
		fmt.Println("已是最新")
		return nil
	}

	fmt.Printf("远程快照: %s\n", remoteHead)
	fmt.Printf("本地快照: %s\n", localHead)

	// 加载远程快照
	remoteSnap, err := loadRemoteSnapshot(client, key, remoteHead)
	if err != nil {
		return fmt.Errorf("加载远程快照失败: %w", err)
	}

	// 无本地基线 → 首次拉取，直接全部下载
	if localHead == "" {
		return pullFirstTime(client, key, remoteSnap, remoteHead, dryRun)
	}

	// 加载本地快照
	localSnap, err := loadRemoteSnapshot(client, key, localHead)
	if err != nil {
		fmt.Printf("警告：加载本地快照失败，降级为文件级比较: %v\n", err)
		return pullDegraded(client, key, remoteSnap, cfg, force, dryRun, remoteHead)
	}

	// 尝试找共同祖先
	ancestorID, err := findAncestor(client, key, localHead, remoteHead)
	if err != nil || ancestorID == "" {
		fmt.Println("未找到共同祖先，使用文件级双向合并")
		return pullDegraded(client, key, remoteSnap, cfg, force, dryRun, remoteHead)
	}

	// 有共同祖先，执行三方合并
	return pullThreeWay(client, key, localSnap, remoteSnap, ancestorID, cfg, force, dryRun, remoteHead)
}

// pullFirstTime 首次拉取（无本地快照基线）
func pullFirstTime(client *webdav.Client, key []byte, remoteSnap *snapshot.Snapshot, remoteHead string, dryRun bool) error {
	fmt.Printf("\n首次拉取，下载 %d 个文件\n", len(remoteSnap.Files))

	if dryRun {
		for path := range remoteSnap.Files {
			fmt.Printf("  ↓ %s\n", path)
		}
		fmt.Println("\n(dry-run 模式，未实际下载)")
		return nil
	}

	store := object.NewStore(client, key, "")
	applied := applyFiles(store, remoteSnap.Files)

	updateLocalHEAD(remoteHead)
	saveLocalSnapshot(remoteSnap)
	cacheRemoteETag(remoteHead)
	fmt.Printf("\n已拉取 %d 个文件\n", applied)
	return nil
}

// pullThreeWay 基于共同祖先的三方合并
func pullThreeWay(client *webdav.Client, key []byte, localSnap, remoteSnap *snapshot.Snapshot, ancestorID string, cfg *config.Config, force, dryRun bool, remoteHead string) error {
	ancestorSnap, err := loadRemoteSnapshot(client, key, ancestorID)
	if err != nil {
		fmt.Printf("警告：加载祖先快照失败，降级为文件级比较: %v\n", err)
		return pullDegraded(client, key, remoteSnap, cfg, force, dryRun, remoteHead)
	}

	merger := sync.NewMerger(cfg.Sync.ConflictStrategy)
	store := object.NewStore(client, key, "")

	// 扫描本地文件内容
	scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
	scanResult, err := scanner.Scan()
	if err != nil {
		return fmt.Errorf("扫描本地文件失败: %w", err)
	}

	// 收集所有涉及的文件路径
	allPaths := make(map[string]bool)
	for p := range ancestorSnap.Files {
		allPaths[p] = true
	}
	for p := range localSnap.Files {
		allPaths[p] = true
	}
	for p := range remoteSnap.Files {
		allPaths[p] = true
	}

	var toDownload []string
	var toDelete []string
	var conflictPaths []string

	for path := range allPaths {
		ancEntry, inAnc := ancestorSnap.Files[path]
		localEntry, inLocal := scanResult.Files[path]
		remoteEntry, inRemote := remoteSnap.Files[path]

		localChanged := false
		remoteChanged := false

		if inLocal && inAnc {
			localChanged = localEntry.Hash != ancEntry.Hash
		} else if inLocal && !inAnc {
			localChanged = true // 本地新增
		}

		if inRemote && inAnc {
			remoteChanged = remoteEntry.Hash != ancEntry.Hash
		} else if inRemote && !inAnc {
			remoteChanged = true // 远程新增
		}

		switch {
		case !inLocal && !inRemote:
			// 双方都删除 → 不做任何事
		case !inLocal && inRemote:
			if !inAnc {
				// 远程新增，下载
				toDownload = append(toDownload, path)
			} else if remoteChanged {
				// 本地删除，远程修改 → 下载远程
				toDownload = append(toDownload, path)
			}
			// 本地删除，远程未变 → 保持删除
		case inLocal && !inRemote:
			// 本地有，远程删除
			if !inAnc {
				// 本地新增，远程无 → 保留本地
			} else if localChanged {
				// 本地修改，远程删除 → 保留本地（保守策略）
			} else {
				// 本地未变，远程删除 → 删除
				toDelete = append(toDelete, path)
			}
		case inLocal && inRemote:
			if !localChanged && !remoteChanged {
				// 都没变
			} else if localChanged && !remoteChanged {
				// 仅本地变 → 保留本地
			} else if !localChanged && remoteChanged {
				// 仅远程变 → 下载远程
				toDownload = append(toDownload, path)
			} else {
				// 双方都变，哈希相同则无需操作
				if localEntry.Hash == remoteEntry.Hash {
					// 不做任何事
				} else {
					// 尝试三方合并
					resolved := tryThreeWayMerge(merger, store, path, ancEntry, localEntry, remoteEntry, localSnap)
					switch resolved {
					case "remote":
						toDownload = append(toDownload, path)
					case "conflict":
						if force {
							toDownload = append(toDownload, path)
						} else {
							conflictPaths = append(conflictPaths, path)
						}
					}
				}
			}
		}
	}

	// 显示结果
	fmt.Printf("\n三方合并结果:\n")
	for _, p := range toDownload {
		fmt.Printf("  ↓ %s\n", p)
	}
	for _, p := range toDelete {
		fmt.Printf("  ✗ %s\n", p)
	}
	if len(conflictPaths) > 0 {
		fmt.Printf("\n冲突文件 (%d):\n", len(conflictPaths))
		for _, p := range conflictPaths {
			fmt.Printf("  ⚠ %s\n", p)
		}
		if !force {
			fmt.Print("冲突文件将保存到本地，可用 cc-box resolve 解决。继续？[y/N] ")
			var answer string
			fmt.Scanln(&answer)
			if answer != "y" && answer != "Y" {
				return nil
			}
		}
	}

	if dryRun {
		fmt.Println("\n(dry-run 模式，未实际下载)")
		return nil
	}

	// 下载并应用
	applied := applyFiles(store, collectEntries(remoteSnap.Files, toDownload))

	// 删除文件
	for _, p := range toDelete {
		fullPath := filepath.Join(config.ClaudeDir(), filepath.FromSlash(p))
		os.Remove(fullPath)
	}

	// 保存冲突文件
	saveConflictFiles(store, remoteSnap.Files, scanResult.Files, conflictPaths, localSnap)

	// 更新本地 HEAD
	updateLocalHEAD(remoteHead)
	saveLocalSnapshot(remoteSnap)
	cacheRemoteETag(remoteHead)

	fmt.Printf("\n已拉取 %d 个文件，删除 %d 个，冲突 %d 个\n", applied, len(toDelete), len(conflictPaths))
	return nil
}

// pullDegraded 降级为文件级双向合并（无共同祖先）
func pullDegraded(client *webdav.Client, key []byte, remoteSnap *snapshot.Snapshot, cfg *config.Config, force, dryRun bool, remoteHead string) error {
	store := object.NewStore(client, key, "")

	scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
	scanResult, err := scanner.Scan()
	if err != nil {
		return fmt.Errorf("扫描本地文件失败: %w", err)
	}

	var toDownload []string
	var conflictPaths []string

	for path, remoteEntry := range remoteSnap.Files {
		localEntry, exists := scanResult.Files[path]
		if !exists {
			toDownload = append(toDownload, path)
		} else if localEntry.Hash != remoteEntry.Hash {
			conflictPaths = append(conflictPaths, path)
		}
	}

	fmt.Printf("\n文件级合并:\n")
	for _, p := range toDownload {
		fmt.Printf("  ↓ %s\n", p)
	}
	if len(conflictPaths) > 0 {
		fmt.Printf("\n冲突文件 (%d) — 双方都有但哈希不同:\n", len(conflictPaths))
		for _, p := range conflictPaths {
			fmt.Printf("  ⚠ %s\n", p)
		}
		if !force {
			fmt.Print("冲突文件将保存到本地（.local/.remote），可用 cc-box resolve 解决。继续？[y/N] ")
			var answer string
			fmt.Scanln(&answer)
			if answer != "y" && answer != "Y" {
				fmt.Println("已取消，使用 --force 跳过确认")
				return nil
			}
		} else {
			// force 模式下冲突直接用远程覆盖
			toDownload = append(toDownload, conflictPaths...)
			conflictPaths = nil
		}
	}

	if len(toDownload) == 0 && len(conflictPaths) == 0 {
		updateLocalHEAD(remoteHead)
		saveLocalSnapshot(remoteSnap)
		fmt.Println("已是最新")
		return nil
	}

	if dryRun {
		fmt.Println("\n(dry-run 模式，未实际下载)")
		return nil
	}

	// 下载非冲突文件
	applied := applyFiles(store, collectEntries(remoteSnap.Files, toDownload))

	// 保存冲突文件
	var localSnap *snapshot.Snapshot
	localHead, _ := loadLocalHEAD()
	if localHead != "" {
		localSnap, _ = loadRemoteSnapshot(client, key, localHead)
	}
	saveConflictFiles(store, remoteSnap.Files, scanResult.Files, conflictPaths, localSnap)

	updateLocalHEAD(remoteHead)
	saveLocalSnapshot(remoteSnap)
	cacheRemoteETag(remoteHead)

	fmt.Printf("\n已拉取 %d 个文件，冲突 %d 个已保存\n", applied, len(conflictPaths))
	return nil
}

// findAncestor 沿本地和远程快照链查找共同祖先
func findAncestor(client *webdav.Client, key []byte, localHead, remoteHead string) (string, error) {
	loader := func(id string) (*snapshot.Snapshot, error) {
		return loadRemoteSnapshot(client, key, id)
	}

	localChain, err := snapshot.BuildChain(localHead, loader, 50)
	if err != nil {
		return "", err
	}

	remoteChain, err := snapshot.BuildChain(remoteHead, loader, 50)
	if err != nil {
		return "", err
	}

	return snapshot.FindCommonAncestor(localChain, remoteChain, loader)
}

// tryThreeWayMerge 对单个文件尝试三方合并
func tryThreeWayMerge(merger *sync.Merger, store *object.Store, path string, ancEntry, localEntry, remoteEntry snapshot.FileEntry, localSnap *snapshot.Snapshot) string {
	// 读取三方内容
	var ancestorData, localData, remoteData []byte

	if ancEntry.Hash != "" {
		ancestorData, _ = store.Download(ancEntry.Hash)
	}

	// 本地文件从磁盘读取
	localFilePath := config.ClaudeDir() + "/" + strings.ReplaceAll(path, "/", string(filepath.Separator))
	localData, err := os.ReadFile(localFilePath)
	if err != nil {
		return "remote"
	}

	remoteData, _ = store.Download(remoteEntry.Hash)
	if remoteData == nil {
		return "local"
	}

	input := &sync.ThreeWayInput{
		Ancestor: ancestorData,
		Local:    normalizeContent(localData),
		Remote:   remoteData,
	}

	result, err := merger.MergeFile(path, input)
	if err != nil {
		return "conflict"
	}

	switch result.Action {
	case sync.ActionKeepLocal:
		return "local"
	case sync.ActionKeepRemote:
		return "remote"
	case sync.ActionMerged:
		// 写入合并结果到本地
		os.WriteFile(localFilePath, result.Data, 0600)
		return "local"
	case sync.ActionConflict:
		return "conflict"
	default:
		return "remote"
	}
}

// saveConflictFiles 将冲突文件保存到 conflicts 目录
func saveConflictFiles(store *object.Store, remoteFiles map[string]snapshot.FileEntry, localFiles map[string]snapshot.FileEntry, conflictPaths []string, localSnap *snapshot.Snapshot) {
	if len(conflictPaths) == 0 {
		return
	}

	conflictDir := config.CCBoxDir() + "/conflicts"
	os.MkdirAll(conflictDir, 0755)

	for _, path := range conflictPaths {
		// 保存本地版本
		localFilePath := config.ClaudeDir() + "/" + strings.ReplaceAll(path, "/", string(filepath.Separator))
		if localData, err := os.ReadFile(localFilePath); err == nil {
			os.WriteFile(conflictDir+"/"+path+".local", localData, 0600)
		}

		// 保存远程版本
		if remoteEntry, ok := remoteFiles[path]; ok {
			if remoteData, err := store.Download(remoteEntry.Hash); err == nil {
				os.WriteFile(conflictDir+"/"+path+".remote", remoteData, 0600)
			}
		}
	}

	fmt.Printf("  冲突文件已保存到 %s\n", conflictDir)
}

// applyFiles 下载并应用文件到本地
func applyFiles(store *object.Store, files map[string]snapshot.FileEntry) int {
	applied := 0
	for path, entry := range files {
		data, err := store.Download(entry.Hash)
		if err != nil {
			fmt.Printf("  下载失败 %s: %v\n", path, err)
			continue
		}

		fullPath := filepath.Join(config.ClaudeDir(), filepath.FromSlash(path))
		os.MkdirAll(filepath.Dir(fullPath), 0700)
		if err := os.WriteFile(fullPath, data, 0600); err != nil {
			fmt.Printf("  写入失败 %s: %v\n", path, err)
			continue
		}
		applied++
	}
	return applied
}

// collectEntries 从完整文件列表中收集指定路径的条目
func collectEntries(all map[string]snapshot.FileEntry, paths []string) map[string]snapshot.FileEntry {
	result := make(map[string]snapshot.FileEntry)
	for _, p := range paths {
		if entry, ok := all[p]; ok {
			result[p] = entry
		}
	}
	return result
}

// --- ETag 缓存（坚果云优化） ---

func etagCachePath() string {
	return config.CCBoxDir() + "/cache/remote-head-etag"
}

func readCachedRemoteETag() string {
	data, err := os.ReadFile(etagCachePath())
	if err != nil {
		return ""
	}
	parts := strings.SplitN(string(data), "\n", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

func cacheRemoteETag(headID string) {
	os.MkdirAll(filepath.Dir(etagCachePath()), 0755)
	os.WriteFile(etagCachePath(), []byte(headID+"\n"), 0600)
}
