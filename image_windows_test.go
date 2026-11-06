//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

func imageTestGraphics(t *testing.T) {
	t.Helper()
	runtime.LockOSThread()
	if err := startGraphics(); err != nil {
		runtime.UnlockOSThread()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		call("GdipDeleteFont", font)
		call("GdiplusShutdown", gdiToken)
		runtime.UnlockOSThread()
	})
}

func rasterTestInputs(t *testing.T) map[string][]byte {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			im.SetNRGBA(x, y, color.NRGBA{R: 240, G: 30, B: 10, A: 255})
		}
	}
	inputs := make(map[string][]byte)
	encoders := map[string]func(io.Writer) error{
		"png":  func(w io.Writer) error { return png.Encode(w, im) },
		"jpeg": func(w io.Writer) error { return jpeg.Encode(w, im, &jpeg.Options{Quality: 100}) },
		"gif": func(w io.Writer) error {
			first := image.NewPaletted(im.Bounds(), color.Palette{color.NRGBA{R: 240, G: 30, B: 10, A: 255}})
			second := image.NewPaletted(im.Bounds(), color.Palette{color.NRGBA{G: 255, A: 255}})
			return gif.EncodeAll(w, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{10, 10}})
		},
		"bmp":  func(w io.Writer) error { return bmp.Encode(w, im) },
		"tiff": func(w io.Writer) error { return tiff.Encode(w, im, nil) },
	}
	for format, encode := range encoders {
		var buf bytes.Buffer
		if err := encode(&buf); err != nil {
			t.Fatal(err)
		}
		inputs[format] = buf.Bytes()
	}
	for _, format := range []string{"webp", "avif"} {
		data, err := os.ReadFile("testdata/sample." + format)
		if err != nil {
			t.Fatal(err)
		}
		inputs[format] = data
	}
	return inputs
}

func TestRasterImageFormats(t *testing.T) {
	imageTestGraphics(t)
	for format, data := range rasterTestInputs(t) {
		t.Run(format, func(t *testing.T) {
			// Use the reported misleading extension and a Unicode filename for every format.
			path := filepath.Join(t.TempDir(), "Odabaşı.jpg")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			item, err := loadImage(path)
			if err != nil {
				t.Fatal(err)
			}
			defer call("GdipDisposeImage", item.Bitmap)
			if item.W != 4 || item.H != 2 || item.Bitmap == 0 || len(item.Pixels) != 4*2*4 {
				t.Fatalf("invalid bitmap: %+v", item)
			}
			for i, want := range []byte{10, 30, 240, 255} {
				if delta := int(item.Pixels[i]) - int(want); delta < -3 || delta > 3 {
					t.Fatalf("BGRA pixel = %v, want approximately [10 30 240 255]", item.Pixels[:4])
				}
			}
			var pixel uint32
			if status := call("GdipBitmapGetPixel", item.Bitmap, 0, 0, uintptr(unsafe.Pointer(&pixel))); status != 0 {
				t.Fatalf("read native bitmap: %d", status)
			}
			if pixel>>24 != 255 || byte(pixel>>16) != item.Pixels[2] || byte(pixel>>8) != item.Pixels[1] || byte(pixel) != item.Pixels[0] {
				t.Fatalf("native pixel %#08x differs from BGRA %v", pixel, item.Pixels[:4])
			}
		})
	}
}

func TestRasterImageRejectsCorruptFiles(t *testing.T) {
	imageTestGraphics(t)
	inputs := rasterTestInputs(t)
	animated, err := os.ReadFile("testdata/animated.webp")
	if err != nil {
		t.Fatal(err)
	}
	inputs["animated.webp"] = animated
	for format, data := range inputs {
		t.Run(format, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "truncated."+format)
			if err := os.WriteFile(path, data[:12], 0600); err != nil {
				t.Fatal(err)
			}
			item, err := loadImage(path)
			if item != nil {
				call("GdipDisposeImage", item.Bitmap)
			}
			if err == nil {
				t.Fatal("truncated image accepted")
			}
		})
	}
}

func TestAVIFImageSizeLimit(t *testing.T) {
	data, err := os.ReadFile("testdata/sample.avif")
	if err != nil {
		t.Fatal(err)
	}
	ispe := bytes.Index(data, []byte("ispe"))
	if ispe < 0 || ispe+16 > len(data) {
		t.Fatal("fixture has no spatial extent")
	}
	binary.BigEndian.PutUint32(data[ispe+8:], 10001)
	binary.BigEndian.PutUint32(data[ispe+12:], 4000)
	path := filepath.Join(t.TempDir(), "oversized.jpg")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	item, err := loadImage(path)
	if item != nil {
		call("GdipDisposeImage", item.Bitmap)
	}
	if err == nil || !strings.Contains(err.Error(), "40 megapixel") {
		t.Fatalf("expected size-limit rejection before decoding, got %v", err)
	}
}

func TestRasterPremultipliedAlpha(t *testing.T) {
	imageTestGraphics(t)
	im := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			im.SetNRGBA(x, y, color.NRGBA{R: 240, G: 30, B: 10, A: 128})
		}
	}
	var pngData, tiffData bytes.Buffer
	if err := png.Encode(&pngData, im); err != nil {
		t.Fatal(err)
	}
	if err := tiff.Encode(&tiffData, im, nil); err != nil {
		t.Fatal(err)
	}
	inputs := rasterTestInputs(t)
	inputs["png"] = pngData.Bytes()
	inputs["tiff"] = tiffData.Bytes()
	for _, format := range []string{"png", "tiff", "webp", "avif"} {
		t.Run(format, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "alpha."+format)
			if err := os.WriteFile(path, inputs[format], 0600); err != nil {
				t.Fatal(err)
			}
			item, err := loadImage(path)
			if err != nil {
				t.Fatal(err)
			}
			defer call("GdipDisposeImage", item.Bitmap)
			pixel := item.Pixels[len(item.Pixels)-4:]
			if !bytes.Equal(pixel, []byte{5, 15, 120, 128}) {
				t.Fatalf("premultiplied BGRA = %v, want [5 15 120 128]", pixel)
			}
		})
	}
}
