//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestAnimatedWebPFirstFrame(t *testing.T) {
	imageTestGraphics(t)
	path := filepath.Join(t.TempDir(), "animation.jpg")
	data, err := os.ReadFile("testdata/animated.webp")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	item, err := loadImage(path)
	if err != nil {
		t.Fatal(err)
	}
	defer call("GdipDisposeImage", item.Bitmap)
	if item.W != 4 || item.H != 2 || item.Pixels[2] < 250 || item.Pixels[1] > 3 || item.Pixels[0] > 3 || item.Pixels[3] != 255 {
		t.Fatalf("expected first red frame, got %.0fx%.0f, BGRA %v", item.W, item.H, item.Pixels[:4])
	}
}

func TestAnimatedWebPCanvas(t *testing.T) {
	imageTestGraphics(t)
	still, err := os.ReadFile("testdata/sample.webp")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(still[12:16], []byte("VP8L")) {
		t.Fatal("expected a single lossless frame fixture")
	}
	for _, tc := range []struct {
		name  string
		flags byte
		want  []byte
	}{
		{"blend", 0, []byte{132, 15, 120, 255}},
		{"replace", 2, []byte{5, 15, 120, 128}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte("RIFF\x00\x00\x00\x00WEBP")
			addChunk := func(tag string, payload []byte) {
				data = append(data, tag...)
				data = binary.LittleEndian.AppendUint32(data, uint32(len(payload)))
				data = append(data, payload...)
				if len(payload)%2 != 0 {
					data = append(data, 0)
				}
			}
			// An 8x4 blue canvas, with a 4x2 frame placed at (2,2).
			addChunk("VP8X", []byte{0x12, 0, 0, 0, 7, 0, 0, 3, 0, 0})
			addChunk("ANIM", []byte{255, 0, 0, 255, 0, 0})
			frame := []byte{1, 0, 0, 1, 0, 0, 3, 0, 0, 1, 0, 0, 100, 0, 0, tc.flags}
			addChunk("ANMF", append(frame, still[12:]...))
			binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
			path := filepath.Join(t.TempDir(), "canvas.webp")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			item, err := loadImage(path)
			if err != nil {
				t.Fatal(err)
			}
			defer call("GdipDisposeImage", item.Bitmap)
			if item.W != 8 || item.H != 4 {
				t.Fatalf("canvas = %.0fx%.0f, want 8x4", item.W, item.H)
			}
			if !bytes.Equal(item.Pixels[:4], []byte{255, 0, 0, 255}) {
				t.Fatalf("background = %v, want opaque blue", item.Pixels[:4])
			}
			i := (3*8 + 2) * 4
			if !bytes.Equal(item.Pixels[i:i+4], tc.want) {
				t.Fatalf("frame pixel = %v, want %v", item.Pixels[i:i+4], tc.want)
			}
			// Move the same frame outside the declared canvas.
			anmf := bytes.Index(data, []byte("ANMF"))
			data[anmf+8] = 3
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			bad, err := loadImage(path)
			if bad != nil {
				call("GdipDisposeImage", bad.Bitmap)
			}
			if err == nil {
				t.Fatal("out-of-canvas frame accepted")
			}
		})
	}
}
