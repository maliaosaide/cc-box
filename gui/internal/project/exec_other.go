//go:build !windows

package project

import "os/exec"

func hideCommandWindow(cmd *exec.Cmd) {}
