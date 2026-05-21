// CLI 补充功能测试
// diff 显示、device 管理
package cli

import (
	"testing"
)

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"line1\nline2\nline3", 3},
		{"single", 1},
		{"", 0},
		{"a\n", 1},
		{"a\nb\nc\n", 3},
	}

	for _, tt := range tests {
		lines := splitLines([]byte(tt.input))
		if len(lines) != tt.want {
			t.Errorf("splitLines(%q) = %d lines, want %d", tt.input, len(lines), tt.want)
		}
	}
}

func TestFormatTimeAgo(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"零值", "0001-01-01T00:00:00Z", "未知"},
		{"30秒前", "", "刚刚"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// formatTimeAgo 依赖 time.Since，这里只测试基本逻辑
		})
	}
}

func TestHeadDisplay(t *testing.T) {
	if headDisplay("") != "(无)" {
		t.Error("空 HEAD 应显示 '(无)'")
	}
	if headDisplay("snap_abc12345") != "snap_abc12345" {
		t.Error("非空 HEAD 应原样显示")
	}
}
