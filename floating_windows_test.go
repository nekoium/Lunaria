//go:build windows

package main

import (
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestFloatingBoardModes(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := startGraphics(); err != nil {
		t.Fatal(err)
	}
	defer call("GdiplusShutdown", gdiToken)
	defer call("GdipDeleteFont", font)
	instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	class := u16("Lunaria.FloatingTest")
	wc := wndClass{Proc: syscall.NewCallback(windowProc), Instance: instance, ClassName: class}
	wc.Size = uint32(unsafe.Sizeof(wc))
	if ok, _, err := user32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc))); ok == 0 {
		t.Fatal(err)
	}
	defer user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(class)), instance)
	// Production creates the editor with this class; register it only for this test.
	productionClass := u16("Lunaria.NativeBoard")
	wc.ClassName = productionClass
	if ok, _, err := user32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc))); ok == 0 {
		t.Fatal(err)
	}
	defer user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(productionClass)), instance)
	previous := app
	previousForeground := editorForegroundWindow
	editorForegroundWindow = func() uintptr { return 0 }
	defer func() { editorForegroundWindow = previousForeground }()
	a := &application{width: 64, height: 64, selected: 0, smoke: true}
	app = a
	defer func() { app = previous }()
	a.host, _, _ = createWindowEx.Call(wsExToolWindow, uintptr(unsafe.Pointer(class)), 0, wsPopup, 0, 0, 0, 0, 0, 0, instance, 0)
	a.display, _, _ = createWindowEx.Call(wsExLayered|wsExToolWindow, uintptr(unsafe.Pointer(class)), 0, wsPopup, 0, 0, 64, 64, 0, 0, instance, 0)
	a.board = a.display
	if a.host == 0 || a.display == 0 {
		t.Fatal("create test windows")
	}
	defer func() {
		user32.NewProc("KillTimer").Call(a.host, 3)
		destroyWindow.Call(a.editor)
		destroyWindow.Call(a.display)
		destroyWindow.Call(a.host)
	}()
	im, err := loadImage("testdata/sample.png")
	if err != nil {
		t.Fatal(err)
	}
	defer call("GdipDisposeImage", im.Bitmap)
	im.X, im.Y, im.W, im.H = 30, 30, 20, 20
	a.images = []*boardImage{im}
	check := func(top, edit bool) {
		t.Helper()
		if a.floating != top || a.editing != edit {
			t.Fatalf("mode = top %v edit %v; want %v %v", a.floating, a.editing, top, edit)
		}
		ex, _, _ := getWindowLong.Call(a.board, gwlExStyle)
		if (ex&8 != 0) != top {
			t.Fatalf("topmost style = %#x; top = %v", ex, top)
		}
		if (ex&wsExTransparent == 0) != edit || (ex&wsExNoActivate == 0) != edit {
			t.Fatalf("input styles = %#x; edit = %v", ex, edit)
		}
		if top {
			parent, _, _ := getParent.Call(a.board)
			if parent != 0 || a.board != a.editor {
				t.Fatal("floating board must use unparented editor")
			}
		}
		if len(a.images) != 1 || a.images[0] != im || im.X != 30 || im.Y != 30 {
			t.Fatal("mode switch changed board content")
		}
	}
	foreground, _, _ := getForegroundWindow.Call()
	windowProc(a.host, wmHotKey, 2, 0)
	check(true, false)
	if after, _, _ := getForegroundWindow.Call(); after != foreground {
		t.Fatal("passive floating mode changed foreground window")
	}
	// Passive input must not move or select board content even if delivered directly.
	a.mouseDown(vector{30, 30})
	a.mouseMove(vector{45, 45})
	check(true, false)
	a.rememberForeground()
	if foreground != 0 && a.foregroundBeforeEdit != foreground {
		t.Fatal("did not remember the application before editing")
	}
	// Do not steal focus when the user has already switched to another app.
	a.smoke = false
	a.restoreForeground()
	a.smoke = true
	if after, _, _ := getForegroundWindow.Call(); after != foreground {
		t.Fatal("focus restoration interrupted another foreground application")
	}
	windowProc(a.host, wmHotKey, 1, 0)
	check(true, true)
	windowProc(a.host, wmHotKey, 2, 0)
	check(false, true)
	if a.selected != 0 {
		t.Fatal("placement toggle lost selection")
	}
	windowProc(a.host, wmHotKey, 2, 0)
	check(true, true)
	windowProc(a.board, wmKeyDown, 0x1b, 0)
	check(true, false)
	// Reverse an in-progress exit fade without dropping topmost or input.
	windowProc(a.host, wmHotKey, 1, 0)
	check(true, true)
	a.setEdit(false)
	a.editFadeStart = time.Now().Add(-time.Second)
	windowProc(a.host, wmTimer, 3, 0)
	check(true, false)
	// Explorer recovery must leave top mode unattached.
	windowProc(a.host, wmRecover, 0, 0)
	check(true, false)
	// Display recovery retains placement and resizes the top-level surface.
	windowProc(a.host, wmDisplayChange, 0, 0)
	windowProc(a.host, wmRecover, 0, 0)
	check(true, false)
	var bounds rect
	user32.NewProc("GetWindowRect").Call(a.board, uintptr(unsafe.Pointer(&bounds)))
	if int(bounds.Right-bounds.Left) != metric(0) || int(bounds.Bottom-bounds.Top) != metric(1) {
		t.Fatal("floating surface did not resize after display change")
	}
	// Return to desktop while closing: finish the fade in the new placement.
	a.setEdit(true)
	a.setEdit(false)
	a.setFloating(false)
	check(false, false)
	parent, _, _ := getParent.Call(a.board)
	if parent == 0 || parent != a.desktop {
		t.Fatal("desktop normal did not restore Explorer attachment")
	}
	windowProc(a.host, wmHotKey, 1, 0)
	check(false, true)
	// Enter top mode during the desktop exit fade, then return immediately.
	a.setEdit(false)
	a.setFloating(true)
	check(true, false)
	a.setFloating(false)
	check(false, false)

	// Simulate app activation without stealing focus from the user's applications.
	cover, _, _ := createWindowEx.Call(wsExToolWindow|wsExNoActivate, uintptr(unsafe.Pointer(class)), 0, wsPopup, 0, 0, 64, 64, 0, 0, instance, 0)
	if cover == 0 {
		t.Fatal("create covering app")
	}
	defer destroyWindow.Call(cover)
	for _, tc := range []struct {
		name       string
		style      uintptr
		foreground string
		wantEdit   bool
	}{
		{"maximized app", wsPopup | 0x10000000 | 0x01000000, "app", false},
		{"editor over maximized app", wsPopup | 0x10000000 | 0x01000000, "editor", false},
		{"tray over maximized app", wsPopup | 0x10000000 | 0x01000000, "tray", false},
		{"non-maximized app", wsPopup | 0x10000000, "app", true},
		{"hidden maximized app", wsPopup | 0x01000000, "app", true},
		{"minimized maximized app", wsPopup | 0x10000000 | 0x01000000 | 0x20000000, "app", true},
	} {
		// Native windows must stay on the creating OS thread; t.Run uses another goroutine.
		func() {
			editorForegroundWindow = func() uintptr { return 0 }
			a.setFloating(true)
			a.setEdit(true)
			setWindowLong.Call(cover, gwlStyle, tc.style)
			a.foregroundBeforeEdit = cover
			foreground := cover
			if tc.foreground == "editor" {
				foreground = a.editor
			}
			if tc.foreground == "tray" {
				foreground = a.host
			}
			editorForegroundWindow = func() uintptr { return foreground }
			if a.desktopEditCovered(foreground) {
				t.Fatal("top editing must ignore maximized applications")
			}
			windowProc(a.host, wmHotKey, 2, 0)
			if a.editing != tc.wantEdit {
				t.Fatalf("%s: editing = %v; want %v", tc.name, a.editing, tc.wantEdit)
			}
			check(false, tc.wantEdit)
			if !tc.wantEdit {
				a.editFadeStart = time.Now().Add(-time.Second)
				windowProc(a.host, wmTimer, 3, 0)
				check(false, false)
			}
		}()
	}
	// The timer must apply the same rule after an app maximizes during desktop editing.
	editorForegroundWindow = func() uintptr { return 0 }
	a.setFloating(true)
	a.setEdit(true)
	a.setFloating(false)
	timerCover, _, _ := createWindowEx.Call(wsExToolWindow|wsExNoActivate, uintptr(unsafe.Pointer(class)), 0, wsPopup, 0, 0, 64, 64, 0, 0, instance, 0)
	if timerCover == 0 {
		t.Fatal("create timer covering app")
	}
	defer destroyWindow.Call(timerCover)
	setWindowLong.Call(timerCover, gwlStyle, wsPopup|0x10000000|0x01000000)
	editorForegroundWindow = func() uintptr { return timerCover }
	if !a.desktopEditCovered(timerCover) {
		t.Fatal("timer fixture is not covered")
	}
	a.smoke = false
	a.watchEditor()
	a.smoke = true
	check(false, false)
	// Covered entry on a fresh launch must not try to fade the desktop surface.
	a.editFadeStart = time.Now().Add(-time.Second)
	windowProc(a.host, wmTimer, 3, 0)
	destroyWindow.Call(a.editor)
	a.editor = 0
	windowProc(a.host, wmHotKey, 1, 0)
	check(false, false)
	if a.editor != 0 || a.editClosing || a.board != a.display {
		t.Fatal("covered desktop entry created an editor or left a fade active")
	}
}
