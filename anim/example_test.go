// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package anim_test

import (
	"fmt"
	"time"

	"github.com/go-widgets/toolkit"
	"github.com/go-widgets/toolkit/anim"
)

// ExampleDriver_spinner drives a toolkit.Spinner's Phase from the anim Driver
// instead of the widget's manual Tick(dt). The host loop advances the driver
// with wall-clock time each frame and stops scheduling frames once Tick
// reports busy=false. This is the load-bearing consumer: the Spinner's cadence
// now comes from an eased anim.Animation over the toolkit's Easing curves.
func ExampleDriver_spinner() {
	sp := toolkit.NewSpinner()
	sp.Active().Set(true)

	d := anim.NewDriver()
	// One full 0..1 phase sweep over 1s, linear so the spin is steady.
	d.Start(&anim.Animation{
		Dur:   time.Second,
		Ease:  toolkit.Linear,
		Apply: func(p float64) { sp.Phase = p },
	})

	// A fake host loop: step 250ms per frame, quit when the driver goes idle.
	origin := time.Unix(0, 0)
	for frame := 0; ; frame++ {
		busy := d.Tick(origin.Add(time.Duration(frame) * 250 * time.Millisecond))
		fmt.Printf("frame %d phase=%.2f busy=%v\n", frame, sp.Phase, busy)
		if !busy {
			break
		}
	}
	// Output:
	// frame 0 phase=0.00 busy=true
	// frame 1 phase=0.25 busy=true
	// frame 2 phase=0.50 busy=true
	// frame 3 phase=0.75 busy=true
	// frame 4 phase=1.00 busy=false
}
