//go:build !windows

package binary

import "os/exec"

func hideCommandWindow(cmd *exec.Cmd) {}
