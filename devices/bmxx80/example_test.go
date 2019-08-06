// Copyright 2018 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bmxx80_test

import (
	"fmt"
	"log"

	"periph.io/x/periph/conn/environment"
	"periph.io/x/periph/conn/i2c/i2creg"
	"periph.io/x/periph/devices/bmxx80"
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
		log.Fatalf("failed to open I²C: %v", err)
	}
	defer b.Close()

	d, err := bmxx80.NewI2C(b, 0x76, &bmxx80.DefaultOpts)
	if err != nil {
		log.Fatalf("failed to initialize bme280: %v", err)
	}
	w := environment.Weather{}
	if err := d.SenseWeather(&w); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%8s %10s %9s\n", w.Temperature, w.Pressure, w.Humidity)
}
