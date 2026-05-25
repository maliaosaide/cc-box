//go:build !windows

package binary

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const pathBlockStart = "# >>> CC-Box Claude PATH >>>"
const pathBlockEnd = "# <<< CC-Box Claude PATH <<<"

func configureUserPathDir(dir string) (string, bool, error) {
	profile := shellProfilePath()
	block := pathBlock(dir)
	data, err := os.ReadFile(profile)
	if err != nil && !os.IsNotExist(err) {
		return profile, false, err
	}
	content := string(data)
	if strings.Contains(content, pathBlockStart) && strings.Contains(content, pathBlockEnd) {
		updated := replaceMarkedBlock(content, block)
		if updated == content {
			return profile, false, nil
		}
		return profile, true, os.WriteFile(profile, []byte(updated), 0600)
	}
	if strings.Contains(content, dir) {
		return profile, false, nil
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "\n" + block + "\n"
	return profile, true, os.WriteFile(profile, []byte(content), 0600)
}

func shellProfilePath() string {
	home, _ := os.UserHomeDir()
	shell := filepath.Base(os.Getenv("SHELL"))
	switch shell {
	case "zsh":
		return filepath.Join(home, ".zshrc")
	case "bash":
		return filepath.Join(home, ".bashrc")
	default:
		return filepath.Join(home, ".profile")
	}
}

func pathBlock(dir string) string {
	return fmt.Sprintf("%s\nexport PATH=%s:\"$PATH\"\n%s", pathBlockStart, shellQuote(filepath.ToSlash(dir)), pathBlockEnd)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func replaceMarkedBlock(content, block string) string {
	start := strings.Index(content, pathBlockStart)
	end := strings.Index(content, pathBlockEnd)
	if start < 0 || end < start {
		return content
	}
	end += len(pathBlockEnd)
	return content[:start] + block + content[end:]
}
