// 三方合并主逻辑
// 根据文件类型分发到对应的合并器
package sync

import "fmt"

// Merger 三方合并器
type Merger struct {
	conflictStrategy string // ask / local / remote
}

// NewMerger 创建合并器
func NewMerger(conflictStrategy string) *Merger {
	return &Merger{conflictStrategy: conflictStrategy}
}

// MergeFile 对单个文件执行三方合并
func (m *Merger) MergeFile(path string, input *ThreeWayInput) (*MergeResult, error) {
	// 快速路径：仅一方修改
	if input.LocalOnly() {
		return &MergeResult{Path: path, Action: ActionKeepLocal, Data: input.Local}, nil
	}
	if input.RemoteOnly() {
		return &MergeResult{Path: path, Action: ActionKeepRemote, Data: input.Remote}, nil
	}

	// 双方都没修改
	if !input.HasLocalChange() && !input.HasRemoteChange() {
		return &MergeResult{Path: path, Action: ActionNoChange, Data: input.Local}, nil
	}

	// 双方都修改，且内容相同
	if string(input.Local) == string(input.Remote) {
		return &MergeResult{Path: path, Action: ActionMerged, Data: input.Local}, nil
	}

	// 双方都修改且不同 → 按文件类型合并
	fileType := ClassifyFile(path)
	switch fileType {
	case FileText:
		return m.mergeText(path, input)
	case FileJSON:
		return m.mergeJSON(path, input)
	case FileHistory:
		return m.mergeHistory(path, input)
	default:
		// 二进制文件无法合并
		return m.resolveBinaryConflict(path, input)
	}
}

// resolveBinaryConflict 处理二进制文件冲突
func (m *Merger) resolveBinaryConflict(path string, input *ThreeWayInput) (*MergeResult, error) {
	switch m.conflictStrategy {
	case "local":
		return &MergeResult{Path: path, Action: ActionKeepLocal, Data: input.Local}, nil
	case "remote":
		return &MergeResult{Path: path, Action: ActionKeepRemote, Data: input.Remote}, nil
	default:
		return &MergeResult{
			Path:      path,
			Action:    ActionConflict,
			Data:      input.Local,
			Conflicts: []string{fmt.Sprintf("二进制文件冲突，本地和远程版本不同")},
		}, nil
	}
}
