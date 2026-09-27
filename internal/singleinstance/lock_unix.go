//go:build !windows

package singleinstance

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// lockFile opens path and takes the claim on it, in a way no other process can
// take at the same time.
//
// The claim is flock(2) rather than a pid file: it belongs to the open file, so
// a copy that is killed — which is how a desktop application usually goes away —
// releases it as part of dying, and there is no stale claim left to time out or
// to guess about. Two handles on one file conflict even inside a single process,
// which is what lets "a second copy is refused" be tested without starting one.
func lockFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, fileMode)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrAlreadyRunning
		}
		return nil, err
	}
	return f, nil
}
