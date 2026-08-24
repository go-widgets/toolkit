// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"
	"testing"
	"time"
)

// fixedTime is a deterministic instant used across the Clock tests: 15:04 on
// 2 Jan 2006 (the Go reference time), so the default layout renders "15:04".
var fixedTime = time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC)

// TestClockDefaultFormat checks a fresh Clock renders its instant with the
// default 24-hour layout and exposes it through the Time observable + the Label.
func TestClockDefaultFormat(t *testing.T) {
	c := NewClock(fixedTime)
	if !c.Time().Get().Equal(fixedTime) {
		t.Fatalf("Time = %v, want the constructor instant", c.Time().Get())
	}
	if got := c.reading(); got != "15:04" {
		t.Fatalf("default reading = %q, want %q", got, "15:04")
	}
	// The internal Label mirrors the reading after a sync (constructor Set fired
	// the subscription).
	if got := c.label().Text().Get(); got != "15:04" {
		t.Fatalf("label text = %q, want %q", got, "15:04")
	}
	// NewClock centres the reading.
	if c.Align != AlignCenter {
		t.Fatalf("NewClock Align = %v, want AlignCenter", c.Align)
	}
}

// TestClockBareZeroValue covers the lazy-init branches: a bare &Clock{} (no
// constructor) creates its Time observable and Label on demand, and with the zero
// instant + default layout reads "00:00".
func TestClockBareZeroValue(t *testing.T) {
	c := &Clock{}
	if c.t != nil || c.lbl != nil {
		t.Fatal("bare Clock must not allocate before first use")
	}
	if got := c.reading(); got != "00:00" {
		t.Fatalf("zero-time reading = %q, want %q", got, "00:00")
	}
	// reading() lazily created the observable; label() lazily creates the Label.
	if c.t == nil {
		t.Fatal("reading must create the Time observable")
	}
	if l := c.label(); l == nil {
		t.Fatal("label() must create the Label")
	}
}

// TestClockSetTimeUpdatesAndNotifies checks SetTime advances the instant, returns
// the widget for chaining, refreshes the Label, and notifies external
// subscribers of the Time observable.
func TestClockSetTimeUpdatesAndNotifies(t *testing.T) {
	c := &Clock{}

	var seen time.Time
	calls := 0
	c.Time().Subscribe(func(v time.Time) {
		calls++
		seen = v
	})

	if got := c.SetTime(fixedTime); got != c {
		t.Fatal("SetTime should return the widget for chaining")
	}
	if calls != 1 || !seen.Equal(fixedTime) {
		t.Fatalf("subscriber calls=%d seen=%v, want 1 notification with the new instant", calls, seen)
	}
	if got := c.label().Text().Get(); got != "15:04" {
		t.Fatalf("SetTime must refresh the label, got %q", got)
	}

	next := fixedTime.Add(time.Minute)
	c.SetTime(next)
	if calls != 2 {
		t.Fatalf("second SetTime must notify again, calls=%d", calls)
	}
	if got := c.label().Text().Get(); got != "15:05" {
		t.Fatalf("label after +1min = %q, want %q", got, "15:05")
	}
}

// TestClockTimeIdempotent checks Time() wires its subscription exactly once, so
// repeated access does not stack duplicate refreshers.
func TestClockTimeIdempotent(t *testing.T) {
	c := &Clock{}
	a := c.Time()
	b := c.Time() // subscribed already true: exercises the guard's false branch
	if a != b {
		t.Fatal("Time() must return the same observable")
	}
}

// TestClockCustomFormat checks a set-once Format changes the reading (and the
// layout helper's non-empty branch).
func TestClockCustomFormat(t *testing.T) {
	c := NewClock(fixedTime)
	c.Format = "3:04 PM"
	if got := c.layout(); got != "3:04 PM" {
		t.Fatalf("layout = %q, want the Format", got)
	}
	if got := c.reading(); got != "3:04 PM" {
		t.Fatalf("custom-format reading = %q, want %q", got, "3:04 PM")
	}
}

// TestClockFuncOverridesFormat checks the Func seam takes precedence over Format
// (the Func-vs-Format branch in reading()).
func TestClockFuncOverridesFormat(t *testing.T) {
	c := NewClock(fixedTime)
	c.Format = "15:04" // must be ignored while Func is set
	c.Func = func(tm time.Time) string {
		return "day " + strings.ToLower(tm.Weekday().String())
	}
	if got := c.reading(); got != "day monday" {
		t.Fatalf("Func reading = %q, want %q", got, "day monday")
	}
}

// TestClockA11y checks the accessible description carries the reading as both
// Name and Value, and tracks the current time.
func TestClockA11y(t *testing.T) {
	c := NewClock(fixedTime)
	info := c.A11y()
	if info.Role != RoleText {
		t.Fatalf("A11y role = %q, want RoleText", info.Role)
	}
	if info.Value != "15:04" {
		t.Fatalf("A11y value = %q, want the formatted reading %q", info.Value, "15:04")
	}
	if info.Name != "15:04" {
		t.Fatalf("A11y name = %q, want the formatted reading %q", info.Name, "15:04")
	}
	// A11y reflects a later instant without a Draw.
	c.SetTime(fixedTime.Add(time.Hour))
	if got := c.A11y().Value; got != "16:04" {
		t.Fatalf("A11y value after +1h = %q, want %q", got, "16:04")
	}
}

// TestClockDrawsReading renders the Clock and asserts the reading is painted,
// centred (per AlignCenter) and fills the widget bounds via the internal Label.
func TestClockDrawsReading(t *testing.T) {
	const w, h = 120, 20
	c := NewClock(fixedTime)
	c.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), DefaultLight())

	// Draw laid the Label over the whole Clock rect.
	if b := c.label().Bounds(); b != (Rect{X: 0, Y: 0, W: w, H: h}) {
		t.Fatalf("label bounds = %+v, want the full clock rect", b)
	}
	// Something was painted, and it is horizontally centred (equal margins ±1).
	lo, hi := paintedCols(buf, w, h)
	if lo < 0 {
		t.Fatal("Clock painted nothing")
	}
	leftMargin, rightMargin := lo, w-1-hi
	if diff := leftMargin - rightMargin; diff < -1 || diff > 1 {
		t.Fatalf("reading not horizontally centred: left=%d right=%d", leftMargin, rightMargin)
	}
}

// TestClockDrawLeftAligned covers the non-centred Align branch reaching the Label
// through Draw: a left-aligned reading starts at the left edge.
func TestClockDrawLeftAligned(t *testing.T) {
	const w, h = 120, 20
	c := &Clock{Align: AlignLeft}
	c.SetTime(fixedTime)
	c.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	c.Draw(newP(buf, w), DefaultLight())

	// A left-aligned reading hugs the left edge (allow a small glyph left bearing),
	// unlike the ~40px margin a centred one would leave in a 120px box.
	lo, _ := paintedCols(buf, w, h)
	if lo < 0 || lo > 2 {
		t.Fatalf("left-aligned reading starts at col %d, want the left edge", lo)
	}
	if c.label().Align != AlignLeft {
		t.Fatalf("Draw must propagate Align to the label, got %v", c.label().Align)
	}
}
