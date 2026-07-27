// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// Orientation selects whether a linear widget (Scale, RangeSlider, ProgressBar,
// LevelBar, …) runs across its width or up its height. Horizontal is the zero
// value, so an unset Orientation keeps the original left-to-right layout.
//
// A vertical widget fills / travels from the BOTTOM up, matching how a physical
// meter, fader, or level indicator reads.
type Orientation int

const (
	// Horizontal runs left-to-right (the default).
	Horizontal Orientation = iota
	// Vertical runs bottom-to-top.
	Vertical
)
