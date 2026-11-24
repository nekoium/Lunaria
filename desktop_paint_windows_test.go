//go:build windows

package main

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

func TestDesktopReattachHidesStaleFrame(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := startGraphics(); err != nil {
		t.Fatal(err)
	}
	defer call("GdiplusShutdown", gdiToken)
	defer call("GdipDeleteFont", font)
	instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	class := u16("Lunaria.DesktopPaintTest")
	a := &application{width: 64, height: 64, selected: -1, smoke: true}
	previous := app
	app = a
	defer func() { app = previous }()
	shows := 0
	proc := syscall.NewCallback(func(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
		if message == 0x0047 && hwnd == a.display && a.desktopAfter != 0 { // WM_WINDOWPOSCHANGED
			visible, _, _ := user32.NewProc("IsWindowVisible").Call(hwnd)
			if visible != 0 {
				shows++
				var alpha byte
				ok, _, _ := user32.NewProc("GetLayeredWindowAttributes").Call(hwnd, 0, uintptr(unsafe.Pointer(&alpha)), 0)
				if ok == 0 || alpha != 0 {
					t.Errorf("desktop surface exposed its previous frame before repainting: alpha = %d", alpha)
				}
			}
		}
		return windowProc(hwnd, message, wparam, lparam)
	})
	wc := wndClass{Proc: proc, Instance: instance, ClassName: class}
	wc.Size = uint32(unsafe.Sizeof(wc))
	if ok, _, err := user32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc))); ok == 0 {
		t.Fatal(err)
	}
	defer user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(class)), instance)
	a.display, _, _ = createWindowEx.Call(wsExLayered|wsExToolWindow, uintptr(unsafe.Pointer(class)), 0, wsPopup, 0, 0, 64, 64, 0, 0, instance, 0)
	if a.display == 0 {
		t.Fatal("create desktop test window")
	}
	a.board = a.display
	defer destroyWindow.Call(a.display)
	im, err := loadImage("testdata/sample.png")
	if err != nil {
		t.Fatal(err)
	}
	defer call("GdipDisposeImage", im.Bitmap)
	im.X, im.Y, im.W, im.H = 10, 10, 16, 16
	a.images = []*boardImage{im}
	if err := a.attach(); err != nil {
		t.Fatal(err)
	}
	if a.desktopAfter == 0 {
		t.Skip("this Explorer desktop uses per-pixel layering")
	}
	pixel := func(x, y uintptr) uintptr {
		dc, _, _ := getDC.Call(a.display)
		defer releaseDC.Call(a.display, dc)
		value, _, _ := gdi32.NewProc("GetPixel").Call(dc, x, y)
		return value
	}
	originalPixel := pixel(10, 10)
	if originalPixel == 0x030201 || originalPixel == 0xffffffff {
		t.Fatal("fixture did not paint an image pixel")
	}
	// Editing hides this surface and moves the image on the separate overlay.
	for _, position := range []float64{45, 10, 45} {
		showWindow.Call(a.display, swHide)
		im.X, im.Y = position, position
		if err := a.attach(); err != nil {
			t.Fatal(err)
		}
		var alpha byte
		user32.NewProc("GetLayeredWindowAttributes").Call(a.display, 0, uintptr(unsafe.Pointer(&alpha)), 0)
		if alpha != 255 {
			t.Fatalf("repainted desktop remains transparent: alpha = %d", alpha)
		}
		x := uintptr(position)
		other := uintptr(55) - x
		if got := pixel(x, x); got != originalPixel {
			t.Fatalf("new image position = %#x; want %#x", got, originalPixel)
		}
		if got := pixel(other, other); got != 0x030201 {
			t.Fatalf("old image position = %#x; want transparent color key", got)
		}
	}
	if shows < 4 {
		t.Fatalf("observed only %d desktop shows", shows)
	}
}
