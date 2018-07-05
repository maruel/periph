// Copyright 2018 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package pinregtest

import (
	"periph.io/x/periph/conn/gpio"
)

// Reset removes all registered entries.
//
// This is meant to be used in unit tests. This is because aliases to non
// existing pins cannot be removed otherwise.
//
// Users should also call gpioregtest.Reset().
func Reset() {
	internal.Mu.Lock()
	defer internal.Mu.Unlock()
	internal.ByName = map[string]gpio.PinIO{}
	internal.ByAlias = map[string]string{}
}
