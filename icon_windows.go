//go:build windows

package main

import "unsafe"

func makeTrayIcon() uintptr {
	var mask [128]byte
	var pixels [32 * 32 * 4]byte
	for i := range mask {
		mask[i] = 255
	}
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			dx, dy := float64(x)-15.5, float64(y)-15.5
			if dx*dx+dy*dy > 15*15 {
				continue
			}
			mask[y*4+x/8] &= ^(byte(0x80) >> uint(x%8))
			r, g, b := byte(100), byte(91), byte(191)
			mx, my := float64(x)-14, float64(y)-16
			cutX, cutY := float64(x)-19, float64(y)-12
			if mx*mx+my*my < 10*10 && cutX*cutX+cutY*cutY > 9*9 {
				r, g, b = 249, 240, 207
			}
			i := (y*32 + x) * 4
			pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = b, g, r, 255
		}
	}
	icon, _, _ := user32.NewProc("CreateIcon").Call(0, 32, 32, 1, 32, uintptr(unsafe.Pointer(&mask[0])), uintptr(unsafe.Pointer(&pixels[0])))
	return icon
}
