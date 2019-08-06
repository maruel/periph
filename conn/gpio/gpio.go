// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package gpio defines digital pins.
//
// All GPIO implementations are expected to implement PinIO but the device
// driver may accept a more specific one like PinIn or PinOut.
package gpio

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/conn/pin"
)

// Interfaces

// Level is the level of the pin: Low or High.
type Level bool

const (
	// Low represents 0v.
	Low Level = false
	// High represents Vin, generally 3.3v or 5v.
	High Level = true
)

func (l Level) String() string {
	if l == Low {
		return "Low"
	}
	return "High"
}

// Pull specifies the internal pull-up or pull-down for a pin set as input.
type Pull uint8

// Acceptable pull values.
const (
	PullNoChange Pull = 0 // Do not change the previous pull resistor setting or an unknown value
	Float        Pull = 1 // Let the input float
	PullDown     Pull = 2 // Apply pull-down
	PullUp       Pull = 3 // Apply pull-up
)

const pullName = "PullNoChangeFloatPullDownPullUp"

var pullIndex = [...]uint8{0, 12, 17, 25, 31}

func (i Pull) String() string {
	if i >= Pull(len(pullIndex)-1) {
		return "Pull(" + strconv.Itoa(int(i)) + ")"
	}
	return pullName[pullIndex[i]:pullIndex[i+1]]
}

// Edge specifies if an input pin should have edge detection enabled.
//
// Only enable it when needed, since this causes system interrupts.
type Edge int

// Acceptable edge detection values.
const (
	NoEdge      Edge = 0
	RisingEdge  Edge = 1
	FallingEdge Edge = 2
	BothEdges   Edge = 3
)

const edgeName = "NoEdgeRisingEdgeFallingEdgeBothEdges"

var edgeIndex = [...]uint8{0, 6, 16, 27, 36}

func (i Edge) String() string {
	if i >= Edge(len(edgeIndex)-1) {
		return "Edge(" + strconv.Itoa(int(i)) + ")"
	}
	return edgeName[edgeIndex[i]:edgeIndex[i+1]]
}

const (
	// DutyMax is a duty cycle of 100%.
	DutyMax Duty = 1 << 24
	// DutyHalf is a 50% duty PWM, which boils down to a normal clock.
	DutyHalf Duty = DutyMax / 2
)

// Duty is the duty cycle for a PWM.
//
// Valid values are between 0 and DutyMax.
type Duty int32

func (d Duty) String() string {
	// TODO(maruel): Implement one fractional number.
	return strconv.Itoa(int((d+50)/(DutyMax/100))) + "%"
}

// Valid returns true if the Duty cycle value is valid.
func (d Duty) Valid() bool {
	return d >= 0 && d <= DutyMax
}

// ParseDuty parses a string and converts it to a Duty value.
func ParseDuty(s string) (Duty, error) {
	percent := strings.HasSuffix(s, "%")
	if percent {
		s = s[:len(s)-1]
	}
	i64, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, err
	}
	i := Duty(i64)
	if percent {
		// TODO(maruel): Add support for fractional number.
		if i < 0 {
			return 0, errors.New("duty must be >= 0%")
		}
		if i > 100 {
			return 0, errors.New("duty must be <= 100%")
		}
		return ((i * DutyMax) + 49) / 100, nil
	}
	if i < 0 {
		return 0, errors.New("duty must be >= 0")
	}
	if i > DutyMax {
		return 0, errors.New("duty must be <= " + strconv.Itoa(int(DutyMax)))
	}
	return i, nil
}

// EdgeSample is a sample that occurred at the specified moment.
//
// It is used by Edges().
type EdgeSample struct {
	Edge Edge
	// T is the moment at which the edge was detected.
	T time.Time
	// Err is set if sensing failed. In this case it can be assumed that
	// Edges() is aborting.
	Err error
}

// PinIn is an input GPIO pin.
//
// It may optionally support internal pull resistor and edge based triggering.
//
// A button is semantically a PinIn. So if you are looking to read from a
// button, PinIn is the interface you are looking for.
type PinIn interface {
	pin.Pin
	// In setups a pin as an input.
	//
	// Calling In() will try to empty the accumulated edges but it cannot be 100%
	// reliable due to the OS (linux) and its driver. It is possible that on a
	// gpio that is as input, doing a quick Out(), In() may return an edge that
	// occurred before the Out() call.
	In(pull Pull) error
	// Read return the current pin level.
	//
	// Behavior is undefined if In() wasn't used before.
	//
	// In some rare case, it is possible that Read() fails silently. This happens
	// if another process on the host messes up with the pin after In() was
	// called. In this case, call In() again.
	Read() Level
	// Edges() listens to edges and return the ones detected to the channel until
	// the context is canceled.
	//
	// If the context passed in is already canceled, no measurement is done and
	// nothing is sent to the channel.
	Edges(ctx context.Context, edge Edge, c chan<- EdgeSample)
	// Pull returns the internal pull resistor if the pin is set as input pin.
	//
	// Returns PullNoChange if the value cannot be read.
	Pull() Pull
	// DefaultPull returns the pull that is initialized on CPU/device reset. This
	// is useful to determine if the pin is acceptable for operation with
	// certain devices.
	DefaultPull() Pull
}

// PinOut is an output GPIO pin.
//
// A LED, a buzzer, a servo, are semantically a PinOut. So if you are looking
// to control these, PinOut is the interface you are looking for.
type PinOut interface {
	pin.Pin
	// Out sets a pin as output if it wasn't already and sets the initial value.
	//
	// After the initial call to ensure that the pin has been set as output, it
	// is generally safe to ignore the error returned.
	//
	// Out() tries to empty the accumulated edges detected if the gpio was
	// previously set as input but this is not 100% guaranteed due to the OS.
	Out(l Level) error
	// PWM sets the PWM output on supported pins, if the pin has hardware PWM
	// support.
	//
	// To use as a general purpose clock, set duty to DutyHalf. Some pins may
	// only support DutyHalf and no other value.
	//
	// Using 0 as frequency will use the optimal value as supported/preferred by
	// the pin.
	//
	// To use as a servo, see https://en.wikipedia.org/wiki/Servo_control as an
	// explanation how to calculate duty.
	PWM(ctx context.Context, duty Duty, f physic.Frequency) error
}

// PinIO is a GPIO pin that supports both input and output. It matches both
// interfaces PinIn and PinOut.
//
// A GPIO pin implementing PinIO may fail at either input or output or both.
type PinIO interface {
	pin.Pin
	// PinIn
	In(pull Pull) error
	Read() Level
	Edges(ctx context.Context, edge Edge, c chan<- EdgeSample)
	Pull() Pull
	DefaultPull() Pull
	// PinOut
	Out(l Level) error
	PWM(ctx context.Context, duty Duty, f physic.Frequency) error
}

// INVALID implements PinIO and fails on all access.
var INVALID PinIO

// RealPin is implemented by aliased pin and allows the retrieval of the real
// pin underlying an alias.
//
// Aliases are created by RegisterAlias. Aliases permits presenting a user
// friendly GPIO pin name while representing the underlying real pin.
//
// The purpose of the RealPin is to be able to cleanly test whether an arbitrary
// gpio.PinIO returned by ByName is an alias for another pin, and resolve it.
type RealPin interface {
	Real() PinIO // Real returns the real pin behind an Alias
}

//

// errInvalidPin is returned when trying to use INVALID.
var errInvalidPin = errors.New("gpio: invalid pin")

func init() {
	INVALID = invalidPin{}
}

// invalidPin implements PinIO for compatibility but fails on all access.
type invalidPin struct {
}

func (invalidPin) String() string {
	return "INVALID"
}

func (invalidPin) Halt() error {
	return nil
}

func (invalidPin) Number() int {
	return -1
}

func (invalidPin) Name() string {
	return "INVALID"
}

func (invalidPin) Func() pin.Func {
	return pin.FuncNone
}

func (invalidPin) SupportedFuncs() []pin.Func {
	return nil
}

func (invalidPin) SetFunc(f pin.Func) error {
	return errInvalidPin
}

func (invalidPin) In(Pull) error {
	return errInvalidPin
}

func (invalidPin) Read() Level {
	return Low
}

func (invalidPin) Edges(ctx context.Context, edge Edge, c chan<- EdgeSample) {
	<-ctx.Done()
}

func (invalidPin) Pull() Pull {
	return PullNoChange
}

func (invalidPin) DefaultPull() Pull {
	return PullNoChange
}

func (invalidPin) Out(Level) error {
	return errInvalidPin
}

func (invalidPin) PWM(context.Context, Duty, physic.Frequency) error {
	return errInvalidPin
}

var _ PinIn = INVALID
var _ PinOut = INVALID
var _ PinIO = INVALID
