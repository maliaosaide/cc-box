package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/cc-box/core/config"
)

func safeJoin(root, relPath string) (string, error) {
	if relPath == "" || strings.ContainsRune(relPath, 0) {
		return "", fmt.Errorf("无效路径: %s", relPath)
	}
	clean := filepath.Clean(filepath.FromSlash(relPath))
	if filepath.IsAbs(clean) || filepath.VolumeName(clean) != "" || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("路径越界: %s", relPath)
	}
	fullPath := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, fullPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("路径越界: %s", relPath)
	}
	return fullPath, nil
}

func safeClaudePath(relPath string) (string, error) {
	return safeJoin(config.ClaudeDir(), relPath)
}
