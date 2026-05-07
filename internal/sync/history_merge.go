// history.jsonl 去重追加合并
// 不做标准三方合并，按行内容去重
package sync

import (
	"strings"
)

// mergeHistory 对 history.jsonl 执行去重追加合并
// 规则：
//   取 ancestor → local 的新增行（本地独有）
//   取 ancestor → remote 的新增行（远程独有）
//   合并去重后追加
//   去重依据：command + timestamp 组合键（同一分钟内视为重复）
func (m *Merger) mergeHistory(path string, input *ThreeWayInput) (*MergeResult, error) {
	ancestorLines := splitHistoryLines(input.Ancestor)
	localLines := splitHistoryLines(input.Local)
	remoteLines := splitHistoryLines(input.Remote)

	// ancestor 作为基线
	ancestorSet := make(map[string]bool)
	for _, line := range ancestorLines {
		ancestorSet[dedupKey(line)] = true
	}

	// 找本地新增
	var localNew []string
	for _, line := range localLines {
		if !ancestorSet[dedupKey(line)] {
			localNew = append(localNew, line)
		}
	}

	// 找远程新增
	var remoteNew []string
	for _, line := range remoteLines {
		if !ancestorSet[dedupKey(line)] {
			remoteNew = append(remoteNew, line)
		}
	}

	// 合并去重
	seen := make(map[string]bool)
	var merged []string

	// 先保留 ancestor 的行
	merged = append(merged, ancestorLines...)
	for _, line := range ancestorLines {
		seen[dedupKey(line)] = true
	}

	// 加入本地新增
	for _, line := range localNew {
		key := dedupKey(line)
		if !seen[key] {
			seen[key] = true
			merged = append(merged, line)
		}
	}

	// 加入远程新增
	for _, line := range remoteNew {
		key := dedupKey(line)
		if !seen[key] {
			seen[key] = true
			merged = append(merged, line)
		}
	}

	if len(merged) == 0 {
		return &MergeResult{Path: path, Action: ActionNoChange, Data: nil}, nil
	}

	data := []byte(strings.Join(merged, "\n") + "\n")
	return &MergeResult{Path: path, Action: ActionMerged, Data: data}, nil
}

// splitHistoryLines 拆分 history.jsonl 行
func splitHistoryLines(data []byte) []string {
	return splitLines(data)
}

// dedupKey 生成去重键
// 从 JSONL 行中提取 command + 精确到分钟的 timestamp
func dedupKey(line string) string {
	// 简化实现：直接用整行内容做 key
	// JSONL 中每行包含完整的 command 和 timestamp，
	// 完全相同的行视为重复
	return strings.TrimSpace(line)
}
