//go:build windows

package main

import (
	"runtime"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// We hand the shell structs we laid out ourselves, so their layout is part of
// the contract: NOTIFYICONDATAW's fields have to sit at the offsets the SDK
// header puts them at, and `cbSize` has to be the header's sizeof — that is how
// the shell knows which version of the structure it was given.
//
// The numbers below are the 64-bit ones, which is every build we ship
// (windows/amd64, windows/arm64); a 32-bit build would have different ones.
func TestNotifyIconDataFollowsTheSDKLayout(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("only 64-bit builds are shipped, and these are their offsets")
	}
	var d notifyIconData
	for _, field := range []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"Wnd", unsafe.Offsetof(d.Wnd), 8},
		{"ID", unsafe.Offsetof(d.ID), 16},
		{"Flags", unsafe.Offsetof(d.Flags), 20},
		{"CallbackMessage", unsafe.Offsetof(d.CallbackMessage), 24},
		{"Icon", unsafe.Offsetof(d.Icon), 32},
		{"Tip", unsafe.Offsetof(d.Tip), 40},
		{"State", unsafe.Offsetof(d.State), 296},
		{"StateMask", unsafe.Offsetof(d.StateMask), 300},
		{"Info", unsafe.Offsetof(d.Info), 304},
		{"TimeoutOrVersion", unsafe.Offsetof(d.TimeoutOrVersion), 816},
		{"InfoTitle", unsafe.Offsetof(d.InfoTitle), 820},
		{"InfoFlags", unsafe.Offsetof(d.InfoFlags), 948},
		{"GUID", unsafe.Offsetof(d.GUID), 952},
		{"BalloonIcon", unsafe.Offsetof(d.BalloonIcon), 968},
	} {
		if field.got != field.want {
			t.Errorf("notifyIconData.%s sits at %d, want %d", field.name, field.got, field.want)
		}
	}
	if got := unsafe.Sizeof(d); got != 976 {
		t.Errorf("notifyIconData is %d bytes, want 976", got)
	}
}

// GetMessageW fills in a whole MSG, the SDK's private tail included, so our
// struct has to be at least as long as the C one or the call would write past it.
func TestMsgCoversTheSDKStruct(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("only 64-bit builds are shipped, and these are their offsets")
	}
	var m msg
	if got := unsafe.Sizeof(m); got != 48 {
		t.Errorf("msg is %d bytes, want 48", got)
	}
	if got := unsafe.Offsetof(m.Pt); got != 36 {
		t.Errorf("msg.Pt sits at %d, want 36", got)
	}
}

// appendTrayRows is what a right click actually builds, so reading the menu back
// out of Win32 is the closest thing to a test of the menu the user sees: the
// labels have to arrive intact — in whichever language is in force — the version
// row has to be greyed out, the separators have to be separators, and the two
// settings have to really be submenus with the setting in force ticked.
func TestTrayMenuReachesWin32(t *testing.T) {
	// Deliberately not the defaults: the rows a right click builds depend on the
	// settings, and this is the path that has to carry them through.
	prefs := trayPrefs{theme: trayThemeDark, language: "zh-TW"}

	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		t.Fatal("CreatePopupMenu failed")
	}
	defer procDestroyMenu.Call(menu)

	appendTrayRows(menu, prefs)
	checkMenu(t, menu, trayMenuRows(prefs), "menu")
}

// checkMenu reads one level of a Win32 menu back and compares it with the rows the
// Go side asked for. A row holding a submenu is followed into it, so a heading
// that was never given one — or a submenu whose ticks went missing — fails here
// rather than in front of a user.
func checkMenu(t *testing.T, menu uintptr, rows []trayMenuRow, where string) {
	t.Helper()

	count, _, _ := procGetMenuItemCount.Call(menu)
	if got := int(count); got != len(rows) {
		t.Fatalf("%s has %d rows, want %d", where, got, len(rows))
	}

	for i, row := range rows {
		state, _, _ := procGetMenuState.Call(menu, uintptr(i), menuByPosition)
		if row.separator {
			if state&menuSeparator == 0 {
				t.Errorf("%s: row %d is not a separator", where, i)
			}
			continue
		}

		buf := make([]uint16, 128)
		length, _, _ := procGetMenuString.Call(
			menu,
			uintptr(i),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)),
			menuByPosition,
		)
		runtime.KeepAlive(buf)
		if got := windows.UTF16ToString(buf[:length]); got != row.label {
			t.Errorf("%s: row %d says %q, want %q", where, i, got, row.label)
		}

		disabled := state&(menuDisabled|menuGrayed) != 0
		if disabled == row.enabled {
			t.Errorf("%s: row %d (%q): clickable=%v, want clickable=%v", where, i, row.label, !disabled, row.enabled)
		}
		if checked := state&menuChecked != 0; checked != row.checked {
			t.Errorf("%s: row %d (%q): ticked=%v, want ticked=%v", where, i, row.label, checked, row.checked)
		}

		sub, _, _ := procGetSubMenu.Call(menu, uintptr(i))
		switch {
		case row.submenu == nil && sub != 0:
			t.Errorf("%s: row %d (%q) opened a submenu of its own", where, i, row.label)
		case row.submenu != nil && sub == 0:
			t.Errorf("%s: row %d (%q) has no submenu", where, i, row.label)
		case row.submenu != nil:
			if state&menuPopup == 0 {
				t.Errorf("%s: row %d (%q) is not a popup row", where, i, row.label)
			}
			checkMenu(t, sub, row.submenu, where+" > "+row.label)
		}
	}
}

// The icon has to exist: a tray icon that could not be loaded is no tray at all,
// and the app hides its window behind it.
func TestTrayIconExists(t *testing.T) {
	if icon := trayIcon(); icon == 0 {
		t.Error("trayIcon found neither an icon in the executable nor the stock one")
	}
}

// utf16Fill hands Win32 a buffer that has to end with a null, whatever the string
// does — a window title read past its terminator is a classic way to crash the
// shell's message loop.
func TestUTF16FillTerminates(t *testing.T) {
	buf := make([]uint16, 8)
	utf16Fill(buf, "hi")
	if buf[0] != 'h' || buf[1] != 'i' || buf[2] != 0 {
		t.Errorf("utf16Fill(\"hi\") = %v, want h i then a terminator", buf[:4])
	}

	// Longer than the buffer: it must be cut short, and still terminated.
	long := make([]uint16, 4)
	utf16Fill(long, "abcdefgh")
	if long[3] != 0 {
		t.Errorf("utf16Fill of a long string left the buffer unterminated: %v", long)
	}
}
