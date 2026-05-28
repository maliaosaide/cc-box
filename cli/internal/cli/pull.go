// pull 命令
// 三方合并拉取：尝试找共同祖先 → 三方合并 → 降级为文件级比较
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/crypto"
	"github.com/user/cc-box/core/object"
	"github.com/user/cc-box/core/snapshot"
	"github.com/user/cc-box/core/sync"
	"github.com/user/cc-box/core/webdav"
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
	client := webdav.NewClient(config.ConfiguredWebDAVURL(cfg), cfg.WebDAV.Username, pass)

	localHead, err := loadLocalHEAD()
	if err != nil {
		localHead = ""
	}

	// 坚果云优化：先 HEAD 检查 ETag，没变就跳过
	if cachedHead, cachedETag := readCachedRemoteHeadETag(); !cfg.Binary.SyncEnabled && cachedHead != "" && cachedETag != "" && cachedHead == localHead {
		info, err := client.HEAD("HEAD")
		if err == nil && info.ETag == cachedETag {
			fmt.Println("已是最新（ETag 未变）")
			return nil
		}
	}

	// 读取远程 HEAD
	remoteHeadData, remoteHeadETag, err := client.GET("HEAD")
	if err != nil {
		if err == webdav.ErrNotFound {
			return fmt.Errorf("远程没有数据")
		}
		return fmt.Errorf("读取远程 HEAD 失败: %w", err)
	}
	remoteHead := strings.TrimSpace(string(remoteHeadData))

	if localHead == remoteHead {
		cacheRemoteETag(remoteHead, remoteHeadETag)
		if cfg.Binary.SyncEnabled {
			remoteSnap, err := loadRemoteSnapshot(client, key, remoteHead)
			if err != nil {
				return fmt.Errorf("加载远程快照失败: %w", err)
			}
			if dryRun {
				if err := printSnapshotClaudeRestoreDryRun(client, key, remoteSnap); err != nil {
					return err
				}
				fmt.Println("\n(dry-run 模式，未实际下载)")
				return nil
			}
			applied, err := applySnapshotClaudeRestore(client, key, remoteSnap)
			if err != nil {
				return err
			}
			if applied {
				return nil
			}
		}
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
		return pullFirstTime(client, key, remoteSnap, cfg, remoteHead, remoteHeadETag, dryRun)
	}

	// 加载本地快照
	localSnap, err := loadRemoteSnapshot(client, key, localHead)
	if err != nil {
		fmt.Printf("警告：加载本地快照失败，降级为文件级比较: %v\n", err)
		return pullDegraded(client, key, remoteSnap, cfg, force, dryRun, remoteHead, remoteHeadETag)
	}

	// 尝试找共同祖先
	ancestorID, err := findAncestor(client, key, localHead, remoteHead)
	if err != nil || ancestorID == "" {
		fmt.Println("未找到共同祖先，使用文件级双向合并")
		return pullDegraded(client, key, remoteSnap, cfg, force, dryRun, remoteHead, remoteHeadETag)
	}

	// 有共同祖先，执行三方合并
	return pullThreeWay(client, key, localSnap, remoteSnap, ancestorID, cfg, force, dryRun, remoteHead, remoteHeadETag)
}

// pullFirstTime 首次拉取（无本地快照基线）
func pullFirstTime(client *webdav.Client, key []byte, remoteSnap *snapshot.Snapshot, cfg *config.Config, remoteHead, remoteHeadETag string, dryRun bool) error {
	fmt.Printf("\n首次拉取，下载 %d 个文件\n", len(remoteSnap.Files))

	if dryRun {
		for path := range remoteSnap.Files {
			fmt.Printf("  ↓ %s\n", path)
		}
		if cfg.Binary.SyncEnabled {
			if err := printSnapshotClaudeRestoreDryRun(client, key, remoteSnap); err != nil {
				return err
			}
		}
		fmt.Println("\n(dry-run 模式，未实际下载)")
		return nil
	}

	store := object.NewStore(client, key, "")
	applied, err := applyFiles(store, remoteSnap.Files)
	if err != nil {
		return err
	}
	binaryApplied := false
	if cfg.Binary.SyncEnabled {
		binaryApplied, err = applySnapshotClaudeRestore(client, key, remoteSnap)
		if err != nil {
			return err
		}
	}

	if err := updateLocalHEAD(remoteHead); err != nil {
		return err
	}
	if err := saveLocalSnapshot(remoteSnap); err != nil {
		return fmt.Errorf("缓存快照失败: %w", err)
	}
	cacheRemoteETag(remoteHead, remoteHeadETag)
	if binaryApplied {
		fmt.Printf("\n已拉取 %d 个文件并恢复 Claude binary\n", applied)
	} else {
		fmt.Printf("\n已拉取 %d 个文件\n", applied)
	}
	return nil
}

// pullThreeWay 基于共同祖先的三方合并
func pullThreeWay(client *webdav.Client, key []byte, localSnap, remoteSnap *snapshot.Snapshot, ancestorID string, cfg *config.Config, force, dryRun bool, remoteHead, remoteHeadETag string) error {
	ancestorSnap, err := loadRemoteSnapshot(client, key, ancestorID)
	if err != nil {
		fmt.Printf("警告：加载祖先快照失败，降级为文件级比较: %v\n", err)
		return pullDegraded(client, key, remoteSnap, cfg, force, dryRun, remoteHead, remoteHeadETag)
	}

	merger := sync.NewMerger(cfg.Sync.ConflictStrategy)
	store := object.NewStore(client, key, "")

	// 扫描本地文件内容
	scanner := newClaudeScanner(cfg.Exclude.Patterns)
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
	var mergedPaths []string
	mergedFiles := make(map[string][]byte)

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
					resolved, mergedData := tryThreeWayMerge(merger, store, path, ancEntry, localEntry, remoteEntry)
					switch resolved {
					case "remote":
						toDownload = append(toDownload, path)
					case "merged":
						mergedFiles[path] = mergedData
						mergedPaths = append(mergedPaths, path)
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
	for _, p := range mergedPaths {
		fmt.Printf("  ↔ %s\n", p)
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
		if cfg.Binary.SyncEnabled && len(conflictPaths) == 0 {
			if err := printSnapshotClaudeRestoreDryRun(client, key, remoteSnap); err != nil {
				return err
			}
		}
		fmt.Println("\n(dry-run 模式，未实际下载)")
		return nil
	}

	// 下载并应用
	applied, err := applyFiles(store, collectEntries(remoteSnap.Files, toDownload))
	if err != nil {
		return err
	}

	merged, err := applyMergedFiles(mergedFiles)
	if err != nil {
		return err
	}

	deleted, err := applyDeletes(toDelete)
	if err != nil {
		return err
	}

	// 保存冲突文件
	if err := saveConflictFiles(store, remoteSnap.Files, scanResult.Files, conflictPaths, localSnap); err != nil {
		return err
	}

	binaryApplied := false
	if len(conflictPaths) == 0 {
		if cfg.Binary.SyncEnabled {
			binaryApplied, err = applySnapshotClaudeRestore(client, key, remoteSnap)
			if err != nil {
				return err
			}
		}
		if err := updateLocalHEAD(remoteHead); err != nil {
			return err
		}
		if err := saveLocalSnapshot(remoteSnap); err != nil {
			return fmt.Errorf("缓存快照失败: %w", err)
		}
		cacheRemoteETag(remoteHead, remoteHeadETag)
	}

	if binaryApplied {
		fmt.Printf("\n已拉取 %d 个文件，合并 %d 个，删除 %d 个，冲突 %d 个，并恢复 Claude binary\n", applied, merged, deleted, len(conflictPaths))
	} else {
		fmt.Printf("\n已拉取 %d 个文件，合并 %d 个，删除 %d 个，冲突 %d 个\n", applied, merged, deleted, len(conflictPaths))
	}
	if len(conflictPaths) > 0 {
		fmt.Println("存在未解决冲突，本地 HEAD 暂未推进；解决冲突后请重新同步。")
	}
	return nil
}

// pullDegraded 降级为文件级双向合并（无共同祖先）
func pullDegraded(client *webdav.Client, key []byte, remoteSnap *snapshot.Snapshot, cfg *config.Config, force, dryRun bool, remoteHead, remoteHeadETag string) error {
	store := object.NewStore(client, key, "")

	scanner := newClaudeScanner(cfg.Exclude.Patterns)
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
		if dryRun {
			if cfg.Binary.SyncEnabled {
				if err := printSnapshotClaudeRestoreDryRun(client, key, remoteSnap); err != nil {
					return err
				}
			}
			fmt.Println("\n(dry-run 模式，未实际下载)")
			return nil
		}
		binaryApplied := false
		if cfg.Binary.SyncEnabled {
			binaryApplied, err = applySnapshotClaudeRestore(client, key, remoteSnap)
			if err != nil {
				return err
			}
		}
		if err := updateLocalHEAD(remoteHead); err != nil {
			return err
		}
		if err := saveLocalSnapshot(remoteSnap); err != nil {
			return fmt.Errorf("缓存快照失败: %w", err)
		}
		cacheRemoteETag(remoteHead, remoteHeadETag)
		if binaryApplied {
			fmt.Println("已恢复 Claude binary")
		} else {
			fmt.Println("已是最新")
		}
		return nil
	}

	if dryRun {
		if cfg.Binary.SyncEnabled && len(conflictPaths) == 0 {
			if err := printSnapshotClaudeRestoreDryRun(client, key, remoteSnap); err != nil {
				return err
			}
		}
		fmt.Println("\n(dry-run 模式，未实际下载)")
		return nil
	}

	// 下载非冲突文件
	applied, err := applyFiles(store, collectEntries(remoteSnap.Files, toDownload))
	if err != nil {
		return err
	}

	// 保存冲突文件
	var localSnap *snapshot.Snapshot
	localHead, _ := loadLocalHEAD()
	if localHead != "" {
		localSnap, _ = loadRemoteSnapshot(client, key, localHead)
	}
	if err := saveConflictFiles(store, remoteSnap.Files, scanResult.Files, conflictPaths, localSnap); err != nil {
		return err
	}

	binaryApplied := false
	if len(conflictPaths) == 0 {
		if cfg.Binary.SyncEnabled {
			binaryApplied, err = applySnapshotClaudeRestore(client, key, remoteSnap)
			if err != nil {
				return err
			}
		}
		if err := updateLocalHEAD(remoteHead); err != nil {
			return err
		}
		if err := saveLocalSnapshot(remoteSnap); err != nil {
			return fmt.Errorf("缓存快照失败: %w", err)
		}
		cacheRemoteETag(remoteHead, remoteHeadETag)
	}

	if binaryApplied {
		fmt.Printf("\n已拉取 %d 个文件，冲突 %d 个已保存，并恢复 Claude binary\n", applied, len(conflictPaths))
	} else {
		fmt.Printf("\n已拉取 %d 个文件，冲突 %d 个已保存\n", applied, len(conflictPaths))
	}
	if len(conflictPaths) > 0 {
		fmt.Println("存在未解决冲突，本地 HEAD 暂未推进；解决冲突后请重新同步。")
	}
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
func tryThreeWayMerge(merger *sync.Merger, store *object.Store, path string, ancEntry, localEntry, remoteEntry snapshot.FileEntry) (string, []byte) {
	// 读取三方内容
	var ancestorData, localData, remoteData []byte

	if ancEntry.Hash != "" {
		var err error
		ancestorData, err = store.Download(ancEntry.Hash)
		if err != nil {
			return "conflict", nil
		}
	}

	// 本地文件从磁盘读取
	localFilePath, err := safeClaudePath(path)
	if err != nil {
		return "conflict", nil
	}
	localData, err = os.ReadFile(localFilePath)
	if err != nil {
		return "remote", nil
	}

	remoteData, err = store.Download(remoteEntry.Hash)
	if err != nil {
		return "conflict", nil
	}

	input := &sync.ThreeWayInput{
		Ancestor: ancestorData,
		Local:    localData,
		Remote:   remoteData,
	}

	result, err := merger.MergeFile(path, input)
	if err != nil {
		return "conflict", nil
	}

	switch result.Action {
	case sync.ActionKeepLocal:
		return "local", nil
	case sync.ActionKeepRemote:
		return "remote", nil
	case sync.ActionMerged:
		return "merged", result.Data
	case sync.ActionConflict:
		return "conflict", nil
	default:
		return "remote", nil
	}
}

// saveConflictFiles 将冲突文件保存到 conflicts 目录
func saveConflictFiles(store *object.Store, remoteFiles map[string]snapshot.FileEntry, localFiles map[string]snapshot.FileEntry, conflictPaths []string, localSnap *snapshot.Snapshot) error {
	if len(conflictPaths) == 0 {
		return nil
	}

	conflictDir := config.CCBoxDir() + "/conflicts"
	if err := os.MkdirAll(conflictDir, 0755); err != nil {
		return fmt.Errorf("创建冲突目录失败: %w", err)
	}

	for _, path := range conflictPaths {
		// 保存本地版本
		localFilePath, err := safeClaudePath(path)
		if err != nil {
			return fmt.Errorf("冲突文件路径无效 %s: %w", path, err)
		}
		localData, err := os.ReadFile(localFilePath)
		if err != nil {
			return fmt.Errorf("读取本地冲突文件 %s 失败: %w", path, err)
		}
		conflictLocal, err := safeJoin(conflictDir, path+".local")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(conflictLocal), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(conflictLocal, localData, 0600); err != nil {
			return fmt.Errorf("保存本地冲突文件 %s 失败: %w", path, err)
		}

		// 保存远程版本
		remoteEntry, ok := remoteFiles[path]
		if !ok {
			return fmt.Errorf("远程冲突文件不存在: %s", path)
		}
		remoteData, err := store.Download(remoteEntry.Hash)
		if err != nil {
			return fmt.Errorf("下载远程冲突文件 %s 失败: %w", path, err)
		}
		conflictRemote, err := safeJoin(conflictDir, path+".remote")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(conflictRemote), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(conflictRemote, remoteData, 0600); err != nil {
			return fmt.Errorf("保存远程冲突文件 %s 失败: %w", path, err)
		}
	}

	fmt.Printf("  冲突文件已保存到 %s\n", conflictDir)
	return nil
}

// applyFiles 下载并应用文件到本地
func applyFiles(store *object.Store, files map[string]snapshot.FileEntry) (int, error) {
	applied := 0
	for path, entry := range files {
		data, err := store.Download(entry.Hash)
		if err != nil {
			return applied, fmt.Errorf("下载文件 %s 失败: %w", path, err)
		}

		fullPath, err := safeClaudePath(path)
		if err != nil {
			return applied, err
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0700); err != nil {
			return applied, fmt.Errorf("创建目录 %s 失败: %w", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, data, 0600); err != nil {
			return applied, fmt.Errorf("写入文件 %s 失败: %w", path, err)
		}
		applied++
	}
	return applied, nil
}

func applyMergedFiles(files map[string][]byte) (int, error) {
	applied := 0
	for path, data := range files {
		fullPath, err := safeClaudePath(path)
		if err != nil {
			return applied, err
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0700); err != nil {
			return applied, fmt.Errorf("创建目录 %s 失败: %w", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, data, 0600); err != nil {
			return applied, fmt.Errorf("写入合并文件 %s 失败: %w", path, err)
		}
		applied++
	}
	return applied, nil
}

func applyDeletes(paths []string) (int, error) {
	deleted := 0
	for _, p := range paths {
		fullPath, err := safeClaudePath(p)
		if err != nil {
			return deleted, err
		}
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			return deleted, fmt.Errorf("删除文件 %s 失败: %w", p, err)
		}
		deleted++
	}
	return deleted, nil
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

func readCachedRemoteHeadETag() (string, string) {
	data, err := os.ReadFile(etagCachePath())
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(string(data), "\n", 3)
	if len(parts) < 2 {
		return "", ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func cacheRemoteETag(headID, etag string) {
	if headID == "" || etag == "" {
		return
	}
	os.MkdirAll(filepath.Dir(etagCachePath()), 0755)
	os.WriteFile(etagCachePath(), []byte(headID+"\n"+etag+"\n"), 0600)
}
