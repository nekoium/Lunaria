//go:build windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unsafe"
)

func waitForImport(t *testing.T, a *application) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for len(a.imports.result) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("import did not complete")
		}
		time.Sleep(time.Millisecond)
	}
}

func drainImports(t *testing.T, a *application) {
	t.Helper()
	for a.imports.active != nil {
		waitForImport(t, a)
		a.importTick()
	}
}

func TestQueuedImportOrderAndShutdown(t *testing.T) {
	imageTestGraphics(t)
	// Hidden layered window exercises rendering without taking focus or showing a board.
	window, _, err := createWindowEx.Call(wsExLayered|wsExToolWindow|wsExNoActivate, uintptr(unsafe.Pointer(u16("STATIC"))), 0, wsPopup, 0, 0, 1000, 800, 0, 0, 0, 0)
	if window == 0 {
		t.Fatal(apiError("create test surface", err))
	}
	defer destroyWindow.Call(window)
	a := &application{board: window, width: 1000, height: 800, selected: -1, storage: t.TempDir(), imports: &imageImports{}}
	defer func() {
		a.stopImports()
		for _, item := range a.images {
			call("GdipDisposeImage", item.Bitmap)
		}
	}()
	data, err := os.ReadFile("testdata/sample.png")
	if err != nil {
		t.Fatal(err)
	}
	position := vector{-10, 900}
	a.queueImport(imageImport{path: "testdata/sample.png", position: &position, clamp: true})
	a.queueImport(imageImport{data: data, position: &vector{150, 200}})
	position = vector{500, 500}
	if len(a.images) != 0 || len(a.imports.queue) != 1 {
		t.Fatal("queue must not decode synchronously or run concurrent imports")
	}
	waitForImport(t, a)
	// A finished import cannot change selection or geometry during an active drag.
	a.drag = "move"
	a.importTick()
	if len(a.images) != 0 {
		t.Fatal("import changed board during a drag")
	}
	a.drag = ""
	a.importTick()
	if len(a.images) != 1 || len(a.imports.queue) != 0 || a.imports.active == nil {
		t.Fatal("first result was not added before starting the second import")
	}
	first := a.images[0]
	if first.X != first.W/2 || first.Y != 800-first.H/2 {
		t.Fatalf("drop position not copied and clamped: %v,%v", first.X, first.Y)
	}
	drainImports(t, a)
	if len(a.images) != 2 || a.images[1].X != 150 || a.images[1].Y != 200 || a.editing {
		t.Fatal("import order, position, or closed editing state changed")
	}
	for _, item := range a.images {
		stored, err := os.ReadFile(filepath.Join(a.storage, "images", item.File))
		if err != nil || !bytes.Equal(stored, data) {
			t.Fatalf("queued import changed original: %v", err)
		}
	}
	// Quit while one import is running and another is waiting. Neither is saved.
	a.queueImport(imageImport{path: "testdata/sample.png"})
	a.queueImport(imageImport{path: "testdata/sample.png"})
	a.stopImports()
	entries, err := os.ReadDir(filepath.Join(a.storage, "images"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("shutdown left an uncommitted original: entries=%d error=%v", len(entries), err)
	}
	if a.imports != nil {
		t.Fatal("worker still attached after shutdown")
	}
}

func TestIssue8ImportResponsiveness(t *testing.T) {
	if os.Getenv("LUNARIA_MEASURE") != "1" {
		t.Skip("set LUNARIA_MEASURE=1 to measure importing responsiveness")
	}
	imageTestGraphics(t)
	window, _, err := createWindowEx.Call(wsExLayered|wsExToolWindow|wsExNoActivate, uintptr(unsafe.Pointer(u16("STATIC"))), 0, wsPopup, 0, 0, 1000, 800, 0, 0, 0, 0)
	if window == 0 {
		t.Fatal(apiError("create test surface", err))
	}
	defer destroyWindow.Call(window)
	item, err := loadImage("testdata/sample.png")
	if err != nil {
		t.Fatal(err)
	}
	defer call("GdipDisposeImage", item.Bitmap)
	a := &application{board: window, width: 1000, height: 800, editing: true, selected: 0, images: []*boardImage{item}, storage: t.TempDir(), imports: &imageImports{}}
	defer a.stopImports()
	start := time.Now()
	a.queueImport(imageImport{path: filepath.Join(".tmp", "issue-8", "issue4.avif")})
	enqueue := time.Since(start)
	if enqueue >= 100*time.Millisecond {
		t.Fatalf("queueing an import blocked for %v", enqueue)
	}
	a.drag, a.original, a.start = "move", *item, vector{}
	var frames int
	var total, longest time.Duration
	for {
		frameStart := time.Now()
		a.mouseMove(vector{float64(frames % 100), float64(frames % 50)})
		elapsed := time.Since(frameStart)
		frames++
		total += elapsed
		longest = max(longest, elapsed)
		if len(a.imports.result) != 0 && frames >= 60 {
			break
		}
		if time.Since(start) > 15*time.Second {
			t.Fatal("background AVIF import did not complete")
		}
		time.Sleep(10 * time.Millisecond)
	}
	result := <-a.imports.result
	a.imports.active = nil
	if result.err != nil {
		t.Fatal(result.err)
	}
	defer call("GdipDisposeImage", result.item.Bitmap)
	t.Logf("queue=%.2f ms, load=%.2f s, drag frames=%d average=%.2f ms maximum=%.2f ms", float64(enqueue)/float64(time.Millisecond), time.Since(start).Seconds(), frames, float64(total)/float64(frames)/float64(time.Millisecond), float64(longest)/float64(time.Millisecond))
	if total/time.Duration(frames) >= 16*time.Millisecond || longest >= 100*time.Millisecond {
		t.Error("dragging stalled while decoding AVIF")
	}
}

func TestQueuedImportFailureCleansCopy(t *testing.T) {
	imageTestGraphics(t)
	a := &application{storage: t.TempDir(), imports: &imageImports{}}
	defer a.stopImports()
	a.queueImport(imageImport{data: []byte("not an image")})
	waitForImport(t, a)
	result := <-a.imports.result
	a.imports.active = nil
	if result.err == nil || result.item != nil {
		t.Fatal("invalid image accepted")
	}
	entries, err := os.ReadDir(filepath.Join(a.storage, "images"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed import left a partial original: %v", err)
	}
	a.queueImport(imageImport{path: "testdata/sample.png"})
	waitForImport(t, a)
	if len(a.imports.result) != 1 {
		t.Fatal("later import did not run after a failure")
	}
}
