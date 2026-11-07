//go:build windows

package main

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	_ "github.com/gen2brain/avif"
	"github.com/gen2brain/h265/heic"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

type boardImage struct {
	File              string
	X, Y, W, H, Angle float64
	Bitmap            uintptr
	Pixels            []byte
}
type bitmapInfo struct {
	Size                   uint32
	Width, Height          int32
	Planes, BitCount       uint16
	Compression, SizeImage uint32
	XPels, YPels           int32
	Used, Important        uint32
}
type rectF struct{ X, Y, W, H float32 }

var gdiToken, font uintptr
var uiScale = 1.0
var handleGap = 32.0

func startGraphics() error {
	if proc := user32.NewProc("GetDpiForSystem"); proc.Find() == nil {
		if dpi, _, _ := proc.Call(); dpi > 0 {
			uiScale = float64(dpi) / 96
			handleGap = 32 * uiScale
		}
	}
	input := struct {
		Version                        uint32
		Callback                       uintptr
		SuppressThread, SuppressCodecs int32
	}{Version: 1}
	if status := call("GdiplusStartup", uintptr(unsafe.Pointer(&gdiToken)), uintptr(unsafe.Pointer(&input)), 0); status != 0 {
		return fmt.Errorf("GDI+ startup: %d", status)
	}
	var family uintptr
	if call("GdipCreateFontFamilyFromName", uintptr(unsafe.Pointer(u16("Segoe UI"))), 0, uintptr(unsafe.Pointer(&family))) != 0 {
		return fmt.Errorf("Segoe UI unavailable")
	}
	status := call("GdipCreateFont", family, f32(17*uiScale), 0, 2, uintptr(unsafe.Pointer(&font)))
	call("GdipDeleteFontFamily", family)
	if status != 0 {
		return fmt.Errorf("font creation: %d", status)
	}
	return nil
}

func loadImage(path string) (*boardImage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readImage(file)
}

func readImage(file io.ReadSeeker) (*boardImage, error) {
	cfg, format, err := image.DecodeConfig(file)
	if err != nil {
		return nil, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 40_000_000 {
		return nil, fmt.Errorf("image exceeds the prototype's 40 megapixel limit")
	}
	if _, err = file.Seek(0, 0); err != nil {
		return nil, err
	}
	var decoded image.Image
	if format == "heic" {
		decoded, err = heic.Decode(file, heic.Options{AutoRotate: true, FrameSizeLimit: 40_000_000, Threads: 4})
	} else {
		decoded, format, err = image.Decode(file)
	}
	if err != nil && format == "webp" {
		if _, err = file.Seek(0, 0); err != nil {
			return nil, err
		}
		decoded, err = decodeWebPFirstFrame(file, cfg)
	}
	if err != nil {
		return nil, err
	}
	bounds := decoded.Bounds()
	if format == "heic" {
		// Container rotation and cropping can change the configured dimensions.
		cfg.Width, cfg.Height = bounds.Dx(), bounds.Dy()
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 40_000_000 {
		return nil, fmt.Errorf("image exceeds the prototype's 40 megapixel limit")
	}
	pixels := make([]byte, cfg.Width*cfg.Height*4)
	rgba := &image.RGBA{Pix: pixels, Stride: cfg.Width * 4, Rect: image.Rect(0, 0, cfg.Width, cfg.Height)}
	draw.Draw(rgba, rgba.Bounds(), decoded, bounds.Min, draw.Src)
	for i := 0; i < len(pixels); i += 4 {
		pixels[i], pixels[i+2] = pixels[i+2], pixels[i]
	}
	item := &boardImage{Pixels: pixels, W: float64(cfg.Width), H: float64(cfg.Height)}
	// Scan0 retains this buffer; each item's Pixels field keeps it alive until bitmap disposal.
	status := call("GdipCreateBitmapFromScan0", uintptr(cfg.Width), uintptr(cfg.Height), uintptr(cfg.Width*4), 0xe200b, uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&item.Bitmap)))
	runtime.KeepAlive(pixels)
	if status != 0 {
		return nil, fmt.Errorf("create image: %d", status)
	}
	return item, nil
}

func (a *application) render() error {
	if a.width <= 0 || a.height <= 0 {
		return nil
	}
	dc, _, _ := getDC.Call(0)
	defer releaseDC.Call(0, dc)
	mem, _, _ := createCompatibleDC.Call(dc)
	if mem == 0 {
		return fmt.Errorf("create drawing context")
	}
	defer deleteDC.Call(mem)
	info := bitmapInfo{Size: 40, Width: int32(a.width), Height: -int32(a.height), Planes: 1, BitCount: 32}
	var pixels uintptr
	dib, _, _ := createDIBSection.Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&pixels)), 0, 0)
	if dib == 0 {
		return fmt.Errorf("create drawing surface")
	}
	defer deleteObject.Call(dib)
	old, _, _ := selectObject.Call(mem, dib)
	defer selectObject.Call(mem, old)
	var bitmap, graphics uintptr
	status := call("GdipCreateBitmapFromScan0", uintptr(a.width), uintptr(a.height), uintptr(a.width*4), 0xe200b, pixels, uintptr(unsafe.Pointer(&bitmap)))
	if status != 0 {
		return fmt.Errorf("create surface: %d", status)
	}
	defer call("GdipDisposeImage", bitmap)
	if call("GdipGetImageGraphicsContext", bitmap, uintptr(unsafe.Pointer(&graphics))) != 0 {
		return fmt.Errorf("create renderer")
	}
	call("GdipGraphicsClear", graphics, 0)
	if !a.editing && a.desktopAfter != 0 {
		// shortcut: color-keyed GDI flattens partial alpha and keys out #010203; use alpha-capable composition when transparency fidelity is required.
		call("GdipGraphicsClear", graphics, 0xff010203)
	}
	call("GdipSetSmoothingMode", graphics, 4)
	call("GdipSetInterpolationMode", graphics, 7)
	call("GdipSetPixelOffsetMode", graphics, 4)
	if a.editing {
		call("GdipGraphicsClear", graphics, 0x90e8ebf5)
	}
	for _, item := range a.images {
		call("GdipResetWorldTransform", graphics)
		call("GdipTranslateWorldTransform", graphics, f32(item.X), f32(item.Y), 0)
		call("GdipRotateWorldTransform", graphics, f32(item.Angle), 0)
		// Draw the entire source bitmap into the transformed destination rectangle.
		call("GdipDrawImageRectI", graphics, item.Bitmap, signed(int32(-item.W/2)), signed(int32(-item.H/2)), uintptr(int(item.W)), uintptr(int(item.H)))
		runtime.KeepAlive(item.Pixels)
	}
	call("GdipResetWorldTransform", graphics)
	if a.editing {
		if a.selected >= 0 && a.selected < len(a.images) {
			drawHandles(graphics, a.images[a.selected])
		}
		drawHint(graphics, a.width)
	}
	call("GdipDeleteGraphics", graphics)
	if a.smoke && a.smokeStage == 2 && a.selected >= 0 {
		pngEncoder := syscall.GUID{Data1: 0x557cf406, Data2: 0x1a04, Data3: 0x11d3, Data4: [8]byte{0x9a, 0x73, 0, 0, 0xf8, 0x1e, 0xf3, 0x2e}}
		call("GdipSaveImageToFile", bitmap, uintptr(unsafe.Pointer(u16(".tmp/edit-preview.png"))), uintptr(unsafe.Pointer(&pngEncoder)), 0)
	}
	if !a.editing && a.desktopAfter != 0 {
		// Raised Explorer desktops need GDI-compatible layering below the icon view.
		ok, _, err := user32.NewProc("SetLayeredWindowAttributes").Call(a.board, 0x030201, 255, 3)
		if ok == 0 {
			return apiError("SetLayeredWindowAttributes", err)
		}
		target, _, _ := getDC.Call(a.board)
		defer releaseDC.Call(a.board, target)
		ok, _, err = gdi32.NewProc("BitBlt").Call(target, 0, 0, uintptr(a.width), uintptr(a.height), mem, 0, 0, 0x00cc0020)
		if ok == 0 {
			return apiError("paint passive board", err)
		}
		return nil
	}
	position := point{}
	source := point{}
	extent := size{int32(a.width), int32(a.height)}
	blend := [4]byte{0, 0, 255, 1}
	ok, _, err := updateLayeredWindow.Call(a.board, dc, uintptr(unsafe.Pointer(&position)), uintptr(unsafe.Pointer(&extent)), mem, uintptr(unsafe.Pointer(&source)), 0, uintptr(unsafe.Pointer(&blend)), 2)
	if ok == 0 {
		return apiError("UpdateLayeredWindow", err)
	}
	return nil
}

func drawHint(graphics uintptr, width int) {
	var brush uintptr
	call("GdipCreateSolidFill", 0xf5222635, uintptr(unsafe.Pointer(&brush)))
	w := math.Min(float64(width)-48*uiScale, 1050*uiScale)
	call("GdipFillRectangle", graphics, brush, f32(24*uiScale), f32(24*uiScale), f32(w), f32(66*uiScale))
	call("GdipDeleteBrush", brush)
	call("GdipCreateSolidFill", 0xfff5f6ff, uintptr(unsafe.Pointer(&brush)))
	area := rectF{float32(40 * uiScale), float32(33 * uiScale), float32(w - 28*uiScale), float32(54 * uiScale)}
	text := "Lunaria · Edit mode · Ctrl+Alt+L to finish\nDrop or Ctrl+V to paste images · Drag to move · Corners to resize · Top handle to rotate · Select, then Delete to remove"
	call("GdipDrawString", graphics, uintptr(unsafe.Pointer(u16(text))), ^uintptr(0), font, uintptr(unsafe.Pointer(&area)), 0, brush)
	call("GdipDeleteBrush", brush)
}

func drawHandles(graphics uintptr, item *boardImage) {
	var pen, brush uintptr
	call("GdipCreatePen1", 0xff6863df, f32(2*uiScale), 2, uintptr(unsafe.Pointer(&pen)))
	call("GdipCreateSolidFill", 0xfffefeff, uintptr(unsafe.Pointer(&brush)))
	corners := item.corners()
	for i := 0; i < 4; i++ {
		p, q := corners[i], corners[(i+1)%4]
		call("GdipDrawLine", graphics, pen, f32(p.x), f32(p.y), f32(q.x), f32(q.y))
	}
	top := item.world(0, -item.H/2)
	rot := item.world(0, -item.H/2-handleGap)
	call("GdipDrawLine", graphics, pen, f32(top.x), f32(top.y), f32(rot.x), f32(rot.y))
	for _, p := range append(corners[:], rot) {
		r := 6 * uiScale
		call("GdipFillEllipse", graphics, brush, f32(p.x-r), f32(p.y-r), f32(2*r), f32(2*r))
		call("GdipDrawEllipse", graphics, pen, f32(p.x-r), f32(p.y-r), f32(2*r), f32(2*r))
	}
	call("GdipDeletePen", pen)
	call("GdipDeleteBrush", brush)
}
