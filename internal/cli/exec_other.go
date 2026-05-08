// 非 Windows 平台：空实现
//go:build !windows

package cli

import "os/exec"

func setHideWindow(cmd *exec.Cmd) {}
