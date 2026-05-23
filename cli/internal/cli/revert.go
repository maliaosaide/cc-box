// revert 命令
// 回滚到指定快照
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/object"
	"github.com/user/cc-box/core/snapshot"
)

var revertCmd = &cobra.Command{
	Use:   "revert <snapshot-id>",
	Short: "回滚到指定快照",
	Args:  cobra.ExactArgs(1),
	RunE:  runRevert,
}

func init() {
	rootCmd.AddCommand(revertCmd)
}

func runRevert(cmd *cobra.Command, args []string) error {
	snapID := args[0]
	if err := validateSnapshotID(snapID); err != nil {
		return err
	}

	cfg, client, key, err := loadClientAndKey()
	if err != nil {
		return err
	}

	localHead, err := loadLocalHEAD()
	if err != nil {
		return fmt.Errorf("读取本地 HEAD 失败: %w", err)
	}

	// 加载目标快照
	targetSnap, err := loadRemoteSnapshot(client, key, snapID)
	if err != nil {
		return fmt.Errorf("加载快照 %s 失败: %w", snapID, err)
	}

	// 扫描当前文件
	scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
	scanResult, err := scanner.Scan()
	if err != nil {
		return fmt.Errorf("扫描失败: %w", err)
	}

	// 对比差异
	currentSnap := snapshot.CreateSnapshot("", cfg.Device.ID, "", scanResult.Files)
	changes := currentSnap.Diff(targetSnap)

	if len(changes) == 0 {
		fmt.Println("当前状态与目标快照一致，无需回滚")
		return nil
	}

	// 显示变更
	fmt.Printf("将回滚到快照 %s，以下文件将变更:\n", snapID)
	for _, c := range changes {
		switch c.Type {
		case snapshot.Added:
			fmt.Printf("  ↓ 恢复 %s\n", c.Path)
		case snapshot.Modified:
			fmt.Printf("  ~ 还原 %s\n", c.Path)
		case snapshot.Deleted:
			fmt.Printf("  × 删除 %s（目标快照中不存在）\n", c.Path)
		}
	}

	fmt.Print("\n确认回滚？[y/N] ")
	var answer string
	fmt.Scanln(&answer)
	if strings.ToLower(answer) != "y" {
		return fmt.Errorf("取消回滚")
	}

	// 应用变更
	store := object.NewStore(client, key, "")
	applied := 0
	for _, c := range changes {
		fullPath, err := safeClaudePath(c.Path)
		if err != nil {
			return err
		}
		if c.Type == snapshot.Deleted {
			if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("删除文件 %s 失败: %w", c.Path, err)
			}
			fmt.Printf("  × 删除 %s\n", c.Path)
			applied++
			continue
		}

		// 下载目标版本的文件
		targetEntry, ok := targetSnap.Files[c.Path]
		if !ok {
			return fmt.Errorf("目标快照缺少文件: %s", c.Path)
		}

		data, err := store.Download(targetEntry.Hash)
		if err != nil {
			return fmt.Errorf("下载文件 %s 失败: %w", c.Path, err)
		}

		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, data, 0600); err != nil {
			return fmt.Errorf("写入文件 %s 失败: %w", c.Path, err)
		}

		fmt.Printf("  ✓ %s\n", c.Path)
		applied++
	}

	// 创建 revert 快照
	newSnap := snapshot.CreateSnapshot(localHead, cfg.Device.ID, "revert to "+snapID, targetSnap.Files)

	// 上传快照
	if err := uploadSnapshot(client, store, newSnap); err != nil {
		return fmt.Errorf("上传快照失败: %w", err)
	}

	if err := pushUpdateHEAD(client, cfg, newSnap.ID, localHead); err != nil {
		return err
	}
	if err := updateLocalHEAD(newSnap.ID); err != nil {
		return fmt.Errorf("更新本地 HEAD 失败: %w", err)
	}
	if err := saveLocalSnapshot(newSnap); err != nil {
		return fmt.Errorf("缓存快照失败: %w", err)
	}

	fmt.Printf("\n已回滚到快照 %s（%d 个文件变更）\n", snapID, applied)
	return nil
}
