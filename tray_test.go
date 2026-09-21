package main

import (
	"strings"
	"testing"
)

// The tray menu is the app's only menu that is always there, so its rows are
// pinned: Show window / Project page / Report an issue / <version> / Quit, in
// that order, with the version as something to read rather than to click.
func TestTrayMenuIsTheRowsWePromise(t *testing.T) {
	var (
		labels   []string
		commands []trayCommand
		enabled  = map[trayCommand]bool{}
	)
	for _, row := range trayMenuRows() {
		if row.separator {
			labels = append(labels, "---")
			continue
		}
		labels = append(labels, row.label)
		commands = append(commands, row.command)
		enabled[row.command] = row.enabled
	}

	want := []string{"Show window", "Project page", "Report an issue", "---", "v" + Version, "---", "Quit"}
	if got := strings.Join(labels, "|"); got != strings.Join(want, "|") {
		t.Errorf("tray menu = %q, want %q", got, strings.Join(want, "|"))
	}

	// Every row needs its own id: the Win32 menu reports the picked row by id, so
	// two rows sharing one would make a click act twice.
	seen := map[trayCommand]bool{}
	for _, cmd := range commands {
		if seen[cmd] {
			t.Errorf("tray command %d is used by two rows", cmd)
		}
		seen[cmd] = true
	}

	for _, cmd := range []trayCommand{
		trayCommandShowWindow,
		trayCommandProjectPage,
		trayCommandReportIssue,
		trayCommandQuit,
	} {
		if !enabled[cmd] {
			t.Errorf("tray command %d is disabled, want it clickable", cmd)
		}
	}
	if enabled[trayCommandVersion] {
		t.Error("the version row is clickable, want it greyed out")
	}
}

// The tooltip has to say which program this icon belongs to and which build it
// is, since the tray icon is all a user sees of a hidden window.
func TestTrayTooltipNamesTheVersion(t *testing.T) {
	tip := trayTooltip()
	if !strings.Contains(tip, appName) || !strings.Contains(tip, Version) {
		t.Errorf("tray tooltip = %q, want it to mention %q and %q", tip, appName, Version)
	}
}

// The two links are the ones the project-info pane shows, and they belong to
// this project rather than to whatever the tray icon was built from.
func TestTrayLinksPointAtTheProject(t *testing.T) {
	if repoURL != "https://github.com/freewu/db-manager" {
		t.Errorf("repoURL = %q", repoURL)
	}
	// "Report an issue" should land on the form, not on the list of what is
	// already known.
	if issuesURL != repoURL+"/issues/new" {
		t.Errorf("issuesURL = %q, want %q", issuesURL, repoURL+"/issues/new")
	}
}

// Every action has to be filled in: the Windows tray calls them from inside a
// window procedure, where a nil function would take the whole process down.
func TestTrayActionsAreAllWired(t *testing.T) {
	actions := new(App).trayActions()
	if actions.showWindow == nil {
		t.Error("the tray has no way to show the window")
	}
	if actions.openRepo == nil {
		t.Error("the tray has no project link")
	}
	if actions.openIssue == nil {
		t.Error("the tray has no issue link")
	}
	if actions.quit == nil {
		t.Error("the tray has no way to quit")
	}
}
