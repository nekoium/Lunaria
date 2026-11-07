//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gen2brain/h265/heic"
)

func heicTestInput(t *testing.T) []byte {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			im.SetNRGBA(x, y, color.NRGBA{R: 240, G: 30, B: 10, A: 128})
		}
	}
	var buf bytes.Buffer
	if err := heic.Encode(&buf, im, heic.EncodeOptions{Lossless: true, Chroma: heic.Chroma444}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestHEICImport(t *testing.T) {
	imageTestGraphics(t)
	data := heicTestInput(t)
	for _, brand := range []string{"heic", "mif1"} {
		t.Run(brand, func(t *testing.T) {
			input := bytes.Clone(data)
			copy(input[8:12], brand)
			path := filepath.Join(t.TempDir(), "Odabaşı.jpg")
			if err := os.WriteFile(path, input, 0600); err != nil {
				t.Fatal(err)
			}
			item, err := loadImage(path)
			if err != nil {
				t.Fatal(err)
			}
			defer call("GdipDisposeImage", item.Bitmap)
			if item.W != 4 || item.H != 2 {
				t.Fatalf("dimensions %vx%v", item.W, item.H)
			}
			pixel := item.Pixels[:4]
			for i, want := range []byte{5, 15, 120, 128} {
				if delta := int(pixel[i]) - int(want); delta < -2 || delta > 2 {
					t.Fatalf("BGRA %v", pixel)
				}
			}
		})
	}
}

func TestHEICImportRotation(t *testing.T) {
	imageTestGraphics(t)
	data := heicTestInput(t)
	// Reuse the fixture's component-depth property slot for a quarter-turn property.
	pixi := bytes.Index(data, []byte("pixi"))
	if pixi < 0 || pixi+4 >= len(data) {
		t.Fatal("missing property slot")
	}
	copy(data[pixi:pixi+4], "irot")
	data[pixi+4] = 1
	path := filepath.Join(t.TempDir(), "portrait.heic")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	item, err := loadImage(path)
	if err != nil {
		t.Fatal(err)
	}
	defer call("GdipDisposeImage", item.Bitmap)
	if item.W != 2 || item.H != 4 || len(item.Pixels) != 32 {
		t.Fatalf("rotated dimensions %vx%v, bytes %d", item.W, item.H, len(item.Pixels))
	}
}

func TestHEICImportRejectsOversizeAndCorrupt(t *testing.T) {
	data := heicTestInput(t)
	oversized := bytes.Clone(data)
	ispe := bytes.Index(oversized, []byte("ispe"))
	if ispe < 0 {
		t.Fatal("missing spatial extent")
	}
	binary.BigEndian.PutUint32(oversized[ispe+8:], 10001)
	binary.BigEndian.PutUint32(oversized[ispe+12:], 4000)
	for name, input := range map[string][]byte{"oversized": oversized, "truncated": data[:12]} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), name+".heif")
			if err := os.WriteFile(path, input, 0600); err != nil {
				t.Fatal(err)
			}
			item, err := loadImage(path)
			if item != nil {
				call("GdipDisposeImage", item.Bitmap)
			}
			if err == nil {
				t.Fatal("invalid HEIC accepted")
			}
			if name == "oversized" && !strings.Contains(err.Error(), "40 megapixel") {
				t.Fatalf("wrong error: %v", err)
			}
		})
	}
}
