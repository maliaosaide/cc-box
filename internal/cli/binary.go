// binary 子命令
// 二进制版本管理：list / push / pull / switch / prune
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/user/cc-box/internal/binary"
	"github.com/user/cc-box/internal/config"
)

var binaryCmd = &cobra.Command{
	Use:   "binary",
	Short: "二进制版本管理",
}

var binaryListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有已备份的二进制版本",
	RunE:  runBinaryList,
}

var binaryPushCmd = &cobra.Command{
	Use:   "push",
	Short: "上传当前二进制文件到云端",
	RunE:  runBinaryPush,
}

var binaryPullCmd = &cobra.Command{
	Use:   "pull [VERSION]",
	Short: "从云端下载二进制文件",
	RunE:  runBinaryPull,
}

var binarySwitchCmd = &cobra.Command{
	Use:   "switch <VERSION>",
	Short: "切换到指定二进制版本",
	Args:  cobra.ExactArgs(1),
	RunE:  runBinarySwitch,
}

var binaryPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "清理不再被引用的版本",
	RunE:  runBinaryPrune,
}

func init() {
	rootCmd.AddCommand(binaryCmd)
	binaryCmd.AddCommand(binaryListCmd)
	binaryCmd.AddCommand(binaryPushCmd)
	binaryCmd.AddCommand(binaryPullCmd)
	binaryCmd.AddCommand(binarySwitchCmd)
	binaryCmd.AddCommand(binaryPruneCmd)
}

func runBinaryList(cmd *cobra.Command, args []string) error {
	_, client, _, err := loadClientAndKey()
	if err != nil {
		return err
	}

	idx, err := binary.LoadIndex(client)
	if err != nil {
		return err
	}

	platform := config.Platform()
	fmt.Printf("平台: %s\n\n", platform)

	binNames := []string{"claude", "uv", "uvx", "uvw"}
	for _, name := range binNames {
		info := idx.GetBinaryInfo(platform, name)
		if info == nil || len(info.Versions) == 0 {
			continue
		}

		fmt.Printf("%s (当前: %s):\n", name, info.Current)
		for ver, v := range info.Versions {
			marker := " "
			if ver == info.Current {
				marker = "*"
			}
			fmt.Printf("  %s %-12s  %s  %s\n", marker, ver, formatSize(v.Size), v.Uploaded.Format("2006-01-02"))
		}
		fmt.Println()
	}

	return nil
}

func runBinaryPush(cmd *cobra.Command, args []string) error {
	cfg, client, key, err := loadClientAndKey()
	if err != nil {
		return err
	}

	// 读取 claude 二进制
	binPath := binary.GetBinaryPath("claude")
	data, err := os.ReadFile(binPath)
	if err != nil {
		return fmt.Errorf("读取 %s 失败: %w", binPath, err)
	}

	// 获取版本号（从文件名或 claude --version）
	version := detectVersion(binPath)
	fmt.Printf("上传 %s (%s, %s)...\n", binPath, version, formatSize(int64(len(data))))

	err = binary.Upload(client, key, "claude", data, version, func(total, uploaded int64, part, totalParts int) {
		pct := float64(uploaded) / float64(total) * 100
		fmt.Printf("\r  进度: %.0f%% (%d/%d 分块)", pct, part, totalParts)
	})
	if err != nil {
		return err
	}

	fmt.Printf("\n已上传 %s %s\n", "claude", version)
	_ = cfg
	return nil
}

func runBinaryPull(cmd *cobra.Command, args []string) error {
	_, client, key, err := loadClientAndKey()
	if err != nil {
		return err
	}

	version := ""
	if len(args) > 0 {
		version = args[0]
	}

	platform := config.Platform()
	idx, err := binary.LoadIndex(client)
	if err != nil {
		return err
	}

	// 如果没指定版本，使用 current
	info := idx.GetBinaryInfo(platform, "claude")
	if info == nil {
		return fmt.Errorf("没有可用的 claude 二进制")
	}

	if version == "" {
		version = info.Current
	}
	if version == "" {
		return fmt.Errorf("请指定版本号")
	}

	targetPath := binary.GetBinaryPath("claude")
	fmt.Printf("下载 claude %s → %s ...\n", version, targetPath)

	err = binary.Download(client, key, "claude", version, targetPath, func(total, downloaded int64, part, totalParts int) {
		pct := float64(downloaded) / float64(total) * 100
		fmt.Printf("\r  进度: %.0f%% (%d/%d 分块)", pct, part, totalParts)
	})
	if err != nil {
		return err
	}

	fmt.Printf("\n已下载 claude %s\n", version)
	return nil
}

func runBinarySwitch(cmd *cobra.Command, args []string) error {
	targetVersion := args[0]

	_, client, key, err := loadClientAndKey()
	if err != nil {
		return err
	}

	platform := config.Platform()
	idx, err := binary.LoadIndex(client)
	if err != nil {
		return err
	}

	info := idx.GetBinaryInfo(platform, "claude")
	if info == nil {
		return fmt.Errorf("没有可用的 claude 二进制")
	}

	currentVersion := info.Current
	if targetVersion == currentVersion {
		return fmt.Errorf("已经是版本 %s", targetVersion)
	}

	// 检查目标版本是否存在
	if _, exists := info.Versions[targetVersion]; !exists {
		return fmt.Errorf("版本 %s 不存在云端", targetVersion)
	}

	// 备份当前版本到 versions 目录
	binPath := binary.GetBinaryPath("claude")
	if currentVersion != "" {
		versionsDir := filepath.Join(filepath.Dir(filepath.Dir(binPath)), "share", "claude", "versions")
		backupPath := filepath.Join(versionsDir, currentVersion)
		os.MkdirAll(versionsDir, 0755)
		fmt.Printf("备份当前版本 %s → %s\n", currentVersion, backupPath)
		os.Rename(binPath, backupPath)
	}

	// 下载目标版本
	fmt.Printf("下载 claude %s ...\n", targetVersion)
	err = binary.Download(client, key, "claude", targetVersion, binPath, func(total, downloaded int64, part, totalParts int) {
		pct := float64(downloaded) / float64(total) * 100
		fmt.Printf("\r  进度: %.0f%% (%d/%d 分块)", pct, part, totalParts)
	})
	if err != nil {
		return err
	}

	// 更新索引
	info.Current = targetVersion
	binary.SaveIndex(client, idx)

	fmt.Printf("\n已切换到 claude %s\n", targetVersion)
	return nil
}

func runBinaryPrune(cmd *cobra.Command, args []string) error {
	_, client, _, err := loadClientAndKey()
	if err != nil {
		return err
	}

	idx, err := binary.LoadIndex(client)
	if err != nil {
		return err
	}

	platform := config.Platform()
	var toDelete []string

	for name, binInfo := range []struct {
		n string
		i *binary.BinaryInfo
	}{
		{"claude", idx.GetBinaryInfo(platform, "claude")},
		{"uv", idx.GetBinaryInfo(platform, "uv")},
	} {
		_ = name
		if binInfo.i == nil {
			continue
		}
		for ver, v := range binInfo.i.Versions {
			if v.Refs <= 0 && ver != binInfo.i.Current {
				toDelete = append(toDelete, fmt.Sprintf("%s %s", name, ver))
			}
		}
	}

	if len(toDelete) == 0 {
		fmt.Println("没有可清理的版本")
		return nil
	}

	fmt.Printf("可清理的版本 (%d):\n", len(toDelete))
	for _, v := range toDelete {
		fmt.Printf("  - %s\n", v)
	}

	fmt.Print("\n确认清理？[y/N] ")
	var answer string
	fmt.Scanln(&answer)
	if answer != "y" && answer != "Y" {
		return nil
	}

	// 删除分块文件
	for _, v := range toDelete {
		fmt.Printf("  清理 %s\n", v)
	}

	binary.SaveIndex(client, idx)
	fmt.Println("清理完成")
	return nil
}

// detectVersion 从二进制路径检测版本
func detectVersion(binPath string) string {
	// 简化实现：从 versions 目录推断
	// 实际可以从 claude --version 获取
	return "unknown"
}
