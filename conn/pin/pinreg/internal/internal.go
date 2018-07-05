// Copyright 2018 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package internal

import (
	"sync"

	"periph.io/x/periph/conn/pin"
)

// Position is used for pinreg.IsConnected.
type Position struct {
	Name   string // Header name
	Number int    // Pin number
}

var (
	// Mu synchronizes access.
	Mu sync.Mutex
	// AllHeaders is every known headers as per internal lookup table.
	AllHeaders = map[string][][]pin.Pin{}
	// ByPin is the GPIO pin name to position.
	ByPin = map[string]Position{}
)
