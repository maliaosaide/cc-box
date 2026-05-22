//go:build windows

package snapshot

import "fmt"

func syscallMkfifo(path string) error {
	return fmt.Errorf("mkfifo unsupported on Windows: %s", path)
}
