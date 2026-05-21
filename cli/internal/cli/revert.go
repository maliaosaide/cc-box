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

	cfg, client, key, err := loadClientAndKey()
	if err != nil {
		return err
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
	changes := targetSnap.Diff(currentSnap)

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
		if c.Type == snapshot.Deleted {
			// 删除本地文件
			fullPath := filepath.Join(config.ClaudeDir(), filepath.FromSlash(c.Path))
			os.Remove(fullPath)
			fmt.Printf("  × 删除 %s\n", c.Path)
			applied++
			continue
		}

		// 下载目标版本的文件
		targetEntry, ok := targetSnap.Files[c.Path]
		if !ok {
			continue
		}

		data, err := store.Download(targetEntry.Hash)
		if err != nil {
			fmt.Printf("  下载失败 %s: %v\n", c.Path, err)
			continue
		}

		fullPath := filepath.Join(config.ClaudeDir(), filepath.FromSlash(c.Path))
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err := os.WriteFile(fullPath, data, 0600); err != nil {
			fmt.Printf("  写入失败 %s: %v\n", c.Path, err)
			continue
		}

		fmt.Printf("  ✓ %s\n", c.Path)
		applied++
	}

	// 创建 revert 快照
	localHead, _ := loadLocalHEAD()
	newSnap := snapshot.CreateSnapshot(localHead, cfg.Device.ID, "revert to "+snapID, targetSnap.Files)

	// 上传快照
	if err := uploadSnapshot(client, store, newSnap); err != nil {
		return fmt.Errorf("上传快照失败: %w", err)
	}

	updateRemoteHEAD(client, newSnap.ID, "")
	updateLocalHEAD(newSnap.ID)
	saveLocalSnapshot(newSnap)

	fmt.Printf("\n已回滚到快照 %s（%d 个文件变更）\n", snapID, applied)
	return nil
}
