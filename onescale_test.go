// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// The toolkit has ONE scale. Menu and Browser grew their own Scale field before
// the global existed, and a widget that answered only to its own field made a
// host choose: set the global and watch the menu stay small, or set both and
// risk everything being scaled twice.
//
// Deferring settles it. The field REPLACES the global when it is set, so a host
// that predates the knob gets exactly what it asks for; when it is not set, the
// widget follows the knob like everything else.
func TestMenuScaleDefersToTheGlobal(t *testing.T) {
	m := NewMenu([]MenuItem{{Label: "one"}})

	if got := m.scale(); got != 1 {
		t.Errorf("an untold menu at the default global scale = %v, want 1", got)
	}

	SetMetricScale(2)
	defer SetMetricScale(1)
	if got := m.scale(); got != 2 {
		t.Errorf("an untold menu under SetMetricScale(2) = %v, want 2 -- it is ignoring the one knob a host is told to turn", got)
	}

	// An explicit field wins, and does NOT multiply: 1.5 under a global of 2 is
	// 1.5, not 3. A host that set the field before the global existed must not
	// have its menu quietly grow.
	m.Scale = 1.5
	if got := m.scale(); got != 1.5 {
		t.Errorf("a menu told 1.5 under a global of 2 = %v, want 1.5", got)
	}
}

func TestBrowserScaleDefersToTheGlobal(t *testing.T) {
	b := &Browser{}

	if got := b.scale(); got != 1 {
		t.Errorf("an untold browser at the default global scale = %v, want 1", got)
	}

	SetMetricScale(2)
	defer SetMetricScale(1)
	if got := b.scale(); got != 2 {
		t.Errorf("an untold browser under SetMetricScale(2) = %v, want 2", got)
	}
	b.Scale = 3
	if got := b.scale(); got != 3 {
		t.Errorf("a browser told 3 under a global of 2 = %v, want 3", got)
	}
}

// The scale a widget draws with must be the one it reports, at both ends: a
// menu row is taller under the global, and the same height whether the host set
// the field or the knob.
func TestMenuRowHeightFollowsWhicheverScaleIsSet(t *testing.T) {
	rows := func(m *Menu) int { return m.rowsHeight() }

	plain := NewMenu([]MenuItem{{Label: "one"}, {Label: "two"}})
	base := rows(plain)

	SetMetricScale(2)
	global := rows(NewMenu([]MenuItem{{Label: "one"}, {Label: "two"}}))
	SetMetricScale(1)

	field := NewMenu([]MenuItem{{Label: "one"}, {Label: "two"}})
	field.Scale = 2
	explicit := rows(field)

	if global != explicit {
		t.Errorf("a menu scaled by the global is %d tall and one scaled by its field is %d: "+
			"the two ways of saying the same thing disagree", global, explicit)
	}
	if global <= base {
		t.Errorf("a menu at twice the scale is %d tall against %d at one: it did not grow", global, base)
	}
}

// ColorPickerNaturalSize is the device-pixel form of the picker's logical
// footprint constants, which are compile-time and so cannot scale themselves.
func TestColorPickerNaturalSize(t *testing.T) {
	w, h := ColorPickerNaturalSize()
	if w != ColorPickerWidth || h != ColorPickerHeight {
		t.Errorf("at scale 1 the natural size is %dx%d, want the logical %dx%d",
			w, h, ColorPickerWidth, ColorPickerHeight)
	}

	SetMetricScale(2)
	defer SetMetricScale(1)
	w2, h2 := ColorPickerNaturalSize()
	if w2 != 2*ColorPickerWidth || h2 != 2*ColorPickerHeight {
		t.Errorf("at scale 2 the natural size is %dx%d, want %dx%d -- a host laying out to it "+
			"would give the picker half the room it needs", w2, h2, 2*ColorPickerWidth, 2*ColorPickerHeight)
	}
}
