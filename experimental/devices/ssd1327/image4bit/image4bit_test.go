// Copyright 2018 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package image4bit

import (
	"image"
	"image/color"
	"testing"
)

func TestGray4(t *testing.T) {
	if r, g, b, a := Gray4(15).RGBA(); r != 65535 || g != r || b != r || a != r {
		t.Fatal(r, g, b, a)
	}
	if r, g, b, a := Gray4(0).RGBA(); r != 0 || g != r || b != r || a != 65535 {
		t.Fatal(r, g, b, a)
	}
	if Gray4(15).String() != "Gray4(15)" || Gray4(0).String() != "Gray4(0)" {
		t.Fail()
	}
	if Gray4(7) != convertGray4(Gray4(7)) {
		t.Fail()
	}
}

func TestVerticalLSB_NewVerticalLSB(t *testing.T) {
	data := []struct {
		r      image.Rectangle
		l      int
		stride int
	}{
		// Empty.
		{
			image.Rect(0, 0, 0, 1),
			0,
			0,
		},
		// Empty.
		{
			image.Rect(0, 0, 1, 0),
			0,
			1,
		},
		// 1 horizontal band of 1px high, 1px wide.
		{
			image.Rect(0, 0, 1, 1),
			1,
			1,
		},
		{
			image.Rect(0, 1, 1, 2),
			1,
			1,
		},
		// 1 horizontal band of 8px high, 1px wide.
		{
			image.Rect(0, 0, 1, 8),
			1,
			1,
		},
		// 1 horizontal band of 1px high, 9px wide.
		{
			image.Rect(0, 0, 9, 1),
			9,
			9,
		},
		// 2 horizontal bands of 1px high, 1px wide.
		{
			image.Rect(0, 0, 1, 9),
			2,
			1,
		},
		// 2 horizontal bands, 1px wide.
		{
			image.Rect(0, 1, 1, 9),
			2,
			1,
		},
		// 2 horizontal bands, 1px wide.
		{
			image.Rect(0, 7, 1, 9),
			2,
			1,
		},
		// 2 horizontal bands, 1px wide.
		{
			image.Rect(0, 7, 1, 16),
			2,
			1,
		},
		// 3 horizontal bands, 1px wide.
		{
			image.Rect(0, 7, 1, 17),
			3,
			1,
		},
		// 3 horizontal bands, 1px wide.
		{
			image.Rect(0, 7, 1, 17),
			3,
			1,
		},
		// 3 horizontal bands, 9px wide.
		{
			image.Rect(0, 7, 9, 17),
			3 * 9,
			9,
		},
		// Negative X.
		{
			image.Rect(-1, 0, 0, 1),
			1,
			1,
		},
		// Negative Y.
		{
			image.Rect(0, -1, 1, 0),
			1,
			1,
		},
		{
			image.Rect(0, -1, 1, 1),
			2,
			1,
		},
	}
	for i, line := range data {
		img := NewVerticalLSB(line.r)
		if r := img.Bounds(); r != line.r {
			t.Fatalf("#%d: expected %v; actual %v", i, line.r, r)
		}
		if l := len(img.Pix); l != line.l {
			t.Fatalf("#%d: len(img.Pix) expected %v; actual %v for %v", i, line.l, l, line.r)
		}
		if img.Stride != line.stride {
			t.Fatalf("#%d: img.Stride expected %v; actual %v for %v", i, line.stride, img.Stride, line.r)
		}
	}
}

func TestVerticalLSB_At(t *testing.T) {
	img := NewVerticalLSB(image.Rect(0, 0, 1, 1))
	img.SetGray4(0, 0, Gray4(15))
	c := img.At(0, 0)
	if g, ok := c.(Gray4); !ok || g != Gray4(15) {
		t.Fatal(c, g)
	}
	c = img.At(0, 1)
	if g, ok := c.(Gray4); !ok || g != Gray4(0) {
		t.Fatal(c, g)
	}
}

func TestVerticalLSB_BitAt(t *testing.T) {
	img := NewVerticalLSB(image.Rect(0, 0, 1, 1))
	img.SetGray4(0, 0, Gray4(15))
	if g := img.Gray4At(0, 0); g != Gray4(15) {
		t.Fatal(g)
	}
	if g := img.Gray4At(0, 1); g != Gray4(0) {
		t.Fatal(g)
	}
}

func TestVerticalLSB_ColorModel(t *testing.T) {
	img := NewVerticalLSB(image.Rect(0, 0, 1, 8))
	if v := img.ColorModel(); v != Gray4Model {
		t.Fatalf("%s", v)
	}
	if v := img.ColorModel().Convert(color.NRGBA{0x80, 0x80, 0x80, 0xFF}).(Gray4); v != Gray4(8) {
		t.Fatalf("%s", v)
	}
	if v := img.ColorModel().Convert(color.NRGBA{0x7F, 0x7F, 0x7F, 0xFF}).(Gray4); v != Gray4(7) {
		t.Fatalf("%s", v)
	}
}

func TestVerticalLSB_Opaque(t *testing.T) {
	if !NewVerticalLSB(image.Rect(0, 0, 1, 8)).Opaque() {
		t.Fatal("image is always opaque")
	}
}

func TestVerticalLSB_PixOffset(t *testing.T) {
	data := []struct {
		r      image.Rectangle
		x, y   int
		offset int
		mask   byte
	}{
		{
			image.Rect(0, 0, 1, 1),
			0, 0,
			0, 0x01,
		},
		{
			image.Rect(0, 0, 1, 8),
			0, 1,
			0, 0x02,
		},
		{
			image.Rect(0, 0, 3, 16),
			1, 5,
			1, 0x20,
		},
		{
			image.Rect(-1, -1, 3, 16),
			1, 5,
			6, 0x20,
		},
	}
	for i, line := range data {
		img := NewVerticalLSB(line.r)
		offset, mask := img.PixOffset(line.x, line.y)
		if offset != line.offset || mask != line.mask {
			t.Fatalf("#%d: expected offset:%v, mask:0x%02X; actual offset:%v, mask:0x%02X", i, line.offset, line.mask, offset, mask)
		}
	}
}

func TestVerticalLSB_SetBit1x1(t *testing.T) {
	img := NewVerticalLSB(image.Rect(0, 0, 1, 1))
	if img.Pix[0] != 0 {
		t.Fatal(img.Pix)
	}
	if img.SetGray4(0, 1, Gray4(15)); img.Pix[0] != 0 {
		t.Fatal(img.Pix)
	}
	if img.SetGray4(0, 0, Gray4(15)); img.Pix[0] != 0xF {
		t.Fatal(img.Pix)
	}
	if img.SetGray4(0, 0, Gray4(0)); img.Pix[0] != 0 {
		t.Fatal(img.Pix)
	}
}

func TestVerticalLSB_SetBit1x2(t *testing.T) {
	img := NewVerticalLSB(image.Rect(0, 0, 1, 2))
	if img.Pix[0] != 0 {
		t.Fatal(img.Pix)
	}
	if img.SetGray4(0, 1, Gray4(15)); img.Pix[0] != 0xF0 {
		t.Fatal(img.Pix)
	}
	if img.SetGray4(0, 0, Gray4(15)); img.Pix[0] != 0xFF {
		t.Fatal(img.Pix)
	}
	if img.SetGray4(0, 1, Gray4(0)); img.Pix[0] != 0x0F {
		t.Fatal(img.Pix)
	}
}

func TestVerticalLSB_Set(t *testing.T) {
	img := NewVerticalLSB(image.Rect(0, 0, 1, 4))
	img.Set(0, 0, color.NRGBA{0x80, 0x80, 0x80, 0xFF})
	img.Set(0, 1, color.NRGBA{0x7F, 0x80, 0x80, 0xFF})
	img.Set(0, 2, color.NRGBA{0x7F, 0x7F, 0x80, 0xFF})
	img.Set(0, 3, color.NRGBA{0x7F, 0x7F, 0x7F, 0xFF})
	img.Set(0, 4, color.NRGBA{0x80, 0x80, 0x80, 0x7F})
	if img.Pix[0] != 0x88 || img.Pix[1] != 0x77 {
		t.Fatal(img.Pix)
	}
}
