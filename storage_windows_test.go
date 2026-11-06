//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

func TestBoardStorage(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := startGraphics(); err != nil {
		t.Fatal(err)
	}
	defer call("GdiplusShutdown", gdiToken)
	defer call("GdipDeleteFont", font)
	dispose := func(a *application) {
		for _, im := range a.images {
			call("GdipDisposeImage", im.Bitmap)
		}
	}
	t.Run("restart retains copies order and transforms after originals disappear", func(t *testing.T) {
		root := t.TempDir()
		source := filepath.Join(root, "original.png")
		data, err := os.ReadFile("testdata/sample.png")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(source, data, 0600); err != nil {
			t.Fatal(err)
		}
		a := &application{storage: filepath.Join(root, "storage")}
		defer dispose(a)
		for i := 0; i < 2; i++ {
			im, err := a.importImage(source)
			if err != nil {
				t.Fatal(err)
			}
			im.X, im.Y, im.W, im.H, im.Angle = float64(i)*120-20, 45, 150, 90, 37
			a.images = append(a.images, im)
		}
		if err := a.saveBoard(); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(source); err != nil {
			t.Fatal(err)
		}
		b := &application{storage: a.storage}
		defer dispose(b)
		if err := b.loadBoard(); err != nil {
			t.Fatal(err)
		}
		if len(b.images) != 2 || b.images[0].File == b.images[1].File {
			t.Fatal("image count or unique copies lost")
		}
		for i, got := range b.images {
			want := a.images[i]
			if got.File != want.File || got.X != want.X || got.Y != want.Y || got.W != want.W || got.H != want.H || got.Angle != want.Angle || !bytes.Equal(got.Pixels, want.Pixels) {
				t.Fatalf("image %d did not survive restart", i)
			}
		}
		// Exercise the actual deletion handler without creating a window.
		previous := app
		app = b
		defer func() { app = previous }()
		b.selected, b.editing = 0, true
		removed := b.images[0].File
		windowProc(0, wmKeyDown, 0x2e, 0)
		if _, err := os.Stat(filepath.Join(b.storage, "images", removed)); !os.IsNotExist(err) {
			t.Fatalf("deleted image copy remains: %v", err)
		}
		b.images[0].X = 713
		b.drag = "move"
		b.stopDrag()
		c := &application{storage: b.storage}
		defer dispose(c)
		if err := c.loadBoard(); err != nil || len(c.images) != 1 || c.images[0].X != 713 {
			t.Fatalf("delete or completed drag not saved: %v", err)
		}
		b.selected = 0
		windowProc(0, wmKeyDown, 0x2e, 0)
		empty := &application{storage: b.storage}
		if err := empty.loadBoard(); err != nil || len(empty.images) != 0 {
			t.Fatalf("empty board not restored: %v", err)
		}
	})
	t.Run("bad boards are preserved", func(t *testing.T) {
		for _, data := range []string{
			`{broken`, `null`, `{}`, `{"Version":2,"Images":[]}`,
			`{"Version":1,"Images":[{"File":"../outside","W":10,"H":10}]}`,
			`{"Version":1,"Images":[{"File":"image-1:stream","W":10,"H":10}]}`,
			`{"Version":1,"Images":[{"File":"image-1","W":0,"H":10}]}`,
			`{"Version":1,"Images":[{"File":"image-1","W":10,"H":10}]}`,
			`{"Version":1,"Images":[{"File":"image-1","W":10,"H":10},{"File":"image-1","W":10,"H":10}]}`,
		} {
			a := &application{storage: t.TempDir()}
			path := filepath.Join(a.storage, "board.json")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if err := a.loadBoard(); err == nil {
				t.Fatalf("accepted invalid or unrestorable board: %s", data)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != data || len(a.images) != 0 {
				t.Fatalf("failed load changed saved board: %v", err)
			}
		}
	})
	t.Run("failed replacement preserves previous JSON", func(t *testing.T) {
		a := &application{storage: t.TempDir()}
		if err := a.saveBoard(); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(a.storage, "board.json")
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		handle, err := syscall.CreateFile(syscall.StringToUTF16Ptr(path), syscall.GENERIC_READ, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer syscall.CloseHandle(handle)
		a.images = []*boardImage{{File: "image-1", W: 10, H: 10}}
		if err := a.saveBoard(); err == nil {
			t.Fatal("replacement succeeded despite delete-sharing lock")
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("failed save changed previous board: %v", err)
		}
		files, err := filepath.Glob(filepath.Join(a.storage, ".board-*"))
		if err != nil || len(files) != 0 {
			t.Fatalf("temporary JSON files leaked: %v %v", files, err)
		}
	})
	t.Run("invalid geometry cannot replace saved board", func(t *testing.T) {
		a := &application{storage: t.TempDir()}
		if err := a.saveBoard(); err != nil {
			t.Fatal(err)
		}
		a.images = []*boardImage{{File: "image-1", W: 10, H: 10, X: math.NaN()}}
		if err := a.saveBoard(); err == nil {
			t.Fatal("accepted nonfinite geometry")
		}
		b := &application{storage: a.storage}
		if err := b.loadBoard(); err != nil || len(b.images) != 0 {
			t.Fatalf("invalid geometry replaced saved board: %v", err)
		}
	})
	t.Run("invalid imports remove their copy", func(t *testing.T) {
		a := &application{storage: t.TempDir()}
		source := filepath.Join(a.storage, "bad.png")
		if err := os.WriteFile(source, []byte("not an image"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := a.importImage(source); err == nil {
			t.Fatal("accepted invalid import")
		}
		files, err := os.ReadDir(filepath.Join(a.storage, "images"))
		if err != nil || len(files) != 0 {
			t.Fatalf("failed import leaked a copy: %v", err)
		}
	})
	t.Run("smoke mode never opens user storage", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("LOCALAPPDATA", root)
		a := &application{smoke: true}
		if err := a.openStorage(); err != nil || a.storage != "" {
			t.Fatalf("smoke opened persistence: %v", err)
		}
		if err := a.saveBoard(); err != nil {
			t.Fatal(err)
		}
		files, err := os.ReadDir(root)
		if err != nil || len(files) != 0 {
			t.Fatalf("smoke changed user storage: %v", err)
		}
	})
	t.Run("normal mode uses local app data", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("LOCALAPPDATA", root)
		a := &application{}
		if err := a.openStorage(); err != nil || a.storage != filepath.Join(root, "Lunaria") {
			t.Fatalf("wrong storage path: %q %v", a.storage, err)
		}
		if err := a.saveBoard(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(a.storage, "board.json"))
		if err != nil {
			t.Fatal(err)
		}
		var board storedBoard
		if err := json.Unmarshal(data, &board); err != nil || board.Version != 1 || board.Images == nil {
			t.Fatalf("invalid empty board: %s %v", data, err)
		}
	})
}
