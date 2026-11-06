//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

func TestPasteClipboard(t *testing.T) {
	// Use a private window station so tests never read or replace the user's clipboard.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	previous, _, _ := user32.NewProc("GetProcessWindowStation").Call()
	station, _, err := user32.NewProc("CreateWindowStationW").Call(0, 0, 0xf037f, 0)
	if station == 0 {
		t.Fatal(apiError("create test window station", err))
	}
	defer user32.NewProc("CloseWindowStation").Call(station)
	if ok, _, err := user32.NewProc("SetProcessWindowStation").Call(station); ok == 0 {
		t.Fatal(apiError("use test window station", err))
	}
	defer user32.NewProc("SetProcessWindowStation").Call(previous)
	thread, _, _ := kernel32.NewProc("GetCurrentThreadId").Call()
	previousDesktop, _, _ := user32.NewProc("GetThreadDesktop").Call(thread)
	desktop, _, err := user32.NewProc("CreateDesktopW").Call(uintptr(unsafe.Pointer(u16("LunariaPasteTest"))), 0, 0, 0, 0x1ff, 0)
	if desktop == 0 {
		t.Fatal(apiError("create test desktop", err))
	}
	defer user32.NewProc("CloseDesktop").Call(desktop)
	if ok, _, err := user32.NewProc("SetThreadDesktop").Call(desktop); ok == 0 {
		t.Fatal(apiError("use test desktop", err))
	}
	defer user32.NewProc("SetThreadDesktop").Call(previousDesktop)
	owner, _, err := createWindowEx.Call(0, uintptr(unsafe.Pointer(u16("STATIC"))), 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	if owner == 0 {
		t.Fatal(apiError("create clipboard owner", err))
	}
	defer destroyWindow.Call(owner)
	imageTestGraphics(t)
	set := func(format uintptr, data []byte) {
		t.Helper()
		if ok, _, err := user32.NewProc("OpenClipboard").Call(owner); ok == 0 {
			t.Fatal(err)
		}
		defer user32.NewProc("CloseClipboard").Call()
		if ok, _, err := user32.NewProc("EmptyClipboard").Call(); ok == 0 {
			t.Fatal(err)
		}
		handle, _, err := kernel32.NewProc("GlobalAlloc").Call(2, uintptr(len(data)))
		if handle == 0 {
			t.Fatal(err)
		}
		memory, _, err := kernel32.NewProc("GlobalLock").Call(handle)
		if memory == 0 {
			kernel32.NewProc("GlobalFree").Call(handle)
			t.Fatal(err)
		}
		kernel32.NewProc("RtlMoveMemory").Call(memory, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
		kernel32.NewProc("GlobalUnlock").Call(handle)
		if ok, _, err := user32.NewProc("SetClipboardData").Call(format, handle); ok == 0 {
			kernel32.NewProc("GlobalFree").Call(handle)
			t.Fatal(err)
		}
	}
	pngFormat, _, _ := user32.NewProc("RegisterClipboardFormatW").Call(uintptr(unsafe.Pointer(u16("PNG"))))
	im := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	im.SetNRGBA(0, 0, color.NRGBA{R: 240, G: 30, B: 10, A: 128})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, im); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "画像.png")
	if err := os.WriteFile(path, encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	files := append(append(syscall.StringToUTF16(path), syscall.StringToUTF16(path)...), 0)
	drop := make([]byte, 20+len(files)*2)
	binary.LittleEndian.PutUint32(drop, 20)
	binary.LittleEndian.PutUint32(drop[16:], 1)
	for i, v := range files {
		binary.LittleEndian.PutUint16(drop[20+i*2:], v)
	}
	// A bottom-up 24-bit DIB exercises Windows' CF_BITMAP synthesis.
	dib := make([]byte, 40+12*2)
	binary.LittleEndian.PutUint32(dib, 40)
	binary.LittleEndian.PutUint32(dib[4:], 4)
	binary.LittleEndian.PutUint32(dib[8:], 2)
	binary.LittleEndian.PutUint16(dib[12:], 1)
	binary.LittleEndian.PutUint16(dib[14:], 24)
	copy(dib[52:], []byte{10, 30, 240})
	for _, tc := range []struct {
		name   string
		format uintptr
		data   []byte
		count  int
		pixel  uint32
	}{
		{"PNG with alpha", pngFormat, encoded.Bytes(), 1, 0x80f01e0a},
		{"Unicode files", cfHDrop, drop, 2, 0x80f01e0a},
		{"DIB screenshot", 8, dib, 1, 0xfff01e0a},
		{"unsupported text", 13, []byte{'x', 0, 0, 0}, 0, 0},
		{"invalid PNG", pngFormat, []byte("invalid"), 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set(tc.format, tc.data)
			a := &application{editor: owner, editing: true, selected: -1}
			err := a.pasteClipboard()
			defer func() {
				for _, im := range a.images {
					call("GdipDisposeImage", im.Bitmap)
				}
			}()
			if (err != nil) != (tc.name == "invalid PNG") {
				t.Fatalf("paste error: %v", err)
			}
			if len(a.images) != tc.count {
				t.Fatalf("got %d images, want %d", len(a.images), tc.count)
			}
			if tc.count == 0 {
				return
			}
			if a.selected != tc.count-1 || !a.editing {
				t.Fatal("pasted image was not selected in edit mode")
			}
			// Replacing the clipboard must not invalidate the imported bitmap.
			set(13, []byte{'x', 0, 0, 0})
			var width, height uint32
			call("GdipGetImageWidth", a.images[0].Bitmap, uintptr(unsafe.Pointer(&width)))
			call("GdipGetImageHeight", a.images[0].Bitmap, uintptr(unsafe.Pointer(&height)))
			if width != 4 || height != 2 {
				t.Fatalf("native image dimensions = %dx%d, want 4x2", width, height)
			}
			var pixel uint32
			status := call("GdipBitmapGetPixel", a.images[0].Bitmap, 0, 0, uintptr(unsafe.Pointer(&pixel)))
			if status != 0 {
				t.Fatalf("read pasted pixel: %d", status)
			}
			for shift := 0; shift < 32; shift += 8 {
				if delta := int(byte(pixel>>shift)) - int(byte(tc.pixel>>shift)); delta < -2 || delta > 2 {
					t.Fatalf("pixel = %#08x, want approximately %#08x", pixel, tc.pixel)
				}
			}
		})
	}
}
