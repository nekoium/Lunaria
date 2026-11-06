//go:build windows

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"syscall"
	"unsafe"
)

const (
	cfBitmap = 2
	cfHDrop  = 15
)

func (a *application) paste() {
	position := a.pastePosition()
	if err := a.pasteClipboard(position); err != nil {
		a.dialog = true
		notifyError(fmt.Sprintf("Cannot paste image:\n%v", err))
		a.dialog = false
	}
}

func (a *application) pastePosition() *vector {
	var p point
	if ok, _, _ := getCursorPos.Call(uintptr(unsafe.Pointer(&p))); ok == 0 {
		return nil
	}
	return a.boardPosition(p)
}

func (a *application) boardPosition(p point) *vector {
	if ok, _, _ := user32.NewProc("ScreenToClient").Call(a.board, uintptr(unsafe.Pointer(&p))); ok == 0 {
		return nil
	}
	return &vector{float64(p.X), float64(p.Y)}
}

func (a *application) pasteClipboard(position *vector) error {
	ok, _, err := user32.NewProc("OpenClipboard").Call(a.editor)
	if ok == 0 {
		return apiError("OpenClipboard", err)
	}
	defer user32.NewProc("CloseClipboard").Call()
	get := user32.NewProc("GetClipboardData")
	if drop, _, _ := get.Call(cfHDrop); drop != 0 {
		count, _, _ := dragQueryFile.Call(drop, 0xffffffff, 0, 0)
		before := len(a.images)
		for i := uintptr(0); i < count; i++ {
			var path [32768]uint16
			dragQueryFile.Call(drop, i, uintptr(unsafe.Pointer(&path[0])), uintptr(len(path)))
			var target *vector
			if position != nil {
				offset := float64(len(a.images)-before) * 24
				target = &vector{position.x + offset, position.y + offset}
			}
			a.addFileAt(syscall.UTF16ToString(path[:]), target)
		}
		// Clipboard handles belong to Windows; do not call DragFinish here.
		return nil
	}
	pngFormat, _, _ := user32.NewProc("RegisterClipboardFormatW").Call(uintptr(unsafe.Pointer(u16("PNG"))))
	if pngFormat != 0 {
		if handle, _, _ := get.Call(pngFormat); handle != 0 {
			size, _, _ := kernel32.NewProc("GlobalSize").Call(handle)
			memory, _, err := kernel32.NewProc("GlobalLock").Call(handle)
			if memory == 0 {
				return apiError("lock clipboard PNG", err)
			}
			data := make([]byte, int(size))
			if len(data) > 0 {
				kernel32.NewProc("RtlMoveMemory").Call(uintptr(unsafe.Pointer(&data[0])), memory, size)
			}
			kernel32.NewProc("GlobalUnlock").Call(handle)
			item, err := a.importReader(bytes.NewReader(data))
			if err != nil {
				return err
			}
			a.addImage(item, position)
			return nil
		}
	}
	// Windows synthesizes CF_BITMAP for DIB clipboard images, including screenshots.
	bitmap, _, _ := get.Call(cfBitmap)
	if bitmap == 0 {
		return nil
	}
	var info struct {
		Type, Width, Height, Stride int32
		Planes, BitsPixel           uint16
		Bits                        uintptr
	}
	if ok, _, err := gdi32.NewProc("GetObjectW").Call(bitmap, unsafe.Sizeof(info), uintptr(unsafe.Pointer(&info))); ok == 0 {
		return apiError("read clipboard bitmap", err)
	}
	if info.Width <= 0 || info.Height <= 0 || int64(info.Width)*int64(info.Height) > 40_000_000 {
		return fmt.Errorf("image exceeds the prototype's 40 megapixel limit")
	}
	// Request top-down 32-bit BGRX: rows are width*4 bytes with no padding.
	pixels := make([]byte, int(info.Width)*int(info.Height)*4)
	header := bitmapInfo{Size: 40, Width: info.Width, Height: -info.Height, Planes: 1, BitCount: 32}
	dc, _, _ := getDC.Call(0)
	defer releaseDC.Call(0, dc)
	rows, _, err := gdi32.NewProc("GetDIBits").Call(dc, bitmap, 0, uintptr(info.Height), uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&header)), 0)
	if rows != uintptr(info.Height) {
		return apiError("read clipboard pixels", err)
	}
	for i := 0; i < len(pixels); i += 4 {
		pixels[i], pixels[i+2] = pixels[i+2], pixels[i]
		// CF_BITMAP has no alpha channel; its reserved byte is not transparency.
		pixels[i+3] = 255
	}
	im := &image.NRGBA{Pix: pixels, Stride: int(info.Width) * 4, Rect: image.Rect(0, 0, int(info.Width), int(info.Height))}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, im); err != nil {
		return err
	}
	item, err := a.importReader(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		return err
	}
	a.addImage(item, position)
	return nil
}
