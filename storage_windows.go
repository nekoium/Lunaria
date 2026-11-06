//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unsafe"
)

type storedImage struct {
	File              string
	X, Y, W, H, Angle float64
}

type storedBoard struct {
	Version int
	Images  []storedImage
}

func (a *application) openStorage() error {
	if a.smoke {
		return nil
	}
	root, err := os.UserCacheDir() // LOCALAPPDATA on Windows, not the roaming profile.
	if err != nil {
		return err
	}
	a.storage = filepath.Join(root, "Lunaria")
	return a.loadBoard()
}

func validStoredImage(im storedImage) bool {
	if !strings.HasPrefix(im.File, "image-") || len(im.File) == len("image-") {
		return false
	}
	for _, c := range strings.TrimPrefix(im.File, "image-") {
		if c < '0' || c > '9' {
			return false
		}
	}
	for _, v := range []float64{im.X, im.Y, im.W, im.H, im.Angle} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return im.W > 0 && im.H > 0
}

func (a *application) loadBoard() error {
	path := filepath.Join(a.storage, "board.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read board %s: %w", path, err)
	}
	var board storedBoard
	if err := json.Unmarshal(data, &board); err != nil {
		return fmt.Errorf("read board %s (file left unchanged): %w", path, err)
	}
	if board.Version != 1 || board.Images == nil {
		return fmt.Errorf("unsupported or incomplete board %s; file left unchanged", path)
	}
	seen := make(map[string]bool)
	for _, im := range board.Images {
		if !validStoredImage(im) || seen[im.File] {
			return fmt.Errorf("invalid or duplicate image record in %s; file left unchanged", path)
		}
		seen[im.File] = true
	}
	var images []*boardImage
	for _, im := range board.Images {
		item, err := loadImage(filepath.Join(a.storage, "images", im.File))
		if err != nil {
			for _, loaded := range images {
				call("GdipDisposeImage", loaded.Bitmap)
			}
			return fmt.Errorf("restore board image %s (board left unchanged): %w", im.File, err)
		}
		item.File = im.File
		item.X, item.Y, item.W, item.H, item.Angle = im.X, im.Y, im.W, im.H, im.Angle
		images = append(images, item)
	}
	a.images = images
	return nil
}

func (a *application) importImage(path string) (*boardImage, error) {
	if a.storage == "" {
		return loadImage(path)
	}
	source, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	dir := filepath.Join(a.storage, "images")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	copy, err := os.CreateTemp(dir, "image-*")
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		copy.Close()
		if !keep {
			os.Remove(copy.Name())
		}
	}()
	if _, err := io.Copy(copy, source); err != nil {
		return nil, err
	}
	if err := copy.Sync(); err != nil {
		return nil, err
	}
	if err := copy.Close(); err != nil {
		return nil, err
	}
	item, err := loadImage(copy.Name())
	if err != nil {
		return nil, err
	}
	item.File = filepath.Base(copy.Name())
	keep = true
	return item, nil
}

func (a *application) saveBoard() error {
	if a.storage == "" {
		return nil
	}
	board := storedBoard{Version: 1, Images: make([]storedImage, 0, len(a.images))}
	for _, item := range a.images {
		im := storedImage{item.File, item.X, item.Y, item.W, item.H, item.Angle}
		if !validStoredImage(im) {
			return fmt.Errorf("invalid image geometry or storage filename; previous board left unchanged")
		}
		board.Images = append(board.Images, im)
	}
	data, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(a.storage, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(a.storage, ".board-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	// Windows os.Rename does not promise atomic replacement of an existing file.
	from, to := u16(file.Name()), u16(filepath.Join(a.storage, "board.json"))
	ok, _, err := kernel32.NewProc("MoveFileExW").Call(uintptr(unsafe.Pointer(from)), uintptr(unsafe.Pointer(to)), 1|8)
	if ok == 0 {
		return fmt.Errorf("replace board: %w", err)
	}
	return nil
}

func (a *application) persistBoard() bool {
	if err := a.saveBoard(); err != nil {
		wasDialog := a.dialog
		a.dialog = true
		notifyError(fmt.Sprintf("Cannot save board to %s. Changes remain in memory; the previous saved board is unchanged.\n%v", a.storage, err))
		a.dialog = wasDialog
		return false
	}
	return true
}
