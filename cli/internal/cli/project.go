// project 子命令
// 项目级 .claude.json 同步
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/cc-box/cli/internal/project"
	"github.com/user/cc-box/core/crypto"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "项目级配置同步",
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出已追踪的项目",
	RunE:  runProjectList,
}

var projectPushCmd = &cobra.Command{
	Use:   "push [PATH]",
	Short: "推送项目的 .claude.json 到云端",
	RunE:  runProjectPush,
}

var projectPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "拉取所有项目配置",
	RunE:  runProjectPull,
}

var projectOrphansCmd = &cobra.Command{
	Use:   "orphans",
	Short: "列出未匹配的远程项目",
	RunE:  runProjectOrphans,
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectPushCmd)
	projectCmd.AddCommand(projectPullCmd)
	projectCmd.AddCommand(projectOrphansCmd)
}

func runProjectList(cmd *cobra.Command, args []string) error {
	projects, err := project.DiscoverProjects()
	if err != nil {
		return fmt.Errorf("扫描项目失败: %w", err)
	}

	if len(projects) == 0 {
		fmt.Println("没有已追踪的项目")
		fmt.Println("在包含 .claude.json 的项目目录中使用 'cc-box project push' 开始追踪")
		return nil
	}

	fmt.Printf("%-40s %s\n", "Remote URL", "本地路径")
	fmt.Println(strings.Repeat("-", 80))
	for _, p := range projects {
		remote := p.RemoteURL
		if len(remote) > 38 {
			remote = "..." + remote[len(remote)-35:]
		}
		fmt.Printf("%-40s %s\n", remote, p.LocalPath)
	}

	return nil
}

func runProjectPush(cmd *cobra.Command, args []string) error {
	_, client, key, err := loadClientAndKey()
	if err != nil {
		return err
	}

	// 确定目标项目
	var targetPath string
	if len(args) > 0 {
		targetPath = args[0]
	} else {
		// 使用当前目录
		targetPath, _ = os.Getwd()
	}

	// 检查 .claude.json 是否存在
	claudeJSONPath := filepath.Join(targetPath, ".claude.json")
	data, err := os.ReadFile(claudeJSONPath)
	if err != nil {
		return fmt.Errorf("找不到 %s，请确保在项目根目录下运行", claudeJSONPath)
	}

	// 获取 remote URL
	remoteURL, remoteName := project.GetGitRemote(targetPath)
	if remoteURL == "" {
		remoteURL = "local:" + project.EncodeRemote(targetPath)
		remoteName = "local"
	}

	// 编码为安全路径
	encoded := project.EncodeRemote(remoteURL)
	remotePath := "projects/" + encoded + "/.claude.json.enc"

	// 加密
	encrypted, err := crypto.Encrypt(data, key)
	if err != nil {
		return fmt.Errorf("加密失败: %w", err)
	}

	// 上传
	if err := client.EnsureDir(remotePath); err != nil {
		return err
	}
	if _, err := client.PUT(remotePath, encrypted, ""); err != nil {
		return fmt.Errorf("上传失败: %w", err)
	}

	fmt.Printf("已推送 %s → %s\n", targetPath, remoteURL)
	if remoteName != "local" {
		fmt.Printf("  remote: %s (%s)\n", remoteURL, remoteName)
	}
	return nil
}

func runProjectPull(cmd *cobra.Command, args []string) error {
	_, client, key, err := loadClientAndKey()
	if err != nil {
		return err
	}

	// 列出云端项目
	files, err := listRemoteFilesRecursive(client, "projects/")
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			fmt.Println("云端没有项目配置")
			return nil
		}
		return fmt.Errorf("列出云端项目失败: %w", err)
	}

	// 发现本地项目
	localProjects, _ := project.DiscoverProjects()
	localMap := make(map[string]project.Project)
	for _, p := range localProjects {
		localMap[project.EncodeRemote(p.RemoteURL)] = p
	}

	// 加载 orphan 索引
	orphanIdx, _ := project.LoadOrphanIndex()

	pulled := 0
	for _, remotePath := range files {
		if !strings.HasSuffix(remotePath, ".claude.json.enc") {
			continue
		}

		// 从路径提取 encoded remote
		relPath := strings.TrimPrefix(remotePath, "projects/")
		parts := strings.Split(relPath, "/")
		if len(parts) < 2 {
			continue
		}
		encoded := parts[0]

		// 下载并解密
		encrypted, _, err := client.GET(remotePath)
		if err != nil {
			fmt.Printf("  下载失败: %v\n", err)
			continue
		}

		decrypted, err := crypto.Decrypt(encrypted, key)
		if err != nil {
			fmt.Printf("  解密失败: %v\n", err)
			continue
		}

		// 查找匹配的本地项目
		localProj, matched := localMap[encoded]
		if !matched {
			// 存为 orphan
			remoteURL := "unknown:" + encoded
			orphanIdx.Orphans = append(orphanIdx.Orphans, project.OrphanProject{
				RemoteURL:  remoteURL,
				RemoteName: encoded,
				Discovered: time.Now().UTC().Format(time.RFC3339),
			})
			continue
		}

		// 读取本地 .claude.json
		claudeJSONPath := filepath.Join(localProj.LocalPath, ".claude.json")
		var merged map[string]interface{}

		localData, err := os.ReadFile(claudeJSONPath)
		if err != nil {
			// 本地没有，直接使用远程版本
			if err := os.WriteFile(claudeJSONPath, decrypted, 0600); err != nil {
				fmt.Printf("  写入失败 %s: %v\n", localProj.LocalPath, err)
				continue
			}
			fmt.Printf("  ✓ %s (新文件)\n", localProj.LocalPath)
			pulled++
			continue
		}

		// 合并
		var localJSON, remoteJSON map[string]interface{}
		json.Unmarshal(localData, &localJSON)
		json.Unmarshal(decrypted, &remoteJSON)

		merged = project.MergeClaudeJSON(localJSON, remoteJSON)
		mergedData, _ := json.MarshalIndent(merged, "", "  ")

		if err := os.WriteFile(claudeJSONPath, mergedData, 0600); err != nil {
			fmt.Printf("  写入失败 %s: %v\n", localProj.LocalPath, err)
			continue
		}

		fmt.Printf("  ✓ %s\n", localProj.LocalPath)
		pulled++
	}

	// 保存 orphan 索引
	if len(orphanIdx.Orphans) > 0 {
		project.SaveOrphanIndex(orphanIdx)
		fmt.Printf("\n有 %d 个远程项目未匹配本地项目，使用 'cc-box project orphans' 查看\n", len(orphanIdx.Orphans))
	}

	fmt.Printf("\n已拉取 %d 个项目配置\n", pulled)
	return nil
}

func runProjectOrphans(cmd *cobra.Command, args []string) error {
	idx, err := project.LoadOrphanIndex()
	if err != nil {
		return err
	}

	if len(idx.Orphans) == 0 {
		fmt.Println("没有未匹配的远程项目")
		return nil
	}

	fmt.Printf("未匹配的远程项目 (%d):\n\n", len(idx.Orphans))
	for i, o := range idx.Orphans {
		fmt.Printf("  %d. %s\n", i+1, o.RemoteURL)
		fmt.Printf("     发现于: %s\n", o.Discovered)
	}
	fmt.Println("\n克隆对应的项目到本地后再次 'cc-box project pull' 即可自动匹配")
	return nil
}
