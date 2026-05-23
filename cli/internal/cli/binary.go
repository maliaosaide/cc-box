// binary 子命令
// 二进制版本管理：list / push / pull / switch / prune
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/user/cc-box/core/binary"
	"github.com/user/cc-box/core/config"
)

var binaryCmd = &cobra.Command{
	Use:   "binary",
	Short: "Claude 二进制版本管理",
}

var binaryListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出已备份的 Claude 二进制版本",
	RunE:  runBinaryList,
}

var binaryPushCmd = &cobra.Command{
	Use:   "push",
	Short: "上传当前 Claude 二进制到云端",
	RunE:  runBinaryPush,
}

var binaryPullCmd = &cobra.Command{
	Use:   "pull [VERSION]",
	Short: "从云端下载 Claude 二进制",
	RunE:  runBinaryPull,
}

var binarySwitchCmd = &cobra.Command{
	Use:   "switch <VERSION>",
	Short: "切换到指定 Claude 二进制版本",
	Args:  cobra.ExactArgs(1),
	RunE:  runBinarySwitch,
}

var binaryPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "清理不再被引用的 Claude 版本",
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

	info := idx.GetBinaryInfo(platform, "claude")
	if info == nil || len(info.Versions) == 0 {
		return nil
	}

	fmt.Printf("claude (当前: %s):\n", info.Current)
	for ver, v := range info.Versions {
		marker := " "
		if ver == info.Current {
			marker = "*"
		}
		fmt.Printf("  %s %-12s  %s  %s\n", marker, ver, formatSize(v.Size), v.Uploaded.Format("2006-01-02"))
	}
	fmt.Println()

	return nil
}

func runBinaryPush(cmd *cobra.Command, args []string) error {
	cfg, client, key, err := loadClientAndKey()
	if err != nil {
		return err
	}

	resolution := binary.ResolveClaudeBinary()
	if !resolution.Valid {
		return fmt.Errorf("%s", resolution.Error)
	}
	if resolution.IsShim {
		return fmt.Errorf("当前 Claude 路径是脚本 shim，不支持上传；请手动选择真实二进制或使用受管目录")
	}
	binPath := resolution.CurrentPath
	version := resolution.Version
	data, err := os.ReadFile(binPath)
	if err != nil {
		return fmt.Errorf("读取 %s 失败: %w", binPath, err)
	}

	if version == "" {
		version = detectVersion(binPath)
	}
	fmt.Printf("上传 %s (%s, %s)...\n", binPath, version, formatSize(int64(len(data))))

	err = binary.Upload(client, key, "claude", data, version, func(total, uploaded int64, part, totalParts int) {
		pct := float64(uploaded) / float64(total) * 100
		fmt.Printf("\r  进度: %.0f%% (%d/%d 分块)", pct, part, totalParts)
	})
	if err != nil {
		return err
	}

	fmt.Printf("\n已上传 claude %s\n", version)
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

	info := idx.GetBinaryInfo(platform, "claude")
	if info == nil {
		return fmt.Errorf("没有可用的 Claude 二进制")
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

	_ = binary.ClearClaudeResolutionCache()
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
		return fmt.Errorf("没有可用的 Claude 二进制")
	}

	binPath := binary.GetBinaryPath("claude")
	currentVersion := ""
	if _, err := os.Stat(binPath); err == nil {
		currentVersion = detectVersion(binPath)
		if targetVersion == currentVersion {
			return fmt.Errorf("已经是版本 %s", targetVersion)
		}
	}

	if _, exists := info.Versions[targetVersion]; !exists {
		return fmt.Errorf("版本 %s 不存在云端", targetVersion)
	}

	if currentVersion != "" && currentVersion != "unknown" {
		versionsDir := config.VersionsDir()
		backupPath := filepath.Join(versionsDir, currentVersion)
		fmt.Printf("备份当前版本 %s → %s\n", currentVersion, backupPath)
		if err := binary.BackupFileIfMissing(binPath, backupPath); err != nil {
			return fmt.Errorf("备份当前版本失败: %w", err)
		}
	}

	fmt.Printf("下载 claude %s ...\n", targetVersion)
	err = binary.Download(client, key, "claude", targetVersion, binPath, func(total, downloaded int64, part, totalParts int) {
		pct := float64(downloaded) / float64(total) * 100
		fmt.Printf("\r  进度: %.0f%% (%d/%d 分块)", pct, part, totalParts)
	})
	if err != nil {
		return err
	}

	if err := binary.UpdateIndex(client, func(idx *binary.Index) error {
		info := idx.GetBinaryInfo(platform, "claude")
		if info == nil {
			return fmt.Errorf("没有可用的 Claude 二进制")
		}
		if _, exists := info.Versions[targetVersion]; !exists {
			return fmt.Errorf("版本 %s 不存在云端", targetVersion)
		}
		info.Current = targetVersion
		return nil
	}); err != nil {
		return fmt.Errorf("更新远程二进制索引失败: %w", err)
	}
	_ = binary.ClearClaudeResolutionCache()

	fmt.Printf("\n已切换到 claude %s\n", targetVersion)
	return nil
}

type pruneTarget struct {
	platform string
	name     string
	version  string
	hash     string
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
	targets := collectPruneTargets(idx, platform)

	if len(targets) == 0 {
		fmt.Println("没有可清理的版本")
		return nil
	}

	fmt.Printf("可清理的版本 (%d):\n\n", len(targets))
	for _, t := range targets {
		fmt.Printf("  %s/%s %-12s  %s\n", t.platform, t.name, t.version, t.hash)
	}
	fmt.Printf("\n总空间将释放: %s\n", formatSize(totalPruneSize(idx, targets)))

	fmt.Print("\n确认清理？[y/N] ")
	var answer string
	fmt.Scanln(&answer)
	if answer != "y" && answer != "Y" {
		return nil
	}

	cleaned := 0
	for _, t := range targets {
		if err := binary.DeleteRemoteVersion(client, nil, t.name, t.version, t.platform); err != nil {
			fmt.Printf("  删除失败 %s/%s %s: %v\n", t.platform, t.name, t.version, err)
			continue
		}

		fmt.Printf("  ✓ %s/%s %s\n", t.platform, t.name, t.version)
		cleaned++
	}

	fmt.Printf("\n已清理 %d 个版本\n", cleaned)
	return nil
}

func collectPruneTargets(idx *binary.Index, platform string) []pruneTarget {
	pBins, ok := idx.Platforms[platform]
	if !ok || pBins.Claude == nil {
		return nil
	}

	var targets []pruneTarget
	for ver, v := range pBins.Claude.Versions {
		if ver == pBins.Claude.Current || v.Refs > 0 {
			continue
		}
		targets = append(targets, pruneTarget{
			platform: platform,
			name:     "claude",
			version:  ver,
			hash:     v.Hash,
		})
	}
	return targets
}

func totalPruneSize(idx *binary.Index, targets []pruneTarget) int64 {
	var total int64
	for _, t := range targets {
		info := idx.GetBinaryInfo(t.platform, t.name)
		if info != nil {
			if v, ok := info.Versions[t.version]; ok {
				total += v.Size
			}
		}
	}
	return total
}

// detectVersion 从二进制路径检测版本
func detectVersion(binPath string) string {
	version, err := binary.DetectVersion(binPath)
	if err != nil {
		return "unknown"
	}
	return version
}
