//go:build windows

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"
	"unsafe"
)

type imageImport struct {
	path     string
	data     []byte
	bitmap   image.Image
	position *vector
	clamp    bool
}

type importResult struct {
	item *boardImage
	err  error
}

type imageImports struct {
	queue   []imageImport
	active  *imageImport
	result  chan importResult
	started time.Time
	warned  bool
}

func (a *application) queueImport(request imageImport) {
	if request.position != nil {
		position := *request.position
		request.position = &position
	}
	a.imports.queue = append(a.imports.queue, request)
	a.startImport()
}

func (a *application) startImport() {
	imports := a.imports
	if imports.active != nil || len(imports.queue) == 0 {
		return
	}
	request := imports.queue[0]
	imports.queue[0] = imageImport{}
	imports.queue = imports.queue[1:]
	imports.active = &imageImport{path: request.path, position: request.position, clamp: request.clamp}
	imports.started = time.Now()
	imports.result = make(chan importResult, 1)
	result, storage := imports.result, a.storage
	go func() {
		// Only this worker touches its bitmap; the UI takes ownership through result.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		loader := &application{storage: storage}
		var item *boardImage
		var err error
		if request.bitmap != nil {
			var encoded bytes.Buffer
			err = png.Encode(&encoded, request.bitmap)
			if err == nil {
				item, err = loader.importReader(bytes.NewReader(encoded.Bytes()))
			}
		} else if request.data != nil {
			item, err = loader.importReader(bytes.NewReader(request.data))
		} else {
			item, err = loader.importImage(request.path)
		}
		// Persistent imports release decoder memory in loadImage.
		if storage == "" && request.path == "" {
			debug.FreeOSMemory()
		}
		result <- importResult{item, err}
	}()
}

func (a *application) importTick() {
	imports := a.imports
	if imports == nil || imports.active == nil || a.dialog || a.drag != "" {
		return
	}
	select {
	case result := <-imports.result:
		request := imports.active
		imports.active = nil
		if result.err != nil {
			a.dialog = true
			notifyError(fmt.Sprintf("Cannot add image:\n%v", result.err))
			a.dialog = false
		} else {
			a.addImageAt(result.item, request.position, request.clamp, false)
		}
		a.startImport()
		a.redraw()
	default:
		if !imports.warned && time.Since(imports.started) >= time.Second {
			imports.warned = true
			notice := a.tray
			notice.Flags = 0x10 // NIF_INFO: nonblocking notification, once per session.
			notice.InfoFlags = 1
			copy(notice.InfoTitle[:], syscall.StringToUTF16("Image is still loading"))
			copy(notice.Info[:], syscall.StringToUTF16("Some images, including large AVIF files, take a few seconds to add. You can keep editing while they load."))
			shellNotifyIcon.Call(1, uintptr(unsafe.Pointer(&notice)))
		}
	}
}

func (a *application) stopImports() {
	if a.imports == nil {
		return
	}
	imports := a.imports
	imports.queue = nil
	if imports.active != nil {
		// Wait before GDI+ shutdown and remove the uncommitted original copy.
		result := <-imports.result
		if result.item != nil {
			call("GdipDisposeImage", result.item.Bitmap)
			runtime.KeepAlive(result.item.Pixels)
			if a.storage != "" && result.item.File != "" {
				os.Remove(filepath.Join(a.storage, "images", result.item.File))
			}
		}
	}
	a.imports = nil
}
