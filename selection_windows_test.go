//go:build windows

package main

import "testing"

func TestSelectingImageBringsItToFront(t *testing.T) {
	imageTestGraphics(t)
	a := &application{editing: true, selected: -1, storage: t.TempDir()}
	for _, x := range []float64{150, 250, 600} {
		item, err := a.importImage("testdata/sample.png")
		if err != nil {
			t.Fatal(err)
		}
		item.X, item.Y, item.W, item.H = x, 150, 200, 200
		a.images = append(a.images, item)
	}
	defer func() {
		for _, item := range a.images {
			call("GdipDisposeImage", item.Bitmap)
		}
	}()
	bottom, middle, top := a.images[0], a.images[1], a.images[2]
	// The left part of the bottom image is visible beside the overlapping middle image.
	a.mouseDown(vector{100, 150})
	if a.selected != 2 || a.images[0] != middle || a.images[1] != top || a.images[2] != bottom {
		t.Fatal("click did not bring the image to the front while keeping the other images in order")
	}
	a.stopDrag()
	restored := &application{storage: a.storage}
	if err := restored.loadBoard(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, item := range restored.images {
			call("GdipDisposeImage", item.Bitmap)
		}
	}()
	if len(restored.images) != 3 || restored.images[0].File != middle.File || restored.images[1].File != top.File || restored.images[2].File != bottom.File {
		t.Fatal("front-to-back order did not survive restart after a click without movement")
	}
	// Hit-testing at the overlap must now select the image that was just raised.
	a.mouseDown(vector{200, 150})
	a.mouseMove(vector{220, 160})
	a.stopDrag()
	if a.images[a.selected] != bottom || bottom.X != 170 || bottom.Y != 160 || middle.X != 250 || top.X != 600 {
		t.Fatal("drag moved the wrong image after changing layer order")
	}
	// Existing resize handles must keep pointing at the raised image.
	corner, anchor := bottom.corners()[2], bottom.corners()[0]
	a.mouseDown(corner)
	a.mouseMove(vector{anchor.x + (corner.x-anchor.x)*1.2, anchor.y + (corner.y-anchor.y)*1.2})
	a.stopDrag()
	if bottom.W != 240 || bottom.H != 240 || middle.W != 200 {
		t.Fatal("resize targeted the wrong image after changing layer order")
	}
	a.mouseDown(vector{900, 900})
	a.stopDrag()
	if a.selected != -1 || a.images[2] != bottom {
		t.Fatal("clicking empty space changed layer order")
	}
	a.editing = false
	a.mouseDown(vector{300, 150})
	if a.selected != -1 || a.images[0] != middle || a.images[2] != bottom {
		t.Fatal("passive-mode click changed selection or layer order")
	}
}
