//go:build windows

package main

//go:generate go run github.com/akavel/rsrc@v0.10.2 -manifest lunaria.manifest -o lunaria_windows_amd64.syso -arch amd64

import (
	"fmt"
	"math"
	"syscall"
	"unsafe"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	gdi32               = syscall.NewLazyDLL("gdi32.dll")
	shell32             = syscall.NewLazyDLL("shell32.dll")
	comdlg32            = syscall.NewLazyDLL("comdlg32.dll")
	gdiplus             = syscall.NewLazyDLL("gdiplus.dll")
	createWindowEx      = user32.NewProc("CreateWindowExW")
	defWindowProc       = user32.NewProc("DefWindowProcW")
	destroyWindow       = user32.NewProc("DestroyWindow")
	showWindow          = user32.NewProc("ShowWindow")
	setWindowPos        = user32.NewProc("SetWindowPos")
	setParent           = user32.NewProc("SetParent")
	getParent           = user32.NewProc("GetParent")
	getWindow           = user32.NewProc("GetWindow")
	getWindowLong       = user32.NewProc("GetWindowLongPtrW")
	setWindowLong       = user32.NewProc("SetWindowLongPtrW")
	findWindow          = user32.NewProc("FindWindowW")
	findWindowEx        = user32.NewProc("FindWindowExW")
	enumWindows         = user32.NewProc("EnumWindows")
	sendMessageTimeout  = user32.NewProc("SendMessageTimeoutW")
	setForegroundWindow = user32.NewProc("SetForegroundWindow")
	getForegroundWindow = user32.NewProc("GetForegroundWindow")
	registerHotKey      = user32.NewProc("RegisterHotKey")
	unregisterHotKey    = user32.NewProc("UnregisterHotKey")
	getAsyncKeyState    = user32.NewProc("GetAsyncKeyState")
	setCapture          = user32.NewProc("SetCapture")
	releaseCapture      = user32.NewProc("ReleaseCapture")
	setCursor           = user32.NewProc("SetCursor")
	loadCursor          = user32.NewProc("LoadCursorW")
	loadIcon            = user32.NewProc("LoadIconW")
	getCursorPos        = user32.NewProc("GetCursorPos")
	createPopupMenu     = user32.NewProc("CreatePopupMenu")
	appendMenu          = user32.NewProc("AppendMenuW")
	trackPopupMenu      = user32.NewProc("TrackPopupMenu")
	destroyMenu         = user32.NewProc("DestroyMenu")
	postMessage         = user32.NewProc("PostMessageW")
	messageBox          = user32.NewProc("MessageBoxW")
	getSystemMetrics    = user32.NewProc("GetSystemMetrics")
	getDC               = user32.NewProc("GetDC")
	releaseDC           = user32.NewProc("ReleaseDC")
	updateLayeredWindow = user32.NewProc("UpdateLayeredWindow")
	createCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	createDIBSection    = gdi32.NewProc("CreateDIBSection")
	selectObject        = gdi32.NewProc("SelectObject")
	deleteObject        = gdi32.NewProc("DeleteObject")
	deleteDC            = gdi32.NewProc("DeleteDC")
	shellNotifyIcon     = shell32.NewProc("Shell_NotifyIconW")
	dragAcceptFiles     = shell32.NewProc("DragAcceptFiles")
	dragQueryFile       = shell32.NewProc("DragQueryFileW")
	dragFinish          = shell32.NewProc("DragFinish")
	getOpenFileName     = comdlg32.NewProc("GetOpenFileNameW")
)

const (
	wmDestroy        = 0x0002
	wmActivate       = 0x0006
	wmClose          = 0x0010
	wmDisplayChange  = 0x007e
	wmKeyDown        = 0x0100
	wmMouseMove      = 0x0200
	wmLButtonDown    = 0x0201
	wmLButtonUp      = 0x0202
	wmRButtonUp      = 0x0205
	wmCaptureChanged = 0x0215
	wmDropFiles      = 0x0233
	wmHotKey         = 0x0312
	wmTimer          = 0x0113
	wmTray           = 0x8001
	wmRecover        = 0x8002
	wsPopup          = 0x80000000
	wsChild          = 0x40000000
	wsExLayered      = 0x80000
	wsExTransparent  = 0x20
	wsExToolWindow   = 0x80
	wsExNoActivate   = 0x08000000
	swHide           = 0
	swShow           = 5
	swShowNoActivate = 4
	swpNoSize        = 1
	swpNoMove        = 2
	swpNoActivate    = 0x10
	swpFrameChanged  = 0x20
	swpShowWindow    = 0x40
)

// Negative window-long indexes, represented as unsigned API arguments.
var gwlStyle = ^uintptr(15)
var gwlExStyle = ^uintptr(19)

type point struct{ X, Y int32 }
type size struct{ W, H int32 }
type rect struct{ Left, Top, Right, Bottom int32 }
type msg struct {
	HWND           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             point
	Private        uint32
}
type wndClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	IconSmall                          uintptr
}
type notifyIconData struct {
	Size                       uint32
	HWND                       uintptr
	ID, Flags, CallbackMessage uint32
	Icon                       uintptr
	Tip                        [128]uint16
	State, StateMask           uint32
	Info                       [256]uint16
	Version                    uint32
	InfoTitle                  [64]uint16
	InfoFlags                  uint32
	GUID                       [16]byte
	BalloonIcon                uintptr
}
type openFileName struct {
	Size                         uint32
	Owner, Instance              uintptr
	Filter, CustomFilter         *uint16
	MaxCustomFilter, FilterIndex uint32
	File                         *uint16
	MaxFile                      uint32
	FileTitle                    *uint16
	MaxFileTitle                 uint32
	InitialDir, Title            *uint16
	Flags                        uint32
	FileOffset, FileExtension    uint16
	DefExt                       *uint16
	CustData, Hook               uintptr
	TemplateName                 *uint16
	Reserved                     uintptr
	Reserved2, FlagsEx           uint32
}

func u16(s string) *uint16 { return syscall.StringToUTF16Ptr(s) }
func call(name string, args ...uintptr) uintptr {
	r, _, _ := gdiplus.NewProc(name).Call(args...)
	return r
}
func f32(v float64) uintptr  { return uintptr(math.Float32bits(float32(v))) }
func signed(v int32) uintptr { return uintptr(int64(v)) }
func notifyError(text string) {
	messageBox.Call(0, uintptr(unsafe.Pointer(u16(text))), uintptr(unsafe.Pointer(u16("Lunaria"))), 0x10)
}
func apiError(name string, err error) error { return fmt.Errorf("%s failed: %v", name, err) }

var desktopCandidate uintptr
var enumDesktopCallback = syscall.NewCallback(func(hwnd, lparam uintptr) uintptr {
	view, _, _ := findWindowEx.Call(hwnd, 0, uintptr(unsafe.Pointer(u16("SHELLDLL_DefView"))), 0)
	if view != 0 {
		desktopCandidate, _, _ = findWindowEx.Call(0, hwnd, uintptr(unsafe.Pointer(u16("WorkerW"))), 0)
	}
	return 1
})

func desktopHost() (uintptr, uintptr) {
	prog, _, _ := findWindow.Call(uintptr(unsafe.Pointer(u16("Progman"))), 0)
	if prog == 0 {
		return 0, 0
	}
	for _, request := range [][2]uintptr{{0, 0}, {0xD, 0}, {0xD, 1}} {
		var result uintptr
		sendMessageTimeout.Call(prog, 0x052c, request[0], request[1], 2, 1000, uintptr(unsafe.Pointer(&result)))
		desktopCandidate = 0
		enumWindows.Call(enumDesktopCallback, 0)
		if desktopCandidate != 0 {
			return desktopCandidate, 0
		}
	}
	ex, _, _ := getWindowLong.Call(prog, gwlExStyle)
	view, _, _ := findWindowEx.Call(prog, 0, uintptr(unsafe.Pointer(u16("SHELLDLL_DefView"))), 0)
	if view != 0 && ex&0x00200000 != 0 {
		wallpaper, _, _ := findWindowEx.Call(prog, 0, uintptr(unsafe.Pointer(u16("WorkerW"))), 0)
		if wallpaper != 0 {
			return prog, view
		}
	}
	return 0, 0
}
