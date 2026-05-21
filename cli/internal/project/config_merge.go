// .claude.json 合并策略
// MCP server 配置合并、allowedTools 并集、permissions 合并
package project

// MergeClaudeJSON 合并两份 .claude.json 内容
// 策略: mcpServers 保留双方独有 + 同名远程优先, allowedTools 并集
func MergeClaudeJSON(local, remote map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// 复制 remote 作为基础
	for k, v := range remote {
		result[k] = v
	}

	// 合并 mcpServers
	if localServers, ok := local["mcpServers"].(map[string]interface{}); ok {
		remoteServers, _ := result["mcpServers"].(map[string]interface{})
		if remoteServers == nil {
			remoteServers = make(map[string]interface{})
		}
		// 保留 local 独有的 server（remote 已有的保持 remote 版本）
		for name, config := range localServers {
			if _, exists := remoteServers[name]; !exists {
				remoteServers[name] = config
			}
		}
		result["mcpServers"] = remoteServers
	}

	// 合并 allowedTools（并集）
	localTools := toStringSlice(local["allowedTools"])
	remoteTools := toStringSlice(result["allowedTools"])
	if len(localTools) > 0 || len(remoteTools) > 0 {
		merged := unionStrings(remoteTools, localTools)
		result["allowedTools"] = merged
	}

	// 合并 permissions
	if localPerms, ok := local["permissions"].(map[string]interface{}); ok {
		remotePerms, _ := result["permissions"].(map[string]interface{})
		if remotePerms == nil {
			remotePerms = make(map[string]interface{})
		}
		for k, v := range localPerms {
			if _, exists := remotePerms[k]; !exists {
				remotePerms[k] = v
			}
		}
		result["permissions"] = remotePerms
	}

	return result
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

func unionStrings(slices ...[]string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, slice := range slices {
		for _, s := range slice {
			if !seen[s] {
				seen[s] = true
				result = append(result, s)
			}
		}
	}
	return result
}
