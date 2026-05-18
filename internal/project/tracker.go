// 项目发现与 git remote 匹配
// 扫描 ~/.claude/projects/ 下的项目目录，匹配 git remote URL
package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// Project 项目信息
type Project struct {
	RemoteURL  string `json:"remote_url"`  // git remote URL（项目唯一标识）
	LocalPath  string `json:"local_path"`  // 本地项目根目录
	RemoteName string `json:"remote_name"` // 使用的 remote 名称（如 "origin"）
}

// OrphanProject 未匹配的远程项目
type OrphanProject struct {
	RemoteURL  string `json:"remote_url"`
	RemoteName string `json:"remote_name"`
	Discovered string `json:"discovered"` // 发现时间
}

// OrphanIndex orphan 项目索引
type OrphanIndex struct {
	Orphans []OrphanProject `json:"orphans"`
}

// TrackedIndex 用户手动添加的项目索引
type TrackedIndex struct {
	Projects []Project `json:"projects"`
}

// EncodeRemote 将 remote URL 编码为安全的路径名
func EncodeRemote(remoteURL string) string {
	h := sha256.Sum256([]byte(remoteURL))
	return hex.EncodeToString(h[:16])
}

// DiscoverProjects 扫描本地已配置的 Claude 项目
// 通过读取 ~/.claude/projects/ 下的目录结构发现项目
func DiscoverProjects() ([]Project, error) {
	home, _ := os.UserHomeDir()
	projectsDir := filepath.Join(home, ".claude", "projects")

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var projects []Project
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// ~/.claude/projects/ 下的目录名是编码后的项目路径
		// 格式可能是 -Users-a-Desktop-myproject 这样的

		// 检查是否有 .claude.json（通过路径映射找到实际项目目录）
		localPath := decodeProjectDir(entry.Name())
		if localPath == "" {
			continue
		}

		// 检查项目目录是否存在
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			continue
		}

		// 获取 git remote
		remoteURL, remoteName := GetGitRemote(localPath)
		if remoteURL == "" {
			// 使用路径哈希作为后备 ID
			remoteURL = "local:" + EncodeRemote(localPath)
		}

		// 检查是否有 .claude.json
		claudeJSON := filepath.Join(localPath, ".claude.json")
		if _, err := os.Stat(claudeJSON); err != nil {
			continue
		}

		projects = append(projects, Project{
			RemoteURL:  remoteURL,
			LocalPath:  localPath,
			RemoteName: remoteName,
		})
	}

	tracked, err := LoadTrackedIndex()
	if err == nil {
		seen := make(map[string]bool)
		for _, p := range projects {
			seen[p.LocalPath] = true
			seen[p.RemoteURL] = true
		}
		for _, p := range tracked.Projects {
			if p.LocalPath == "" || seen[p.LocalPath] || seen[p.RemoteURL] {
				continue
			}
			if _, err := os.Stat(filepath.Join(p.LocalPath, ".claude.json")); err != nil {
				continue
			}
			projects = append(projects, p)
			seen[p.LocalPath] = true
			seen[p.RemoteURL] = true
		}
	}

	return projects, nil
}

// GetGitRemote 获取项目的 git remote URL
// 优先 origin，其次第一个可用的 remote
func GetGitRemote(dir string) (url string, name string) {
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = dir
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	}
	output, err := cmd.Output()
	if err != nil {
		return "", ""
	}

	lines := strings.Split(string(output), "\n")
	var firstURL, firstRemote string

	for _, line := range lines {
		// 格式: origin  git@github.com:user/repo.git (fetch)
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		remoteName := parts[0]
		remoteURL := parts[1]

		// 去掉 (fetch)/(push) 后缀
		remoteURL = strings.TrimSuffix(remoteURL, " (fetch)")
		remoteURL = strings.TrimSuffix(remoteURL, " (push)")

		if remoteName == "origin" {
			return remoteURL, "origin"
		}

		if firstURL == "" {
			firstURL = remoteURL
			firstRemote = remoteName
		}
	}

	return firstURL, firstRemote
}

// decodeProjectDir 将 ~/.claude/projects/ 下的目录名还原为实际路径
// 目录名格式: -Users-a-Desktop-myproject → /Users/a/Desktop/myproject (macOS)
//
//	-C-Users-a-Desktop-myproject → C:\Users\a\Desktop\myproject (Windows)
func decodeProjectDir(dirName string) string {
	parts := strings.Split(dirName, "-")

	// Windows 路径: -C-Users-a-...
	if len(parts) > 1 && len(parts[1]) == 1 {
		// 驱动器号 + 路径
		result := parts[1] + ":"
		for _, p := range parts[2:] {
			if p != "" {
				result += "\\" + p
			}
		}
		return result
	}

	// Unix 路径: -Users-a-Desktop-...
	result := ""
	for _, p := range parts[1:] {
		if p != "" {
			result += "/" + p
		}
	}
	return result
}

// LoadClaudeJSON 读取项目的 .claude.json
func LoadClaudeJSON(projectPath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(filepath.Join(projectPath, ".claude.json"))
	if err != nil {
		return nil, err
	}

	var content map[string]interface{}
	if err := json.Unmarshal(data, &content); err != nil {
		return nil, fmt.Errorf("解析 .claude.json 失败: %w", err)
	}
	return content, nil
}

// SaveClaudeJSON 写入项目的 .claude.json
func SaveClaudeJSON(projectPath string, content map[string]interface{}) error {
	data, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(projectPath, ".claude.json"), data, 0600)
}

func trackedIndexPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cc-box", "tracked_projects.json")
}

func LoadTrackedIndex() (*TrackedIndex, error) {
	data, err := os.ReadFile(trackedIndexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &TrackedIndex{}, nil
		}
		return nil, err
	}
	var idx TrackedIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

func SaveTrackedIndex(idx *TrackedIndex) error {
	path := trackedIndexPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func AddTrackedProject(dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(absDir, ".claude.json")); err != nil {
		return fmt.Errorf("该目录下没有 .claude.json 文件")
	}
	remoteURL, remoteName := GetGitRemote(absDir)
	if remoteURL == "" {
		remoteURL = "local:" + EncodeRemote(absDir)
	}
	idx, err := LoadTrackedIndex()
	if err != nil {
		return err
	}
	for _, p := range idx.Projects {
		if p.LocalPath == absDir || p.RemoteURL == remoteURL {
			return nil
		}
	}
	idx.Projects = append(idx.Projects, Project{RemoteURL: remoteURL, LocalPath: absDir, RemoteName: remoteName})
	return SaveTrackedIndex(idx)
}

// LoadOrphanIndex 加载 orphan 索引
func LoadOrphanIndex() (*OrphanIndex, error) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".cc-box", "orphan_projects.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &OrphanIndex{}, nil
		}
		return nil, err
	}

	var idx OrphanIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

// SaveOrphanIndex 保存 orphan 索引
func SaveOrphanIndex(idx *OrphanIndex) error {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".cc-box", "orphan_projects.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
