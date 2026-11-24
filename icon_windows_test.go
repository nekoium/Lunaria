//go:build windows

package main

import (
	"testing"
	"unsafe"
)

func TestIconResources(t *testing.T) {
	instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	for _, size := range []uintptr{16, 20, 24, 32, 40, 48, 64, 128, 256} {
		icon, _, err := user32.NewProc("LoadImageW").Call(instance, 2, 1, size, size, 0)
		if icon == 0 {
			t.Fatalf("load %d px icon: %v", size, err)
		}
		checkIconSize(t, icon, int32(size), int32(size))
		user32.NewProc("DestroyIcon").Call(icon)
	}
	icon := makeTrayIcon()
	if icon == 0 {
		t.Fatal("tray icon could not load the embedded artwork")
	}
	defer user32.NewProc("DestroyIcon").Call(icon)
	checkIconSize(t, icon, int32(metric(49)), int32(metric(50)))
}

func checkIconSize(t *testing.T, icon uintptr, width, height int32) {
	t.Helper()
	var info struct {
		IsIcon, HotspotX, HotspotY uint32
		Mask, Color                uintptr
	}
	if ok, _, err := user32.NewProc("GetIconInfo").Call(icon, uintptr(unsafe.Pointer(&info))); ok == 0 {
		t.Fatalf("GetIconInfo: %v", err)
	}
	defer deleteObject.Call(info.Mask)
	defer deleteObject.Call(info.Color)
	var bitmap struct {
		Type, Width, Height, WidthBytes int32
		Planes, BitsPixel               uint16
		Bits                            uintptr
	}
	if ok, _, err := gdi32.NewProc("GetObjectW").Call(info.Color, unsafe.Sizeof(bitmap), uintptr(unsafe.Pointer(&bitmap))); ok == 0 {
		t.Fatalf("GetObjectW: %v", err)
	}
	if bitmap.Width != width || bitmap.Height != height || bitmap.BitsPixel != 32 {
		t.Fatalf("icon bitmap: got %dx%d at %d bits; want %dx%d at 32 bits", bitmap.Width, bitmap.Height, bitmap.BitsPixel, width, height)
	}
}
