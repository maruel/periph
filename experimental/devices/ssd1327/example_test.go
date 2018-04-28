// Copyright 2018 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package ssd1327_test

import (
	"image"
	"log"

	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/experimental/devices/ssd1327"
	"periph.io/x/periph/experimental/devices/ssd1327/image4bit"
	"periph.io/x/periph/host"
)

func Example() {
	// Make sure periph is initialized.
	if _, err := host.Init(); err != nil {
		log.Fatal(err)
	}

	// Use i2creg I²C bus registry to find the first available I²C bus.
	b, err := i2creg.Open("")
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()

	dev, err := ssd1327.NewI2C(b, 128, 64, false)
	if err != nil {
		log.Fatalf("failed to initialize ssd1327: %v", err)
	}

	// Draw on it.
	img := image4bit.NewVerticalLSB(dev.Bounds())
	// Note: this code is commented out so periph does not depend on:
	//    "golang.org/x/image/font"
	//    "golang.org/x/image/font/basicfont"
	//    "golang.org/x/image/math/fixed"
	//
	// f := basicfont.Face7x13
	// drawer := font.Drawer{
	// 	Dst:  img,
	// 	Src:  &image.Uniform{image4bit.Gray4(15)},
	// 	Face: f,
	// 	Dot:  fixed.P(0, img.Bounds().Dy()-1-f.Descent),
	// }
	// drawer.DrawString("Hello from periph!")
	dev.Draw(dev.Bounds(), img, image.Point{})
	if err := dev.Err(); err != nil {
		log.Fatal(err)
	}
}
