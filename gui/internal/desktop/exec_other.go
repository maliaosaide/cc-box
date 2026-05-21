//go:build !windows

package desktop

import "os/exec"

func hideCommandWindow(cmd *exec.Cmd) {}
