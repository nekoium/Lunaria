//go:build windows

package main

import (
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
	"unsafe"
)

func TestIssue8Measure(t *testing.T) {
	if os.Getenv("LUNARIA_MEASURE") != "1" {
		t.Skip("set LUNARIA_MEASURE=1 to measure issue 8")
	}
	imageTestGraphics(t)
	dir := filepath.Join(".tmp", "issue-8")
	avifPath := filepath.Join(dir, "issue4.avif")
	file, err := os.Open(avifPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(file)
	file.Close()
	if err != nil || format != "avif" || cfg.Width != 4000 || cfg.Height != 6000 {
		t.Fatalf("issue #4 fixture: format=%q size=%dx%d error=%v; want 4000x6000 AVIF", format, cfg.Width, cfg.Height, err)
	}
	for _, ext := range []string{"jpg", "png"} {
		path := filepath.Join(dir, "large."+ext)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		im := image.NewNRGBA(image.Rect(0, 0, 4000, 6000))
		for y := 0; y < 6000; y++ {
			for x := 0; x < 4000; x++ {
				i := y*im.Stride + x*4
				im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = byte(x), byte(y), 128, 255
			}
		}
		out, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if ext == "jpg" {
			err = jpeg.Encode(out, im, &jpeg.Options{Quality: 90})
		} else {
			err = png.Encode(out, im)
		}
		closeErr := out.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("create %s: encode=%v close=%v", path, err, closeErr)
		}
	}
	inputs := make([]string, 0, 20)
	for _, group := range []struct {
		name  string
		count int
	}{{"issue4.avif", 7}, {"large.jpg", 7}, {"large.png", 6}} {
		for range group.count {
			inputs = append(inputs, filepath.Join(dir, group.name))
		}
	}
	if value := os.Getenv("LUNARIA_MEASURE_COUNT"); value != "" {
		count, err := strconv.Atoi(value)
		if err != nil || count < 1 || count > len(inputs) {
			t.Fatalf("invalid LUNARIA_MEASURE_COUNT %q", value)
		}
		inputs = inputs[:count]
	}
	var items []*boardImage
	dispose := func() {
		for _, item := range items {
			call("GdipDisposeImage", item.Bitmap)
			runtime.KeepAlive(item.Pixels)
		}
		items = nil
	}
	t.Cleanup(dispose)
	load := func() {
		for _, path := range inputs {
			start := time.Now()
			item, err := loadImage(path)
			elapsed := time.Since(start)
			if err != nil {
				t.Fatalf("load %s: %v", path, err)
			}
			items = append(items, item)
			t.Logf("load %s: %.1f ms", path, float64(elapsed)/float64(time.Millisecond))
			if elapsed >= time.Second {
				t.Errorf("load %s took %v; must be under 1 second", path, elapsed)
			}
		}
	}
	type processMemory struct {
		Size, PageFaults                   uint32
		PeakWorkingSetSize, WorkingSetSize uintptr
		QuotaPeakPaged, QuotaPaged         uintptr
		QuotaPeakNonPaged, QuotaNonPaged   uintptr
		PagefileUsage, PeakPagefileUsage   uintptr
		PrivateUsage                       uintptr
	}
	memory := func() processMemory {
		var m processMemory
		m.Size = uint32(unsafe.Sizeof(m))
		handle, _, _ := kernel32.NewProc("GetCurrentProcess").Call()
		ok, _, err := kernel32.NewProc("K32GetProcessMemoryInfo").Call(handle, uintptr(unsafe.Pointer(&m)), uintptr(m.Size))
		if ok == 0 || m.WorkingSetSize == 0 {
			t.Fatalf("read process memory: result=%d workingSet=%d error=%v", ok, m.WorkingSetSize, err)
		}
		return m
	}
	const mib = 1024 * 1024
	load()
	t.Logf("peak MiB: %.2f", float64(memory().PeakWorkingSetSize)/mib)
	time.Sleep(30 * time.Second)
	stable := memory()
	t.Logf("stable MiB: working set=%.2f private=%.2f", float64(stable.WorkingSetSize)/mib, float64(stable.PrivateUsage)/mib)
	if stable.WorkingSetSize >= 100*mib {
		t.Errorf("stable working set must be under 100 MiB")
	}
	var bitmap, graphics uintptr
	if status := call("GdipCreateBitmapFromScan0", 1920, 1080, 0, 0xe200b, 0, uintptr(unsafe.Pointer(&bitmap))); status != 0 {
		t.Fatalf("create render bitmap: %d", status)
	}
	defer call("GdipDisposeImage", bitmap)
	if status := call("GdipGetImageGraphicsContext", bitmap, uintptr(unsafe.Pointer(&graphics))); status != 0 {
		t.Fatalf("create render graphics: %d", status)
	}
	defer call("GdipDeleteGraphics", graphics)
	call("GdipSetSmoothingMode", graphics, 4)
	call("GdipSetInterpolationMode", graphics, 7)
	call("GdipSetPixelOffsetMode", graphics, 4)
	start := time.Now()
	for range 60 {
		for _, item := range items {
			scale := min(1.0, 1056/item.W, 594/item.H)
			status := call("GdipDrawImageRectI", graphics, item.Bitmap, 0, 0, uintptr(int(item.W*scale)), uintptr(int(item.H*scale)))
			runtime.KeepAlive(item.Pixels)
			if status != 0 {
				t.Fatalf("draw image: %d", status)
			}
		}
	}
	frame := time.Since(start) / 60
	t.Logf("average frame: %.2f ms", float64(frame)/float64(time.Millisecond))
	if frame >= 16*time.Millisecond {
		t.Errorf("average frame must be under 16 ms")
	}
	var first uintptr
	for round := 1; round <= 10; round++ {
		dispose()
		load()
		time.Sleep(5 * time.Second)
		m := memory()
		t.Logf("round %d working set MiB: %.2f", round, float64(m.WorkingSetSize)/mib)
		if round == 1 {
			first = m.WorkingSetSize
		} else if round == 10 && m.WorkingSetSize > first+10*mib {
			t.Errorf("working set grew more than 10 MiB from the first round")
		}
	}
}
