//go:build darwin

package desktop

import "os/exec"

type platformFileOpener struct{}

func (platformFileOpener) Open(path string) error {
	expanded, _, err := existingPath(path)
	if err != nil {
		return err
	}
	return exec.Command("open", expanded).Start()
}

func (platformFileOpener) Reveal(path string) error {
	expanded, info, err := existingPath(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return exec.Command("open", expanded).Start()
	}
	return exec.Command("open", "-R", expanded).Start()
}
