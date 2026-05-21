// maintenance 命令
// gc / verify / backup / restore
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/crypto"
	"github.com/user/cc-box/core/object"
	"github.com/user/cc-box/core/snapshot"
	"github.com/user/cc-box/core/webdav"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "创建本地完整备份",
	RunE:  runBackup,
}

var restoreCmd = &cobra.Command{
	Use:   "restore [snapshot-id]",
	Short: "从备份或快照恢复",
	RunE:  runRestore,
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "验证本地文件完整性 + 远程可达性",
	RunE:  runVerify,
}

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "清理过期 objects 和快照",
	RunE:  runGC,
}

func init() {
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(gcCmd)
	verifyCmd.Flags().Bool("deep", false, "验证远程快照链和 objects 完整性")
}

func runBackup(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	backupDir := config.CCBoxDir() + "/backups"
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(backupDir, "backup-"+timestamp)

	os.MkdirAll(backupPath, 0755)

	// 备份 config.toml
	configData, err := os.ReadFile(config.CCBoxDir() + "/config.toml")
	if err == nil {
		os.WriteFile(filepath.Join(backupPath, "config.toml"), configData, 0600)
	}

	// 备份 HEAD
	headData, err := os.ReadFile(config.CCBoxDir() + "/HEAD")
	if err == nil {
		os.WriteFile(filepath.Join(backupPath, "HEAD"), headData, 0600)
	}

	// 备份 key.bin
	keyData, err := os.ReadFile(config.KeyPath())
	if err == nil {
		os.WriteFile(filepath.Join(backupPath, "key.bin"), keyData, 0600)
	}

	// 备份快照缓存
	snapDir := config.CCBoxDir() + "/snapshots"
	if entries, err := os.ReadDir(snapDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				data, err := os.ReadFile(filepath.Join(snapDir, entry.Name()))
				if err == nil {
					os.WriteFile(filepath.Join(backupPath, entry.Name()), data, 0600)
				}
			}
		}
	}

	fmt.Printf("备份已创建: %s\n", backupPath)
	_ = cfg
	return nil
}

func runRestore(cmd *cobra.Command, args []string) error {
	// 如果提供了 snapshot-id，从快照恢复
	if len(args) > 0 && args[0] != "" {
		return restoreFromSnapshot(args[0])
	}

	// 否则从本地备份恢复
	backupDir := config.CCBoxDir() + "/backups"
	entries, err := os.ReadDir(backupDir)
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("没有可用的备份")
	}

	// 使用最新的备份
	latest := entries[len(entries)-1]
	backupPath := filepath.Join(backupDir, latest.Name())

	fmt.Printf("将从备份 %s 恢复\n", latest.Name())

	// 恢复各文件
	files := []string{"config.toml", "HEAD", "key.bin"}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(backupPath, f))
		if err != nil {
			continue
		}
		targetPath := filepath.Join(config.CCBoxDir(), f)
		os.WriteFile(targetPath, data, 0600)
		fmt.Printf("  已恢复 %s\n", f)
	}

	// 恢复快照
	targetSnapDir := config.CCBoxDir() + "/snapshots"
	if entries, err := os.ReadDir(backupPath); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == "config.toml" || entry.Name() == "HEAD" || entry.Name() == "key.bin" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(backupPath, entry.Name()))
			if err == nil {
				os.WriteFile(filepath.Join(targetSnapDir, entry.Name()), data, 0600)
			}
		}
	}

	fmt.Println("恢复完成")
	return nil
}

// restoreFromSnapshot 从指定快照恢复配置文件
func restoreFromSnapshot(snapID string) error {
	_, client, key, err := loadClientAndKey()
	if err != nil {
		return err
	}

	// 加载目标快照
	snap, err := loadRemoteSnapshot(client, key, snapID)
	if err != nil {
		return fmt.Errorf("加载快照 %s 失败: %w", snapID, err)
	}

	// 扫描本地文件
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
	scanResult, err := scanner.Scan()
	if err != nil {
		return fmt.Errorf("扫描本地文件失败: %w", err)
	}

	// 计算需要恢复的文件
	var toRestore []string
	var toDelete []string

	for path := range scanResult.Files {
		if _, exists := snap.Files[path]; !exists {
			toDelete = append(toDelete, path)
		}
	}
	for path, entry := range snap.Files {
		localEntry, exists := scanResult.Files[path]
		if !exists || localEntry.Hash != entry.Hash {
			toRestore = append(toRestore, path)
		}
	}

	if len(toRestore) == 0 && len(toDelete) == 0 {
		fmt.Println("当前状态与快照一致，无需恢复")
		return nil
	}

	fmt.Printf("将从快照 %s 恢复:\n", snapID[:12])
	fmt.Printf("  恢复 %d 个文件\n", len(toRestore))
	if len(toDelete) > 0 {
		fmt.Printf("  删除 %d 个快照中不存在的文件\n", len(toDelete))
	}

	fmt.Print("确认恢复？[y/N] ")
	var answer string
	fmt.Scanln(&answer)
	if answer != "y" && answer != "Y" {
		return nil
	}

	// 下载并恢复文件
	store := object.NewStore(client, key, "")
	applied := 0
	for _, path := range toRestore {
		entry, ok := snap.Files[path]
		if !ok {
			continue
		}
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

	// 删除快照中不存在的文件
	for _, path := range toDelete {
		fullPath := filepath.Join(config.ClaudeDir(), filepath.FromSlash(path))
		os.Remove(fullPath)
	}

	// 创建恢复快照
	newSnap := snapshot.CreateSnapshot(snap.ID, cfg.Device.ID, "restore from "+snapID[:12], snap.Files)
	snapData, _ := newSnap.Serialize()
	encrypted, _ := crypto.Encrypt(snapData, key)
	client.EnsureDir("snapshots/")
	client.PUT("snapshots/"+newSnap.ID+".json.enc", encrypted, "")

	// 更新本地 HEAD
	os.WriteFile(config.CCBoxDir()+"/HEAD", []byte(newSnap.ID), 0600)
	os.WriteFile(config.CCBoxDir()+"/snapshots/"+newSnap.ID+".json", snapData, 0600)

	fmt.Printf("已恢复 %d 个文件（快照 %s）\n", applied, snapID[:12])
	return nil
}

func runVerify(cmd *cobra.Command, args []string) error {
	deep, _ := cmd.Flags().GetBool("deep")
	cfg, client, key, err := loadClientAndKey()
	if err != nil {
		return err
	}

	fmt.Println("=== 验证本地 ===")

	// 检查配置
	if _, err := config.Load(); err != nil {
		fmt.Printf("  ✗ 配置文件: %v\n", err)
	} else {
		fmt.Println("  ✓ 配置文件")
	}

	// 检查密钥
	if _, err := os.Stat(config.KeyPath()); err != nil {
		fmt.Printf("  ✗ 密钥文件: %v\n", err)
	} else {
		fmt.Println("  ✓ 密钥文件")
	}

	// 检查 HEAD
	if headData, err := os.ReadFile(config.CCBoxDir() + "/HEAD"); err != nil {
		fmt.Printf("  ✗ HEAD: %v\n", err)
	} else {
		fmt.Printf("  ✓ HEAD: %s\n", string(headData))
	}

	fmt.Println()
	fmt.Println("=== 验证远程 ===")

	// 测试连接
	if _, err := client.Exists("/"); err != nil {
		fmt.Printf("  ✗ WebDAV 连接: %v\n", err)
	} else {
		fmt.Println("  ✓ WebDAV 连接")
	}

	// 检查 HEAD
	if headData, _, err := client.GET("HEAD"); err != nil {
		fmt.Printf("  ✗ 远程 HEAD: %v\n", err)
	} else {
		fmt.Printf("  ✓ 远程 HEAD: %s\n", string(headData))
	}

	// 检查 salt
	if _, _, err := client.GET("salt.bin"); err != nil {
		fmt.Printf("  ✗ salt: %v\n", err)
	} else {
		fmt.Println("  ✓ salt")
	}

	if deep {
		if err := runDeepVerify(client, key); err != nil {
			return err
		}
	}

	_ = cfg
	return nil
}

func runDeepVerify(client *webdav.Client, key []byte) error {
	fmt.Println()
	fmt.Println("=== 深度验证 ===")

	headData, _, err := client.GET("HEAD")
	if err != nil {
		return fmt.Errorf("读取远程 HEAD 失败: %w", err)
	}
	snapID := strings.TrimSpace(string(headData))
	if snapID == "" {
		return fmt.Errorf("远程 HEAD 为空")
	}

	store := object.NewStore(client, key, "")
	seenSnapshots := make(map[string]bool)
	checkedObjects := make(map[string]bool)
	for snapID != "" {
		if seenSnapshots[snapID] {
			return fmt.Errorf("快照链存在循环: %s", snapID)
		}
		seenSnapshots[snapID] = true
		snap, err := loadRemoteSnapshot(client, key, snapID)
		if err != nil {
			return fmt.Errorf("加载快照 %s 失败: %w", snapID, err)
		}
		for path, entry := range snap.Files {
			if _, err := safeClaudePath(path); err != nil {
				return fmt.Errorf("快照 %s 包含不安全路径 %s: %w", snapID, path, err)
			}
			if checkedObjects[entry.Hash] {
				continue
			}
			if _, err := store.Download(entry.Hash); err != nil {
				return fmt.Errorf("验证 object %s 失败: %w", entry.Hash, err)
			}
			checkedObjects[entry.Hash] = true
		}
		snapID = snap.Parent
	}

	fmt.Printf("  ✓ 快照链 %d 个，objects %d 个\n", len(seenSnapshots), len(checkedObjects))
	return nil
}

func runGC(cmd *cobra.Command, args []string) error {
	_, client, key, err := loadClientAndKey()
	if err != nil {
		return err
	}

	fmt.Println("扫描快照链...")
	reachableObjects := make(map[string]bool)

	headData, _, err := client.GET("HEAD")
	if err != nil {
		return fmt.Errorf("读取远程 HEAD 失败: %w", err)
	}

	snapID := strings.TrimSpace(string(headData))
	snapCount := 0
	for snapID != "" && snapCount < 50 {
		snap, err := loadRemoteSnapshot(client, key, snapID)
		if err != nil {
			return fmt.Errorf("加载快照 %s 失败，中止 GC: %w", snapID, err)
		}
		for _, entry := range snap.Files {
			reachableObjects[entry.Hash] = true
		}
		snapID = snap.Parent
		snapCount++
	}
	fmt.Printf("  扫描了 %d 个快照，%d 个可达 object\n", snapCount, len(reachableObjects))

	fmt.Println("扫描远程 objects...")
	prefixFiles, err := client.PROPFIND("objects/", 1)
	if err != nil {
		return fmt.Errorf("列出 objects 失败: %w", err)
	}

	// 辅助函数：从 PROPFIND 返回路径中提取最后一段
	extractName := func(p string) string {
		p = strings.TrimSuffix(p, "/")
		if idx := strings.LastIndex(p, "/"); idx >= 0 {
			return p[idx+1:]
		}
		return p
	}

	var orphanObjects []string
	for _, dir := range prefixFiles {
		if !dir.IsDir {
			continue
		}
		dirName := extractName(dir.Path)

		objFiles, err := client.PROPFIND("objects/"+dirName+"/", 1)
		if err != nil {
			continue
		}

		for _, f := range objFiles {
			if f.IsDir || !strings.HasSuffix(f.Path, ".enc") {
				continue
			}
			encFileName := extractName(f.Path)
			hash := strings.TrimSuffix(encFileName, ".enc")

			if !reachableObjects[hash] {
				orphanObjects = append(orphanObjects, "objects/"+dirName+"/"+encFileName)
			}
		}
	}

	if len(orphanObjects) == 0 {
		fmt.Println("没有可清理的 object")
		return nil
	}

	fmt.Printf("\n发现 %d 个孤立 object（不再被任何快照引用）\n", len(orphanObjects))
	fmt.Print("确认清理？[y/N] ")
	var answer string
	fmt.Scanln(&answer)
	if strings.ToLower(answer) != "y" {
		return nil
	}

	cleaned := 0
	for _, path := range orphanObjects {
		if err := client.DELETE(path); err != nil {
			fmt.Printf("  删除失败 %s: %v\n", path, err)
			continue
		}
		cleaned++
	}

	fmt.Printf("已清理 %d 个孤立 object\n", cleaned)
	return nil
}
