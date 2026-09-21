//go:build !windows

package main

// macOS and Linux get no notification-area icon (see tray.go), so there is
// nothing to create, nothing to take down, and no balloon to show: closing the
// window still quits the app, exactly as it did before the tray existed.
type tray struct{}

func newTray(trayActions) *tray { return &tray{} }

// running is always false — there is no icon behind the window, so the close
// hook must let the window close.
func (*tray) running() bool { return false }

func (*tray) close() {}

func (*tray) noticeHidden() {}
