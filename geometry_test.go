//go:build windows

package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRotatedGeometry(t *testing.T) {
	item := boardImage{X: 200, Y: 150, W: 120, H: 80, Angle: 37}
	for _, p := range []vector{{0, 0}, {-60, -40}, {23, 17}} {
		got := item.local(item.world(p.x, p.y))
		if math.Abs(got.x-p.x) > 1e-8 || math.Abs(got.y-p.y) > 1e-8 {
			t.Fatalf("transform did not round trip: %v -> %v", p, got)
		}
	}
	if !item.contains(item.world(59, 39)) || item.contains(item.world(61, 0)) {
		t.Fatal("rotated hit test failed")
	}
}
func TestCardinalSnapping(t *testing.T) {
	for _, tc := range []struct{ in, want float64 }{{-2, 0}, {358, 0}, {87, 90}, {184, 180}, {266, 270}, {42, 42}} {
		if got := snapAngle(tc.in); got != tc.want {
			t.Errorf("snapAngle(%v)=%v, want %v", tc.in, got, tc.want)
		}
	}
}
func TestNativeImageLoad(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := startGraphics(); err != nil {
		t.Fatal(err)
	}
	defer call("GdiplusShutdown", gdiToken)
	defer call("GdipDeleteFont", font)
	im := image.NewNRGBA(image.Rect(0, 0, 640, 420))
	for y := 0; y < 420; y++ {
		for x := 0; x < 640; x++ {
			c := color.NRGBA{R: 30 + uint8(x/5), G: 35 + uint8(y/4), B: 100, A: 255}
			if math.Hypot(float64(x-320), float64(y-210)) < 115 {
				c = color.NRGBA{220, 211, 174, 255}
			}
			if x > 340 && math.Hypot(float64(x-375), float64(y-180)) < 100 {
				c = color.NRGBA{45, 45, 107, 255}
			}
			im.SetNRGBA(x, y, c)
		}
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	err = png.Encode(f, im)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	item, err := loadImage(path)
	if err != nil {
		t.Fatal(err)
	}
	defer call("GdipDisposeImage", item.Bitmap)
	if item.W != 640 || item.H != 420 || item.Bitmap == 0 || len(item.Pixels) != 640*420*4 {
		t.Fatal("invalid decoded image")
	}
	if _, err := loadImage(filepath.Join(dir, "missing.png")); err == nil {
		t.Fatal("missing image accepted")
	}
}
