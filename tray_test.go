package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The tray menu is the app's only menu that is always there, so its rows are
// pinned: Show window, the two settings, the two links, the version as something
// to read rather than to click, and Quit — in that order. The settings come
// second because they are the reason the menu has to be reachable at all: a
// window that has been put away cannot be read or re-themed from anywhere else.
func TestTrayMenuIsTheRowsWePromise(t *testing.T) {
	var labels []string
	for _, row := range trayMenuRows(defaultTrayPrefs()) {
		if row.separator {
			labels = append(labels, "---")
			continue
		}
		labels = append(labels, row.label)
	}

	want := []string{
		"Show window", "---", "Display theme", "Interface language", "---",
		"Project page", "Report an issue", "---", "v" + Version, "---", "Quit",
	}
	if got := strings.Join(labels, "|"); got != strings.Join(want, "|") {
		t.Errorf("tray menu = %q, want %q", got, strings.Join(want, "|"))
	}

	enabled := map[trayCommand]bool{}
	for _, row := range trayMenuRows(defaultTrayPrefs()) {
		enabled[row.command] = row.enabled
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

// Every row that runs a command needs its own id: Win32 reports the picked row by
// id, so two rows sharing one would make a click act twice. Rows are collected
// from the whole tree — the settings live one level down.
func TestTrayCommandsAreUnique(t *testing.T) {
	rows := flattenTrayRows(trayMenuRows(defaultTrayPrefs()))
	seen := map[trayCommand]trayMenuRow{}
	clickable := 0
	version := false
	for _, row := range rows {
		if row.separator || row.submenu != nil {
			continue
		}
		if first, ok := seen[row.command]; ok {
			t.Errorf("two rows share the command id %d: %q and %q", row.command, first.label, row.label)
		}
		seen[row.command] = row
		if row.enabled {
			clickable++
		}
		if row.command == trayCommandVersion {
			version = true
			if row.enabled {
				t.Error("the version row is clickable, want it greyed out")
			}
		}
	}

	// Four rows that do something and the six settings; a menu that lost one would
	// still pass the labels above only if the labels were changed with it, so this
	// is the count that keeps the ids honest.
	want := 4 + len(trayThemeChoices(trayWordsFor(trayLanguageEnglish))) + len(trayLanguageChoices())
	if clickable != want {
		t.Errorf("%d rows carry a command, want %d", clickable, want)
	}
	if !version {
		t.Error("no row says which version this is")
	}
}

// The two submenus are the interface's own preferences, so what they show has to
// be what is in force: exactly one row ticked per submenu, and the wording in the
// language the interface is in.
func TestTraySettingsShowWhatIsInForce(t *testing.T) {
	rows := trayMenuRows(trayPrefs{theme: trayThemeDark, language: "zh-CN"})

	headings := map[string]trayMenuRow{}
	for _, row := range rows {
		if row.submenu != nil {
			headings[row.label] = row
		}
	}
	theme, ok := headings["显示主题"]
	if !ok {
		t.Fatalf("the theme submenu is missing from %v", headings)
	}
	language, ok := headings["界面语言"]
	if !ok {
		t.Fatalf("the language submenu is missing from %v", headings)
	}

	if got := tickedRows(theme.submenu); len(got) != 1 || got[0] != "深色" {
		t.Errorf("the theme submenu ticks %v, want only 深色", got)
	}
	if got := tickedRows(language.submenu); len(got) != 1 || got[0] != "简体中文" {
		t.Errorf("the language submenu ticks %v, want only 简体中文", got)
	}

	// A fresh install ticks the defaults, so the menu never comes up with nothing
	// ticked at all.
	defaults := trayMenuRows(defaultTrayPrefs())
	for _, row := range defaults {
		if row.submenu == nil {
			continue
		}
		if got := tickedRows(row.submenu); len(got) != 1 {
			t.Errorf("the submenu %q ticks %v, want exactly one row", row.label, got)
		}
	}
}

// A settings row has to know which setting it is: the platform layer gets nothing
// back from Win32 but the id, so a row whose id maps to the wrong value would
// silently switch the wrong thing.
func TestTraySettingsRowsPickTheirOwnSetting(t *testing.T) {
	words := trayWordsFor(trayLanguageEnglish)
	for _, choice := range trayThemeChoices(words) {
		theme, ok := trayThemeOf(choice.command)
		if !ok || theme != choice.value {
			t.Errorf("row %q maps to (%q, %v), want %q", choice.label, theme, ok, choice.value)
		}
		// A row that can be picked but never ticked is a setting the user cannot
		// see the state of, so the offered values and the recognised ones are the
		// same set.
		if !isTrayTheme(choice.value) {
			t.Errorf("the menu offers the theme %q but reads it back as unknown", choice.value)
		}
	}
	for _, choice := range trayLanguageChoices() {
		language, ok := trayLanguageOf(choice.command)
		if !ok || language != choice.value {
			t.Errorf("row %q maps to (%q, %v), want %q", choice.label, language, ok, choice.value)
		}
		if !isTrayLanguage(choice.value) {
			t.Errorf("the menu offers the language %q but reads it back as unknown", choice.value)
		}
	}
	// Nothing else may look like a setting, or a plain command would set one.
	for _, cmd := range []trayCommand{trayCommandShowWindow, trayCommandProjectPage, trayCommandReportIssue, trayCommandQuit, trayCommandVersion} {
		if theme, ok := trayThemeOf(cmd); ok {
			t.Errorf("row %d is read as the theme %q", cmd, theme)
		}
		if language, ok := trayLanguageOf(cmd); ok {
			t.Errorf("row %d is read as the language %q", cmd, language)
		}
	}
}

// Every language that can be picked has to have words of its own, or picking it
// would leave the menu in the language the user just left — the one thing a
// language picker must not do. The two lists are compared as sets: a language
// offered but not spoken is the bug this is here for.
func TestTraySpeaksEveryLanguage(t *testing.T) {
	offered := map[string]bool{}
	for _, choice := range trayLanguageChoices() {
		offered[choice.value] = true
	}
	for language := range offered {
		if _, ok := trayWordsByLanguage[language]; !ok {
			t.Errorf("the menu offers %q but has no words for it", language)
		}
	}
	for language := range trayWordsByLanguage {
		if !offered[language] {
			t.Errorf("the menu speaks %q but cannot be switched to it", language)
		}
	}

	english := trayWordsByLanguage[trayLanguageEnglish]
	for language, words := range trayWordsByLanguage {
		for _, line := range trayWordsLines(words) {
			if strings.TrimSpace(line) == "" {
				t.Errorf("%s has an empty line: %+v", language, words)
				break
			}
		}
		if language == trayLanguageEnglish {
			continue
		}
		// A line left in English is a line nobody translated. The language names in
		// the picker are the deliberate exception, and they are not in here.
		for _, pair := range [][2]string{
			{words.showWindow, english.showWindow},
			{words.displayTheme, english.displayTheme},
			{words.themeLight, english.themeLight},
			{words.themeDark, english.themeDark},
			{words.themeSystem, english.themeSystem},
			{words.interfaceLanguage, english.interfaceLanguage},
			{words.projectPage, english.projectPage},
			{words.reportIssue, english.reportIssue},
			{words.quit, english.quit},
		} {
			if pair[0] == pair[1] {
				t.Errorf("%s still says %q in English", language, pair[0])
			}
		}
	}

	// Anything unrecognised gets English rather than an empty menu.
	if got := trayWordsFor("zh-Hans"); got != english {
		t.Errorf("trayWordsFor(\"zh-Hans\") = %+v, want the English words", got)
	}
}

// The menu's ticks come from the state file the frontend writes, so reading it
// back has to survive everything a file can be: missing, half written, or written
// by a build that offered settings this one does not.
func TestTraySettingsComeFromTheStoredState(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state map[string]any
		want  trayPrefs
	}{
		{"nothing stored", nil, defaultTrayPrefs()},
		{"nothing under the key", map[string]any{"other": 1}, defaultTrayPrefs()},
		{
			"what the frontend writes",
			map[string]any{uiStateKey: map[string]any{"theme": "dark", "language": "zh-TW"}},
			trayPrefs{theme: trayThemeDark, language: "zh-TW"},
		},
		{
			"values from another build",
			map[string]any{uiStateKey: map[string]any{"theme": "neon", "language": "fr"}},
			defaultTrayPrefs(),
		},
		{
			"one setting at a time",
			map[string]any{uiStateKey: map[string]any{"theme": trayThemeSystem}},
			trayPrefs{theme: trayThemeSystem, language: trayLanguageEnglish},
		},
		{
			"a shape that is not an object",
			map[string]any{uiStateKey: "dark"},
			defaultTrayPrefs(),
		},
		{
			"values that are not strings",
			map[string]any{uiStateKey: map[string]any{"theme": 7.0, "language": nil}},
			defaultTrayPrefs(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := prefsFromState(tc.state); got != tc.want {
				t.Errorf("prefsFromState(%v) = %+v, want %+v", tc.state, got, tc.want)
			}
		})
	}
}

// The store is what the menu draws from while the window is hidden, so it has to
// have an answer before anything was ever stored, and to follow the last write.
func TestTraySettingsStoreKeepsTheLastWrite(t *testing.T) {
	var store trayPrefsStore
	if got := store.get(); got != defaultTrayPrefs() {
		t.Errorf("a store that was never refreshed = %+v, want the defaults", got)
	}

	store.refresh(map[string]any{uiStateKey: map[string]any{"theme": trayThemeDark, "language": "zh-CN"}})
	want := trayPrefs{theme: trayThemeDark, language: "zh-CN"}
	if got := store.get(); got != want {
		t.Errorf("after refreshing = %+v, want %+v", got, want)
	}

	// The frontend writes all of its preferences at once, so a later write always
	// replaces the whole answer rather than merging with it.
	store.refresh(map[string]any{uiStateKey: map[string]any{"theme": trayThemeLight}})
	want = trayPrefs{theme: trayThemeLight, language: trayLanguageEnglish}
	if got := store.get(); got != want {
		t.Errorf("after the second write = %+v, want %+v", got, want)
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

// The two event names are a contract with the frontend, which is what listens for
// them and what applies the pick. A typo on either side is a menu row that does
// nothing at all — silently, and only on Windows — so the names are read back out
// of the module that listens.
func TestTrayEventsAreTheNamesTheFrontendListensFor(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("frontend", "src", "lib", "tray.ts"))
	if err != nil {
		t.Skipf("the frontend is not checked out beside this package: %v", err)
	}
	for _, event := range []string{trayThemeEvent, trayLanguageEvent} {
		if !strings.Contains(string(source), "'"+event+"'") {
			t.Errorf("nothing in frontend/src/lib/tray.ts listens for %q", event)
		}
	}
}

// Every action has to be filled in: the Windows tray calls them from inside a
// window procedure, where a nil function would take the whole process down.
func TestTrayActionsAreAllWired(t *testing.T) {
	actions := new(App).trayActions()
	for _, fn := range []struct {
		name string
		fn   any
	}{
		{"show the window", actions.showWindow},
		{"open the project page", actions.openRepo},
		{"open the issue form", actions.openIssue},
		{"quit", actions.quit},
		{"read the settings the menu shows", actions.prefs},
		{"set the theme", actions.setTheme},
		{"set the language", actions.setLanguage},
	} {
		if fn.fn == nil {
			t.Errorf("the tray has no way to %s", fn.name)
		}
	}
	if got := actions.prefs(); got != defaultTrayPrefs() {
		t.Errorf("the tray's settings before anything is stored = %+v, want the defaults", got)
	}
}

// flattenTrayRows walks the whole menu, submenus included, so the tests that care
// about ids and wording see the rows the user sees.
func flattenTrayRows(rows []trayMenuRow) []trayMenuRow {
	var out []trayMenuRow
	for _, row := range rows {
		out = append(out, row)
		out = append(out, flattenTrayRows(row.submenu)...)
	}
	return out
}

// tickedRows is the labels of the ticked rows of a submenu.
func tickedRows(rows []trayMenuRow) []string {
	var out []string
	for _, row := range rows {
		if row.checked {
			out = append(out, row.label)
		}
	}
	return out
}

// trayWordsLines is every line of one language, for the checks that go over all
// of them.
func trayWordsLines(words trayWords) []string {
	return []string{
		words.showWindow,
		words.displayTheme,
		words.themeLight,
		words.themeDark,
		words.themeSystem,
		words.interfaceLanguage,
		words.projectPage,
		words.reportIssue,
		words.quit,
	}
}
