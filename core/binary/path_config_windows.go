//go:build windows

package binary

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func configureUserPathDir(dir string) (string, bool, error) {
	dir = filepath.Clean(dir)
	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return `HKEY_CURRENT_USER\Environment\Path`, false, err
	}
	defer key.Close()
	current, valueType, err := key.GetStringValue("Path")
	if err == registry.ErrNotExist {
		current = ""
		valueType = registry.EXPAND_SZ
	} else if err != nil {
		return `HKEY_CURRENT_USER\Environment\Path`, false, err
	}
	for _, part := range filepath.SplitList(current) {
		if samePath(part, dir) {
			return `HKEY_CURRENT_USER\Environment\Path`, false, nil
		}
	}
	newValue := dir
	if strings.TrimSpace(current) != "" {
		newValue += string(os.PathListSeparator) + current
	}
	if valueType == registry.EXPAND_SZ {
		return `HKEY_CURRENT_USER\Environment\Path`, true, key.SetExpandStringValue("Path", newValue)
	}
	return `HKEY_CURRENT_USER\Environment\Path`, true, key.SetStringValue("Path", newValue)
}
