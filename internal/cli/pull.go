// pull 命令
// 下载远程快照 → 差异对比 → 下载 objects → 应用到本地
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

	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("请先运行 cc-box init")
	}

	// 加载密钥
	key, err := crypto.LoadKey(config.KeyPath())
	if err != nil {
		return fmt.Errorf("加载密钥失败")
	}

	// 创建 WebDAV 客户端
	pass, err := config.LoadWebDAVPassword()
	if err != nil {
		return err
	}
	client := webdav.NewClient(cfg.WebDAV.URL, cfg.WebDAV.Username, pass)

	// 读取远程 HEAD
	remoteHeadData, _, err := client.GET("HEAD")
	if err != nil {
		if err == webdav.ErrNotFound {
			return fmt.Errorf("远程没有数据")
		}
		return fmt.Errorf("读取远程 HEAD 失败: %w", err)
	}
	remoteHead := string(remoteHeadData)

	// 读取本地 HEAD
	localHead, err := loadLocalHEAD()
	if err != nil {
		localHead = ""
	}

	// 已是最新
	if localHead == remoteHead {
		fmt.Println("已是最新")
		return nil
	}

	fmt.Printf("远程快照: %s\n", remoteHead)
	fmt.Printf("本地快照: %s\n", localHead)

	// 下载远程快照
	remoteSnap, err := loadRemoteSnapshot(client, key, remoteHead)
	if err != nil {
		return fmt.Errorf("加载远程快照失败: %w", err)
	}

	// 扫描本地文件
	scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
	scanResult, err := scanner.Scan()
	if err != nil {
		return fmt.Errorf("扫描本地文件失败: %w", err)
	}

	// 计算本地快照
	localSnap := snapshot.CreateSnapshot(localHead, cfg.Device.ID, "", scanResult.Files)

	// Phase 1: 简单策略 - 对比远程快照和本地文件状态
	// 对于远程有但本地没有的文件 → 下载
	// 对于远程和本地哈希不同的文件 → 下载远程版本
	// 对于本地有但远程没有的文件 → 保留本地（不删除）
	var toDownload []string
	var conflicts []string

	for path, remoteEntry := range remoteSnap.Files {
		localEntry, exists := localSnap.Files[path]
		if !exists {
			// 本地没有，下载
			toDownload = append(toDownload, path)
		} else if localEntry.Hash != remoteEntry.Hash {
			// 哈希不同
			if localHead != "" {
				// 有本地快照基线，可以判断变更来源
				conflicts = append(conflicts, path)
			} else {
				// 无基线，采纳远程
				toDownload = append(toDownload, path)
			}
		}
	}

	if len(toDownload) == 0 && len(conflicts) == 0 {
		// 只需更新 HEAD
		updateLocalHEAD(remoteHead)
		saveLocalSnapshot(remoteSnap)
		fmt.Println("已是最新")
		return nil
	}

	// 显示变更
	fmt.Printf("\n远程变更:\n")
	for _, path := range toDownload {
		fmt.Printf("  ↓ %s\n", path)
	}
	if len(conflicts) > 0 {
		fmt.Printf("\n冲突文件（远程优先）:\n")
		for _, path := range conflicts {
			fmt.Printf("  ⚠ %s\n", path)
		}
		// Phase 1: 冲突文件也用远程版本
		toDownload = append(toDownload, conflicts...)
	}

	if dryRun {
		fmt.Println("\n(dry-run 模式，未实际下载)")
		return nil
	}

	// 下载并应用文件
	store := object.NewStore(client, key, "")
	applied := 0
	for _, path := range toDownload {
		remoteEntry, ok := remoteSnap.Files[path]
		if !ok {
			continue
		}

		data, err := store.Download(remoteEntry.Hash)
		if err != nil {
			fmt.Printf("  下载失败 %s: %v\n", path, err)
			continue
		}

		// 写入本地文件
		fullPath := filepath.Join(config.ClaudeDir(), strings.ReplaceAll(path, "/", string(filepath.Separator)))
		os.MkdirAll(filepath.Dir(fullPath), 0700)
		if err := os.WriteFile(fullPath, data, 0600); err != nil {
			fmt.Printf("  写入失败 %s: %v\n", path, err)
			continue
		}
		applied++
	}

	// 更新本地 HEAD
	updateLocalHEAD(remoteHead)
	saveLocalSnapshot(remoteSnap)

	fmt.Printf("\n已拉取 %d 个文件（快照 %s）\n", applied, remoteHead)
	return nil
}
