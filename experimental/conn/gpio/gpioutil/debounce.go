// Copyright 2018 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package gpioutil

import (
	"time"

	"periph.io/x/periph/conn/gpio"
)

// debounced is a gpio.PinIO where reading and edge detection pass through a
// debouncing algorithm.
type debounced struct {
	// Immutable.
	gpio.PinIO
	// denoise delays state changes. It waits for this amount before reporting it.
	denoise time.Duration
	// debounce locks on after a steady state change. Once a state change
	// happened, don't change again for this amount of time.
	debounce time.Duration

	// Mutable.
	// edge is the WaitForEdge edge it should return on.
	edge gpio.Edge
	// steady is the last steady value.
	steady gpio.Level
	// steadyChange is the time at which the last steady value was determined.
	// The returned level won't change until 'debounce' amount of time is passed.
	lastSteady time.Time
	// Last occurence of a glitch detection. Once 'denoise' amount of time is
	// passed, the steady value will change.
	lastGlitch time.Time
}

// Debounce returns a debounced gpio.PinIO from a gpio.PinIO source. Only the
// PinIn behavior is mutated.
//
// denoise is a noise filter, which waits a pin to be steady for this amount
// of time BEFORE reporting the new level.
//
// debounce will lock on a level for this amount of time AFTER the pin changed
// state, ignoring following state changes.
//
// Either value can be 0.
func Debounce(p gpio.PinIO, denoise, debounce time.Duration, edge gpio.Edge) (gpio.PinIO, error) {
	if denoise == 0 && debounce == 0 {
		return p, nil
	}
	if err := p.In(gpio.PullNoChange, gpio.BothEdges); err != nil {
		return nil, err
	}
	l := p.Read()
	t := now()
	return &debounced{
		// Immutable.
		PinIO:    p,
		denoise:  denoise,
		debounce: debounce,
		// Mutable.
		edge:       edge,
		steady:     l,
		lastSteady: t,
		lastGlitch: t,
	}, nil
}

// String implements gpio.PinIO.
func (d *debounced) String() string {
	return "Debounced{" + d.PinIO.String() + "}"
}

// In implements gpio.PinIO.
func (d *debounced) In(pull gpio.Pull, edge gpio.Edge) error {
	err := d.PinIO.In(pull, gpio.BothEdges)
	d.edge = edge
	// Read even if In() failed.
	d.steady = d.Read()
	// Reset the glitch and debounce detection algorithms.
	t := now()
	d.lastSteady = t
	d.lastGlitch = t
	return err
}

// Read implements gpio.PinIO.
//
// It is the smoothed out value from the underlying gpio.PinIO.
func (d *debounced) Read() gpio.Level {
	l := d.PinIO.Read()
	//log.Printf("%-4s n%s g%s s%s", l, n.Format("15:04:05"), d.lastGlitch.Format("15:04:05"), d.lastSteady.Format("15:04:05"))
	if l == d.steady {
		// The read value is the same as the previous steady reported value.
		if d.lastGlitch.After(d.lastSteady) {
			// There had been a glitch but it didn't last more than d.denoise, zap its
			// detection.
			d.lastGlitch = d.lastSteady
		}
		// Was in steady state, no glitch. Fast path.
		return d.steady
	}

	// State change. Either denoise or debounce filter.
	n := now()
	delta := d.lastGlitch.Sub(d.lastSteady)
	if delta >= 0 {
		// denoise filtering mode.
		if delta >= d.denoise {
			// It has been long enough, signal state change. Filtering will now be
			// debounce.
			d.steady = l
			d.lastSteady = n
		} else {
			d.lastGlitch = n
		}
		return d.steady
	}

	// debounce filtering mode.
	delta = -delta
	if delta >= d.debounce {
		// It has been long enough, signal state change. Filtering will now be
		// debounce.
		d.steady = l
		d.lastSteady = n
	}
	return d.steady
}

// WaitForEdge implements gpio.PinIO.
//
// It is the smoothed out value from the underlying gpio.PinIO.
func (d *debounced) WaitForEdge(timeout time.Duration) bool {
	// TODO(maruel): Actual algorithm: Use time based debouncing.
	//start := now()
	if !d.PinIO.WaitForEdge(timeout) {
		return false
	}
	if d.edge != gpio.BothEdges {
		// Ignore wrong side edge.
	}
	l := d.PinIO.Read()
	// It could have detected an edge that the user doesn't care about.
	d.steady = l
	n := now()
	d.lastSteady = n
	d.lastGlitch = n
	return true
}

// Real implements gpio.RealPin.
func (d *debounced) Real() gpio.PinIO {
	if r, ok := d.PinIO.(gpio.RealPin); ok {
		return r.Real()
	}
	return d.PinIO
}

// isDenoising returns the amount of time left where this filter is still in
// effect, if it is.
func (d *debounced) isDenoising() time.Duration {
	delta := d.lastGlitch.Sub(d.lastSteady)
	if delta >= 0 && delta < d.denoise {
		return d.denoise - delta
	}
	return 0
}

// isDebouncing returns the amount of time left where this filter is still in
// effect, if it is.
func (d *debounced) isDebouncing() time.Duration {
	delta := d.lastSteady.Sub(d.lastGlitch)
	if delta >= 0 && delta < d.debounce {
		return d.debounce - delta
	}
	return 0
}

var now = time.Now
var _ gpio.PinIO = &debounced{}
