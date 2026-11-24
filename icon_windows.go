//go:build windows

package main

func makeTrayIcon() uintptr {
	instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	// The resource compiler assigns ID 1 to the manifest and ID 2 to the icon group.
	icon, _, _ := user32.NewProc("LoadImageW").Call(
		instance, 2, 1, uintptr(metric(49)), uintptr(metric(50)), 0,
	)
	return icon
}
