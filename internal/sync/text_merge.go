// 文本文件行级合并
// 类似 git merge-file 的三方合并
package sync

import (
	"strings"
)

// mergeText 对文本文件执行行级三方合并
func (m *Merger) mergeText(path string, input *ThreeWayInput) (*MergeResult, error) {
	ancestor := splitLines(input.Ancestor)
	local := splitLines(input.Local)
	remote := splitLines(input.Remote)

	result, hasConflict := threeWayMerge(ancestor, local, remote)

	if hasConflict {
		switch m.conflictStrategy {
		case "local":
			return &MergeResult{Path: path, Action: ActionKeepLocal, Data: input.Local}, nil
		case "remote":
			return &MergeResult{Path: path, Action: ActionKeepRemote, Data: input.Remote}, nil
		default:
			return &MergeResult{
				Path:      path,
				Action:    ActionConflict,
				Data:      []byte(strings.Join(result, "\n")),
				Conflicts: []string{"文本合并冲突"},
			}, nil
		}
	}

	return &MergeResult{
		Path:   path,
		Action: ActionMerged,
		Data:   []byte(strings.Join(result, "\n")),
	}, nil
}

// threeWayMerge 行级三方合并
// 使用简化的逐行对比策略：
// 1. 本地和远程各自相对祖先产生变更
// 2. 变更不重叠 → 合并
// 3. 变更重叠 → 冲突
func threeWayMerge(ancestor, local, remote []string) ([]string, bool) {
	if len(ancestor) == 0 {
		if linesEqual(local, remote) {
			return local, false
		}
		return generateConflictMarkers(local, remote), true
	}

	// 构建行索引：ancestor 行号 → local 行号，ancestor 行号 → remote 行号
	localMap := buildLineMap(ancestor, local)
	remoteMap := buildLineMap(ancestor, remote)

	// 逐行处理 ancestor
	var result []string
	hasConflict := false

	li := 0 // local cursor
	ri := 0 // remote cursor

	for ai := 0; ai < len(ancestor); ai++ {
		// 收集 local 中在 ancestor[ai] 之前新增的行
		localInserted := linesBeforeAnchor(local, li, localMap[ai])
		// 收集 remote 中在 ancestor[ai] 之前新增的行
		remoteInserted := linesBeforeAnchor(remote, ri, remoteMap[ai])

		if len(localInserted) > 0 && len(remoteInserted) > 0 {
			if linesEqual(localInserted, remoteInserted) {
				result = append(result, localInserted...)
			} else {
				hasConflict = true
				result = append(result, generateConflictMarkers(localInserted, remoteInserted)...)
			}
		} else if len(localInserted) > 0 {
			result = append(result, localInserted...)
		} else if len(remoteInserted) > 0 {
			result = append(result, remoteInserted...)
		}

		// ancestor[ai] 这一行是否被修改
		localModified := localMap[ai] == -1
		remoteModified := remoteMap[ai] == -1

		if localModified && remoteModified {
			// 双方都删除/修改了这一行
			// 检查各自的替换内容
			localReplaced := replacementLines(local, li, localMap, ai)
			remoteReplaced := replacementLines(remote, ri, remoteMap, ai)

			if linesEqual(localReplaced, remoteReplaced) {
				result = append(result, localReplaced...)
			} else {
				hasConflict = true
				result = append(result, generateConflictMarkers(localReplaced, remoteReplaced)...)
			}
			li = skipPast(localMap, ai, li)
			ri = skipPast(remoteMap, ai, ri)
		} else if localModified {
			replaced := replacementLines(local, li, localMap, ai)
			result = append(result, replaced...)
			li = skipPast(localMap, ai, li)
		} else if remoteModified {
			replaced := replacementLines(remote, ri, remoteMap, ai)
			result = append(result, replaced...)
			ri = skipPast(remoteMap, ai, ri)
		} else {
			// 未修改
			result = append(result, ancestor[ai])
			li = localMap[ai] + 1
			ri = remoteMap[ai] + 1
		}
	}

	// 处理尾部新增
	if li < len(local) || ri < len(remote) {
		tailLocal := local[li:]
		tailRemote := remote[ri:]
		if len(tailLocal) > 0 && len(tailRemote) > 0 {
			if linesEqual(tailLocal, tailRemote) {
				result = append(result, tailLocal...)
			} else {
				hasConflict = true
				result = append(result, generateConflictMarkers(tailLocal, tailRemote)...)
			}
		} else if len(tailLocal) > 0 {
			result = append(result, tailLocal...)
		} else if len(tailRemote) > 0 {
			result = append(result, tailRemote...)
		}
	}

	return result, hasConflict
}

// lineMap: ancestor 行号 → 目标文件行号（-1 表示删除/替换）
func buildLineMap(ancestor, target []string) []int {
	m := make([]int, len(ancestor))
	ti := 0
	for ai := 0; ai < len(ancestor); ai++ {
		found := false
		for ti < len(target) {
			if target[ti] == ancestor[ai] {
				m[ai] = ti
				ti++
				found = true
				break
			}
			ti++
		}
		if !found {
			m[ai] = -1
			// 重置 ti 到开头重新搜索（简化实现）
			ti = 0
		}
	}
	return m
}

func linesBeforeAnchor(target []string, cursor, anchorIdx int) []string {
	if anchorIdx == -1 || cursor >= anchorIdx {
		return nil
	}
	return target[cursor:anchorIdx]
}

func replacementLines(target []string, cursor int, lineMap []int, ancestorIdx int) []string {
	if lineMap[ancestorIdx] != -1 {
		return nil
	}
	// 找到下一个 anchor 点
	nextAnchor := len(target)
	for i := ancestorIdx + 1; i < len(lineMap); i++ {
		if lineMap[i] != -1 {
			nextAnchor = lineMap[i]
			break
		}
	}
	if cursor >= nextAnchor {
		return nil
	}
	return target[cursor:nextAnchor]
}

func skipPast(lineMap []int, ancestorIdx, cursor int) int {
	nextAnchor := -1
	for i := ancestorIdx + 1; i < len(lineMap); i++ {
		if lineMap[i] != -1 {
			nextAnchor = lineMap[i]
			break
		}
	}
	if nextAnchor == -1 {
		return len(lineMap) // 跳到末尾
	}
	return nextAnchor
}

func generateConflictMarkers(local, remote []string) []string {
	var result []string
	result = append(result, "<<<<<<< local")
	result = append(result, local...)
	result = append(result, "=======")
	result = append(result, remote...)
	result = append(result, ">>>>>>> remote")
	return result
}

func splitLines(data []byte) []string {
	if data == nil {
		return nil
	}
	s := strings.TrimSuffix(string(data), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func linesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
