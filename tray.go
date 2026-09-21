package main

// The notification-area icon ("tray").
//
// Only Windows has one: Win32 gives the shell a first-class API for it
// (Shell_NotifyIcon), whereas the desktops we ship on otherwise either have no
// notification area at all (macOS keeps apps in the Dock and their commands in
// the application menu) or have no single one to talk to (Linux). So the menu is
// described here — the wording, the order and the version row — and drawn by
// tray_windows.go; tray_other.go has nothing to draw.
//
// Everything in the menu is a fact the app already knows: the version is the one
// the build stamped into main.Version, and the two links are the same ones the
// empty-state "project info" pane shows.

// repoURL is what the tray's project and issue rows open.
const repoURL = "https://github.com/freewu/db-manager"

// issuesURL goes straight to a new issue: the row says "Report an issue", so it
// should land on the form rather than on the list of what is already known.
const issuesURL = repoURL + "/issues/new"

// trayActions is what the tray menu can ask of the app. The platform layer holds
// these and calls the matching one when a row is picked.
type trayActions struct {
	showWindow func()
	openRepo   func()
	openIssue  func()
	quit       func()
}

// trayCommand names a row of the tray menu. The Windows layer uses the value as
// the Win32 menu id.
type trayCommand uint16

const (
	trayCommandShowWindow trayCommand = iota + 1
	trayCommandProjectPage
	trayCommandReportIssue
	trayCommandQuit
	// trayCommandVersion is a label rather than a command: its row is greyed out
	// and can never be picked, it is only there to be read.
	trayCommandVersion
)

// trayMenuRow is one line of the tray menu.
type trayMenuRow struct {
	command   trayCommand
	label     string
	enabled   bool
	separator bool
}

// trayMenuRows is the tray menu, top to bottom: the way back to the window, the
// two places to go for the project, which version this is, and the way out.
//
// It lives here rather than in the platform file so the wording and the order
// have one home, and so a test can check them without a desktop.
func trayMenuRows() []trayMenuRow {
	return []trayMenuRow{
		{command: trayCommandShowWindow, label: "Show window", enabled: true},
		{command: trayCommandProjectPage, label: "Project page", enabled: true},
		{command: trayCommandReportIssue, label: "Report an issue", enabled: true},
		{separator: true},
		{command: trayCommandVersion, label: "v" + Version},
		{separator: true},
		{command: trayCommandQuit, label: "Quit", enabled: true},
	}
}

// trayTooltip is what hovering the icon says.
func trayTooltip() string { return appName + " v" + Version }
