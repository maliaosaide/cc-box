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
}

func runSync(cmd *cobra.Command, args []string) error {
	fmt.Println("=== 拉取远程变更 ===")
	if err := runPull(cmd, args); err != nil {
		return fmt.Errorf("pull 失败: %w", err)
	}

	fmt.Println()
	fmt.Println("=== 推送本地变更 ===")
	if err := runPush(cmd, args); err != nil {
		return fmt.Errorf("push 失败: %w", err)
	}

	fmt.Println()
	fmt.Println("同步完成")
	return nil
}
