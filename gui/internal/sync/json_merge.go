// JSON 字段级合并
// cc-switch 兼容的 settings.json 合并
package sync

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// mergeJSON 对 JSON 文件执行字段级三方合并
func (m *Merger) mergeJSON(path string, input *ThreeWayInput) (*MergeResult, error) {
	var ancestor, local, remote interface{}

	if input.Ancestor != nil {
		if err := json.Unmarshal(input.Ancestor, &ancestor); err != nil {
			return m.resolveBinaryConflict(path, input)
		}
	}
	if err := json.Unmarshal(input.Local, &local); err != nil {
		return nil, fmt.Errorf("解析本地 JSON 失败: %w", err)
	}
	if err := json.Unmarshal(input.Remote, &remote); err != nil {
		return nil, fmt.Errorf("解析远程 JSON 失败: %w", err)
	}

	// 判断是否为 settings.json，使用特殊合并策略
	if isSettingsJSON(path) {
		merged, err := mergeSettingsJSON(ancestor, local, remote)
		if err != nil {
			return nil, err
		}
		data, err := marshalJSON(merged)
		if err != nil {
			return nil, err
		}
		return &MergeResult{Path: path, Action: ActionMerged, Data: data}, nil
	}

	// 普通 JSON 合并
	merged, hasConflict := mergeGenericJSON(ancestor, local, remote)
	if hasConflict {
		switch m.conflictStrategy {
		case "local":
			return &MergeResult{Path: path, Action: ActionKeepLocal, Data: input.Local}, nil
		case "remote":
			return &MergeResult{Path: path, Action: ActionKeepRemote, Data: input.Remote}, nil
		default:
			data, _ := marshalJSON(merged)
			return &MergeResult{
				Path:      path,
				Action:    ActionConflict,
				Data:      data,
				Conflicts: []string{"JSON 合并冲突"},
			}, nil
		}
	}

	data, err := marshalJSON(merged)
	if err != nil {
		return nil, err
	}
	return &MergeResult{Path: path, Action: ActionMerged, Data: data}, nil
}

// mergeSettingsJSON cc-switch 兼容的 settings.json 合并
// 规则：env 双向合并、permissions 并集、其他字段远程优先
func mergeSettingsJSON(ancestor, local, remote interface{}) (interface{}, error) {
	ancestorMap, _ := toMap(ancestor)
	localMap, _ := toMap(local)
	remoteMap, _ := toMap(remote)

	if localMap == nil {
		localMap = make(map[string]interface{})
	}
	if remoteMap == nil {
		remoteMap = make(map[string]interface{})
	}
	if ancestorMap == nil {
		ancestorMap = make(map[string]interface{})
	}

	result := make(map[string]interface{})

	// 收集所有 key
	allKeys := make(map[string]bool)
	for k := range localMap {
		allKeys[k] = true
	}
	for k := range remoteMap {
		allKeys[k] = true
	}

	for key := range allKeys {
		ancestorVal := ancestorMap[key]
		localVal := localMap[key]
		remoteVal := remoteMap[key]

		switch key {
		case "env":
			// env 字段：双向合并，保留双方所有 key
			result[key] = mergeEnvField(ancestorVal, localVal, remoteVal)

		case "permissions":
			// permissions 字段：allow/deny 列表取并集
			result[key] = mergePermissionsField(ancestorVal, localVal, remoteVal)

		default:
			// 其他顶层字段：远程优先
			if remoteVal != nil {
				result[key] = remoteVal
			} else if localVal != nil {
				result[key] = localVal
			}
		}
	}

	return result, nil
}

// mergeEnvField 合并 env 字段，保留双方所有 key
// 双方都有的 key：本地优先（env 变量通常是本机设置的，如 API token）
// 远程独有的 key：保留
func mergeEnvField(ancestor, local, remote interface{}) interface{} {
	ancestorEnv, _ := toMap(ancestor)
	localEnv, _ := toMap(local)
	remoteEnv, _ := toMap(remote)

	if ancestorEnv == nil {
		ancestorEnv = make(map[string]interface{})
	}

	result := make(map[string]interface{})

	// 本地 key 全部保留
	for k, v := range localEnv {
		result[k] = v
	}

	// 远程独有的 key 也保留
	for k, v := range remoteEnv {
		if _, exists := result[k]; !exists {
			result[k] = v
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// mergePermissionsField 合并 permissions 字段
func mergePermissionsField(ancestor, local, remote interface{}) interface{} {
	localMap, _ := toMap(local)
	remoteMap, _ := toMap(remote)

	if localMap == nil && remoteMap == nil {
		return nil
	}

	result := make(map[string]interface{})
	subKeys := make(map[string]bool)
	for k := range localMap {
		subKeys[k] = true
	}
	for k := range remoteMap {
		subKeys[k] = true
	}

	for key := range subKeys {
		localList := toStringSlice(localMap[key])
		remoteList := toStringSlice(remoteMap[key])

		// 取并集
		merged := unionSlices(localList, remoteList)
		if len(merged) > 0 {
			result[key] = merged
		}
	}

	return result
}

// mergeGenericJSON 普通 JSON 合并
// 同名 key 远程优先，新增 key 并集
func mergeGenericJSON(ancestor, local, remote interface{}) (interface{}, bool) {
	localMap, localIsMap := toMap(local)
	remoteMap, remoteIsMap := toMap(remote)

	if !localIsMap && !remoteIsMap {
		// 不是 map，无法合并
		return remote, false
	}

	if localIsMap && remoteIsMap {
		result := make(map[string]interface{})
		allKeys := make(map[string]bool)
		for k := range localMap {
			allKeys[k] = true
		}
		for k := range remoteMap {
			allKeys[k] = true
		}

		for key := range allKeys {
			_, hasLocal := localMap[key]
			_, hasRemote := remoteMap[key]

			if hasLocal && hasRemote {
				// 双方都有，远程优先
				result[key] = remoteMap[key]
			} else if hasLocal {
				result[key] = localMap[key]
			} else {
				result[key] = remoteMap[key]
			}
		}
		return result, false
	}

	return remote, false
}

func isSettingsJSON(path string) bool {
	return strings.EqualFold(path, "settings.json")
}

func toMap(v interface{}) (map[string]interface{}, bool) {
	if v == nil {
		return nil, false
	}
	m, ok := v.(map[string]interface{})
	return m, ok
}

func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []string:
		return val
	case []interface{}:
		var result []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

func unionSlices(a, b []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	sort.Strings(result)
	return result
}

func marshalJSON(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
