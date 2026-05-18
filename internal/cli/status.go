// status 命令
// 本地 vs 远程差异概览
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/cc-box/internal/config"
	"github.com/user/cc-box/internal/crypto"
	"github.com/user/cc-box/internal/snapshot"
	"github.com/user/cc-box/internal/webdav"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看本地与远程的同步状态",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
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
	client := webdav.NewClient(config.ConfiguredWebDAVURL(cfg), cfg.WebDAV.Username, pass)

	// 读取本地 HEAD
	localHead, err := loadLocalHEAD()
	if err != nil {
		localHead = ""
	}

	// 读取远程 HEAD
	remoteHeadData, _, err := client.GET("HEAD")
	if err != nil && err != webdav.ErrNotFound {
		return fmt.Errorf("读取远程 HEAD 失败: %w", err)
	}
	remoteHead := strings.TrimSpace(string(remoteHeadData))

	// 基本状态
	fmt.Printf("设备: %s (%s)\n", cfg.Device.Name, cfg.Device.ID)
	fmt.Printf("本地快照: %s\n", headDisplay(localHead))
	fmt.Printf("远程快照: %s\n", headDisplay(remoteHead))

	// 扫描本地文件
	scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
	scanResult, err := scanner.Scan()
	if err != nil {
		return fmt.Errorf("扫描失败: %w", err)
	}

	// 加载本地快照
	var localSnap *snapshot.Snapshot
	if localHead != "" {
		localSnap, err = loadRemoteSnapshot(client, key, localHead)
		if err != nil {
			localSnap = nil
		}
	}

	// 加载远程快照
	var remoteSnap *snapshot.Snapshot
	if remoteHead != "" {
		remoteSnap, err = loadRemoteSnapshot(client, key, remoteHead)
		if err != nil {
			return fmt.Errorf("加载远程快照失败: %w", err)
		}
	}

	var localChanges []snapshot.Change
	if localSnap != nil {
		currentSnap := snapshot.CreateSnapshot("", cfg.Device.ID, "", scanResult.Files)
		localChanges = localSnap.Diff(currentSnap)
	}

	if localHead == remoteHead && len(localChanges) == 0 {
		fmt.Println("\n状态: 已同步 ✓")
		return nil
	}

	fmt.Println("\n状态: 待同步")

	// 本地变更（相对于本地 HEAD）
	if len(localChanges) > 0 {
		fmt.Printf("\n本地变更 (%d):\n", len(localChanges))
		for _, c := range localChanges {
			fmt.Printf("  %s %s\n", changeIcon(c.Type), c.Path)
		}
	}

	// 远程变更（相对于本地 HEAD）
	if remoteSnap != nil && localHead != remoteHead {
		if localSnap != nil {
			remoteChanges := localSnap.Diff(remoteSnap)
			if len(remoteChanges) > 0 {
				fmt.Printf("\n远程变更 (%d):\n", len(remoteChanges))
				for _, c := range remoteChanges {
					fmt.Printf("  %s %s\n", changeIcon(c.Type), c.Path)
				}
			}
		} else {
			fmt.Printf("\n远程有 %d 个文件待拉取\n", len(remoteSnap.Files))
		}
	}

	return nil
}

func headDisplay(id string) string {
	if id == "" {
		return "(无)"
	}
	return id
}

func changeIcon(t snapshot.ChangeType) string {
	switch t {
	case snapshot.Added:
		return "A"
	case snapshot.Modified:
		return "M"
	case snapshot.Deleted:
		return "D"
	default:
		return "?"
	}
}
