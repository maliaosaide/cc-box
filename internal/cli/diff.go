// diff 命令
// 显示文件内容在本地与远程之间的具体差异
package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/cc-box/internal/config"
	"github.com/user/cc-box/internal/object"
	"github.com/user/cc-box/internal/snapshot"
	"github.com/user/cc-box/internal/webdav"
)

var diffCmd = &cobra.Command{
	Use:   "diff [FILE]",
	Short: "查看文件内容差异",
	Long:  "显示本地与远程之间的文件内容差异。不带参数显示所有变更文件摘要，指定文件显示详细 diff。",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDiff,
}

func init() {
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	cfg, client, key, err := loadClientAndKey()
	if err != nil {
		return err
	}

	// 读取远程 HEAD
	remoteHeadData, _, err := client.GET("HEAD")
	if err != nil {
		return fmt.Errorf("读取远程 HEAD 失败: %w", err)
	}
	remoteHead := string(remoteHeadData)

	// 加载远程快照
	remoteSnap, err := loadRemoteSnapshot(client, key, remoteHead)
	if err != nil {
		return fmt.Errorf("加载远程快照失败: %w", err)
	}

	// 扫描本地文件
	scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
	scanResult, err := scanner.Scan()
	if err != nil {
		return fmt.Errorf("扫描失败: %w", err)
	}

	// 计算差异
	currentSnap := snapshot.CreateSnapshot("", cfg.Device.ID, "", scanResult.Files)
	changes := remoteSnap.Diff(currentSnap)

	if len(changes) == 0 {
		fmt.Println("本地与远程一致，没有差异")
		return nil
	}

	// 如果指定了文件
	if len(args) == 1 {
		return showFileDiff(args[0], changes, remoteSnap, client, key)
	}

	// 显示所有变更摘要
	fmt.Printf("变更文件 (%d):\n\n", len(changes))
	for _, c := range changes {
		switch c.Type {
		case snapshot.Added:
			fmt.Printf("  A  %s  (+%s)\n", c.Path, formatSize(c.NewSize))
		case snapshot.Modified:
			sizeDiff := c.NewSize - c.OldSize
			sign := "+"
			if sizeDiff < 0 {
				sign = ""
			}
			fmt.Printf("  M  %s  (%s%s)\n", c.Path, sign, formatSize(sizeDiff))
		case snapshot.Deleted:
			fmt.Printf("  D  %s  (-%s)\n", c.Path, formatSize(c.OldSize))
		}
	}
	fmt.Println("\n使用 'cc-box diff <file>' 查看具体内容差异")
	return nil
}

func showFileDiff(targetPath string, changes []snapshot.Change, remoteSnap *snapshot.Snapshot, client *webdav.Client, key []byte) error {
	// 找到目标文件的变更
	var found *snapshot.Change
	for i := range changes {
		if changes[i].Path == targetPath {
			found = &changes[i]
			break
		}
	}

	if found == nil {
		fmt.Printf("文件 %s 没有差异\n", targetPath)
		return nil
	}

	// 获取本地内容
	var localData []byte
	if found.Type != snapshot.Deleted {
		localPath := filepath.Join(config.ClaudeDir(), filepath.FromSlash(targetPath))
		data, err := os.ReadFile(localPath)
		if err != nil {
			return fmt.Errorf("读取本地文件失败: %w", err)
		}
		localData = normalizeContent(data)
	}

	// 获取远程内容
	var remoteData []byte
	if found.Type != snapshot.Added {
		remoteEntry, ok := remoteSnap.Files[targetPath]
		if !ok {
			return fmt.Errorf("远程快照中找不到 %s", targetPath)
		}

		store := object.NewStore(client, key, "")
		data, err := store.Download(remoteEntry.Hash)
		if err != nil {
			return fmt.Errorf("下载远程文件失败: %w", err)
		}
		remoteData = data
	}

	// 显示差异
	switch found.Type {
	case snapshot.Added:
		fmt.Printf("--- /dev/null\n")
		fmt.Printf("+++ %s (本地)\n", targetPath)
		for _, line := range bytes.Split(localData, []byte("\n")) {
			fmt.Printf("+%s\n", line)
		}
	case snapshot.Deleted:
		fmt.Printf("--- %s (远程)\n", targetPath)
		fmt.Printf("+++ /dev/null\n")
		for _, line := range bytes.Split(remoteData, []byte("\n")) {
			fmt.Printf("-%s\n", line)
		}
	case snapshot.Modified:
		fmt.Printf("--- %s (远程)\n", targetPath)
		fmt.Printf("+++ %s (本地)\n", targetPath)
		showUnifiedDiff(remoteData, localData)
	}

	return nil
}

// showUnifiedDiff 简单的 unified diff 输出
func showUnifiedDiff(oldData, newData []byte) {
	oldLines := splitLines(oldData)
	newLines := splitLines(newData)

	// 简单逐行对比，找差异块
	oIdx, nIdx := 0, 0
	contextLines := 3

	for oIdx < len(oldLines) || nIdx < len(newLines) {
		// 跳过相同行
		for oIdx < len(oldLines) && nIdx < len(newLines) && oldLines[oIdx] == newLines[nIdx] {
			oIdx++
			nIdx++
		}

		if oIdx >= len(oldLines) && nIdx >= len(newLines) {
			break
		}

		// 找到差异块，显示上下文
		startOld := oIdx - contextLines
		if startOld < 0 {
			startOld = 0
		}
		// 显示前导上下文
		for i := startOld; i < oIdx; i++ {
			fmt.Printf(" %s\n", oldLines[i])
		}

		// 显示删除的行
		delEnd := oIdx
		for delEnd < len(oldLines) && (nIdx >= len(newLines) || oldLines[delEnd] != newLines[nIdx]) {
			// 检查是否在 newLines 中还能找到匹配
			found := false
			for ni := nIdx; ni < len(newLines); ni++ {
				if oldLines[delEnd] == newLines[ni] {
					found = true
					break
				}
			}
			if found {
				break
			}
			delEnd++
		}
		for i := oIdx; i < delEnd; i++ {
			fmt.Printf("-%s\n", oldLines[i])
		}

		// 显示新增的行
		addEnd := nIdx
		for addEnd < len(newLines) && (oIdx >= len(oldLines) || newLines[addEnd] != oldLines[oIdx]) {
			found := false
			for oi := oIdx; oi < len(oldLines); oi++ {
				if newLines[addEnd] == oldLines[oi] {
					found = true
					break
				}
			}
			if found {
				break
			}
			addEnd++
		}
		for i := nIdx; i < addEnd; i++ {
			fmt.Printf("+%s\n", newLines[i])
		}

		// 显示后置上下文
		ctxCount := contextLines
		endOld := oIdx
		if delEnd > endOld {
			endOld = delEnd
		}
		endNew := nIdx
		if addEnd > endNew {
			endNew = addEnd
		}
		for i := 0; i < ctxCount && endOld+i < len(oldLines) && endNew+i < len(newLines); i++ {
			if oldLines[endOld+i] == newLines[endNew+i] {
				fmt.Printf(" %s\n", oldLines[endOld+i])
			} else {
				break
			}
			ctxCount++
		}

		oIdx = endOld
		nIdx = endNew
	}
}

func splitLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	// 去掉末尾空行（由末尾 \n 产生）
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
