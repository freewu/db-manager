//go:build !windows

package main

import (
	"os/exec"
	"runtime"
)

// revealPath opens a directory in the platform file manager.
func revealPath(path string) error {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	return exec.Command(opener, path).Start()
}
