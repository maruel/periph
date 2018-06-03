// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package sysfs

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/physic"
)

func TestLEDByName(t *testing.T) {
	if _, err := LEDByName("FOO"); err == nil {
		t.Fatal("expected error")
	}
}

func TestLED(t *testing.T) {
	l := LED{number: 42, name: "Glow", root: "/tmp/led/priv/"}
	if s := l.String(); s != "Glow(42)" {
		t.Fatal(s)
	}
	if s := l.Name(); s != "Glow" {
		t.Fatal(s)
	}
	if n := l.Number(); n != 42 {
		t.Fatal(n)
	}
}

func TestLEDMock(t *testing.T) {
	defer reset()
	fileIOOpen = func(path string, flag int) (fileIO, error) {
		if flag != os.O_RDWR {
			t.Fatal(flag)
		}
		switch path {
		case "/tmp/led/priv/brightness":
			return &fakeGPIOFile{data: []byte("255")}, nil
		default:
			t.Fatalf("unknown %q", path)
			return nil, errors.New("unknown file")
		}
	}

	l := LED{number: 42, name: "Glow", root: "/tmp/led/priv/"}
	if s := l.Func(); s != "LED/Off" {
		t.Fatal(s)
	}
	if err := l.In(gpio.PullNoChange, gpio.NoEdge); err != nil {
		t.Fatal(err)
	}
	if l := l.Read(); l != gpio.High {
		t.Fatal("got Low, expected High")
	}
	if err := l.Out(gpio.High); err != nil {
		t.Fatal(err)
	}
}

func TestLED_PWM(t *testing.T) {
	defer reset()
	fileIOOpen = func(path string, flag int) (fileIO, error) {
		if flag != os.O_RDWR {
			t.Fatal(flag)
		}
		switch path {
		case "/tmp/led/priv/brightness":
			return &fakeGPIOFile{data: make([]byte, 1)}, nil
		default:
			t.Fatalf("unknown %q", path)
			return nil, errors.New("unknown file")
		}
	}

	l := LED{number: 42, name: "Glow", root: "/tmp/led/priv/"}
	if err := l.PWM(gpio.DutyMax/255, 0); err != nil {
		t.Fatal(err)
	}
	if f := l.fBrightness.(*fakeGPIOFile); !bytes.Equal(f.data, []byte("1")) {
		t.Fatal(f.data)
	}
}

func TestLED_not_supported(t *testing.T) {
	l := LED{number: 42, name: "Glow", root: "/tmp/led/priv/"}
	if err := l.In(gpio.PullDown, gpio.NoEdge); err == nil {
		t.Fatal("sysfs-led no real In() support")
	}
	if l.WaitForEdge(-1) {
		t.Fatal("not supported")
	}
	if pull := l.Pull(); pull != gpio.PullNoChange {
		t.Fatal(pull)
	}
	if l.PWM(gpio.DutyHalf, physic.KiloHertz) == nil {
		t.Fatal("not supported")
	}
}

func TestLEDDriver(t *testing.T) {
	if len((&driverLED{}).Prerequisites()) != 0 {
		t.Fatal("unexpected LED prerequisites")
	}
}
