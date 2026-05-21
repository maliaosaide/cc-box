// 跨平台规范化测试
package normalize

import (
	"testing"
)

func TestPathSlash(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"foo\\bar\\baz", "foo/bar/baz"},
		{"foo/bar/baz", "foo/bar/baz"},
		{`\`, "/"},
		{"", ""},
	}
	for _, tt := range tests {
		got := PathSlash(tt.input)
		if got != tt.want {
			t.Errorf("PathSlash(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"foo\\Bar\\Baz", "foo/bar/baz"},
		{"/foo/bar", "foo/bar"},
		{"FOO/BAR.JSON", "foo/bar.json"},
	}
	for _, tt := range tests {
		got := NormalizePath(tt.input)
		if got != tt.want {
			t.Errorf("NormalizePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeContent(t *testing.T) {
	input := []byte("line1\r\nline2\r\nline3")
	want := []byte("line1\nline2\nline3")
	got := NormalizeContent(input)
	if string(got) != string(want) {
		t.Errorf("NormalizeContent() = %q, want %q", got, want)
	}
}

func TestNormalizeContentNoChange(t *testing.T) {
	input := []byte("line1\nline2\nline3")
	got := NormalizeContent(input)
	if string(got) != string(input) {
		t.Errorf("NormalizeContent() should not modify LF content")
	}
}

func TestIsTextFile(t *testing.T) {
	text := []byte("Hello, world!")
	if !IsTextFile(text) {
		t.Error("text content should be detected as text")
	}

	binary := []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x00}
	if IsTextFile(binary) {
		t.Error("binary content with null byte should not be detected as text")
	}
}

func TestHashContent(t *testing.T) {
	// 相同内容的 CRLF 和 LF 版本应产生相同结果
	cr := []byte("hello\r\nworld")
	lf := []byte("hello\nworld")
	if string(HashContent(cr)) != string(HashContent(lf)) {
		t.Error("CRLF and LF versions should produce same hash content")
	}
}
