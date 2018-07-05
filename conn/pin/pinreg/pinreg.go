// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package pinreg

import (
	"errors"
	"strconv"

	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"periph.io/x/periph/conn/pin"
	"periph.io/x/periph/conn/pin/pinreg/internal"
)

// All contains all the on-board headers on a micro computer.
//
// The map key is the header name, e.g. "P1" or "EULER" and the value is a
// slice of slice of pin.Pin. For a 2x20 header, it's going to be a slice of
// [20][2]pin.Pin.
func All() map[string][][]pin.Pin {
	internal.Mu.Lock()
	defer internal.Mu.Unlock()
	out := make(map[string][][]pin.Pin, len(internal.AllHeaders))
	for k, v := range internal.AllHeaders {
		outV := make([][]pin.Pin, len(v))
		for i, w := range v {
			outW := make([]pin.Pin, len(w))
			copy(outW, w)
			outV[i] = outW
		}
		out[k] = outV
	}
	return out
}

// Position returns the position on a pin if found.
//
// The header and the pin number. Pin numbers are 1-based.
//
// Returns "", 0 if not connected.
func Position(p pin.Pin) (string, int) {
	internal.Mu.Lock()
	defer internal.Mu.Unlock()
	pos := internal.ByPin[realPin(p).Name()]
	return pos.name, pos.number
}

// IsConnected returns true if the pin is on a header.
func IsConnected(p pin.Pin) bool {
	_, i := Position(p)
	return i != 0
}

// Register registers a physical header.
//
// It automatically registers all gpio pins to gpioreg.
func Register(name string, allPins [][]pin.Pin) error {
	internal.Mu.Lock()
	defer internal.Mu.Unlock()
	if _, ok := internal.AllHeaders[name]; ok {
		return errors.New("pinreg: header " + strconv.Quote(name) + " was already registered")
	}
	for i, line := range allPins {
		for j, pin := range line {
			if pin == nil || len(pin.Name()) == 0 {
				return errors.New("pinreg: invalid pin on header " + name + "[" + strconv.Itoa(i+1) + "][" + strconv.Itoa(j+1) + "]")
			}
		}
	}
	internal.AllHeaders[name] = allPins
	number := 1
	for _, line := range allPins {
		for _, p := range line {
			internal.ByPin[realPin(p).Name()] = position{name, number}
			number++
		}
	}

	count := 0
	for _, row := range allPins {
		for _, p := range row {
			count++
			if _, ok := p.(gpio.PinIO); ok {
				if err := gpioreg.RegisterAlias(name+"_"+strconv.Itoa(count), p.Name()); err != nil {
					// Unregister as much as possible.
					_ = unregister(name)
					return errors.New("pinreg: " + err.Error())
				}
			}
		}
	}

	return nil
}

// Unregister removes a previously registered header.
//
// This can happen when an USB device, which exposed an header, is unplugged.
func Unregister(name string) error {
	mu.Lock()
	defer mu.Unlock()
	return unregister(name)
}

//

func unregister(name string) error {
	if hdr, ok := internal.AllHeaders[name]; ok {
		delete(internal.AllHeaders, name)
		count := 0
		for _, row := range hdr {
			for _, p := range row {
				count++
				if _, ok := p.(gpio.PinIO); ok {
					if err := gpioreg.Unregister(name + "_" + strconv.Itoa(count)); err != nil {
						return errors.New("pinreg: " + err.Error())
					}
				}
			}
		}
		return nil
	}
	return errors.New("pinreg: can't unregister unknown header name " + strconv.Quote(name))
}

// realPin returns the real pin from an alias.
func realPin(p pin.Pin) pin.Pin {
	if r, ok := p.(gpio.RealPin); ok {
		p = r.Real()
	}
	return p
}
