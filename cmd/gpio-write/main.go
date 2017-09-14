// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// gpio-write sets a GPIO pin to low or high.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
	"periph.io/x/periph/conn/gpio/stream"
)

func mainImpl() error {
	clk := flag.String("clk", "", "clock/PWM of the following period")
	duty := flag.String("duty", "", fmt.Sprintf("duty cycle, between 0 and %d or 0%% and 100%%", gpio.DutyMax))
	res := flag.String("res", "", "read bit LSB stream from stdin; specify the duration of each bit")
	verbose := flag.Bool("v", false, "verbose mode")
	flag.Parse()
	if !*verbose {
		log.SetOutput(ioutil.Discard)
	}
	log.SetFlags(log.Lmicroseconds)
	if flag.NArg() == 0 {
		return errors.New("specify GPIO pin to write to, and either its level (0 or 1) or -clk and -duty")
	}

	args := flag.Args()
	l := gpio.Low
	switch args[1] {
	case "0":
	case "1":
		l = gpio.High
	default:
		return errors.New("specify level as 0 or 1")
	}

	if _, err := hostInit(); err != nil {
		return err
	}

	p := gpioreg.ByName(args[0])
	if p == nil {
		return errors.New("invalid GPIO pin number")
	}

	if *clk != "" {
		if flag.NArg() != 1 {
			return errors.New("do not specify level when using -clk")
		}
		if len(*res) != 0 {
			return errors.New("cannot use -res with -clk")
		}
		period, err := time.ParseDuration(*clk)
		if err != nil {
			return err
		}
		if len(*duty) == 0 {
			return errors.New("-duty is required with -clk")
		}
		d, err := gpio.ParseDuty(*duty)
		if err != nil {
			return err
		}
		pwm, ok := p.(gpio.PWMer)
		if !ok {
			return fmt.Errorf("%s doesn't support PWM", p)
		}
		log.Printf("PWM(%s, %s)", d, period)
		return pwm.PWM(d, period)
	} else if len(*duty) != 0 {
		return errors.New("-clk is required with -duty")
	}

	if len(*res) != 0 {
		d, err := time.ParseDuration(*res)
		if err != nil {
			return err
		}
		b, _ := ioutil.ReadAll(os.Stdin)
		if len(b) == 0 {
			return errors.New("must provide bit stream via stdin")
		}
		s, ok := p.(gpio.PinStreamer)
		if !ok {
			return fmt.Errorf("%s doesn't support bit stream", p)
		}
		log.Printf("Streaming %d bytes at %s resolution", len(b), d)
		return s.Stream(&stream.Bits{Bits: gpio.Bits(b), Res: d})
	}

	if flag.NArg() != 2 {
		return errors.New("specify GPIO pin to write to, and either its level (0 or 1) or -clk and -duty")
	}
	l := gpio.Low
	switch flag.Arg(1) {
	case "0":
	case "1":
		l = gpio.High
	default:
		return errors.New("specify level as 0 or 1")
	}
	return p.Out(l)
}

func main() {
	if err := mainImpl(); err != nil {
		fmt.Fprintf(os.Stderr, "gpio-write: %s.\n", err)
		os.Exit(1)
	}
}
