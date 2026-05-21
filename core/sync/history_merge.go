// history.jsonl 去重追加合并
// 不做标准三方合并，按行内容去重
package sync

import (
	"encoding/json"
	"strings"
)

// mergeHistory 对 history.jsonl 执行去重追加合并
// 规则：
//
//	取 ancestor → local 的新增行（本地独有）
//	取 ancestor → remote 的新增行（远程独有）
//	合并去重后追加
//	去重依据：command + timestamp 组合键（同一分钟内视为重复）
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
// 从 JSONL 行中提取 command + 精确到分钟的 timestamp 作为去重依据
func dedupKey(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}

	// 尝试解析为 JSON 提取 command 和 timestamp
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		// 非 JSON 行，回退到整行去重
		return line
	}

	var parts []string
	if cmd, ok := entry["command"].(string); ok {
		parts = append(parts, cmd)
	}
	if ts, ok := entry["timestamp"].(string); ok {
		// 截断到分钟：2026-05-07T14:30:45Z → 2026-05-07T14:30
		if len(ts) >= 16 {
			parts = append(parts, ts[:16])
		} else {
			parts = append(parts, ts)
		}
	}

	if len(parts) > 0 {
		return strings.Join(parts, "|")
	}
	return line
}
