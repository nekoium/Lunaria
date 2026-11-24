// Generate Windows icon sizes from the approved overlapping-pods artwork.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"

	"golang.org/x/image/draw"
)

func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate() error {
	file, err := os.Open("assets/lunaria.png")
	if err != nil {
		return err
	}
	defer file.Close()
	source, err := png.Decode(file)
	if err != nil {
		return err
	}
	sizes := []int{16, 20, 24, 32, 40, 48, 64, 128, 256}
	var header bytes.Buffer
	binary.Write(&header, binary.LittleEndian, [3]uint16{0, 1, uint16(len(sizes))})
	var payload bytes.Buffer
	for _, size := range sizes {
		scaled := image.NewNRGBA(image.Rect(0, 0, size, size))
		draw.CatmullRom.Scale(scaled, scaled.Bounds(), source, source.Bounds(), draw.Src, nil)
		var frame bytes.Buffer
		if err := png.Encode(&frame, scaled); err != nil {
			return err
		}
		// ICO encodes 256 pixels as zero in its one-byte dimensions.
		header.Write([]byte{byte(size), byte(size), 0, 0})
		binary.Write(&header, binary.LittleEndian, [2]uint16{1, 32})
		binary.Write(&header, binary.LittleEndian, uint32(frame.Len()))
		binary.Write(&header, binary.LittleEndian, uint32(6+16*len(sizes)+payload.Len()))
		payload.Write(frame.Bytes())
	}
	return os.WriteFile("assets/lunaria.ico", append(header.Bytes(), payload.Bytes()...), 0644)
}
