//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"

	"golang.org/x/image/riff"
	"golang.org/x/image/webp"
)

// The standard WebP decoder handles still images; extract only the first animation frame.
func decodeWebPFirstFrame(r io.Reader, cfg image.Config) (image.Image, error) {
	form, chunks, err := riff.NewReader(r)
	if err != nil {
		return nil, err
	}
	if form != (riff.FourCC{'W', 'E', 'B', 'P'}) {
		return nil, fmt.Errorf("invalid WebP container")
	}
	var background color.NRGBA
	for {
		id, size, chunk, err := chunks.Next()
		if err != nil {
			return nil, fmt.Errorf("read WebP animation: %w", err)
		}
		switch id {
		case riff.FourCC{'A', 'N', 'I', 'M'}:
			var anim [6]byte
			if size != 6 {
				return nil, fmt.Errorf("invalid WebP animation header")
			}
			if _, err := io.ReadFull(chunk, anim[:]); err != nil {
				return nil, err
			}
			background = color.NRGBA{R: anim[2], G: anim[1], B: anim[0], A: anim[3]}
		case riff.FourCC{'A', 'N', 'M', 'F'}:
			var frame [16]byte
			if size < 24 {
				return nil, fmt.Errorf("invalid WebP frame")
			}
			if _, err := io.ReadFull(chunk, frame[:]); err != nil {
				return nil, err
			}
			uint24 := func(b []byte) int { return int(b[0]) | int(b[1])<<8 | int(b[2])<<16 }
			x, y := 2*uint24(frame[:3]), 2*uint24(frame[3:6])
			w, h := 1+uint24(frame[6:9]), 1+uint24(frame[9:12])
			if x+w > cfg.Width || y+h > cfg.Height {
				return nil, fmt.Errorf("WebP frame exceeds canvas")
			}
			payload, err := io.ReadAll(chunk)
			if err != nil {
				return nil, err
			}
			// Wrap the frame's ALPH/VP8/VP8L chunks as a standalone extended WebP.
			header := make([]byte, 30)
			copy(header, "RIFF")
			binary.LittleEndian.PutUint32(header[4:], uint32(22+len(payload)))
			copy(header[8:], "WEBPVP8X")
			binary.LittleEndian.PutUint32(header[16:], 10)
			if bytes.HasPrefix(payload, []byte("ALPH")) {
				header[20] = 0x10
			}
			copy(header[24:27], frame[6:9])
			copy(header[27:30], frame[9:12])
			data := append(header, payload...)
			frameCfg, err := webp.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				return nil, err
			}
			if frameCfg.Width != w || frameCfg.Height != h {
				return nil, fmt.Errorf("WebP frame dimensions disagree")
			}
			decoded, err := webp.Decode(bytes.NewReader(data))
			if err != nil {
				return nil, err
			}
			canvas := image.NewNRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
			draw.Draw(canvas, canvas.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)
			op := draw.Over
			if frame[15]&2 != 0 {
				op = draw.Src
			}
			draw.Draw(canvas, image.Rect(x, y, x+w, y+h), decoded, decoded.Bounds().Min, op)
			return canvas, nil
		}
	}
}
