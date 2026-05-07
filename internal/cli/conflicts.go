// conflicts / resolve 命令
// 列出冲突文件并交互式解决
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/cc-box/internal/config"
)

var conflictsCmd = &cobra.Command{
	Use:   "conflicts",
	Short: "列出未解决的冲突文件",
	RunE:  runConflicts,
}

var resolveCmd = &cobra.Command{
	Use:   "resolve <file>",
	Short: "交互式解决文件冲突",
	Args:  cobra.ExactArgs(1),
	RunE:  runResolve,
}

func init() {
	rootCmd.AddCommand(conflictsCmd)
	rootCmd.AddCommand(resolveCmd)
}

// conflictMarker 冲突标记文件目录
func conflictDir() string {
	return config.CCBoxDir() + "/conflicts"
}

func runConflicts(cmd *cobra.Command, args []string) error {
	dir := conflictDir()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		fmt.Println("没有未解决的冲突")
		return nil
	}

	fmt.Printf("未解决的冲突 (%d):\n", len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 去掉 .local / .remote 后缀
		base := strings.TrimSuffix(name, ".local")
		base = strings.TrimSuffix(base, ".remote")
		fmt.Printf("  %s\n", base)
	}
	return nil
}

func runResolve(cmd *cobra.Command, args []string) error {
	file := args[0]
	dir := conflictDir()

	localFile := dir + "/" + file + ".local"
	remoteFile := dir + "/" + file + ".remote"

	localData, err := os.ReadFile(localFile)
	if err != nil {
		return fmt.Errorf("本地版本不存在: %s", localFile)
	}
	remoteData, err := os.ReadFile(remoteFile)
	if err != nil {
		return fmt.Errorf("远程版本不存在: %s", remoteFile)
	}

	fmt.Printf("冲突文件: %s\n\n", file)
	fmt.Println("--- 本地版本 ---")
	printPreview(localData, 20)
	fmt.Println("\n--- 远程版本 ---")
	printPreview(remoteData, 20)

	fmt.Println("\n选择操作:")
	fmt.Println("  1) 保留本地版本")
	fmt.Println("  2) 采纳远程版本")
	fmt.Println("  3) 合并编辑")
	fmt.Print("\n请选择 [1/2/3]: ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	var result []byte
	switch choice {
	case "1":
		result = localData
	case "2":
		result = remoteData
	case "3":
		fmt.Println("请编辑文件后保存，然后运行 cc-box push")
		// 将本地版本写回目标位置，用户自行编辑
		result = localData
	default:
		return fmt.Errorf("无效选择")
	}

	// 写入目标文件
	targetPath := config.ClaudeDir() + "/" + strings.ReplaceAll(file, "/", string(os.PathSeparator))
	os.MkdirAll(strings.ReplaceAll(targetPath, "/", string(os.PathSeparator)), 0755)
	if err := os.WriteFile(targetPath, result, 0600); err != nil {
		return fmt.Errorf("写入失败: %w", err)
	}

	// 清理冲突文件
	os.Remove(localFile)
	os.Remove(remoteFile)

	fmt.Printf("已解决冲突: %s\n", file)
	return nil
}

func printPreview(data []byte, maxLines int) {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if i >= maxLines {
			fmt.Printf("  ... (%d more lines)\n", len(lines)-maxLines)
			break
		}
		fmt.Printf("  %s\n", line)
	}
}
