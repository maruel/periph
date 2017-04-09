// Copyright 2017 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package spics implements a wrapper to work with a SPI bus while manually
// managing the CS line.
package spics

import (
	"errors"

	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/spi"
)

// Conn is the original spi.ConnCloser reference.
type Conn interface {
	spi.ConnCloser
	spi.Pins
}

// New overrides a spi.ConnCloser with a manually managed CS line.
func New(c spi.ConnCloser) (*ConnGPIO, error) {
	pins, ok := c.(spi.Pins)
	if !ok {
		return nil, errors.New("spics: SPI Bus must implement spi.Pins")
	}
	cs := pins.CS()
	if cs == gpio.INVALID {
		return nil, errors.New("spics: CS line must be known")
	}
	return &ConnGPIO{Conn: c.(Conn), cs: cs}, nil
}

// ConnGPIO is a SPI ConnCloser that uses an arbitrary GPIO pin as the chip
// select line.
type ConnGPIO struct {
	Conn
	cs     gpio.PinOut
	active gpio.Level
}

// DevParams implements spi.Conn.
func (c *ConnGPIO) DevParams(maxHz int64, mode spi.Mode, bits int) error {
	mode |= spi.NoCS
	if err := c.Conn.DevParams(maxHz, mode, bits); err != nil {
		return err
	}
	c.active = gpio.Level(mode&spi.Mode2 == 0)
	return c.cs.Out(!c.active)
}

// Tx implements spi.Conn.
func (c *ConnGPIO) Tx(w, r []byte) error {
	if err := c.cs.Out(c.active); err != nil {
		return err
	}
	// Nanospin(10µs) ?
	defer c.cs.Out(!c.active)
	return c.Conn.Tx(w, r)
}

// TxPackets implements spi.ConnCloser.
func (c *ConnGPIO) TxPackets(p []spi.Packet) error {
	// Do one packet at a time.
	if err := c.cs.Out(c.active); err != nil {
		return err
	}
	// Nanospin(10µs) ?
	defer c.cs.Out(!c.active)
	return c.Conn.TxPackets(p)
}

//

var _ spi.ConnCloser = &ConnGPIO{}
var _ spi.Pins = &ConnGPIO{}
