//go:build windows

package main

import (
	"os"
	"runtime"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The Windows notification-area icon, built on plain Win32 through
// golang.org/x/sys/windows — no cgo, no third-party tray package.
//
// The shell reports tray events by *posting a message to a window we own*, and
// the app's main window belongs to Wails, so this file brings its own window: a
// hidden one that exists only to receive `trayMessage` and WM_COMMAND. Win32
// windows belong to the thread that created them, so that window — and the
// message pump it needs — gets a thread of its own instead of borrowing Wails'
// main thread, which is already pumping the app's windows.
//
// Pointers handed to Win32 are followed by runtime.KeepAlive where the value is
// not read again afterwards: a uintptr argument hides the value from the
// collector, and a call is exactly where a collection may run.

const (
	// trayClassName names the hidden window, and trayWindowTitle is its (never
	// seen) title. The class name is also what a smoke test can look up with
	// FindWindow to check that the tray came up.
	trayClassName   = "DBManagerTray"
	trayWindowTitle = "DB Manager tray"

	// trayIconID tells our icon apart from any other icon the process registers.
	trayIconID = 1

	wmApp = 0x8000

	// trayMessage is the callback message the shell posts for our icon; WM_APP is
	// the first value an application may use for itself.
	trayMessage = wmApp + 1

	wmNull    = 0x0000
	wmDestroy = 0x0002
	wmClose   = 0x0010
	wmCommand = 0x0111

	wmLButtonUp     = 0x0202
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205
	wmRButtonDblClk = 0x0206

	// Shell_NotifyIcon: what to do with the icon, and which of its fields count.
	nimAdd     = 0x00000000
	nimDelete  = 0x00000002
	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	// Menu flags for the popup that every right-click builds from scratch.
	menuString     = 0x00000000
	menuGrayed     = 0x00000001
	menuDisabled   = 0x00000002
	menuChecked    = 0x00000008
	menuPopup      = 0x00000010
	menuSeparator  = 0x00000800
	menuRightClick = 0x0002

	// How GetMenuState and GetMenuString are asked: by row rather than by command
	// id.
	menuByPosition = 0x00000400

	// The stock application icon, for a build with no icon resource to extract
	// (a plain `go run` has none).
	idiApplication = 32512

	cwUseDefault       = 0x80000000
	swHide             = 0
	wsOverlappedWindow = 0x00CF0000
	csHREDRAW          = 0x0001
	csVREDRAW          = 0x0002
	colorWindow        = 6 // COLOR_WINDOW + 1: the class background brush
)

var (
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procShellNotifyIcon     = shell32.NewProc("Shell_NotifyIconW")
	procExtractIconEx       = shell32.NewProc("ExtractIconExW")
	procAppendMenu          = user32.NewProc("AppendMenuW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procCreateWindowEx      = user32.NewProc("CreateWindowExW")
	procDefWindowProc       = user32.NewProc("DefWindowProcW")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procDispatchMessage     = user32.NewProc("DispatchMessageW")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procGetMenuItemCount    = user32.NewProc("GetMenuItemCount")
	procGetMenuState        = user32.NewProc("GetMenuState")
	procGetMenuString       = user32.NewProc("GetMenuStringW")
	procGetSubMenu          = user32.NewProc("GetSubMenu")
	procGetMessage          = user32.NewProc("GetMessageW")
	procLoadIcon            = user32.NewProc("LoadIconW")
	procPostMessage         = user32.NewProc("PostMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procRegisterClassEx     = user32.NewProc("RegisterClassExW")
	procRegisterWindowMsg   = user32.NewProc("RegisterWindowMessageW")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procShowWindow          = user32.NewProc("ShowWindow")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procUnregisterClass     = user32.NewProc("UnregisterClassW")

	procGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
)

// notifyIconData is NOTIFYICONDATAW. The field order and types mirror the SDK
// header, so Go's natural alignment lines up with the C struct; `Size` is filled
// in from unsafe.Sizeof, which is how the shell knows which version of the
// structure it was handed. tray_windows_test.go pins the offsets.
//
// The balloon fields (Info, InfoTitle, InfoFlags, the timeout/version union and
// BalloonIcon) are never written: the icon is the way back to the window, not a
// notification, so nothing here pops a balloon. They stay in the struct because
// `Size` is computed from it as a whole — dropping them would shrink the record
// the shell is handed.
type notifyIconData struct {
	Size             uint32
	Wnd              windows.Handle
	ID               uint32
	Flags            uint32
	CallbackMessage  uint32
	Icon             windows.Handle
	Tip              [128]uint16
	State            uint32
	StateMask        uint32
	Info             [256]uint16
	TimeoutOrVersion uint32
	InfoTitle        [64]uint16
	InfoFlags        uint32
	GUID             windows.GUID
	BalloonIcon      windows.Handle
}

// wndClassEx is WNDCLASSEXW.
type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

// point is POINT.
type point struct{ X, Y int32 }

// msg is MSG.
type msg struct {
	Wnd     windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

// tray is the notification-area icon.
type tray struct {
	actions trayActions

	// hwnd is the hidden window the shell reports to; 0 until it exists. It is
	// read from other threads (close) while the tray thread owns it, hence the
	// atomic.
	hwnd atomic.Uintptr

	// up is true while the icon is really in the notification area. The app only
	// hides its window behind the icon while this holds, so a tray that never
	// appeared cannot leave the process running with no way back to it.
	up atomic.Bool

	// The rest belongs to the tray thread alone.

	// data is the live icon record: Shell_NotifyIcon needs the very same fields
	// (window, id, callback message) on every call, including the re-add after
	// explorer.exe restarts.
	data *notifyIconData

	// taskbarCreated is the message the shell broadcasts when the taskbar (and
	// with it every tray icon) is gone; 0 if it could not be registered.
	taskbarCreated uint32

	// class is the window class, kept for unregistering on the way out.
	class *wndClassEx
}

// activeTray is the tray the window procedure belongs to. There is exactly one
// per process, and the procedure needs to find it from a plain function pointer;
// the tray thread reads it while startup writes it, hence the atomic.
var activeTray atomic.Pointer[tray]

// trayWndProcCallback must outlive every call the shell makes, so it is created
// once at package level rather than per tray.
var trayWndProcCallback = windows.NewCallback(trayWndProc)

// newTray puts the icon in the notification area and returns it. The icon itself
// is created on another thread; until it is really there `running` reports false,
// which the close hook treats as "there is no tray, let the window close".
func newTray(actions trayActions) *tray {
	t := &tray{actions: actions}
	activeTray.Store(t)
	go t.serve()
	return t
}

// running reports whether the icon is in the notification area.
func (t *tray) running() bool { return t.up.Load() }

// close takes the icon down and lets the tray thread finish. Called on the way
// out; if the window never came up this does nothing.
func (t *tray) close() {
	if hwnd := t.hwnd.Load(); hwnd != 0 {
		postMessage(windows.Handle(hwnd), wmClose, 0, 0)
	}
}

// serve owns the tray from beginning to end: it creates the hidden window on a
// thread locked to itself, adds the icon, and pumps messages until that window is
// destroyed.
func (t *tray) serve() {
	// A Win32 window can only be used by the thread that created it, so this
	// goroutine must not be rescheduled onto another one.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hwnd, class, err := createTrayWindow()
	if err != nil {
		return
	}
	t.hwnd.Store(uintptr(hwnd))
	t.class = class
	t.taskbarCreated = registerWindowMessage("TaskbarCreated")

	data := &notifyIconData{
		Size:            uint32(unsafe.Sizeof(notifyIconData{})),
		Wnd:             hwnd,
		ID:              trayIconID,
		Flags:           nifMessage | nifIcon | nifTip,
		CallbackMessage: trayMessage,
		Icon:            trayIcon(),
	}
	utf16Fill(data.Tip[:], trayTooltip())
	t.data = data

	if ret, _, _ := procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(data))); ret == 0 {
		destroyWindow(hwnd, class)
		return
	}
	t.up.Store(true)

	var m msg
	for {
		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 { // 0 is WM_QUIT, -1 an error; both end the loop
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
	t.up.Store(false)
}

// dispatch runs the menu row the user picked.
//
// The two settings rows are looked up rather than listed: their ids come in
// groups of three, and the value each one stands for belongs next to the choices
// in tray.go, not here.
func (t *tray) dispatch(cmd trayCommand) {
	if theme, ok := trayThemeOf(cmd); ok {
		t.actions.setTheme(theme)
		return
	}
	if language, ok := trayLanguageOf(cmd); ok {
		t.actions.setLanguage(language)
		return
	}
	switch cmd {
	case trayCommandShowWindow:
		t.actions.showWindow()
	case trayCommandProjectPage:
		t.actions.openRepo()
	case trayCommandReportIssue:
		t.actions.openIssue()
	case trayCommandQuit:
		t.actions.quit()
	}
}

// showMenu pops the tray menu at the cursor. TrackPopupMenu runs a modal loop and
// sends WM_COMMAND for the picked row, so this blocks the tray thread — and the
// row is acted on by trayWndProc — until the menu goes away.
func (t *tray) showMenu(hwnd windows.Handle) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	// TrackPopupMenu does not destroy the menu, and it is rebuilt per right-click
	// so it can follow the version string.
	defer procDestroyMenu.Call(menu)

	appendTrayRows(menu, t.actions.prefs())

	// The window has to be in the foreground or the shell will not dismiss the
	// menu when the user clicks elsewhere; the null message afterwards is the
	// documented follow-up to that.
	procSetForegroundWindow.Call(uintptr(hwnd))
	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procTrackPopupMenu.Call(
		menu,
		menuRightClick,
		uintptr(pt.X),
		uintptr(pt.Y),
		0,
		uintptr(hwnd),
		0,
	)
	postMessage(hwnd, wmNull, 0, 0)
}

// appendTrayRows turns trayMenuRows into Win32 rows, for the settings in force.
// Split out from showMenu because it can be exercised — and the result read back
// with GetMenuString and GetMenuState — without a window or a message loop.
func appendTrayRows(menu uintptr, prefs trayPrefs) {
	appendRows(menu, trayMenuRows(prefs))
}

// appendRows draws one level of the menu: a separator for a separator, a
// greyed-out row for the version, and a tick on the setting in force. A heading
// gets a popup of its own, which Win32 takes ownership of — it is destroyed with
// the menu it was appended to, which is why only the top-level one is destroyed
// by hand in showMenu.
func appendRows(menu uintptr, rows []trayMenuRow) {
	for _, row := range rows {
		switch {
		case row.separator:
			procAppendMenu.Call(menu, menuSeparator, 0, 0)
		case row.submenu != nil:
			sub, _, _ := procCreatePopupMenu.Call()
			if sub == 0 {
				continue
			}
			appendRows(sub, row.submenu)
			// A popup row is identified by the handle of its submenu rather than by
			// a command id, which is what MF_POPUP tells the menu.
			appendMenu(menu, menuString|menuPopup, sub, row.label)
		default:
			flags := uint32(menuString)
			switch {
			case !row.enabled:
				flags |= menuDisabled | menuGrayed
			case row.checked:
				flags |= menuChecked
			}
			appendMenu(menu, flags, uintptr(row.command), row.label)
		}
	}
}

// trayWndProc handles the messages the shell and TrackPopupMenu send to the
// hidden window.
func trayWndProc(hwnd windows.Handle, message uint32, wparam, lparam uintptr) uintptr {
	t := activeTray.Load()
	switch {
	case message == trayMessage && t != nil:
		switch uint32(lparam) {
		case wmLButtonUp, wmLButtonDblClk:
			t.actions.showWindow()
		case wmRButtonUp, wmRButtonDblClk:
			t.showMenu(hwnd)
		}
		return 0

	case message == wmCommand && t != nil:
		// A picked menu row: the row's id is the low word of wParam.
		t.dispatch(trayCommand(uint16(wparam & 0xFFFF)))
		return 0

	case t != nil && t.taskbarCreated != 0 && message == t.taskbarCreated:
		// explorer.exe restarted and took every icon with it: put ours back.
		if ret, _, _ := procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(t.data))); ret != 0 {
			t.up.Store(true)
		}
		return 0

	case message == wmClose:
		// Our own "take it down" message, sent by close() on the way out.
		t.destroy(hwnd)
		return 0

	case message == wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(message), wparam, lparam)
	return ret
}

// destroy takes the icon out of the notification area and tears the window down,
// which ends the message loop in serve. Nil-safe: the procedure can be called for
// a window whose tray is already gone.
func (t *tray) destroy(hwnd windows.Handle) {
	var class *wndClassEx
	if t != nil {
		t.removeIcon()
		class = t.class
	}
	destroyWindow(hwnd, class)
}

// removeIcon takes the icon out of the notification area.
func (t *tray) removeIcon() {
	if t == nil || t.data == nil {
		return
	}
	// Shell_NotifyIcon answers TRUE/FALSE; the error value a caller gets back is
	// only meaningful when the call itself failed, so the return value is what
	// says whether the icon is really gone.
	if ret, _, _ := procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(t.data))); ret != 0 {
		t.up.Store(false)
	}
}

// createTrayWindow registers the window class and creates the hidden window the
// shell reports to. It must run on the thread that will pump its messages.
func createTrayWindow() (windows.Handle, *wndClassEx, error) {
	instance, _, _ := procGetModuleHandle.Call(0)
	className, err := windows.UTF16PtrFromString(trayClassName)
	if err != nil {
		return 0, nil, err
	}
	title, err := windows.UTF16PtrFromString(trayWindowTitle)
	if err != nil {
		return 0, nil, err
	}

	class := &wndClassEx{
		Style:      csHREDRAW | csVREDRAW,
		WndProc:    trayWndProcCallback,
		Instance:   windows.Handle(instance),
		Background: colorWindow,
		ClassName:  className,
	}
	class.Size = uint32(unsafe.Sizeof(*class))
	if ret, _, callErr := procRegisterClassEx.Call(uintptr(unsafe.Pointer(class))); ret == 0 {
		return 0, nil, callErr
	}
	runtime.KeepAlive(class)

	hwnd, _, callErr := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		wsOverlappedWindow,
		cwUseDefault,
		cwUseDefault,
		cwUseDefault,
		cwUseDefault,
		0,
		0,
		instance,
		0,
	)
	runtime.KeepAlive(className)
	runtime.KeepAlive(title)
	if hwnd == 0 {
		procUnregisterClass.Call(uintptr(unsafe.Pointer(className)), instance)
		return 0, nil, callErr
	}

	// A window without WS_VISIBLE is not shown in the first place; hiding it is
	// belt and braces, and it is what the other tray implementations do.
	procShowWindow.Call(hwnd, swHide)
	return windows.Handle(hwnd), class, nil
}

// destroyWindow tears the window (and its class) down, which ends the message
// loop in serve.
func destroyWindow(hwnd windows.Handle, class *wndClassEx) {
	procDestroyWindow.Call(uintptr(hwnd))
	if class != nil && class.ClassName != nil {
		procUnregisterClass.Call(uintptr(unsafe.Pointer(class.ClassName)), uintptr(class.Instance))
	}
}

// trayIcon returns the icon to put in the notification area.
//
// It is the one the build already embedded in the executable — the same artwork
// as the taskbar icon, from the same single source in asserts/ — and falls back
// to the stock application icon when there is nothing to extract.
func trayIcon() windows.Handle {
	if exe, err := os.Executable(); err == nil {
		if path, err := windows.UTF16PtrFromString(exe); err == nil {
			var large, small windows.Handle
			ret, _, _ := procExtractIconEx.Call(
				uintptr(unsafe.Pointer(path)),
				0,
				uintptr(unsafe.Pointer(&large)),
				uintptr(unsafe.Pointer(&small)),
				1,
			)
			runtime.KeepAlive(path)
			if ret > 0 {
				// Prefer the large one: notification areas are scaled up on
				// high-DPI screens, and the shell downsamples more legibly than
				// it upsamples.
				if large != 0 {
					return large
				}
				if small != 0 {
					return small
				}
			}
		}
	}
	if icon, _, _ := procLoadIcon.Call(0, idiApplication); icon != 0 {
		return windows.Handle(icon)
	}
	return 0
}

// appendMenu adds one row to the popup. The id is what the row reports back: a
// command id for a command, a submenu handle for a heading.
func appendMenu(menu uintptr, flags uint32, id uintptr, label string) {
	text, err := windows.UTF16PtrFromString(label)
	if err != nil {
		return
	}
	procAppendMenu.Call(menu, uintptr(flags), id, uintptr(unsafe.Pointer(text)))
	runtime.KeepAlive(text)
}

// registerWindowMessage asks the shell for the id of a broadcast message; 0
// means it could not be registered.
func registerWindowMessage(name string) uint32 {
	ptr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0
	}
	ret, _, _ := procRegisterWindowMsg.Call(uintptr(unsafe.Pointer(ptr)))
	runtime.KeepAlive(ptr)
	return uint32(ret)
}

// postMessage sends a message to a window without waiting for it to be handled.
func postMessage(hwnd windows.Handle, message uint32, wparam, lparam uintptr) {
	procPostMessage.Call(uintptr(hwnd), uintptr(message), wparam, lparam)
}

// utf16Fill copies a string into one of NOTIFYICONDATAW's fixed-size buffers.
// The copy stops at the buffer's end, and a string too long for it is cut short
// rather than left unterminated.
func utf16Fill(dst []uint16, s string) {
	encoded := windows.StringToUTF16(s) // includes the null terminator
	if len(encoded) > len(dst) {
		encoded = encoded[:len(dst)]
		encoded[len(dst)-1] = 0
	}
	copy(dst, encoded)
}
