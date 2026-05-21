package binary

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func managedBinaryName(name string) string {
	return executableName(name)
}

func claudeCandidateNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"claude.exe", "claude.cmd", "claude.bat", "claude.ps1"}
	}
	return []string{"claude"}
}

func commonClaudeDirs() []string {
	home, _ := os.UserHomeDir()
	var dirs []string
	if home != "" {
		dirs = append(dirs, filepath.Join(home, ".local", "bin"))
	}
	if runtime.GOOS == "windows" {
		for _, item := range []struct {
			env   string
			elems []string
		}{
			{env: "LOCALAPPDATA", elems: []string{"Programs", "Claude"}},
			{env: "ProgramFiles", elems: []string{"Claude"}},
			{env: "APPDATA", elems: []string{"npm"}},
		} {
			base := os.Getenv(item.env)
			if base != "" {
				dirs = append(dirs, filepath.Join(append([]string{base}, item.elems...)...))
			}
		}
		return uniqueStrings(dirs)
	}
	dirs = append(dirs, "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin")
	return uniqueStrings(dirs)
}

func canReplaceFileByRename() bool {
	return runtime.GOOS != "windows"
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return comparablePath(a) == comparablePath(b)
}

func comparablePath(path string) string {
	path = expandBinaryPath(path)
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return path
}
