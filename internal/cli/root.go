// CLI 根命令
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cc-box",
	Short: "CC-Box - 跨平台 Claude Code 配置箱",
	Long:  "配置同步 + 二进制备份 + 版本管理，一个工具搞定 Claude Code 的多设备管理。",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().Bool("allow-http", false, "允许 HTTP（非 HTTPS）连接")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "安静模式，减少输出")
}
