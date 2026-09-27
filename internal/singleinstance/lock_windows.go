//go:build windows

package singleinstance

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// lockFile opens path and takes the claim on it, in a way no other process can
// take at the same time.
//
// The claim is a byte-range lock rather than a handle nobody else may open —
// which is what opening the file with a share mode of zero would give. A file
// that can only be opened by us is also one an antivirus scanner cannot read,
// and a scanner holding the file open for a moment would then be mistaken for a
// second copy of the application. A lock is advisory instead: everybody stays
// free to read the file, and the only thing it stops is another copy taking the
// claim.
//
// The range is byte 0 of an empty file, which is allowed — a lock may extend
// past the end of a file — and it is why nothing ever has to be written here.
func lockFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, fileMode)
	if err != nil {
		return nil, err
	}
	err = windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0,
		new(windows.Overlapped),
	)
	if err != nil {
		_ = f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, ErrAlreadyRunning
		}
		return nil, err
	}
	return f, nil
}
