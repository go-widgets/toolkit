// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "strconv"

// Accessibility bridge.
//
// The toolkit renders to a raw pixel/cell buffer, so it has no native
// accessibility tree — a screen reader sees nothing unless the host publishes
// one. This file gives widgets a back-end-neutral way to describe themselves:
// a widget that implements Accessible returns an A11yInfo (role + name + value),
// and a host walks its composed widget list with CollectA11y and republishes
// the result to the platform layer (WAI-ARIA nodes on wasm, TTY-cell metadata
// on a screen-reader-aware terminal, ...).
//
// It is deliberately opt-in and additive: implementing Accessible is a single
// extra method, so existing widgets and consumers are unaffected.

// Role is a widget's accessibility role, named after the WAI-ARIA roles a host
// maps them onto.
type Role string

// The roles the built-in widgets report.
const (
	RoleButton   Role = "button"
	RoleText     Role = "text"
	RoleTextbox  Role = "textbox"
	RoleCheckbox Role = "checkbox"
	RoleRadio    Role = "radio"
	RoleSwitch   Role = "switch"
	RoleSlider   Role = "slider"
)

// A11yInfo is a widget's accessibility description. Name is the accessible name
// (its label/caption); Value is the current value where meaningful (a textbox's
// text, a checkbox's "checked"/"" state, a slider's number).
type A11yInfo struct {
	Role  Role
	Name  string
	Value string
}

// Accessible is implemented by widgets that expose accessibility metadata.
type Accessible interface {
	Widget
	A11y() A11yInfo
}

// CollectA11y returns the A11yInfo for every widget in the slice that
// implements Accessible, preserving order. A host owns its widget tree, so it
// passes the flat list it composed; widgets that don't implement Accessible are
// skipped.
func CollectA11y(widgets []Widget) []A11yInfo {
	var out []A11yInfo
	for _, w := range widgets {
		if a, ok := w.(Accessible); ok {
			out = append(out, a.A11y())
		}
	}
	return out
}

// checkedValue maps a boolean checked/pressed state to the a11y Value string.
func checkedValue(on bool) string {
	if on {
		return "checked"
	}
	return ""
}

// A11y reports the Button as a button role named by its label.
func (b *Button) A11y() A11yInfo { return A11yInfo{Role: RoleButton, Name: b.Label} }

// A11y reports the Label as static text.
func (l *Label) A11y() A11yInfo { return A11yInfo{Role: RoleText, Name: l.Text} }

// A11y reports the Entry as a textbox carrying its current text.
func (e *Entry) A11y() A11yInfo { return A11yInfo{Role: RoleTextbox, Value: e.Text} }

// A11y reports the CheckButton as a checkbox with its checked state.
func (c *CheckButton) A11y() A11yInfo {
	return A11yInfo{Role: RoleCheckbox, Name: c.Label, Value: checkedValue(c.Checked)}
}

// A11y reports the RadioButton as a radio with its checked state.
func (r *RadioButton) A11y() A11yInfo {
	return A11yInfo{Role: RoleRadio, Name: r.Label, Value: checkedValue(r.Checked)}
}

// A11y reports the Switch as a switch with an on/off value.
func (s *Switch) A11y() A11yInfo {
	v := "off"
	if s.On {
		v = "on"
	}
	return A11yInfo{Role: RoleSwitch, Value: v}
}

// A11y reports the Scale as a slider carrying its current value.
func (s *Scale) A11y() A11yInfo {
	return A11yInfo{Role: RoleSlider, Value: strconv.FormatFloat(s.Value, 'g', -1, 64)}
}

// Compile-time checks that each widget satisfies Accessible.
var (
	_ Accessible = (*Button)(nil)
	_ Accessible = (*Label)(nil)
	_ Accessible = (*Entry)(nil)
	_ Accessible = (*CheckButton)(nil)
	_ Accessible = (*RadioButton)(nil)
	_ Accessible = (*Switch)(nil)
	_ Accessible = (*Scale)(nil)
)
