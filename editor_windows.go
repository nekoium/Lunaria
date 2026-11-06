//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var editorAbove uintptr
var editorOrderCallback = syscall.NewCallback(func(hwnd, param uintptr) uintptr {
	if app == nil || hwnd == app.editor || hwnd == app.display || hwnd == app.host {
		return 1
	}
	visible, _, _ := user32.NewProc("IsWindowVisible").Call(hwnd)
	minimized, _, _ := user32.NewProc("IsIconic").Call(hwnd)
	if visible == 0 || minimized != 0 {
		return 1
	}
	name := windowClass(hwnd)
	if name == "Progman" || name == "WorkerW" {
		return 0
	}
	ex, _, _ := getWindowLong.Call(hwnd, gwlExStyle)
	if ex&8 == 0 {
		editorAbove = hwnd
	}
	return 1
})

func windowClass(hwnd uintptr) string {
	var class [128]uint16
	user32.NewProc("GetClassNameW").Call(hwnd, uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
	return syscall.UTF16ToString(class[:])
}
func editorInsertionPoint() uintptr {
	editorAbove = 0
	enumWindows.Call(editorOrderCallback, 0)
	return editorAbove
}
func (a *application) activateEditor() {
	// Do not interrupt an Explorer drag or raise the editor above application windows.
	showWindow.Call(a.editor, swShowNoActivate)
	setWindowPos.Call(a.editor, editorInsertionPoint(), 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|swpShowWindow)
	if !a.smoke && !mouseHeld() {
		setForegroundWindow.Call(a.editor)
	}
}

type windowPosition struct {
	HWND, After uintptr
	X, Y, W, H  int32
	Flags       uint32
}

func (a *application) keepEditorBelowApps(address uintptr) {
	var position windowPosition
	kernel32.NewProc("RtlMoveMemory").Call(uintptr(unsafe.Pointer(&position)), address, unsafe.Sizeof(position))
	if position.Flags&4 != 0 {
		return
	}
	position.After = editorInsertionPoint()
	kernel32.NewProc("RtlMoveMemory").Call(address, uintptr(unsafe.Pointer(&position)), unsafe.Sizeof(position))
}
func mouseHeld() bool {
	state, _, _ := getAsyncKeyState.Call(1)
	return state&0x8000 != 0
}
func (a *application) watchEditor() {
	if a.smoke || !a.editing || a.changing || a.dialog || a.drag != "" || mouseHeld() {
		return
	}
	foreground, _, _ := getForegroundWindow.Call()
	if foreground == 0 {
		return
	}
	if foreground == a.editor {
		return
	}
	class := windowClass(foreground)
	if class == "Progman" || class == "WorkerW" || class == "Shell_TrayWnd" || class == "NotifyIconOverflowWindow" || class == "#32768" {
		return
	}
	zoomed, _, _ := user32.NewProc("IsZoomed").Call(foreground)
	if zoomed != 0 {
		a.setEdit(false)
	}
}
