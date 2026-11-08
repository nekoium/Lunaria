//go:build windows

package main

import (
	"math"
	"testing"
	"time"
)

func TestOverlayFade(t *testing.T) {
	for _, tc := range []struct {
		name              string
		from, target      float64
		elapsed, duration time.Duration
		want              float64
	}{
		{"opening start", 0, 1, 0, 150 * time.Millisecond, 0},
		{"opening midpoint", 0, 1, 75 * time.Millisecond, 150 * time.Millisecond, .875},
		{"opening end", 0, 1, 150 * time.Millisecond, 150 * time.Millisecond, 1},
		{"closing midpoint", 1, 0, 45 * time.Millisecond, 90 * time.Millisecond, .125},
		{"closing end", 1, 0, time.Second, 90 * time.Millisecond, 0},
		{"before start", 0, 1, -time.Second, 150 * time.Millisecond, 0},
		{"reverse to close", .6, 0, 0, 90 * time.Millisecond, .6},
		{"reverse to open", .2, 1, 0, 150 * time.Millisecond, .2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := overlayFade(tc.from, tc.target, tc.elapsed, tc.duration); math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("fade = %v; want %v", got, tc.want)
			}
		})
	}
}

func TestEditFadeIdle(t *testing.T) {
	a := &application{}
	if got := a.editFade(); got != 1 {
		t.Fatalf("idle opening = %v", got)
	}
	a.editClosing = true
	if got := a.editFade(); got != 0 {
		t.Fatalf("idle closing = %v", got)
	}
}
