//go:build windows

package main

import (
	"time"
	"unsafe"
)

func (a *application) rememberForeground() {
	foreground, _, _ := getForegroundWindow.Call()
	if foreground != 0 && foreground != a.editor && foreground != a.host {
		a.foregroundBeforeEdit = foreground
	}
}

func (a *application) restoreForeground() {
	if a.smoke || !a.floating || a.foregroundBeforeEdit == 0 {
		return
	}
	foreground, _, _ := getForegroundWindow.Call()
	valid, _, _ := user32.NewProc("IsWindow").Call(a.foregroundBeforeEdit)
	if valid != 0 && (foreground == a.editor || foreground == a.host) {
		setForegroundWindow.Call(a.foregroundBeforeEdit)
	}
}

func (a *application) setFloating(floating bool) {
	if a.changing || a.floating == floating {
		return
	}
	a.stopDrag()
	// A placement switch completes an outgoing fade in the new placement.
	if a.editClosing {
		user32.NewProc("KillTimer").Call(a.host, 3)
		a.editFadeStart = time.Time{}
		a.editClosing = false
	}
	a.floating = floating
	a.changing = true
	defer func() { a.changing = false }()
	if err := a.showBoard(); err != nil {
		a.failRender(err)
	}
}

func (a *application) showBoard() error {
	if a.desktopEditCovered(editorForegroundWindow()) {
		if a.editor == 0 || a.board != a.editor {
			// There is no visible editor to fade when entry is already covered.
			user32.NewProc("KillTimer").Call(a.host, 3)
			a.editFadeStart = time.Time{}
			a.editClosing, a.editing = false, false
			a.selected = -1
			return a.showBoard()
		}
		// Apply desktop auto-exit before activation can hide the covering app.
		setWindowPos.Call(a.editor, hwndNotTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
		changing := a.changing
		a.changing = false
		a.setEdit(false)
		a.changing = changing
		return nil
	}
	if a.editing || a.floating {
		return a.showEditor()
	}
	a.board = a.display
	if err := a.attach(); err != nil {
		showWindow.Call(a.board, swHide)
		showWindow.Call(a.editor, swHide)
		return err
	}
	showWindow.Call(a.editor, swHide)
	return nil
}

func (a *application) showEditor() error {
	valid, _, _ := user32.NewProc("IsWindow").Call(a.editor)
	if valid == 0 {
		instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
		var err error
		a.editor, _, err = createWindowEx.Call(wsExLayered|wsExToolWindow, uintptr(unsafe.Pointer(u16("Lunaria.NativeBoard"))), uintptr(unsafe.Pointer(u16("Lunaria Board"))), wsPopup, 0, 0, uintptr(a.width), uintptr(a.height), 0, 0, instance, 0)
		if a.editor == 0 {
			return apiError("create board overlay", err)
		}
	}
	if a.floating {
		user32.NewProc("RemovePropW").Call(a.editor, uintptr(unsafe.Pointer(u16("NonRudeHWND"))))
	} else {
		// Keep Explorer from lowering the taskbar for this screen-sized editor.
		if ok, _, err := user32.NewProc("SetPropW").Call(a.editor, uintptr(unsafe.Pointer(u16("NonRudeHWND"))), 1); ok == 0 {
			return apiError("keep editor below taskbar", err)
		}
	}
	a.board = a.editor
	ex := uintptr(wsExLayered | wsExToolWindow)
	if !a.editing {
		ex |= wsExTransparent | wsExNoActivate
	}
	setWindowLong.Call(a.editor, gwlExStyle, ex)
	if a.editing {
		dragAcceptFiles.Call(a.editor, 1)
	} else {
		dragAcceptFiles.Call(a.editor, 0)
	}
	after := hwndNotTopmost
	if a.floating {
		after = hwndTopmost
	}
	if ok, _, err := setWindowPos.Call(a.editor, after, 0, 0, uintptr(a.width), uintptr(a.height), swpNoActivate|swpFrameChanged); ok == 0 {
		return apiError("place board overlay", err)
	}
	if err := a.render(); err != nil {
		return err
	}
	if a.editing {
		a.activateEditor()
	} else {
		a.restoreForeground()
		showWindow.Call(a.editor, swShowNoActivate)
	}
	// Keep the outgoing surface visible until its replacement is ready.
	showWindow.Call(a.display, swHide)
	return nil
}
