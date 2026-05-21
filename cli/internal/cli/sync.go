// sync 命令
// pull + push 一步完成
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "拉取远程变更并推送本地变更（pull + push）",
	RunE:  runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().Bool("dry-run", false, "仅显示变更，不实际操作")
	syncCmd.Flags().StringP("message", "m", "", "提交信息")
}

func runSync(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	message, _ := cmd.Flags().GetString("message")

	fmt.Println("=== 拉取远程变更 ===")
	// sync 的 dry-run 通过直接跳过实际操作来实现
	if dryRun {
		fmt.Println("(dry-run 模式，跳过实际拉取)")
	} else {
		if err := runPull(pullCmd, []string{}); err != nil {
			return fmt.Errorf("pull 失败: %w", err)
		}
	}

	fmt.Println()
	fmt.Println("=== 推送本地变更 ===")
	if dryRun {
		// dry-run 模式下用 push 的 dry-run 展示变更
		pullCmd.Flags().Set("dry-run", "false")
		pushCmd.Flags().Set("dry-run", "true")
		runPush(pushCmd, []string{})
		pushCmd.Flags().Set("dry-run", "false")
	} else {
		if message != "" {
			pushCmd.Flags().Set("message", message)
		}
		if err := runPush(pushCmd, []string{}); err != nil {
			return fmt.Errorf("push 失败: %w", err)
		}
		if message != "" {
			pushCmd.Flags().Set("message", "")
		}
	}

	fmt.Println()
	fmt.Println("同步完成")
	return nil
}
