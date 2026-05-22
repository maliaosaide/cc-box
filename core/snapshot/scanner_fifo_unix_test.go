//go:build !windows

package snapshot

import "syscall"

func syscallMkfifo(path string) error {
	return syscall.Mkfifo(path, 0600)
}
