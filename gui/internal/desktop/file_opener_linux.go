//go:build linux

package desktop

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const linuxOpenTimeout = 5 * time.Second

var runLinuxOpenCommand = runLinuxCommand

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
	var failures []string
	for _, candidate := range [][]string{{"xdg-open", path}, {"gio", "open", path}} {
		cmdPath, err := exec.LookPath(candidate[0])
		if err != nil {
			failures = append(failures, candidate[0]+": 未找到")
			continue
		}
		output, err := runLinuxOpenCommand(cmdPath, candidate[1:]...)
		if err == nil {
			return nil
		}
		message := strings.TrimSpace(output)
		if message == "" {
			message = err.Error()
		}
		failures = append(failures, candidate[0]+": "+message)
	}
	return fmt.Errorf("打开路径失败: %s", strings.Join(failures, "; "))
}

func runLinuxCommand(cmdPath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), linuxOpenTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdPath, args...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf("命令超时")
	}
	return string(output), err
}
