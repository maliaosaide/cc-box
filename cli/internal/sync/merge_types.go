// 同步引擎核心类型
// 定义三方合并的输入输出和文件类型判断
package sync

import (
	"path/filepath"
	"strings"
)

// MergeAction 合并动作
type MergeAction int

const (
	ActionKeepLocal  MergeAction = iota // 保留本地版本
	ActionKeepRemote                    // 采纳远程版本
	ActionMerged                        // 成功合并
	ActionConflict                      // 冲突，需手动解决
	ActionDelete                        // 删除
	ActionNoChange                      // 无变更
)

// MergeResult 单文件的合并结果
type MergeResult struct {
	Path       string
	Action     MergeAction
	Data       []byte   // 合并后的数据（ActionKeepLocal/ActionKeepRemote/Merged 时有值）
	Conflicts  []string // 冲突描述
}

// FileType 文件类型
type FileType int

const (
	FileText      FileType = iota // 文本文件
	FileJSON                      // JSON 文件
	FileHistory                   // history.jsonl
	FileDirectory                 // 目录
	FileBinary                    // 其他二进制文件
)

// ClassifyFile 根据路径判断文件类型
func ClassifyFile(path string) FileType {
	name := strings.ToLower(path)

	// history.jsonl 特殊处理
	if name == "history.jsonl" {
		return FileHistory
	}

	// JSON 文件
	if strings.HasSuffix(name, ".json") {
		return FileJSON
	}

	// 文本文件
	textExts := []string{".md", ".txt", ".toml", ".yaml", ".yml", ".jsonl", ".sh", ".bat", ".ps1", ".py", ".js", ".ts"}
	for _, ext := range textExts {
		if strings.HasSuffix(name, ext) {
			return FileText
		}
	}

	// 常见的 Claude 配置文件（无扩展名但实际是文本）
	base := filepath.Base(name)
	textNames := []string{"claude.md", "skill.md"}
	for _, n := range textNames {
		if base == n {
			return FileText
		}
	}

	return FileBinary
}

// ThreeWayInput 三方合并的输入
type ThreeWayInput struct {
	Ancestor []byte // 共同祖先版本（可能为 nil）
	Local    []byte // 本地当前版本
	Remote   []byte // 远程最新版本
}

// HasLocalChange 本地是否修改
func (i *ThreeWayInput) HasLocalChange() bool {
	return !bytesEqual(i.Ancestor, i.Local)
}

// HasRemoteChange 远程是否修改
func (i *ThreeWayInput) HasRemoteChange() bool {
	return !bytesEqual(i.Ancestor, i.Remote)
}

// LocalOnly 仅本地修改
func (i *ThreeWayInput) LocalOnly() bool {
	return i.HasLocalChange() && !i.HasRemoteChange()
}

// RemoteOnly 仅远程修改
func (i *ThreeWayInput) RemoteOnly() bool {
	return !i.HasLocalChange() && i.HasRemoteChange()
}

// BothChanged 双方都修改
func (i *ThreeWayInput) BothChanged() bool {
	return i.HasLocalChange() && i.HasRemoteChange()
}

func bytesEqual(a, b []byte) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return string(a) == string(b)
}
