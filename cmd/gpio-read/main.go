// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// gpio-read reads a GPIO pin.
package main

import (
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"time"

	"periph.io/x/periph/conn/gpio"
	"periph.io/x/periph/conn/gpio/gpioreg"
)

func printLevel(l gpio.Level) error {
	if l == gpio.Low {
		_, err := os.Stdout.Write([]byte{'0', '\n'})
		return err
	}
	_, err := os.Stdout.Write([]byte{'1', '\n'})
	return err
}

func doStream(p gpio.PinIn, r time.Duration, stop <-chan os.Signal) error {
	ps, ok := p.(gpio.PinStreamReader)
	if !ok {
		return fmt.Errorf("%s doesn't support streaming", p)
	}
	b := gpio.BitStreamLSB{Res: r, Bits: make(gpio.BitsLSB, 32)}
	for {
		select {
		case <-stop:
			return nil
		default:
		}
		if err := ps.ReadStream(gpio.PullNoChange, &b); err != nil {
			return err
		}
		/*
			for i := range b.Bits {
				for j := 0; j < 8; j++ {
					if b.Bits[i]&1<<uint(j) != 0 {
						if _, err := os.Stdout.Write([]byte{'1'}); err != nil {
							// It's just stdout that closed.
							return nil
						}
					} else {
						if _, err := os.Stdout.Write([]byte{'0'}); err != nil {
							// It's just stdout that closed.
							return nil
						}
					}
				}
			}
		*/
		fmt.Printf("%s\n", hex.EncodeToString(b.Bits))
	}
}

func doEdges(p gpio.PinIn, stop <-chan os.Signal) error {
	for {
		c := make(chan struct{})
		go func() {
			p.WaitForEdge(-1)
			c <- struct{}{}
		}()
		select {
		case <-c:
			printLevel(p.Read())
		case <-stop:
			return nil
		}
	}
}

func mainImpl() error {
	pullUp := flag.Bool("u", false, "pull up")
	pullDown := flag.Bool("d", false, "pull down")
	edges := flag.Bool("e", false, "wait for edges")
	stream := flag.String("s", "", "streams 0 and 1 while reading at the specified frequency")
	verbose := flag.Bool("v", false, "verbose mode")
	flag.Parse()
	if !*verbose {
		log.SetOutput(ioutil.Discard)
	}
	log.SetFlags(log.Lmicroseconds)

	if *edges && *stream != "" {
		return errors.New("can't use both -e and -s")
	}
	pull := gpio.Float
	if *pullUp {
		if *pullDown {
			return errors.New("use only one of -d or -u")
		}
		pull = gpio.PullUp
	}
	if *pullDown {
		pull = gpio.PullDown
	}
	if flag.NArg() != 1 {
		return errors.New("specify GPIO pin to read")
	}

	if _, err := hostInit(); err != nil {
		return err
	}

	p := gpioreg.ByName(flag.Args()[0])
	if p == nil {
		return errors.New("specify a valid GPIO pin number")
	}

	// Handle Ctrl-C gracefully.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	if *stream != "" {
		d, err := time.ParseDuration(*stream)
		if err != nil {
			return err
		}
		return doStream(p, d, stop)
	}

	edge := gpio.NoEdge
	if *edges {
		edge = gpio.BothEdges
	}
	if err := p.In(pull, edge); err != nil {
		return err
	}
	if *edges {
		return doEdges(p, stop)
	}
	return printLevel(p.Read())
}

func main() {
	if err := mainImpl(); err != nil {
		fmt.Fprintf(os.Stderr, "gpio-read: %s.\n", err)
		os.Exit(1)
	}
}
