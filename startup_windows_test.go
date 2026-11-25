//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"golang.org/x/sys/windows/registry"
)

func TestStartAtLoginRegistration(t *testing.T) {
	// Never read or modify the user's real startup registration.
	path := fmt.Sprintf(`Software\Lunaria-Startup-Test-%d`, os.Getpid())
	if key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE); !errors.Is(err, registry.ErrNotExist) {
		if err == nil {
			key.Close()
		}
		t.Fatalf("test key already exists or cannot be checked: %v", err)
	}
	t.Cleanup(func() {
		if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil && !errors.Is(err, registry.ErrNotExist) {
			t.Errorf("remove test registry key: %v", err)
		}
	})
	check := func(want bool) {
		t.Helper()
		got, err := startAtLogin(path)
		if err != nil || got != want {
			t.Fatalf("startAtLogin = %v, %v; want %v", got, err, want)
		}
	}
	check(false)
	if err := setStartAtLogin(path, false, ""); err != nil {
		t.Fatal(err)
	}
	if key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE); !errors.Is(err, registry.ErrNotExist) {
		if err == nil {
			key.Close()
		}
		t.Fatal("reading or disabling missing registration created a key")
	}
	executable := `C:\My apps\花\Lunaria.exe`
	if err := setStartAtLogin(path, true, executable); err != nil {
		t.Fatal(err)
	}
	check(true)
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()
	command, kind, err := key.GetStringValue("Lunaria")
	if err != nil || kind != registry.SZ || command != `"`+executable+`"` {
		t.Fatalf("startup command = %q, type %d, error %v", command, kind, err)
	}
	if err := key.SetStringValue("Other app", "keep"); err != nil {
		t.Fatal(err)
	}
	if err := setStartAtLogin(path, true, `C:\Moved\Lunaria.exe`); err != nil {
		t.Fatal(err)
	}
	command, _, err = key.GetStringValue("Lunaria")
	if err != nil || command != `"C:\Moved\Lunaria.exe"` {
		t.Fatalf("updated command = %q, %v", command, err)
	}
	for range 2 {
		if err := setStartAtLogin(path, false, ""); err != nil {
			t.Fatal(err)
		}
		check(false)
	}
	if value, _, err := key.GetStringValue("Other app"); err != nil || value != "keep" {
		t.Fatalf("another app's value changed: %q, %v", value, err)
	}
	if startupDisabledInWindows(path) {
		t.Fatal("missing approval value reported as disabled")
	}
	for _, test := range []struct {
		data []byte
		want bool
	}{
		{[]byte{3, 0, 0, 0, 42, 42, 42, 42, 42, 42, 42, 42}, true},
		{[]byte{2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, false},
		{[]byte{3}, false},
		{make([]byte, 12), false},
		{[]byte{3, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, false},
		{make([]byte, 13), false},
	} {
		if err := key.SetBinaryValue("Lunaria", test.data); err != nil {
			t.Fatal(err)
		}
		if got := startupDisabledInWindows(path); got != test.want {
			t.Fatalf("disabled state for %v = %v; want %v", test.data, got, test.want)
		}
	}
	if err := key.SetStringValue("Lunaria", "disabled"); err != nil {
		t.Fatal(err)
	}
	if startupDisabledInWindows(path) {
		t.Fatal("string approval value reported as disabled")
	}
	invalidPath := path + "\x00invalid"
	if _, err := startAtLogin(invalidPath); err == nil {
		t.Fatal("invalid registration read did not report an error")
	}
	for _, enabled := range []bool{false, true} {
		if err := setStartAtLogin(invalidPath, enabled, executable); err == nil {
			t.Fatalf("invalid registration write (enabled=%v) did not report an error", enabled)
		}
	}
}

func TestStartupMenuItem(t *testing.T) {
	for _, test := range []struct {
		enabled, disabled bool
		err               error
		flags             uintptr
		label             string
	}{
		{false, false, nil, 0, "Start at login"},
		{false, true, nil, 0, "Start at login"},
		{true, false, nil, 8, "Start at login"},
		{true, true, nil, 8, "Start at login (Disabled in Windows)"},
		{false, false, registry.ErrUnexpectedType, 1, "Start at login (state unavailable)"},
	} {
		flags, label := startupMenuItem(test.enabled, test.err, test.disabled)
		if flags != test.flags || label != test.label {
			t.Fatalf("menu item = %#x, %q; want %#x, %q", flags, label, test.flags, test.label)
		}
	}
}
