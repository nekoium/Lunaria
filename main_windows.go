//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

type application struct {
	board, host, desktop, desktopAfter uintptr
	display, editor                    uintptr
	width, height                      int
	images                             []*boardImage
	storage                            string
	selected                           int
	editing, changing, dialog          bool
	editFadeStart                      time.Time
	editFadeDuration                   time.Duration
	editFadeFrom                       float64
	editClosing                        bool
	drag                               string
	start                              vector
	original                           boardImage
	anchor                             vector
	tray                               notifyIconData
	taskbarCreated                     uint32
	smoke                              bool
	smokeStage                         int
	smokeErrors                        []string
}

var app *application

func main() {
	runtime.LockOSThread()
	if err := run(); err != nil {
		if app != nil && app.smoke {
			fmt.Fprintln(os.Stderr, err)
		} else {
			notifyError(err.Error())
		}
		os.Exit(1)
	}
}

func run() error {
	app = &application{selected: -1}
	for _, arg := range os.Args[1:] {
		if arg == "--smoke-test" {
			app.smoke = true
		}
	}
	// Physical desktop coordinates throughout; no framework DPI virtualization.
	if p := user32.NewProc("SetProcessDpiAwarenessContext"); p.Find() == nil {
		p.Call(^uintptr(3))
	}
	mutex, _, err := kernel32.NewProc("CreateMutexW").Call(0, 0, uintptr(unsafe.Pointer(u16("Local\\Lunaria.Prototype"))))
	if mutex == 0 {
		return apiError("CreateMutex", err)
	}
	defer kernel32.NewProc("CloseHandle").Call(mutex)
	if err == syscall.Errno(183) {
		return fmt.Errorf("Lunaria is already running. Use its notification-area icon")
	}
	if err := startGraphics(); err != nil {
		return err
	}
	defer call("GdiplusShutdown", gdiToken)
	defer call("GdipDeleteFont", font)
	defer func() {
		for _, im := range app.images {
			call("GdipDisposeImage", im.Bitmap)
		}
	}()
	instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	cursor, _, _ := loadCursor.Call(0, 32512)
	icon := makeTrayIcon()
	if icon != 0 {
		defer user32.NewProc("DestroyIcon").Call(icon)
	} else {
		icon, _, _ = loadIcon.Call(0, 32512)
	}
	wc := wndClass{Proc: syscall.NewCallback(windowProc), Instance: instance, Cursor: cursor, Icon: icon, ClassName: u16("Lunaria.NativeBoard")}
	wc.Size = uint32(unsafe.Sizeof(wc))
	if r, _, e := user32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return apiError("RegisterClassEx", e)
	}
	app.width = metric(0)
	app.height = metric(1)
	if err := app.openStorage(); err != nil {
		return err
	}
	app.host, _, err = createWindowEx.Call(wsExToolWindow, uintptr(unsafe.Pointer(wc.ClassName)), uintptr(unsafe.Pointer(u16("Lunaria"))), wsPopup, 0, 0, 0, 0, 0, 0, instance, 0)
	if app.host == 0 {
		return apiError("create tray host", err)
	}
	app.board, _, err = createWindowEx.Call(wsExLayered|wsExTransparent|wsExToolWindow|wsExNoActivate, uintptr(unsafe.Pointer(wc.ClassName)), uintptr(unsafe.Pointer(u16("Lunaria Board"))), wsPopup, 0, 0, uintptr(app.width), uintptr(app.height), 0, 0, instance, 0)
	if app.board == 0 {
		destroyWindow.Call(app.host)
		return apiError("create board", err)
	}
	app.display = app.board
	defer func() { destroyWindow.Call(app.display); destroyWindow.Call(app.editor) }()
	defer destroyWindow.Call(app.host)
	dragAcceptFiles.Call(app.board, 1)
	if r, _, e := registerHotKey.Call(app.host, 1, 0x4000|2|1, 'L'); r == 0 {
		return apiError("Ctrl+Alt+L is unavailable; close the conflicting application and try again", e)
	}
	defer unregisterHotKey.Call(app.host, 1)
	app.taskbarCreated = uint32(registerMessage("TaskbarCreated"))
	app.tray = notifyIconData{HWND: app.host, ID: 1, Flags: 1 | 2 | 4, CallbackMessage: wmTray, Icon: icon}
	app.tray.Size = uint32(unsafe.Sizeof(app.tray))
	copy(app.tray.Tip[:], syscall.StringToUTF16("Lunaria · Ctrl+Alt+L to edit"))
	if r, _, e := shellNotifyIcon.Call(0, uintptr(unsafe.Pointer(&app.tray))); r == 0 {
		return apiError("create tray icon", e)
	}
	defer shellNotifyIcon.Call(2, uintptr(unsafe.Pointer(&app.tray)))
	if err := app.attach(); err != nil {
		return err
	}
	user32.NewProc("SetTimer").Call(app.host, 1, 150, 0)
	defer user32.NewProc("KillTimer").Call(app.host, 1)
	for _, arg := range os.Args[1:] {
		if arg != "--smoke-test" {
			app.addFile(arg)
		}
	}
	if app.smoke {
		if err := os.MkdirAll(".tmp", 0700); err != nil {
			return err
		}
		if len(app.images) > 0 {
			app.images[0].X = app.images[0].W / 2
			app.images[0].Y = app.images[0].H / 2
		}
		app.setEdit(false)
		user32.NewProc("SetTimer").Call(app.host, 2, 600, 0)
	}
	var m msg
	for {
		result, _, e := user32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(result) == -1 {
			return apiError("GetMessage", e)
		}
		if result == 0 {
			break
		}
		user32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&m)))
		user32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&m)))
	}
	if app.smoke {
		return app.writeSmokeResult()
	}
	return nil
}
func metric(index uintptr) int { n, _, _ := getSystemMetrics.Call(index); return int(n) }
func registerMessage(name string) uintptr {
	n, _, _ := user32.NewProc("RegisterWindowMessageW").Call(uintptr(unsafe.Pointer(u16(name))))
	return n
}

func (a *application) attach() error {
	a.changing = true
	defer func() { a.changing = false }()
	showWindow.Call(a.board, swHide)
	valid, _, _ := user32.NewProc("IsWindow").Call(a.board)
	if valid == 0 {
		instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
		var err error
		a.board, _, err = createWindowEx.Call(wsExLayered|wsExTransparent|wsExToolWindow|wsExNoActivate, uintptr(unsafe.Pointer(u16("Lunaria.NativeBoard"))), uintptr(unsafe.Pointer(u16("Lunaria Board"))), wsPopup, 0, 0, uintptr(a.width), uintptr(a.height), 0, 0, instance, 0)
		if a.board == 0 {
			return apiError("recreate desktop board", err)
		}
		dragAcceptFiles.Call(a.board, 1)
	}
	a.display = a.board
	a.desktop, a.desktopAfter = desktopHost()
	if a.desktop == 0 {
		return fmt.Errorf("Explorer's desktop image layer is unavailable. Lunaria has kept its board hidden")
	}
	setWindowLong.Call(a.board, gwlStyle, wsChild)
	setWindowLong.Call(a.board, gwlExStyle, wsExLayered|wsExTransparent|wsExToolWindow|wsExNoActivate)
	dragAcceptFiles.Call(a.board, 1)
	setParent.Call(a.board, a.desktop)
	parent, _, _ := getParent.Call(a.board)
	if parent != a.desktop {
		return fmt.Errorf("could not attach the board to Explorer's desktop layer")
	}
	setWindowPos.Call(a.board, a.desktopAfter, 0, 0, uintptr(a.width), uintptr(a.height), swpNoActivate|swpFrameChanged)
	if err := a.render(); err != nil {
		return err
	}
	showWindow.Call(a.board, swShowNoActivate)
	if a.desktopAfter != 0 {
		if err := a.render(); err != nil {
			return err
		}
		wallpaper, _, _ := findWindowEx.Call(a.desktop, 0, uintptr(unsafe.Pointer(u16("WorkerW"))), 0)
		if wallpaper != 0 {
			setWindowPos.Call(wallpaper, 1, 0, 0, 0, 0, swpNoActivate|swpNoMove|swpNoSize)
		}
		setWindowPos.Call(a.board, a.desktopAfter, 0, 0, 0, 0, swpNoActivate|swpNoMove|swpNoSize)
	}
	return nil
}

func (a *application) setEdit(edit bool) {
	if a.changing || a.editing == edit {
		return
	}
	a.stopDrag()
	from := 0.0
	if a.editing || a.editClosing {
		from = a.editFade()
	}
	user32.NewProc("KillTimer").Call(a.host, 3)
	a.editFadeStart = time.Time{}
	a.editClosing = !edit
	a.editFadeFrom = from
	a.editFadeDuration = 150 * time.Millisecond
	if !edit {
		a.editFadeDuration = 90 * time.Millisecond
	}
	var animate int32 = 1
	user32.NewProc("SystemParametersInfoW").Call(0x1042, 0, uintptr(unsafe.Pointer(&animate)), 0)
	if animate != 0 {
		a.editFadeStart = time.Now()
		if timer, _, _ := user32.NewProc("SetTimer").Call(a.host, 3, 15, 0); timer == 0 {
			a.editFadeStart = time.Time{}
		}
	}
	a.changing = true
	defer func() { a.changing = false }()
	a.editing = edit
	if !edit {
		a.selected = -1
		if !a.editFadeStart.IsZero() {
			// Return desktop input immediately while the overlay finishes fading.
			setWindowLong.Call(a.editor, gwlExStyle, wsExLayered|wsExToolWindow|wsExTransparent|wsExNoActivate)
			a.redraw()
			return
		}
		a.finishEdit()
		return
	}
	a.editClosing = false
	if edit {
		valid, _, _ := user32.NewProc("IsWindow").Call(a.editor)
		if valid == 0 {
			instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
			a.editor, _, _ = createWindowEx.Call(wsExLayered|wsExToolWindow, uintptr(unsafe.Pointer(u16("Lunaria.NativeBoard"))), uintptr(unsafe.Pointer(u16("Lunaria — Edit mode"))), wsPopup, 0, 0, uintptr(a.width), uintptr(a.height), 0, 0, instance, 0)
			if a.editor == 0 {
				a.editing = false
				a.board = a.display
				notifyError("Could not open the editor")
				return
			}
			// Keep Explorer from lowering the taskbar for this screen-sized editor.
			if ok, _, err := user32.NewProc("SetPropW").Call(a.editor, uintptr(unsafe.Pointer(u16("NonRudeHWND"))), 1); ok == 0 {
				destroyWindow.Call(a.editor)
				a.editor = 0
				a.editing = false
				notifyError(apiError("keep editor below taskbar", err).Error())
				showWindow.Call(a.board, swShowNoActivate)
				return
			}
		}
		a.board = a.editor
		setWindowLong.Call(a.editor, gwlExStyle, wsExLayered|wsExToolWindow)
		dragAcceptFiles.Call(a.board, 1)
		setWindowPos.Call(a.board, 0, 0, 0, uintptr(a.width), uintptr(a.height), swpNoActivate|swpFrameChanged)
		if err := a.render(); err != nil {
			a.failRender(err)
			a.board = a.display
			return
		}
		a.activateEditor()
		// Keep the outgoing surface visible until its replacement is ready.
		showWindow.Call(a.display, swHide)
	}
}

func (a *application) finishEdit() {
	a.editClosing = false
	a.board = a.display
	if err := a.attach(); err != nil {
		showWindow.Call(a.board, swHide)
	}
	showWindow.Call(a.editor, swHide)
}

func (a *application) failRender(err error) {
	user32.NewProc("KillTimer").Call(a.host, 3)
	a.editFadeStart = time.Time{}
	a.editClosing = false
	showWindow.Call(a.board, swHide)
	a.editing = false
	notifyError(err.Error())
}
func (a *application) redraw() {
	if err := a.render(); err != nil {
		a.failRender(err)
	}
}

func (a *application) addFile(path string) {
	a.addFileAt(path, nil)
}

func (a *application) addFileAt(path string, position *vector) {
	item, err := a.importImage(path)
	if err != nil {
		a.dialog = true
		notifyError(fmt.Sprintf("Cannot add %s:\n%v", path, err))
		a.dialog = false
		return
	}
	a.addImage(item, position)
}

func (a *application) addImage(item *boardImage, position *vector) {
	scale := math.Min(1, math.Min(float64(a.width)*0.55/item.W, float64(a.height)*0.55/item.H))
	item.W *= scale
	item.H *= scale
	item.X = float64(a.width)/2 + float64(len(a.images)%5)*24
	item.Y = float64(a.height)/2 + float64(len(a.images)%5)*24
	if position != nil {
		item.X, item.Y = position.x, position.y
	}
	a.images = append(a.images, item)
	a.selected = len(a.images) - 1
	a.persistBoard()
	a.setEdit(true)
	a.redraw()
}

func (a *application) chooseFile() {
	a.dialog = true
	defer func() { a.dialog = false }()
	var filename [32768]uint16
	filter := utf16.Encode([]rune("Images (*.png;*.jpg;*.jpeg;*.gif;*.webp;*.avif;*.heic;*.heif;*.bmp;*.tif;*.tiff)\x00*.png;*.jpg;*.jpeg;*.gif;*.webp;*.avif;*.heic;*.heif;*.bmp;*.tif;*.tiff\x00All files\x00*.*\x00\x00"))
	ofn := openFileName{Owner: a.host, Filter: &filter[0], File: &filename[0], MaxFile: uint32(len(filename)), Title: u16("Add image to Lunaria"), Flags: 0x80000 | 0x1000 | 0x800 | 8}
	ofn.Size = uint32(unsafe.Sizeof(ofn))
	if r, _, _ := getOpenFileName.Call(uintptr(unsafe.Pointer(&ofn))); r != 0 {
		a.addFile(syscall.UTF16ToString(filename[:]))
		if a.editing {
			a.activateEditor()
		}
	}
}
func (a *application) trayMenu() {
	a.dialog = true
	defer func() { a.dialog = false }()
	menu, _, _ := createPopupMenu.Call()
	defer destroyMenu.Call(menu)
	label := "Edit board"
	if a.editing {
		label = "Finish editing"
	}
	appendMenu.Call(menu, 0, 1, uintptr(unsafe.Pointer(u16("Add Image…"))))
	appendMenu.Call(menu, 0, 2, uintptr(unsafe.Pointer(u16(label+"\tCtrl+Alt+L"))))
	appendMenu.Call(menu, 0x800, 0, 0)
	appendMenu.Call(menu, 0, 3, uintptr(unsafe.Pointer(u16("Quit"))))
	var p point
	getCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	setForegroundWindow.Call(a.host)
	command, _, _ := trackPopupMenu.Call(menu, 0x100|2, signed(p.X), signed(p.Y), 0, a.host, 0)
	postMessage.Call(a.host, 0, 0, 0)
	switch command {
	case 1:
		a.chooseFile()
	case 2:
		a.setEdit(!a.editing)
	case 3:
		postMessage.Call(a.host, wmClose, 0, 0)
	}
	if a.editing {
		a.activateEditor()
	}
}

func pointerFrom(lparam uintptr) vector {
	return vector{float64(int16(lparam & 0xffff)), float64(int16((lparam >> 16) & 0xffff))}
}
func (a *application) mouseDown(p vector) {
	if !a.editing {
		return
	}
	a.drag = ""
	if a.selected >= 0 {
		item := a.images[a.selected]
		if near(p, item.world(0, -item.H/2-handleGap)) {
			a.drag = "rotate"
		} else {
			for i, c := range item.corners() {
				if near(p, c) {
					a.drag = "resize"
					a.anchor = item.corners()[(i+2)%4]
					break
				}
			}
		}
	}
	if a.drag == "" {
		a.selected = -1
		for i := len(a.images) - 1; i >= 0; i-- {
			if a.images[i].contains(p) {
				a.selected = i
				a.drag = "move"
				break
			}
		}
	}
	if a.selected >= 0 {
		a.original = *a.images[a.selected]
		a.start = p
		setCapture.Call(a.board)
	}
	a.redraw()
}
func (a *application) mouseMove(p vector) {
	if !a.editing || a.drag == "" || a.selected < 0 {
		return
	}
	item := a.images[a.selected]
	original := a.original
	switch a.drag {
	case "move":
		item.X = original.X + p.x - a.start.x
		item.Y = original.Y + p.y - a.start.y
	case "rotate":
		start := math.Atan2(a.start.y-original.Y, a.start.x-original.X)
		now := math.Atan2(p.y-original.Y, p.x-original.X)
		item.Angle = snapAngle(original.Angle + (now-start)*180/math.Pi)
	case "resize":
		// Project the pointer onto the original diagonal, retaining the opposite corner.
		dx, dy := a.start.x-a.anchor.x, a.start.y-a.anchor.y
		scale := ((p.x-a.anchor.x)*dx + (p.y-a.anchor.y)*dy) / (dx*dx + dy*dy)
		scale = math.Max(math.Max(24/original.W, 24/original.H), scale)
		item.W = original.W * scale
		item.H = original.H * scale
		centerX, centerY := original.X-a.anchor.x, original.Y-a.anchor.y
		item.X = a.anchor.x + centerX*scale
		item.Y = a.anchor.y + centerY*scale
	}
	a.redraw()
}
func (a *application) stopDrag() {
	changed := a.drag != ""
	a.drag = ""
	releaseCapture.Call()
	if changed {
		a.persistBoard()
	}
}

func windowProc(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
	a := app
	if a == nil {
		r, _, _ := defWindowProc.Call(hwnd, uintptr(message), wparam, lparam)
		return r
	}
	if message == a.taskbarCreated && a.taskbarCreated != 0 {
		shellNotifyIcon.Call(0, uintptr(unsafe.Pointer(&a.tray)))
		postMessage.Call(a.host, wmRecover, 0, 0)
		return 0
	}
	switch message {
	case 0x000f: // WM_PAINT
		if hwnd == a.display && !a.editing && a.desktopAfter != 0 && !a.changing {
			user32.NewProc("ValidateRect").Call(hwnd, 0)
			a.redraw()
			return 0
		}
	case wmHotKey:
		if wparam == 1 {
			a.setEdit(!a.editing)
		}
		return 0
	case wmTray:
		if lparam == wmRButtonUp {
			a.trayMenu()
		} else if lparam == wmLButtonUp {
			a.setEdit(!a.editing)
		}
		return 0
	case 0x0046: // WM_WINDOWPOSCHANGING
		if hwnd == a.editor && (a.editing || a.editClosing) && lparam != 0 {
			a.keepEditorBelowApps(lparam)
		}
	case wmActivate:
		return 0
	case wmKeyDown:
		if hwnd == a.board && a.editing {
			ctrl, _, _ := getAsyncKeyState.Call(0x11)
			if wparam == 'V' && ctrl&0x8000 != 0 {
				a.paste()
			}
			if wparam == 0x1b || (wparam == 9 && ctrl&0x8000 != 0) {
				a.setEdit(false)
			}
			if wparam == 0x2e && a.selected >= 0 {
				a.stopDrag()
				removed := a.images[a.selected]
				call("GdipDisposeImage", removed.Bitmap)
				a.images = append(a.images[:a.selected], a.images[a.selected+1:]...)
				a.selected = -1
				if a.persistBoard() && a.storage != "" {
					os.Remove(filepath.Join(a.storage, "images", removed.File))
				}
				a.redraw()
			}
		}
		return 0
	case wmLButtonDown:
		if hwnd == a.board {
			a.mouseDown(pointerFrom(lparam))
		}
		return 0
	case wmMouseMove:
		if hwnd == a.board {
			a.mouseMove(pointerFrom(lparam))
		}
		return 0
	case wmLButtonUp:
		a.stopDrag()
		return 0
	case wmCaptureChanged:
		a.stopDrag()
		return 0
	case wmDropFiles:
		if a.editClosing {
			dragFinish.Call(wparam)
			return 0
		}
		var drop point
		shell32.NewProc("DragQueryPoint").Call(wparam, uintptr(unsafe.Pointer(&drop)))
		count, _, _ := dragQueryFile.Call(wparam, 0xffffffff, 0, 0)
		for i := uintptr(0); i < count; i++ {
			var path [32768]uint16
			dragQueryFile.Call(wparam, i, uintptr(unsafe.Pointer(&path[0])), uintptr(len(path)))
			before := len(a.images)
			a.addFile(syscall.UTF16ToString(path[:]))
			if len(a.images) > before {
				item := a.images[len(a.images)-1]
				item.X = math.Max(item.W/2, math.Min(float64(a.width)-item.W/2, float64(drop.X)+float64(i)*24))
				item.Y = math.Max(item.H/2, math.Min(float64(a.height)-item.H/2, float64(drop.Y)+float64(i)*24))
			}
		}
		dragFinish.Call(wparam)
		a.persistBoard()
		a.redraw()
		if !a.smoke {
			setForegroundWindow.Call(a.editor)
		}
		return 0
	case wmDisplayChange:
		a.setEdit(false)
		if a.editClosing {
			user32.NewProc("KillTimer").Call(a.host, 3)
			a.editFadeStart = time.Time{}
			a.finishEdit()
		}
		a.width = metric(0)
		a.height = metric(1)
		postMessage.Call(a.host, wmRecover, 0, 0)
		return 0
	case wmRecover:
		if !a.editing && !a.editClosing {
			if err := a.attach(); err != nil {
				showWindow.Call(a.board, swHide)
			}
		}
		return 0
	case wmTimer:
		if wparam == 3 {
			if a.editFadeStart.IsZero() || time.Since(a.editFadeStart) >= a.editFadeDuration {
				user32.NewProc("KillTimer").Call(a.host, 3)
				a.editFadeStart = time.Time{}
				if a.editClosing {
					a.finishEdit()
					return 0
				}
			}
			if a.editing || a.editClosing {
				a.redraw()
			}
			return 0
		}
		if wparam == 2 {
			a.smokeTick()
			return 0
		}
		a.watchEditor()
		if !a.editing && !a.editClosing && !a.changing {
			valid, _, _ := user32.NewProc("IsWindow").Call(a.board)
			if valid == 0 {
				postMessage.Call(a.host, wmRecover, 0, 0)
			}
		}
		return 0
	case wmClose:
		if hwnd == a.board {
			a.setEdit(false)
			return 0
		}
		if hwnd == a.host {
			a.stopDrag()
			user32.NewProc("PostQuitMessage").Call(0)
			return 0
		}
	case wmDestroy:
		return 0
	}
	r, _, _ := defWindowProc.Call(hwnd, uintptr(message), wparam, lparam)
	return r
}

func (a *application) smokeTick() {
	a.smokeStage++
	check := func(ok bool, label string) {
		if !ok {
			a.smokeErrors = append(a.smokeErrors, label)
		}
	}
	switch a.smokeStage {
	case 1:
		parent, _, _ := getParent.Call(a.board)
		ex, _, _ := getWindowLong.Call(a.board, gwlExStyle)
		style, _, _ := getWindowLong.Call(a.board, gwlStyle)
		check(parent == a.desktop && parent != 0, "normal board is parented to the desktop layer")
		check(style&wsChild != 0, "passive window has the child style")
		if a.desktopAfter != 0 {
			previous, _, _ := getWindow.Call(a.board, 3)
			check(previous == a.desktopAfter, "normal board is ordered directly behind desktop icons")
			var key, flags uint32
			var alpha byte
			ok, _, _ := user32.NewProc("GetLayeredWindowAttributes").Call(a.board, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&alpha)), uintptr(unsafe.Pointer(&flags)))
			check(ok != 0 && flags == 3 && alpha == 255, "raised desktop uses the GDI-compatible layered mode")
		}
		check(ex&wsExTransparent != 0, "normal board is click-through")
		a.setEdit(true)
	case 2:
		ex, _, _ := getWindowLong.Call(a.board, gwlExStyle)
		parent, _, _ := getParent.Call(a.board)
		check(a.editing && parent == 0, "editor is a top-level window")
		check(ex&wsExTransparent == 0, "editor accepts input")
		check(ex&0x10 != 0, "editor retains Shell file-drop registration")
		visible, _, _ := user32.NewProc("IsWindowVisible").Call(a.editor)
		passiveVisible, _, _ := user32.NewProc("IsWindowVisible").Call(a.display)
		check(visible != 0 && passiveVisible == 0, "entering edit mode shows editor and hides passive surface")
		nonRude, _, _ := user32.NewProc("GetPropW").Call(a.editor, uintptr(unsafe.Pointer(u16("NonRudeHWND"))))
		check(nonRude != 0, "editor opts out of Shell fullscreen taskbar lowering")
		if len(a.images) > 0 {
			a.selected = 0
			item := a.images[0]
			start := vector{item.X, item.Y}
			before := item.X
			a.mouseDown(start)
			a.mouseMove(vector{start.x + 20, start.y + 10})
			a.stopDrag()
			check(math.Abs(item.X-before-20) < 0.01, "drag moves image")
			corner := item.corners()[2]
			anchor := item.corners()[0]
			oldW, oldH := item.W, item.H
			a.mouseDown(corner)
			a.mouseMove(vector{anchor.x + (corner.x-anchor.x)*1.2, anchor.y + (corner.y-anchor.y)*1.2})
			a.stopDrag()
			check(math.Abs(item.W-oldW*1.2) < 0.01 && math.Abs(item.H-oldH*1.2) < 0.01, "resize preserves aspect ratio")
			rotationHandle := item.world(0, -item.H/2-handleGap)
			a.mouseDown(rotationHandle)
			a.mouseMove(vector{item.X + item.H/2 + handleGap, item.Y})
			a.stopDrag()
			check(item.Angle == 90, "rotation snaps to 90 degrees")
			a.redraw()
			beforeCount := len(a.images)
			windowProc(a.board, wmKeyDown, 0x2e, 0)
			check(len(a.images) == beforeCount-1 && a.selected == -1, "Delete removes selected image")
		}
		windowProc(a.board, wmKeyDown, 0x1b, 0)
		check(!a.editing, "Escape exits editing")
	case 3:
		parent, _, _ := getParent.Call(a.board)
		check(!a.editing && parent == a.desktop && parent != 0, "exit restores passive desktop surface")
		visible, _, _ := user32.NewProc("IsWindowVisible").Call(a.display)
		editorVisible, _, _ := user32.NewProc("IsWindowVisible").Call(a.editor)
		check(visible != 0 && editorVisible == 0, "exiting edit mode shows passive surface and hides editor")
		// Exercise the real Shell drag/drop payload API without moving the user's pointer.
		path, err := filepath.Abs("testdata/sample.png")
		check(err == nil, "resolve drop fixture")
		files := append(syscall.StringToUTF16(path), 0)
		handle, _, _ := kernel32.NewProc("GlobalAlloc").Call(0x42, uintptr(20+len(files)*2))
		if handle == 0 {
			check(false, "allocate Shell drop payload")
		} else {
			memory, _, _ := kernel32.NewProc("GlobalLock").Call(handle)
			if memory == 0 {
				check(false, "lock Shell drop payload")
				kernel32.NewProc("GlobalFree").Call(handle)
			} else {
				bytes := make([]byte, 20+len(files)*2)
				bytes[0] = 20
				bytes[16] = 1
				for i, v := range files {
					bytes[20+i*2] = byte(v)
					bytes[21+i*2] = byte(v >> 8)
				}
				kernel32.NewProc("RtlMoveMemory").Call(memory, uintptr(unsafe.Pointer(&bytes[0])), uintptr(len(bytes)))
				kernel32.NewProc("GlobalUnlock").Call(handle)
				windowProc(a.board, wmDropFiles, handle, 0)
				check(len(a.images) == 1 && a.editing, "Shell drop adds an image and enters editing")
			}
		}
		a.setEdit(false)
		check(!a.editing, "editing can end after a drop")
	case 4:
		old := a.board
		destroyWindow.Call(old)
		err := a.attach()
		valid, _, _ := user32.NewProc("IsWindow").Call(a.board)
		check(err == nil && valid != 0, "recreate board after desktop child destruction")
		postMessage.Call(a.host, wmClose, 0, 0)
	}
}
func (a *application) writeSmokeResult() error {
	result := map[string]any{"passed": len(a.smokeErrors) == 0, "failures": a.smokeErrors, "width": a.width, "height": a.height, "images": len(a.images), "time": time.Now().Format(time.RFC3339)}
	data, _ := json.MarshalIndent(result, "", "  ")
	if err := os.WriteFile("smoke-result.json", data, 0600); err != nil {
		return err
	}
	if len(a.smokeErrors) > 0 {
		return fmt.Errorf("native smoke test failed: %v", a.smokeErrors)
	}
	return nil
}
