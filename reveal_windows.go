//go:build windows

package main

import "os/exec"

// revealPath opens a directory in Explorer.
func revealPath(path string) error {
	return exec.Command("explorer", path).Start()
}
