// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package image4bit implements 4 bits per pixel 2D graphics.
//
// It is compatible with package image/draw.
//
// VerticalLSB is the only bit packing implemented as it is used by the
// ssd1327. Others would be VerticalMSB, HorizontalLSB and HorizontalMSB.
package image4bit

import (
	"image"
	"image/color"
	"image/draw"
	"strconv"
)

// Gray4 implements a 4 bits color.
type Gray4 uint8

// RGBA returns a grayscale result.
func (g Gray4) RGBA() (uint32, uint32, uint32, uint32) {
	i := 4369 * uint32(g)
	return i, i, i, 65535
}

func (g Gray4) String() string {
	return "Gray4(" + strconv.Itoa(int(g)) + ")"
}

// Gray4Model is the color Model for 4 bits color.
var Gray4Model = color.ModelFunc(convert)

// VerticalLSB is a 4 bits image.
//
// Each byte is 2 vertical pixels. Each stride is an horizontal band of 2
// pixels high with LSB first. So the first byte represent the following
// pixels, with lowest bit being the top left pixel.
//
//   0 x x x x x x x
//   1 x x x x x x x
//
// It is designed specifically to work with SSD1327 OLED display controler.
type VerticalLSB struct {
	// Pix holds the image's pixels, as vertically LSB-first packed bitmap. It
	// can be passed directly to ssd1327.Dev.Write()
	Pix []byte
	// Stride is the Pix stride (in bytes) between vertically adjacent 2 pixels
	// horizontal bands.
	Stride int
	// Rect is the image's bounds.
	Rect image.Rectangle
}

// NewVerticalLSB returns an initialized VerticalLSB instance.
func NewVerticalLSB(r image.Rectangle) *VerticalLSB {
	w := r.Dx()
	// Round down.
	minY := r.Min.Y &^ 1
	// Round up.
	maxY := (r.Max.Y + 1) & ^1
	bands := (maxY - minY) / 2
	return &VerticalLSB{Pix: make([]byte, w*bands), Stride: w, Rect: r}
}

// ColorModel implements image.Image.
func (i *VerticalLSB) ColorModel() color.Model {
	return Gray4Model
}

// Bounds implements image.Image.
func (i *VerticalLSB) Bounds() image.Rectangle {
	return i.Rect
}

// At implements image.Image.
func (i *VerticalLSB) At(x, y int) color.Color {
	return i.Gray4At(x, y)
}

// Gray4At is the optimized version of At().
func (i *VerticalLSB) Gray4At(x, y int) Gray4 {
	if !(image.Point{x, y}.In(i.Rect)) {
		return Gray4(0)
	}
	offset, o := i.PixOffset(x, y)
	return Gray4((i.Pix[offset] >> o) & 0xF)
}

// Opaque scans the entire image and reports whether it is fully opaque.
func (i *VerticalLSB) Opaque() bool {
	return true
}

// PixOffset returns the index of the first element of Pix that corresponds to
// the pixel at (x, y) and the offset.
func (i *VerticalLSB) PixOffset(x, y int) (int, byte) {
	// Adjust band.
	minY := i.Rect.Min.Y &^ 1
	pY := (y - minY)
	offset := pY/2*i.Stride + (x - i.Rect.Min.X)
	return offset, byte(pY&1) * 4
}

// Set implements draw.Image
func (i *VerticalLSB) Set(x, y int, c color.Color) {
	i.SetGray4(x, y, convertGray4(c))
}

// SetGray4 is the optimized version of Set().
func (i *VerticalLSB) SetGray4(x, y int, g Gray4) {
	if !(image.Point{x, y}.In(i.Rect)) {
		return
	}
	offset, o := i.PixOffset(x, y)
	i.Pix[offset] &^= 0xF << o
	i.Pix[offset] |= uint8(g) << o
}

//

var _ draw.Image = &VerticalLSB{}

// Anything not transparent and not pure black is white.
func convert(c color.Color) color.Color {
	return convertGray4(c)
}

// Anything not transparent and not pure black is white.
func convertGray4(c color.Color) Gray4 {
	switch t := c.(type) {
	case Gray4:
		return t
	default:
		r, g, b, _ := c.RGBA()
		// Use the same coefficients than color.GrayModel.
		y := (19595*r + 38470*g + 7471*b + 1<<15) >> 28
		return Gray4(y)
	}
}
