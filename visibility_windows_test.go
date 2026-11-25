//go:build windows

package main

import (
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// Reuse the native-window fixture without starting another app or taking focus.
func checkHiddenBoard(t *testing.T, a *application, im *boardImage) {
	t.Helper()
	checkMenu := func(hidden bool) {
		t.Helper()
		menu := a.createTrayMenu()
		if menu == 0 {
			t.Fatal("create tray menu")
		}
		defer destroyMenu.Call(menu)
		state, _, _ := user32.NewProc("GetMenuState").Call(menu, 5, 0)
		if state == 0xffffffff || (state&8 != 0) != hidden || state&3 != 0 {
			t.Fatalf("Hide Lunaria menu flags = %#x; hidden = %v", state, hidden)
		}
		var label [64]uint16
		user32.NewProc("GetMenuStringW").Call(menu, 5, uintptr(unsafe.Pointer(&label[0])), uintptr(len(label)), 0)
		if got := syscall.UTF16ToString(label[:]); got != "Hide Lunaria" {
			t.Fatalf("hide label = %q; want Hide Lunaria without a shortcut", got)
		}
		for _, id := range []uintptr{1, 2, 4} {
			state, _, _ := user32.NewProc("GetMenuState").Call(menu, id, 0)
			if state == 0xffffffff || (state&3 != 0) != hidden {
				t.Fatalf("menu command %d flags = %#x; hidden = %v", id, state, hidden)
			}
		}
		state, _, _ = user32.NewProc("GetMenuState").Call(menu, 3, 0)
		if state == 0xffffffff || state&3 != 0 {
			t.Fatal("Quit must remain enabled")
		}
		state, _, _ = user32.NewProc("GetMenuState").Call(menu, 6, 0)
		if state == 0xffffffff || (state&3 != 0) != (a.startupError != nil) {
			t.Fatal("Start at login must remain available while hidden unless its state is unreadable")
		}
	}
	checkHidden := func(floating bool) {
		t.Helper()
		if !a.hidden || a.editing || a.editClosing || !a.editFadeStart.IsZero() || a.drag != "" || a.floating != floating {
			t.Fatalf("hidden state: hidden=%v editing=%v closing=%v fade=%v drag=%q floating=%v", a.hidden, a.editing, a.editClosing, a.editFadeStart, a.drag, a.floating)
		}
		for _, hwnd := range []uintptr{a.display, a.editor} {
			if visible, _, _ := user32.NewProc("IsWindowVisible").Call(hwnd); visible != 0 {
				t.Fatal("hidden board surface is visible")
			}
		}
		if a.images[0] != im || im.X != 30 || im.Y != 30 || im.W != 20 || im.H != 20 || im.Angle != 0 {
			t.Fatal("hiding changed existing board content or geometry")
		}
	}
	checkMenu(false)
	for _, mode := range []struct{ floating, editing, closing bool }{
		{false, false, false}, {false, true, false}, {false, true, true},
		{true, false, false}, {true, true, false}, {true, true, true},
	} {
		a.setFloating(mode.floating)
		a.setEdit(mode.editing)
		if mode.editing {
			a.selected, a.drag = 0, "move"
		}
		if mode.closing {
			a.setEdit(false)
			// Exercise pending fade messages even with system animations disabled.
			a.editClosing = true
			a.editFadeStart = time.Now()
		}
		foreground, _, _ := getForegroundWindow.Call()
		a.setHidden(true)
		a.setHidden(true)
		checkHidden(mode.floating)
		checkMenu(true)
		windowProc(a.host, wmHotKey, 1, 0)
		windowProc(a.host, wmHotKey, 2, 0)
		windowProc(a.host, wmTray, 0, wmLButtonUp)
		windowProc(a.host, wmDisplayChange, 0, 0)
		windowProc(a.host, wmRecover, 0, 0)
		windowProc(a.host, wmTimer, 3, 0)
		checkHidden(mode.floating)
		// Explorer may destroy either surface while the user has it hidden.
		destroyWindow.Call(a.editor)
		destroyWindow.Call(a.display)
		a.board, a.editor, a.display = 0, 0, 0
		windowProc(a.host, wmRecover, 0, 0)
		if err := a.render(); err != nil {
			t.Fatalf("hidden rendering after surface destruction: %v", err)
		}
		if a.editor != 0 || a.display != 0 {
			t.Fatal("recovery recreated a surface while hidden")
		}
		checkHidden(mode.floating)
		a.setHidden(false)
		a.setHidden(false)
		if a.hidden || a.editing || a.floating != mode.floating {
			t.Fatal("unhiding did not restore previous placement in normal mode")
		}
		if visible, _, _ := user32.NewProc("IsWindowVisible").Call(a.board); visible == 0 {
			t.Fatal("unhiding did not restore a visible board")
		}
		if mode.floating {
			ex, _, _ := getWindowLong.Call(a.board, gwlExStyle)
			if a.board != a.editor || ex&8 == 0 || ex&wsExTransparent == 0 {
				t.Fatal("unhiding did not restore click-through floating placement")
			}
		} else if parent, _, _ := getParent.Call(a.board); parent == 0 || parent != a.desktop {
			t.Fatal("unhiding did not restore desktop attachment")
		}
		if after, _, _ := getForegroundWindow.Call(); after != foreground {
			t.Fatal("visibility toggle changed the user's foreground application")
		}
		checkMenu(false)
	}
	// A pending import can complete while hidden without showing either surface.
	a.setHidden(true)
	imported := &boardImage{Bitmap: im.Bitmap, Pixels: im.Pixels, W: 20, H: 20}
	a.imports = &imageImports{active: &imageImport{}, result: make(chan importResult, 1)}
	a.imports.result <- importResult{item: imported}
	a.importTick()
	if len(a.images) != 2 || a.images[1] != imported || a.imports.active != nil {
		t.Fatal("pending import did not complete while hidden")
	}
	checkHidden(true)
	a.imports = nil
	a.setHidden(false)
}
