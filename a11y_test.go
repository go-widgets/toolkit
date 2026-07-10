// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestA11yInfoPerWidget(t *testing.T) {
	check := NewCheckButton("Enable", true)
	radio := NewRadioButton("Option")
	radio.Checked = false
	sw := &Switch{On: true}
	scale := NewScale(0, 100, 42)

	cases := []struct {
		name string
		w    Accessible
		want A11yInfo
	}{
		{"button", NewButton("OK", nil), A11yInfo{Role: RoleButton, Name: "OK"}},
		{"label", NewLabel("Hello"), A11yInfo{Role: RoleText, Name: "Hello"}},
		{"entry", &Entry{Text: "query"}, A11yInfo{Role: RoleTextbox, Value: "query"}},
		{"checkbox-on", check, A11yInfo{Role: RoleCheckbox, Name: "Enable", Value: "checked"}},
		{"radio-off", radio, A11yInfo{Role: RoleRadio, Name: "Option", Value: ""}},
		{"switch-on", sw, A11yInfo{Role: RoleSwitch, Value: "on"}},
		{"slider", scale, A11yInfo{Role: RoleSlider, Value: "42"}},
	}
	for _, c := range cases {
		if got := c.w.A11y(); got != c.want {
			t.Errorf("%s A11y() = %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestSwitchA11yOff(t *testing.T) {
	// Covers the "off" arm of Switch.A11y.
	sw := &Switch{On: false}
	if got := sw.A11y().Value; got != "off" {
		t.Errorf("Switch off value = %q, want off", got)
	}
}

func TestCheckedValueUnchecked(t *testing.T) {
	// Covers the false arm of checkedValue via an unchecked box.
	c := NewCheckButton("x", false)
	if got := c.A11y().Value; got != "" {
		t.Errorf("unchecked value = %q, want empty", got)
	}
}

func TestCollectA11ySkipsNonAccessible(t *testing.T) {
	// A recordingWidget (Base only) is NOT Accessible, so it is skipped; the
	// Button and Label around it are collected in order.
	widgets := []Widget{
		NewButton("Save", nil),
		&recordingWidget{}, // no A11y() — skipped
		NewLabel("Status"),
	}
	got := CollectA11y(widgets)
	if len(got) != 2 {
		t.Fatalf("collected %d, want 2 (non-Accessible skipped)", len(got))
	}
	if got[0].Role != RoleButton || got[0].Name != "Save" {
		t.Errorf("first = %+v, want Save button", got[0])
	}
	if got[1].Role != RoleText || got[1].Name != "Status" {
		t.Errorf("second = %+v, want Status label", got[1])
	}
}

func TestCollectA11yEmpty(t *testing.T) {
	if got := CollectA11y(nil); got != nil {
		t.Errorf("CollectA11y(nil) = %v, want nil", got)
	}
}
