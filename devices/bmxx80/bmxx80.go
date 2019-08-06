// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bmxx80

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"periph.io/x/periph/conn"
	"periph.io/x/periph/conn/environment"
	"periph.io/x/periph/conn/i2c"
	"periph.io/x/periph/conn/mmr"
	"periph.io/x/periph/conn/physic"
	"periph.io/x/periph/conn/spi"
)

// Oversampling affects how much time is taken to measure each of temperature,
// pressure and humidity.
//
// Using high oversampling and low standby results in highest power
// consumption, but this is still below 1mA so we generally don't care.
type Oversampling uint8

// Possible oversampling values.
//
// The higher the more time and power it takes to take a measurement. Even at
// 16x for all 3 sensors, it is less than 100ms albeit increased power
// consumption may increase the temperature reading.
const (
	Off  Oversampling = 0
	O1x  Oversampling = 1
	O2x  Oversampling = 2
	O4x  Oversampling = 3
	O8x  Oversampling = 4
	O16x Oversampling = 5
)

const oversamplingName = "Off1x2x4x8x16x"

var oversamplingIndex = [...]uint8{0, 3, 5, 7, 9, 11, 14}

func (o Oversampling) String() string {
	if o >= Oversampling(len(oversamplingIndex)-1) {
		return fmt.Sprintf("Oversampling(%d)", o)
	}
	return oversamplingName[oversamplingIndex[o]:oversamplingIndex[o+1]]
}

func (o Oversampling) asValue() int {
	switch o {
	case O1x:
		return 1
	case O2x:
		return 2
	case O4x:
		return 4
	case O8x:
		return 8
	case O16x:
		return 16
	default:
		return 0
	}
}

func (o Oversampling) to180() uint8 {
	switch o {
	default:
		fallthrough
	case Off, O1x:
		return 0
	case O2x:
		return 1
	case O4x:
		return 2
	case O8x, O16x:
		return 3
	}
}

// Filter specifies the internal IIR filter to get steadier measurements.
//
// Oversampling will get better measurements than filtering but at a larger
// power consumption cost, which may slightly affect temperature measurement.
type Filter uint8

// Possible filtering values.
//
// The higher the filter, the slower the value converges but the more stable
// the measurement is.
const (
	NoFilter Filter = 0
	F2       Filter = 1
	F4       Filter = 2
	F8       Filter = 3
	F16      Filter = 4
)

// DefaultOpts is the recommended default options.
var DefaultOpts = Opts{
	Temperature: O4x,
	Pressure:    O4x,
	Humidity:    O4x,
}

// Opts defines the options for the device.
//
// Recommended sensing settings as per the datasheet:
//
// → Weather monitoring: manual sampling once per minute, all sensors O1x.
// Power consumption: 0.16µA, filter NoFilter. RMS noise: 3.3Pa / 30cm, 0.07%RH.
//
// → Humidity sensing: manual sampling once per second, pressure Off, humidity
// and temperature O1X, filter NoFilter. Power consumption: 2.9µA, 0.07%RH.
//
// → Indoor navigation: continuous sampling at 40ms with filter F16, pressure
// O16x, temperature O2x, humidity O1x, filter F16. Power consumption 633µA.
// RMS noise: 0.2Pa / 1.7cm.
//
// → Gaming: continuous sampling at 40ms with filter F16, pressure O4x,
// temperature O1x, humidity Off, filter F16. Power consumption 581µA. RMS
// noise: 0.3Pa / 2.5cm.
//
// See the datasheet for more details about the trade offs.
type Opts struct {
	// Temperature can only be oversampled on BME280/BMP280.
	//
	// Temperature must be measured for pressure and humidity to be measured.
	Temperature Oversampling
	// Pressure can be oversampled up to 8x on BMP180 and 16x on BME280/BMP280.
	Pressure Oversampling
	// Humidity sensing is only supported on BME280. The value is ignored on other
	// devices.
	Humidity Oversampling
	// Filter is only used while using SenseWeatherContinuous() and is only
	// supported on BMx280.
	Filter Filter
}

func (o *Opts) delayTypical280() time.Duration {
	// Page 51.
	µs := 1000
	if o.Temperature != Off {
		µs += 2000 * o.Temperature.asValue()
	}
	if o.Pressure != Off {
		µs += 2000*o.Pressure.asValue() + 500
	}
	if o.Humidity != Off {
		µs += 2000*o.Humidity.asValue() + 500
	}
	return time.Microsecond * time.Duration(µs)
}

// NewI2C returns an object that communicates over I²C to BMP180/BME280/BMP280
// environmental sensor.
//
// The address must be 0x76 or 0x77. BMP180 uses 0x77. BME280/BMP280 default to
// 0x76 and can optionally use 0x77. The value used depends on HW
// configuration of the sensor's SDO pin.
//
// It is recommended to call Halt() when done with the device so it stops
// sampling.
func NewI2C(b i2c.Bus, addr i2c.Addr, opts *Opts) (*Dev, error) {
	switch addr {
	case 0x76, 0x77:
	default:
		return nil, errors.New("bmxx80: given address not supported by device")
	}
	d := &Dev{d: &i2c.Dev{Bus: b, Addr: addr}, isSPI: false}
	if err := d.makeDev(opts); err != nil {
		return nil, err
	}
	return d, nil
}

// NewSPI returns an object that communicates over SPI to either a BME280 or
// BMP280 environmental sensor.
//
// It is recommended to call Halt() when done with the device so it stops
// sampling.
//
// When using SPI, the CS line must be used.
func NewSPI(p spi.Port, opts *Opts) (*Dev, error) {
	// It works both in Mode0 and Mode3.
	c, err := p.Connect(10*physic.MegaHertz, spi.Mode3, 8)
	if err != nil {
		return nil, fmt.Errorf("bmxx80: %v", err)
	}
	d := &Dev{d: c, isSPI: true}
	if err := d.makeDev(opts); err != nil {
		return nil, err
	}
	return d, nil
}

// Dev is a handle to an initialized BMxx80 device.
//
// The actual device type was auto detected.
type Dev struct {
	// Immutable.
	d         conn.Conn
	isSPI     bool
	is280     bool
	isBME     bool
	opts      Opts
	measDelay time.Duration
	name      string
	os        uint8
	cal180    calibration180
	cal280    calibration280
}

func (d *Dev) String() string {
	// d.dev.Conn
	return fmt.Sprintf("%s{%s}", d.name, d.d)
}

// SenseWeather requests a one time measurement as °C, kPa and % of relative
// humidity.
//
// The very first measurements may be of poor quality.
func (d *Dev) SenseWeather(w *environment.Weather) error {
	if d.is280 {
		err := d.writeCommands([]byte{
			// ctrl_meas
			0xF4, byte(d.opts.Temperature)<<5 | byte(d.opts.Pressure)<<2 | byte(forced),
		})
		if err != nil {
			return d.wrap(err)
		}
		doSleep(d.measDelay)
		for idle := false; !idle; {
			if idle, err = d.isIdle280(); err != nil {
				return d.wrap(err)
			}
		}
		return d.sense280(w)
	}
	return d.sense180(w)
}

// SenseWeatherContinuous returns measurements as °C, kPa and % of relative
// humidity on a continuous basis.
//
// It's the responsibility of the caller to retrieve the values from the
// channel as fast as possible, otherwise the interval may not be respected.
func (d *Dev) SenseWeatherContinuous(ctx context.Context, interval time.Duration, c chan<- environment.WeatherSample) {
	// Validation.
	done := ctx.Done()
	select {
	case <-done:
		return
	default:
	}

	if d.is280 {
		// Initialize continuous reading.
		s := chooseStandby(d.isBME, interval-d.measDelay)
		cmd := [...]byte{
			// config
			0xF5, byte(s)<<5 | byte(d.opts.Filter)<<2,
			// ctrl_meas
			0xF4, byte(d.opts.Temperature)<<5 | byte(d.opts.Pressure)<<2 | byte(normal),
		}
		if err := d.writeCommands(cmd[:]); err != nil {
			c <- environment.WeatherSample{T: time.Now(), Err: d.wrap(err)}
			return
		}
		defer d.stopContinuous280()
	}

	t := time.NewTicker(interval)
	defer t.Stop()

	// First reading.
	w := environment.WeatherSample{T: time.Now()}
	if d.is280 {
		w.Err = d.sense280(&w.Weather)
	} else {
		w.Err = d.sense180(&w.Weather)
	}
	select {
	case c <- w:
		if w.Err != nil {
			return
		}
	case <-done:
		return
	}

	// Reading loop.
	for {
		select {
		case <-done:
			return
		case <-t.C:
			w.T = time.Now()
			if d.is280 {
				w.Err = d.sense280(&w.Weather)
			} else {
				w.Err = d.sense180(&w.Weather)
			}
			select {
			case c <- w:
				if w.Err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}
}

// PrecisionWeather implements environment.SenseWeather.
func (d *Dev) PrecisionWeather(w *environment.Weather) {
	if d.is280 {
		w.Temperature = 10 * physic.MilliKelvin
		w.Pressure = 15625 * physic.MicroPascal / 4
	} else {
		w.Temperature = 100 * physic.MilliKelvin
		w.Pressure = physic.Pascal
	}

	if d.isBME {
		w.Humidity = 10000 / 1024 * physic.MicroRH
	}
}

// Halt implements conn.Resource.
func (d *Dev) Halt() error {
	return nil
}

//

func (d *Dev) makeDev(opts *Opts) error {
	d.opts = *opts
	d.measDelay = d.opts.delayTypical280()

	// The device starts in 2ms as per datasheet. No need to wait for boot to be
	// finished.

	var chipID [1]byte
	// Read register 0xD0 to read the chip id.
	if err := d.readReg(0xD0, chipID[:]); err != nil {
		return err
	}
	switch chipID[0] {
	case 0x55:
		d.name = "BMP180"
		d.os = opts.Pressure.to180()
	case 0x58:
		d.name = "BMP280"
		d.is280 = true
		d.opts.Humidity = Off
	case 0x60:
		d.name = "BME280"
		d.is280 = true
		d.isBME = true
	default:
		return fmt.Errorf("bmxx80: unexpected chip id %x", chipID[0])
	}

	if d.is280 && opts.Temperature == Off {
		// Ignore the value for BMP180, since it's not controllable.
		return d.wrap(errors.New("temperature measurement is required, use at least O1x"))
	}

	if d.is280 {
		// TODO(maruel): We may want to wait for isIdle280().
		// Read calibration data t1~3, p1~9, 8bits padding, h1.
		var tph [0xA2 - 0x88]byte
		if err := d.readReg(0x88, tph[:]); err != nil {
			return err
		}
		// Read calibration data h2~6
		var h [0xE8 - 0xE1]byte
		if d.isBME {
			if err := d.readReg(0xE1, h[:]); err != nil {
				return err
			}
		}
		d.cal280 = newCalibration(tph[:], h[:])
		var b []byte
		if d.isBME {
			b = []byte{
				// ctrl_meas; put it to sleep otherwise the config update may be
				// ignored. This is really just in case the device was somehow put
				// into normal but was not Halt'ed.
				0xF4, byte(d.opts.Temperature)<<5 | byte(d.opts.Pressure)<<2 | byte(sleep),
				// ctrl_hum
				0xF2, byte(d.opts.Humidity),
				// config
				0xF5, byte(s1s)<<5 | byte(NoFilter)<<2,
				// As per page 25, ctrl_meas must be re-written last.
				0xF4, byte(d.opts.Temperature)<<5 | byte(d.opts.Pressure)<<2 | byte(sleep),
			}
		} else {
			// BMP280 doesn't have humidity to control.
			b = []byte{
				// ctrl_meas; put it to sleep otherwise the config update may be
				// ignored. This is really just in case the device was somehow put
				// into normal but was not Halt'ed.
				0xF4, byte(d.opts.Temperature)<<5 | byte(d.opts.Pressure)<<2 | byte(sleep),
				// config
				0xF5, byte(s1s)<<5 | byte(NoFilter)<<2,
				// As per page 25, ctrl_meas must be re-written last.
				0xF4, byte(d.opts.Temperature)<<5 | byte(d.opts.Pressure)<<2 | byte(sleep),
			}
		}
		return d.writeCommands(b)
	}
	// Read calibration data.
	dev := mmr.Dev8{Conn: d.d, Order: binary.BigEndian}
	if err := dev.ReadStruct(0xAA, &d.cal180); err != nil {
		return d.wrap(err)
	}
	if !d.cal180.isValid() {
		return d.wrap(errors.New("calibration data is invalid"))
	}
	return nil
}

func (d *Dev) stopContinuous280() error {
	if d.is280 {
		// Page 27 (for register) and 12~13 section 3.3.
		return d.writeCommands([]byte{
			// config
			0xF5, byte(s1s)<<5 | byte(NoFilter)<<2,
			// ctrl_meas
			0xF4, byte(d.opts.Temperature)<<5 | byte(d.opts.Pressure)<<2 | byte(sleep),
		})
	}
	return nil
}

func (d *Dev) readReg(reg uint8, b []byte) error {
	// Page 32-33
	if d.isSPI {
		// MSB is 0 for write and 1 for read.
		read := make([]byte, len(b)+1)
		write := make([]byte, len(read))
		// Rest of the write buffer is ignored.
		write[0] = reg
		if err := d.d.Tx(write, read); err != nil {
			return d.wrap(err)
		}
		copy(b, read[1:])
		return nil
	}
	if err := d.d.Tx([]byte{reg}, b); err != nil {
		return d.wrap(err)
	}
	return nil
}

// writeCommands writes a command to the device.
//
// Warning: b may be modified!
func (d *Dev) writeCommands(b []byte) error {
	if d.isSPI {
		// Page 33; set RW bit 7 to 0.
		for i := 0; i < len(b); i += 2 {
			b[i] &^= 0x80
		}
	}
	if err := d.d.Tx(b, nil); err != nil {
		return d.wrap(err)
	}
	return nil
}

func (d *Dev) wrap(err error) error {
	return fmt.Errorf("%s: %v", strings.ToLower(d.name), err)
}

var doSleep = time.Sleep

var _ conn.Resource = &Dev{}
var _ environment.SenseWeather = &Dev{}
