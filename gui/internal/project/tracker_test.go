// project 包测试
// 项目发现、编码、合并
package project

import (
	"testing"
)

func TestEncodeRemote(t *testing.T) {
	// 相同 URL 编码结果一致
	url1 := "git@github.com:user/repo.git"
	url2 := "https://github.com/user/repo.git"

	enc1 := EncodeRemote(url1)
	enc2 := EncodeRemote(url1)
	enc3 := EncodeRemote(url2)

	if enc1 != enc2 {
		t.Error("相同 URL 编码结果不一致")
	}
	if enc1 == enc3 {
		t.Error("不同 URL 编码结果不应相同")
	}

	// 编码结果应只包含十六进制字符
	for _, c := range enc1 {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("编码结果包含非十六进制字符: %c", c)
			break
		}
	}
}

func TestMergeClaudeJSON_BothHaveMCPServers(t *testing.T) {
	local := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"local-only": map[string]interface{}{"command": "local"},
			"shared":     map[string]interface{}{"command": "local-ver"},
		},
	}

	remote := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"remote-only": map[string]interface{}{"command": "remote"},
			"shared":      map[string]interface{}{"command": "remote-ver"},
		},
	}

	merged := MergeClaudeJSON(local, remote)

	servers := merged["mcpServers"].(map[string]interface{})

	// 双方独有的应保留
	if _, ok := servers["local-only"]; !ok {
		t.Error("合并结果应保留 local-only")
	}
	if _, ok := servers["remote-only"]; !ok {
		t.Error("合并结果应保留 remote-only")
	}

	// 同名 server 应保留 remote 版本
	shared := servers["shared"].(map[string]interface{})
	if shared["command"] != "remote-ver" {
		t.Error("同名 server 应保留远程版本")
	}
}

func TestMergeClaudeJSON_AllowedToolsUnion(t *testing.T) {
	local := map[string]interface{}{
		"allowedTools": []interface{}{"tool-a", "tool-b"},
	}

	remote := map[string]interface{}{
		"allowedTools": []interface{}{"tool-b", "tool-c"},
	}

	merged := MergeClaudeJSON(local, remote)
	tools := merged["allowedTools"].([]string)

	seen := make(map[string]bool)
	for _, t := range tools {
		seen[t] = true
	}

	if !seen["tool-a"] || !seen["tool-b"] || !seen["tool-c"] {
		t.Errorf("allowedTools 应是并集，实际: %v", tools)
	}
	if len(seen) != 3 {
		t.Errorf("去重后应有 3 个工具，实际 %d", len(seen))
	}
}

func TestMergeClaudeJSON_NilFields(t *testing.T) {
	local := map[string]interface{}{}
	remote := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"server-a": map[string]interface{}{"command": "a"},
		},
	}

	merged := MergeClaudeJSON(local, remote)
	servers := merged["mcpServers"].(map[string]interface{})
	if len(servers) != 1 {
		t.Error("local 为空时应保留 remote 的 servers")
	}
}

func TestDecodeProjectDir_Windows(t *testing.T) {
	dir := "-C-Users-a-Desktop-myproject"
	result := decodeProjectDir(dir)
	if result != "C:\\Users\\a\\Desktop\\myproject" {
		t.Errorf("Windows 路径解码错误: %s", result)
	}
}

func TestDecodeProjectDir_Unix(t *testing.T) {
	dir := "-Users-a-Desktop-myproject"
	result := decodeProjectDir(dir)
	if result != "/Users/a/Desktop/myproject" {
		t.Errorf("Unix 路径解码错误: %s", result)
	}
}
