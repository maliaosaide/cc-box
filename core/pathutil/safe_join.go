package pathutil

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func SafeJoin(root, relPath string) (string, error) {
	if relPath == "" || strings.ContainsRune(relPath, 0) {
		return "", fmt.Errorf("无效路径: %s", relPath)
	}
	slashPath := strings.ReplaceAll(relPath, `\`, "/")
	cleanSlash := path.Clean(slashPath)
	if strings.HasPrefix(cleanSlash, "/") || hasWindowsVolume(cleanSlash) || cleanSlash == "." || cleanSlash == ".." || strings.HasPrefix(cleanSlash, "../") {
		return "", fmt.Errorf("路径越界: %s", relPath)
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("解析根路径失败: %w", err)
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("解析根路径失败: %w", err)
		}
		rootReal = rootAbs
	}

	fullPath := filepath.Join(rootAbs, filepath.FromSlash(cleanSlash))
	if !isWithin(rootAbs, fullPath) {
		return "", fmt.Errorf("路径越界: %s", relPath)
	}
	if err := rejectSymlinkEscape(rootAbs, rootReal, cleanSlash, relPath); err != nil {
		return "", err
	}
	return fullPath, nil
}

func rejectSymlinkEscape(rootAbs, rootReal, cleanSlash, original string) error {
	current := rootAbs
	for _, part := range strings.Split(cleanSlash, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("检查路径失败: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("路径越界: %s", original)
		}
		realCurrent, err := filepath.EvalSymlinks(current)
		if err != nil {
			return fmt.Errorf("解析路径失败: %w", err)
		}
		if !isWithin(rootReal, realCurrent) {
			return fmt.Errorf("路径越界: %s", original)
		}
	}
	return nil
}

func isWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func hasWindowsVolume(p string) bool {
	return len(p) >= 2 && p[1] == ':' && ((p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= 'a' && p[0] <= 'z'))
}
