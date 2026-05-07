// 三方合并引擎测试
package sync

import (
	"encoding/json"
	"testing"
)

func newTestMerger() *Merger {
	return NewMerger("ask")
}

// --- 文本合并测试 ---

func TestTextMergeRemoteOnly(t *testing.T) {
	m := newTestMerger()
	result, err := m.MergeFile("test.md", &ThreeWayInput{
		Ancestor: []byte("line1\nline2\nline3"),
		Local:    []byte("line1\nline2\nline3"),
		Remote:   []byte("line1\nline2-mod\nline3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionKeepRemote {
		t.Errorf("action = %d, want ActionKeepRemote", result.Action)
	}
	if string(result.Data) != "line1\nline2-mod\nline3" {
		t.Errorf("data = %q", string(result.Data))
	}
}

func TestTextMergeLocalOnly(t *testing.T) {
	m := newTestMerger()
	result, err := m.MergeFile("test.md", &ThreeWayInput{
		Ancestor: []byte("line1\nline2\nline3"),
		Local:    []byte("line1\nline2-new\nline3"),
		Remote:   []byte("line1\nline2\nline3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionKeepLocal {
		t.Errorf("action = %d, want ActionKeepLocal", result.Action)
	}
}

func TestTextMergeBothDifferentLines(t *testing.T) {
	m := newTestMerger()
	// 本地改了第2行，远程改了第3行，应能合并
	result, err := m.MergeFile("test.md", &ThreeWayInput{
		Ancestor: []byte("line1\nline2\nline3\nline4"),
		Local:    []byte("line1\nline2-local\nline3\nline4"),
		Remote:   []byte("line1\nline2\nline3-remote\nline4"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action == ActionConflict {
		t.Error("不同行的修改不应该冲突")
	}
}

func TestTextMergeNoAncestor(t *testing.T) {
	m := newTestMerger()
	result, err := m.MergeFile("test.md", &ThreeWayInput{
		Ancestor: nil,
		Local:    []byte("local content"),
		Remote:   []byte("local content"),
	})
	if err != nil {
		t.Fatal(err)
	}
	// 内容相同，应该合并成功
	if result.Action != ActionMerged {
		t.Errorf("action = %d, want ActionMerged", result.Action)
	}
}

// --- JSON 合并测试 ---

func TestJSONMergeRemoteOnly(t *testing.T) {
	m := newTestMerger()
	result, err := m.MergeFile("config.json", &ThreeWayInput{
		Ancestor: []byte(`{"key": "old"}`),
		Local:    []byte(`{"key": "old"}`),
		Remote:   []byte(`{"key": "new"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionKeepRemote {
		t.Errorf("action = %d, want ActionKeepRemote", result.Action)
	}
}

func TestSettingsMergeEnvBidirectional(t *testing.T) {
	ancestor := `{"env":{"KEY_A":"old","KEY_B":"val"}}`
	local := `{"env":{"KEY_A":"local","KEY_B":"val","KEY_C":"local-new"}}`
	remote := `{"env":{"KEY_A":"remote","KEY_B":"val","KEY_D":"remote-new"}}`

	m := newTestMerger()
	result, err := m.MergeFile("settings.json", &ThreeWayInput{
		Ancestor: []byte(ancestor),
		Local:    []byte(local),
		Remote:   []byte(remote),
	})
	if err != nil {
		t.Fatal(err)
	}

	var merged map[string]interface{}
	if err := json.Unmarshal(result.Data, &merged); err != nil {
		t.Fatal(err)
	}

	env := merged["env"].(map[string]interface{})
	// KEY_A: 本地有，保留本地值
	if env["KEY_A"] != "local" {
		t.Errorf("KEY_A = %v, want local", env["KEY_A"])
	}
	// KEY_B: 双方没变
	if env["KEY_B"] != "val" {
		t.Errorf("KEY_B = %v, want val", env["KEY_B"])
	}
	// KEY_C: 本地新增，保留
	if env["KEY_C"] != "local-new" {
		t.Errorf("KEY_C should exist from local")
	}
	// KEY_D: 远程新增，保留
	if env["KEY_D"] != "remote-new" {
		t.Errorf("KEY_D should exist from remote")
	}
}

func TestSettingsMergePermissionsUnion(t *testing.T) {
	local := `{"permissions":{"allow":["tool-a","tool-b"]}}`
	remote := `{"permissions":{"allow":["tool-b","tool-c"]}}`

	m := newTestMerger()
	result, err := m.MergeFile("settings.json", &ThreeWayInput{
		Ancestor: []byte(`{}`),
		Local:    []byte(local),
		Remote:   []byte(remote),
	})
	if err != nil {
		t.Fatal(err)
	}

	var merged map[string]interface{}
	json.Unmarshal(result.Data, &merged)

	perms := merged["permissions"].(map[string]interface{})
	allow := toStringSlice(perms["allow"])
	if len(allow) != 3 {
		t.Errorf("allow count = %d, want 3: %v", len(allow), allow)
	}
}

// --- history.jsonl 合并测试 ---

func TestHistoryMergeDedup(t *testing.T) {
	ancestor := `{"command":"ls","ts":"10:00"}
{"command":"cd","ts":"10:01"}`
	local := `{"command":"ls","ts":"10:00"}
{"command":"cd","ts":"10:01"}
{"command":"pwd","ts":"10:02"}`
	remote := `{"command":"ls","ts":"10:00"}
{"command":"cd","ts":"10:01"}
{"command":"git","ts":"10:03"}`

	m := newTestMerger()
	result, err := m.MergeFile("history.jsonl", &ThreeWayInput{
		Ancestor: []byte(ancestor),
		Local:    []byte(local),
		Remote:   []byte(remote),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != ActionMerged {
		t.Errorf("action = %d, want ActionMerged", result.Action)
	}

	lines := splitHistoryLines(result.Data)
	if len(lines) != 4 {
		t.Errorf("merged lines = %d, want 4: %v", len(lines), lines)
	}
}

// --- 文件类型判断测试 ---

func TestClassifyFile(t *testing.T) {
	tests := []struct {
		path string
		want FileType
	}{
		{"CLAUDE.md", FileText},
		{"skills/test/SKILL.md", FileText},
		{"settings.json", FileJSON},
		{"keybindings.json", FileJSON},
		{"history.jsonl", FileHistory},
		{"plugins/installed_plugins.json", FileJSON},
		{"image.png", FileBinary},
		{"commands/test.sh", FileText},
	}
	for _, tt := range tests {
		got := ClassifyFile(tt.path)
		if got != tt.want {
			t.Errorf("ClassifyFile(%q) = %d, want %d", tt.path, got, tt.want)
		}
	}
}
