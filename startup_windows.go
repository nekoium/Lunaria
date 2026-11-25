//go:build windows

package main

import (
	"encoding/binary"
	"errors"
	"os"

	"golang.org/x/sys/windows/registry"
)

const startupRunKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const startupApprovedKey = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`

func startAtLogin(path string) (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer key.Close()
	_, _, err = key.GetValue("Lunaria", nil)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func setStartAtLogin(path string, enabled bool, executable string) error {
	if enabled {
		key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer key.Close()
		// Quote even paths without spaces so Windows always parses the full executable name.
		return key.SetStringValue("Lunaria", `"`+executable+`"`)
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer key.Close()
	err = key.DeleteValue("Lunaria")
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	return err
}

func startupDisabledInWindows(path string) bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	data, _, err := key.GetBinaryValue("Lunaria")
	// shortcut: StartupApproved is undocumented; revisit if Windows changes the known disabled record.
	return err == nil && len(data) == 12 && binary.LittleEndian.Uint32(data[:4]) == 3
}

func startupMenuItem(enabled bool, err error, disabled bool) (uintptr, string) {
	if err != nil {
		return 1, "Start at login (state unavailable)" // MF_GRAYED
	}
	if enabled {
		if disabled {
			return 8, "Start at login (Disabled in Windows)"
		}
		return 8, "Start at login" // MF_CHECKED
	}
	return 0, "Start at login"
}

func toggleStartAtLogin() error {
	enabled, err := startAtLogin(startupRunKey)
	if err != nil {
		return err
	}
	var executable string
	if !enabled {
		executable, err = os.Executable()
		if err != nil {
			return err
		}
	}
	return setStartAtLogin(startupRunKey, !enabled, executable)
}
