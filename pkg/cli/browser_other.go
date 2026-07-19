//go:build !windows

package cli

import (
	"os/exec"
	"runtime"
)

func openBrowser(url string) error {
	command := "xdg-open"
	if runtime.GOOS == "darwin" {
		command = "open"
	}
	return exec.Command(command, url).Start()
}
