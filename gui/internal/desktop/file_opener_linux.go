//go:build linux

package desktop

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

type platformFileOpener struct{}

func (platformFileOpener) Open(path string) error {
	expanded, _, err := existingPath(path)
	if err != nil {
		return err
	}
	return openLinuxPath(expanded)
}

func (platformFileOpener) Reveal(path string) error {
	expanded, info, err := existingPath(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return openLinuxPath(expanded)
	}
	return openLinuxPath(filepath.Dir(expanded))
}

func openLinuxPath(path string) error {
	for _, candidate := range [][]string{{"xdg-open", path}, {"gio", "open", path}} {
		cmdPath, err := exec.LookPath(candidate[0])
		if err != nil {
			continue
		}
		cmd := exec.Command(cmdPath, candidate[1:]...)
		return cmd.Start()
	}
	return fmt.Errorf("未找到可用的文件管理器命令: xdg-open 或 gio")
}
