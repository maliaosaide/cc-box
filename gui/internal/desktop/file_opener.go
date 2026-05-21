package desktop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FileOpener interface {
	Open(path string) error
	Reveal(path string) error
}

func NewFileOpener() FileOpener {
	return platformFileOpener{}
}

func existingPath(path string) (string, os.FileInfo, error) {
	expanded := expandHome(strings.TrimSpace(path))
	if expanded == "" {
		return "", nil, fmt.Errorf("路径为空")
	}
	info, err := os.Stat(expanded)
	if err != nil {
		return "", nil, fmt.Errorf("路径不存在: %s", path)
	}
	return expanded, info, nil
}

func expandHome(path string) string {
	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
