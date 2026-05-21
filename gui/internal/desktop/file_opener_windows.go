//go:build windows

package desktop

import "os/exec"

type platformFileOpener struct{}

func (platformFileOpener) Open(path string) error {
	expanded, _, err := existingPath(path)
	if err != nil {
		return err
	}
	cmd := exec.Command("explorer", expanded)
	hideCommandWindow(cmd)
	return cmd.Start()
}

func (platformFileOpener) Reveal(path string) error {
	expanded, info, err := existingPath(path)
	if err != nil {
		return err
	}
	var cmd *exec.Cmd
	if info.IsDir() {
		cmd = exec.Command("explorer", expanded)
	} else {
		cmd = exec.Command("explorer", "/select,"+expanded)
	}
	hideCommandWindow(cmd)
	return cmd.Start()
}
