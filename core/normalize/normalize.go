// 跨平台规范化
// 处理路径分隔符、换行符、大小写差异，确保不同平台产生一致的哈希
package normalize

import (
	"bytes"
	"path/filepath"
	"strings"
)

// PathSlash 将路径分隔符统一为 /
func PathSlash(p string) string {
	return strings.ReplaceAll(p, `\`, "/")
}

// PathLower 返回路径的小写形式，用于大小写碰撞检测。
func PathLower(p string) string {
	return strings.ToLower(p)
}

// NormalizePath 路径规范化：统一分隔符并保留原始大小写。
func NormalizePath(p string) string {
	p = PathSlash(p)
	p = strings.TrimPrefix(p, "/")
	return p
}

// RelativePath 计算 base 下的相对路径并规范化
func RelativePath(base, full string) string {
	rel, err := filepath.Rel(base, full)
	if err != nil {
		return NormalizePath(full)
	}
	return NormalizePath(rel)
}

// NormalizeContent 对文本文件内容规范化换行符（CRLF -> LF）后返回
// 仅影响哈希计算，不修改原文件
func NormalizeContent(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

// IsTextFile 根据内容判断是否为文本文件
// 检查前 8KB 内容中是否含 NULL 字节
func IsTextFile(data []byte) bool {
	check := data
	if len(check) > 8192 {
		check = check[:8192]
	}
	return !bytes.ContainsRune(check, 0)
}

// HashContent 返回用于哈希计算的内容：文本文件规范化换行，二进制文件原样
func HashContent(data []byte) []byte {
	if IsTextFile(data) {
		return NormalizeContent(data)
	}
	return data
}
