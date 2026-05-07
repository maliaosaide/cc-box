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
	"github.com/user/cc-box/internal/config"
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

func runVerify(cmd *cobra.Command, args []string) error {
	cfg, client, _, err := loadClientAndKey()
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

	_ = cfg
	return nil
}

func runGC(cmd *cobra.Command, args []string) error {
	_, client, key, err := loadClientAndKey()
	if err != nil {
		return err
	}

	// 1. 收集所有可达的 object 哈希（从快照链中）
	fmt.Println("扫描快照链...")
	reachableObjects := make(map[string]bool)

	// 读取远程 HEAD
	headData, _, err := client.GET("HEAD")
	if err != nil {
		return fmt.Errorf("读取远程 HEAD 失败: %w", err)
	}

	// 沿快照链遍历，收集所有引用的 object 哈希
	snapID := string(headData)
	snapCount := 0
	for snapID != "" && snapCount < 50 {
		snap, err := loadRemoteSnapshot(client, key, snapID)
		if err != nil {
			fmt.Printf("  跳过快照 %s: %v\n", snapID, err)
			break
		}

		for _, entry := range snap.Files {
			reachableObjects[entry.Hash] = true
		}
		snapID = snap.Parent
		snapCount++
	}
	fmt.Printf("  扫描了 %d 个快照，%d 个可达 object\n", snapCount, len(reachableObjects))

	// 2. 列出远程所有 objects
	fmt.Println("扫描远程 objects...")
	// 遍历 objects/ 下的哈希前缀目录
	prefixFiles, err := client.PROPFIND("objects/", 1)
	if err != nil {
		return fmt.Errorf("列出 objects 失败: %w", err)
	}

	var orphanObjects []string
	for _, dir := range prefixFiles {
		if !dir.IsDir {
			continue
		}

		objFiles, err := client.PROPFIND("objects/"+dir.Path, 1)
		if err != nil {
			continue
		}

		for _, f := range objFiles {
			if f.IsDir || !strings.HasSuffix(f.Path, ".enc") {
				continue
			}

			// 从文件名提取哈希: ab/c1234def.enc → sha256:abc1234def
			fileName := strings.TrimSuffix(filepath.Base(f.Path), ".enc")
			prefix := filepath.Base(filepath.Dir(f.Path))
			hash := prefix + fileName

			if !reachableObjects[hash] {
				orphanObjects = append(orphanObjects, "objects/"+dir.Path+f.Path)
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
