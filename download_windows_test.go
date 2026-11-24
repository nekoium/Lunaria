//go:build windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadOriginal(t *testing.T) {
	storage, folder := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(storage, "images"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ fixture, file, ext string }{
		{"sample.avif", "image-123", ".avif"},
		{"sample.png", "image-124", ".png"},
	} {
		data, err := os.ReadFile(filepath.Join("testdata", tc.fixture))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(storage, "images", tc.file), data, 0600); err != nil {
			t.Fatal(err)
		}
		path, err := downloadOriginal(storage, tc.file, folder)
		if err != nil || path != filepath.Join(folder, "Lunaria-"+tc.file+tc.ext) {
			t.Fatalf("download path=%q error=%v", path, err)
		}
		if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, data) {
			t.Fatalf("original bytes changed: %v", err)
		}
		if tc.ext == ".avif" {
			// Existing output must survive even if it has been changed by the user.
			existing := []byte("keep this file")
			if err := os.WriteFile(path, existing, 0600); err != nil {
				t.Fatal(err)
			}
			second, err := downloadOriginal(storage, tc.file, folder)
			if err != nil || second != filepath.Join(folder, "Lunaria-image-123 (2).avif") {
				t.Fatalf("duplicate path=%q error=%v", second, err)
			}
			if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, existing) {
				t.Fatalf("existing download replaced: %v", err)
			}
			if got, err := os.ReadFile(second); err != nil || !bytes.Equal(got, data) {
				t.Fatalf("duplicate bytes changed: %v", err)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(storage, "images", "image-126"), []byte("not an image"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"image-125", "image-126", "../image-123", "image-123/../image-124"} {
		if _, err := downloadOriginal(storage, file, folder); err == nil {
			t.Errorf("invalid source %q accepted", file)
		}
	}
	entries, err := os.ReadDir(folder)
	if err != nil || len(entries) != 3 {
		t.Fatalf("download directory has partial files: entries=%v error=%v", entries, err)
	}
	if _, err := downloadOriginal(storage, "image-123", filepath.Join(folder, "missing")); err == nil {
		t.Fatal("missing destination accepted")
	}
}

func TestDownloadsFolder(t *testing.T) {
	folder, err := downloadsFolder()
	if err != nil || !filepath.IsAbs(folder) || strings.TrimSpace(folder) == "" {
		t.Fatalf("known Downloads folder=%q error=%v", folder, err)
	}
}
