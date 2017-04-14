// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package dma defines a way to control a DMA controller to control a GPIO pin
// as output.
package dma

import (
	"errors"
	"io"
	"sync"

	"github.com/google/periph/conn/gpio"
)

// Mem represents a section of memory that is usable by the DMA controller.
//
// Since this is physically allocated memory, that could potentially have been
// allocated in spite of OS consent, it is important to call Close() before
// process exit.
type Mem interface {
	io.Closer
	Buf() []byte
	// PhysAddr is the physical address. It can be either 32 bits or 64 bits,
	// depending on the OS, not on the user mode build.
	PhysAddr() uint64
}

// Controller defines the interface a concrete DMA driver must implement.
type Controller interface {
	// Alloc allocate physical contiguous memory for use with the DMA controller.
	// If not enough contiguous physical pages are available, it returns
	// scathered blocks totalizing `size`.
	//
	// It is *very* important to call Close() on every memory chunk returned.
	Alloc(size int) ([]Mem, error)
	// Output streams the data at the specified to the gpio pin.
	Output(speed int64, p gpio.PinOut, blocks []Mem) error
}

// Get returns the DMA controller, if a device driver was loaded.
func Get() (Controller, error) {
	mu.Lock()
	defer mu.Unlock()
	if controller == nil {
		return nil, errors.New("dma: no DMA controller found")
	}
	return controller, nil
}

// Register registers a DMA controller.
//
// This should be done by a CPU driver and it is expected that there is only a
// single DMA controller on the host. Eventually multiple controllers could be
// registered, for example in the case of GPU based DMA writes.
func Register(c Controller) error {
	mu.Lock()
	defer mu.Unlock()
	if controller != nil {
		return errors.New("dma: support for multiple DMA controllers is not yet implemented")
	}
	controller = c
	return nil
}

var (
	mu         sync.Mutex
	controller Controller
)

// Notes:
//
// - Use (gpu controller under Mali) on Allwinner CPUs: uses a
//   yet-to-be-determined way to ask the GPU to allocate physical continuous
//   memory on our behalf.
// https://community.arm.com/thread/9972
// http://infocenter.arm.com/help/index.jsp?topic=/com.arm.doc.100614_0300_00_en/ada1432742777004.html
//
// - Videobuf2? videobuf2_memops is loaded on both Armbian and Raspbian. Not
//   sure how to access (if possible) from user mode.
// http://kernel.readthedocs.io/en/latest/media/kapi/v4l2-videobuf2.html
// https://lwn.net/Articles/447435/
//
// - OpenGLES ? Not available out of the box nd quite high level.
//
// - OpenCL ? Not available out of the box and quite high level.
