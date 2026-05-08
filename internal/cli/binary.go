// binary 子命令
// 二进制版本管理：list / push / pull / switch / prune
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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

// binaryName 二进制名称，默认 claude
var binaryName string

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
	binaryPushCmd.Flags().StringVar(&binaryName, "name", "claude", "二进制名称 (claude/uv/uvx/uvw)")
	binaryPullCmd.Flags().StringVar(&binaryName, "name", "claude", "二进制名称 (claude/uv/uvx/uvw)")
	binarySwitchCmd.Flags().StringVar(&binaryName, "name", "claude", "二进制名称 (claude/uv/uvx/uvw)")
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
	name := binaryName
	cfg, client, key, err := loadClientAndKey()
	if err != nil {
		return err
	}

	binPath := binary.GetBinaryPath(name)
	data, err := os.ReadFile(binPath)
	if err != nil {
		return fmt.Errorf("读取 %s 失败: %w", binPath, err)
	}

	version := detectVersion(binPath)
	fmt.Printf("上传 %s (%s, %s)...\n", binPath, version, formatSize(int64(len(data))))

	err = binary.Upload(client, key, name, data, version, func(total, uploaded int64, part, totalParts int) {
		pct := float64(uploaded) / float64(total) * 100
		fmt.Printf("\r  进度: %.0f%% (%d/%d 分块)", pct, part, totalParts)
	})
	if err != nil {
		return err
	}

	fmt.Printf("\n已上传 %s %s\n", name, version)
	_ = cfg
	return nil
}

func runBinaryPull(cmd *cobra.Command, args []string) error {
	name := binaryName
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

	info := idx.GetBinaryInfo(platform, name)
	if info == nil {
		return fmt.Errorf("没有可用的 %s 二进制", name)
	}

	if version == "" {
		version = info.Current
	}
	if version == "" {
		return fmt.Errorf("请指定版本号")
	}

	targetPath := binary.GetBinaryPath(name)
	fmt.Printf("下载 %s %s → %s ...\n", name, version, targetPath)

	err = binary.Download(client, key, name, version, targetPath, func(total, downloaded int64, part, totalParts int) {
		pct := float64(downloaded) / float64(total) * 100
		fmt.Printf("\r  进度: %.0f%% (%d/%d 分块)", pct, part, totalParts)
	})
	if err != nil {
		return err
	}

	fmt.Printf("\n已下载 %s %s\n", name, version)
	return nil
}

func runBinarySwitch(cmd *cobra.Command, args []string) error {
	targetVersion := args[0]
	name := binaryName

	_, client, key, err := loadClientAndKey()
	if err != nil {
		return err
	}

	platform := config.Platform()
	idx, err := binary.LoadIndex(client)
	if err != nil {
		return err
	}

	info := idx.GetBinaryInfo(platform, name)
	if info == nil {
		return fmt.Errorf("没有可用的 %s 二进制", name)
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
	binPath := binary.GetBinaryPath(name)
	if currentVersion != "" {
		versionsDir := config.VersionsDir()
		backupPath := filepath.Join(versionsDir, currentVersion)
		os.MkdirAll(versionsDir, 0755)
		fmt.Printf("备份当前版本 %s → %s\n", currentVersion, backupPath)
		os.Rename(binPath, backupPath)
	}

	// 下载目标版本
	fmt.Printf("下载 %s %s ...\n", name, targetVersion)
	err = binary.Download(client, key, name, targetVersion, binPath, func(total, downloaded int64, part, totalParts int) {
		pct := float64(downloaded) / float64(total) * 100
		fmt.Printf("\r  进度: %.0f%% (%d/%d 分块)", pct, part, totalParts)
	})
	if err != nil {
		return err
	}

	// 更新索引
	info.Current = targetVersion
	binary.SaveIndex(client, idx)

	fmt.Printf("\n已切换到 %s %s\n", name, targetVersion)
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

	var targets []pruneTarget

	// 遍历所有平台的二进制
	for platform, pBins := range idx.Platforms {
		allBins := []*binary.BinaryInfo{
			pBins.Claude, pBins.UV, pBins.UVX, pBins.UVW,
		}
		names := []string{"claude", "uv", "uvx", "uvw"}
		for i, info := range allBins {
			if info == nil {
				continue
			}
			for ver, v := range info.Versions {
				// 安全规则：不删除 current 版本，不删除 refs > 0 的版本
				if ver == info.Current {
					continue
				}
				if v.Refs > 0 {
					continue
				}
				targets = append(targets, pruneTarget{
					platform: platform,
					name:     names[i],
					version:  ver,
					hash:     v.Hash,
				})
			}
		}
		// 检查 custom 二进制
		for name, info := range pBins.Custom {
			if info == nil {
				continue
			}
			for ver, v := range info.Versions {
				if ver == info.Current || v.Refs > 0 {
					continue
				}
				targets = append(targets, pruneTarget{
					platform: platform,
					name:     name,
					version:  ver,
					hash:     v.Hash,
				})
			}
		}
	}

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

	// 删除分块文件和更新索引
	cleaned := 0
	for _, t := range targets {
		// 删除分块目录
		if t.hash != "" {
			partsDir := "binaries/parts/" + t.hash + "/"
			client.DELETE(partsDir)
		}

	// 删除完整文件（尝试两种扩展名）
		for _, ext := range []string{".enc", ".bin"} {
			wholePath := fmt.Sprintf("binaries/%s/%s-%s%s", t.platform, t.name, t.version, ext)
			client.DELETE(wholePath)
		}

		// 从索引中移除
		info := idx.GetBinaryInfo(t.platform, t.name)
		if info != nil {
			delete(info.Versions, t.version)
		}

		fmt.Printf("  ✓ %s/%s %s\n", t.platform, t.name, t.version)
		cleaned++
	}

	// 保存更新后的索引
	binary.SaveIndex(client, idx)
	fmt.Printf("\n已清理 %d 个版本\n", cleaned)
	return nil
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
	cmd := exec.Command(binPath, "--version")
	if runtime.GOOS == "windows" {
		setHideWindow(cmd)
	}
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	// 输出格式: "2.1.126 (Claude Code)" 或类似
	fields := strings.Fields(string(output))
	if len(fields) > 0 {
		return fields[0]
	}
	return "unknown"
}
