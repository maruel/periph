// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package gpiotest is meant to be used to test drivers using fake Pins.
package gpiotest

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/conn/pin"
)

// Pin implements gpio.PinIO.
//
// Modify its members to simulate hardware events.
type Pin struct {
	// These should be immutable.
	N   string
	Num int
	Fn  string // TODO(maruel): pin.Func in v4.

	// Grab the Mutex before accessing the following members.
	sync.Mutex
	L         gpio.Level // Used for both input and output
	P         gpio.Pull
	EdgesChan chan gpio.Level  // Use it to fake edges
	D         gpio.Duty        // PWM duty
	F         physic.Frequency // PWM period
}

// String implements conn.Resource.
func (p *Pin) String() string {
	return fmt.Sprintf("%s(%d)", p.N, p.Num)
}

// Halt implements conn.Resource.
//
// It has no effect.
func (p *Pin) Halt() error {
	return nil
}

// Name implements pin.Pin.
func (p *Pin) Name() string {
	return p.N
}

// Number implements pin.Pin.
func (p *Pin) Number() int {
	return p.Num
}

// Func implements pin.Pin.
func (p *Pin) Func() pin.Func {
	return pin.Func(p.Fn)
}

// SupportedFuncs implements pin.Pin.
func (p *Pin) SupportedFuncs() []pin.Func {
	return []pin.Func{gpio.IN, gpio.OUT}
}

// SetFunc implements pin.Pin.
func (p *Pin) SetFunc(f pin.Func) error {
	return errors.New("gpiotest: not supported")
}

// In implements gpio.PinIn.
func (p *Pin) In(pull gpio.Pull) error {
	p.Lock()
	defer p.Unlock()
	p.P = pull
	if pull == gpio.PullDown {
		p.L = gpio.Low
	} else if pull == gpio.PullUp {
		p.L = gpio.High
	}
	/*
		if edge != gpio.NoEdge && p.EdgesChan == nil {
			return errors.New("gpiotest: please set p.EdgesChan first")
		}
	*/
	// Flush any buffered edges.
	for {
		select {
		case <-p.EdgesChan:
		default:
			return nil
		}
	}
}

// Read implements gpio.PinIn.
func (p *Pin) Read() gpio.Level {
	p.Lock()
	defer p.Unlock()
	return p.L
}

// Edges implements gpio.PinIn.
func (p *Pin) Edges(ctx context.Context, e gpio.Edge, c chan<- gpio.EdgeSample) {
	/*
		if timeout == -1 {
			_ = p.Out(<-p.EdgesChan)
			return true
		}
		select {
		case <-time.After(timeout):
			return false
		case l := <-p.EdgesChan:
			_ = p.Out(l)
			return true
		}
	*/
	<-ctx.Done()
}

// Pull implements gpio.PinIn.
func (p *Pin) Pull() gpio.Pull {
	return p.P
}

// DefaultPull implements gpio.PinIn.
func (p *Pin) DefaultPull() gpio.Pull {
	return p.P
}

// Out implements gpio.PinOut.
func (p *Pin) Out(l gpio.Level) error {
	p.Lock()
	defer p.Unlock()
	p.L = l
	return nil
}

// PWM implements gpio.PinOut.
func (p *Pin) PWM(ctx context.Context, duty gpio.Duty, f physic.Frequency) error {
	p.Lock()
	defer p.Unlock()
	p.D = duty
	p.F = f
	<-ctx.Done()
	return nil
}

// LogPinIO logs when its state changes.
type LogPinIO struct {
	gpio.PinIO
}

// Real implements gpio.RealPin.
func (p *LogPinIO) Real() gpio.PinIO {
	return p.PinIO
}

// In implements gpio.PinIn.
func (p *LogPinIO) In(pull gpio.Pull) error {
	log.Printf("%s.In(%s)", p, pull)
	return p.PinIO.In(pull)
}

// Read implements gpio.PinIn.
func (p *LogPinIO) Read() gpio.Level {
	l := p.PinIO.Read()
	log.Printf("%s.Read() %s", p, l)
	return l
}

// Edges implements gpio.PinIn.
func (p *LogPinIO) Edges(ctx context.Context, e gpio.Edge, c chan<- gpio.EdgeSample) {
	log.Printf("%s.Edges(%s)", p, e)
	p.PinIO.Edges(ctx, e, c)
}

// Out implements gpio.PinOut.
func (p *LogPinIO) Out(l gpio.Level) error {
	log.Printf("%s.Out(%s)", p, l)
	return p.PinIO.Out(l)
}

// PWM implements gpio.PinOut.
func (p *LogPinIO) PWM(ctx context.Context, duty gpio.Duty, f physic.Frequency) error {
	log.Printf("%s.PWM(%v, %s, %s)", p, ctx, duty, f)
	return p.PinIO.PWM(ctx, duty, f)
}

var _ gpio.PinIO = &Pin{}
