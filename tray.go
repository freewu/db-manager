package main

import "sync/atomic"

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
//
// Two rows are the interface's own preferences — the display theme and the
// language. They are here because a window that has been put away is otherwise
// unreachable: with the icon as the only way back, "I cannot read this" or "this
// is too bright" would have to wait for a window that cannot be opened without
// reading it. The preferences themselves stay the frontend's: it paints them and
// it is the one that stores them, so a pick is handed over as an event
// (trayThemeEvent) and never written down on this side. What the menu ticks is
// read back out of the stored state, so the tick follows the setting wherever it
// was last changed.

// repoURL is what the tray's project and issue rows open.
const repoURL = "https://github.com/freewu/db-manager"

// issuesURL goes straight to a new issue: the row says "Report an issue", so it
// should land on the form rather than on the list of what is already known.
const issuesURL = repoURL + "/issues/new"

// trayThemeEvent and trayLanguageEvent carry a pick in the menu's two settings to
// the interface, which is what applies and stores it. The names are a contract
// with the frontend and are written there too (frontend/src/lib/tray.ts).
const (
	trayThemeEvent    = "tray:theme"
	trayLanguageEvent = "tray:language"
)

// The display modes the menu offers. These are the values the frontend stores —
// `ThemeMode` in frontend/src/lib/theme.ts — in the order the settings page
// offers them.
const (
	trayThemeLight  = "light"
	trayThemeDark   = "dark"
	trayThemeSystem = "system"
)

// trayLanguageEnglish is also the fallback: it is what the interface starts in
// and what anything unrecognised falls back to.
const trayLanguageEnglish = "en"

// uiStateKey is the key the frontend stores its preferences under — `STATE_KEY`
// in frontend/src/store/appStore.ts. The menu's two settings are read out of it.
const uiStateKey = "ui"

// trayPrefs is what the menu has to know in order to draw itself: which display
// mode is in force, and which language to be written in.
type trayPrefs struct {
	theme    string
	language string
}

// defaultTrayPrefs is what a fresh install has, and what a state file that cannot
// be read or is written by some other build falls back to. The two answers are
// the frontend's own defaults.
func defaultTrayPrefs() trayPrefs {
	return trayPrefs{theme: trayThemeLight, language: trayLanguageEnglish}
}

// prefsFromState reads the two settings out of the state blob the frontend
// stores, each falling back on its own: a build that knows a theme this one does
// not should still get the language right.
func prefsFromState(state map[string]any) trayPrefs {
	prefs := defaultTrayPrefs()
	ui, ok := state[uiStateKey].(map[string]any)
	if !ok {
		return prefs
	}
	if theme, ok := ui["theme"].(string); ok && isTrayTheme(theme) {
		prefs.theme = theme
	}
	if language, ok := ui["language"].(string); ok && isTrayLanguage(language) {
		prefs.language = language
	}
	return prefs
}

// isTrayTheme and isTrayLanguage report whether a stored value is one this build
// offers. Anything else was written by another build, and the menu leaves that
// setting at its default rather than ticking a row that is not the truth.
func isTrayTheme(theme string) bool {
	switch theme {
	case trayThemeLight, trayThemeDark, trayThemeSystem:
		return true
	}
	return false
}

func isTrayLanguage(language string) bool {
	for _, choice := range trayLanguageChoices() {
		if choice.value == language {
			return true
		}
	}
	return false
}

// trayPrefsStore holds the app's copy of what the tray menu should tick.
//
// The frontend is the only writer of these preferences, and every write goes
// through `App.SaveState`, which is where this copy is refreshed (`startup` seeds
// it for a fresh install, where nothing has been stored yet). Reading the state
// file on every right-click would work too, but a state file is written in place
// rather than renamed: a menu that says "light" because it read the file between
// two writes is a wrong answer for no gain.
type trayPrefsStore struct {
	current atomic.Pointer[trayPrefs]
}

// refresh takes the settings out of a state blob.
func (s *trayPrefsStore) refresh(state map[string]any) {
	prefs := prefsFromState(state)
	s.current.Store(&prefs)
}

// get is what the menu draws from. Before the first refresh it is the default,
// which is also what the interface starts in.
func (s *trayPrefsStore) get() trayPrefs {
	if prefs := s.current.Load(); prefs != nil {
		return *prefs
	}
	return defaultTrayPrefs()
}

// trayWords is every line the menu says, in one language.
//
// This is the only translated text on the Go side, and it is here rather than in
// the frontend's message tables because it is never drawn by the frontend: the
// tray menu is native. Nine lines do not need a translation system, so the three
// languages are spelled out — in the words the settings page already uses for the
// same choices, so that the menu and the page cannot describe one setting two
// ways.
type trayWords struct {
	showWindow        string
	displayTheme      string
	themeLight        string
	themeDark         string
	themeSystem       string
	interfaceLanguage string
	projectPage       string
	reportIssue       string
	quit              string
}

// trayWordsByLanguage is every language the menu can be written in. A language
// missing from here gets English, which is what the interface falls back to as
// well.
var trayWordsByLanguage = map[string]trayWords{
	trayLanguageEnglish: {
		showWindow:        "Show window",
		displayTheme:      "Display theme",
		themeLight:        "Light",
		themeDark:         "Dark",
		themeSystem:       "System",
		interfaceLanguage: "Interface language",
		projectPage:       "Project page",
		reportIssue:       "Report an issue",
		quit:              "Quit",
	},
	"zh-CN": {
		showWindow:        "显示窗口",
		displayTheme:      "显示主题",
		themeLight:        "浅色",
		themeDark:         "深色",
		themeSystem:       "跟随系统",
		interfaceLanguage: "界面语言",
		projectPage:       "项目主页",
		reportIssue:       "报告问题",
		quit:              "退出",
	},
	"zh-TW": {
		showWindow:        "顯示視窗",
		displayTheme:      "顯示主題",
		themeLight:        "淺色",
		themeDark:         "深色",
		themeSystem:       "跟隨系統",
		interfaceLanguage: "介面語言",
		projectPage:       "專案首頁",
		reportIssue:       "回報問題",
		quit:              "結束",
	},
}

// trayWordsFor is the menu's wording.
func trayWordsFor(language string) trayWords {
	if words, ok := trayWordsByLanguage[language]; ok {
		return words
	}
	return trayWordsByLanguage[trayLanguageEnglish]
}

// trayActions is what the tray menu can ask of the app. The platform layer holds
// these and calls the matching one when a row is picked.
type trayActions struct {
	showWindow func()
	openRepo   func()
	openIssue  func()
	quit       func()
	// prefs is read on every right-click rather than once, so the ticks follow the
	// settings the user last chose in the window.
	prefs func() trayPrefs
	// setTheme and setLanguage hand a pick to the frontend, which applies it and
	// stores it. Both take one of the values above; anything else is ignored
	// there, so a menu built by some other build cannot put the interface in a
	// state it does not know.
	setTheme    func(theme string)
	setLanguage func(language string)
}

// trayCommand names a row of the tray menu. The Windows layer uses the value as
// the Win32 menu id.
type trayCommand uint16

const (
	trayCommandShowWindow trayCommand = iota + 1
	trayCommandThemeLight
	trayCommandThemeDark
	trayCommandThemeSystem
	trayCommandLanguageEnglish
	trayCommandLanguageSimplified
	trayCommandLanguageTraditional
	trayCommandProjectPage
	trayCommandReportIssue
	trayCommandQuit
	// trayCommandVersion is a label rather than a command: its row is greyed out
	// and can never be picked, it is only there to be read.
	trayCommandVersion
)

// trayMenuRow is one line of the tray menu.
//
// A row is one of three things: a separator, a command, or a heading that opens a
// submenu. A heading is clickable — that is how a submenu is opened — but its
// `command` is unused: Win32 identifies that row by the submenu's handle instead.
type trayMenuRow struct {
	command   trayCommand
	label     string
	enabled   bool
	separator bool
	// checked marks the setting in force, in a submenu of settings where exactly
	// one row can be ticked.
	checked bool
	// submenu holds the rows of a heading, and is nil for every other kind of row.
	submenu []trayMenuRow
}

// trayChoice is one setting a submenu offers: the value the interface stores, the
// words the row says it in, and the id the platform layer reports back when it is
// picked.
type trayChoice struct {
	value   string
	label   string
	command trayCommand
}

// trayThemeChoices is the display modes, in the order they are offered.
func trayThemeChoices(words trayWords) []trayChoice {
	return []trayChoice{
		{value: trayThemeLight, label: words.themeLight, command: trayCommandThemeLight},
		{value: trayThemeDark, label: words.themeDark, command: trayCommandThemeDark},
		{value: trayThemeSystem, label: words.themeSystem, command: trayCommandThemeSystem},
	}
}

// trayLanguageChoices is the languages, each written in itself: a picker that
// spells 简体中文 in English is no use to the person who needs it, which is why
// these three labels are the one thing here that is not translated.
func trayLanguageChoices() []trayChoice {
	return []trayChoice{
		{value: trayLanguageEnglish, label: "English", command: trayCommandLanguageEnglish},
		{value: "zh-CN", label: "简体中文", command: trayCommandLanguageSimplified},
		{value: "zh-TW", label: "繁體中文", command: trayCommandLanguageTraditional},
	}
}

// trayThemeOf answers which display mode a row picks, if it is one of those rows.
func trayThemeOf(cmd trayCommand) (theme string, ok bool) {
	switch cmd {
	case trayCommandThemeLight:
		return trayThemeLight, true
	case trayCommandThemeDark:
		return trayThemeDark, true
	case trayCommandThemeSystem:
		return trayThemeSystem, true
	}
	return "", false
}

// trayLanguageOf answers which language a row picks, if it is one of those rows.
func trayLanguageOf(cmd trayCommand) (language string, ok bool) {
	for _, choice := range trayLanguageChoices() {
		if choice.command == cmd {
			return choice.value, true
		}
	}
	return "", false
}

// trayMenuRows is the tray menu, top to bottom: the way back to the window, the
// two settings it can change, the two places to go for the project, which version
// this is, and the way out.
//
// It lives here rather than in the platform file so the wording and the order
// have one home, and so a test can check them without a desktop.
func trayMenuRows(prefs trayPrefs) []trayMenuRow {
	words := trayWordsFor(prefs.language)
	return []trayMenuRow{
		{command: trayCommandShowWindow, label: words.showWindow, enabled: true},
		{separator: true},
		{label: words.displayTheme, enabled: true, submenu: choiceRows(trayThemeChoices(words), prefs.theme)},
		{label: words.interfaceLanguage, enabled: true, submenu: choiceRows(trayLanguageChoices(), prefs.language)},
		{separator: true},
		{command: trayCommandProjectPage, label: words.projectPage, enabled: true},
		{command: trayCommandReportIssue, label: words.reportIssue, enabled: true},
		{separator: true},
		{command: trayCommandVersion, label: "v" + Version},
		{separator: true},
		{command: trayCommandQuit, label: words.quit, enabled: true},
	}
}

// choiceRows turns the settings of a submenu into rows, ticking the one in force.
func choiceRows(choices []trayChoice, current string) []trayMenuRow {
	rows := make([]trayMenuRow, 0, len(choices))
	for _, choice := range choices {
		rows = append(rows, trayMenuRow{
			command: choice.command,
			label:   choice.label,
			enabled: true,
			checked: choice.value == current,
		})
	}
	return rows
}

// trayTooltip is what hovering the icon says.
func trayTooltip() string { return appName + " v" + Version }
