//go:build windows

package main

import (
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

func downloadOriginal(storage, file, folder string) (string, error) {
	if !validStoredImage(storedImage{File: file, W: 1, H: 1}) {
		return "", fmt.Errorf("invalid stored image name")
	}
	source, err := os.Open(filepath.Join(storage, "images", file))
	if err != nil {
		return "", err
	}
	defer source.Close()
	_, format, err := image.DecodeConfig(source)
	if err != nil {
		return "", err
	}
	ext := map[string]string{"png": ".png", "jpeg": ".jpg", "gif": ".gif", "webp": ".webp", "avif": ".avif", "heic": ".heic", "bmp": ".bmp", "tiff": ".tif"}[format]
	if ext == "" {
		return "", fmt.Errorf("unsupported image format %q", format)
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	for n := 1; n <= 1000; n++ {
		name := "Lunaria-" + file
		if n > 1 {
			name += fmt.Sprintf(" (%d)", n)
		}
		path := filepath.Join(folder, name+ext)
		out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, err = io.Copy(out, source)
		if err == nil {
			err = out.Sync()
		}
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			if removeErr := os.Remove(path); removeErr != nil {
				return "", fmt.Errorf("download: %w; remove incomplete file %s: %v", err, path, removeErr)
			}
			return "", err
		}
		return path, nil
	}
	return "", fmt.Errorf("no unused download name after 1000 attempts")
}

func downloadsFolder() (string, error) {
	// FOLDERID_Downloads respects redirected and localized Downloads folders.
	id := syscall.GUID{Data1: 0x374de290, Data2: 0x123f, Data3: 0x4565, Data4: [8]byte{0x91, 0x64, 0x39, 0xc4, 0x92, 0x5e, 0x46, 0x7b}}
	var path *uint16
	r, _, _ := shell32.NewProc("SHGetKnownFolderPath").Call(uintptr(unsafe.Pointer(&id)), 0, 0, uintptr(unsafe.Pointer(&path)))
	if path != nil {
		defer syscall.NewLazyDLL("ole32.dll").NewProc("CoTaskMemFree").Call(uintptr(unsafe.Pointer(path)))
	}
	if r != 0 || path == nil {
		return "", fmt.Errorf("find Downloads folder: %#x", r)
	}
	var length int
	for *(*uint16)(unsafe.Add(unsafe.Pointer(path), length*2)) != 0 {
		length++
	}
	return syscall.UTF16ToString(unsafe.Slice(path, length)), nil
}

func (a *application) imageMenu(p vector) {
	if !a.editing || a.storage == "" {
		return
	}
	index := -1
	for i := len(a.images) - 1; i >= 0; i-- {
		if a.images[i].contains(p) {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	a.stopDrag()
	a.selected = index
	item := a.images[index]
	a.redraw()
	a.dialog = true
	defer func() { a.dialog = false }()
	menu, _, err := createPopupMenu.Call()
	if menu == 0 {
		notifyError(apiError("create image menu", err).Error())
		return
	}
	defer destroyMenu.Call(menu)
	appendMenu.Call(menu, 0, 1, uintptr(unsafe.Pointer(u16("Download"))))
	var cursor point
	getCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
	command, _, _ := trackPopupMenu.Call(menu, 0x100|2, signed(cursor.X), signed(cursor.Y), 0, a.board, 0)
	postMessage.Call(a.board, 0, 0, 0)
	if command != 1 {
		return
	}
	folder, err := downloadsFolder()
	if err == nil {
		_, err = downloadOriginal(a.storage, item.File, folder)
	}
	if err != nil {
		notifyError(fmt.Sprintf("Cannot download the image:\n%v", err))
	}
}
