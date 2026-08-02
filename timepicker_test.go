// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// bounds used across the tests; wide enough to hold the hour + minute + AM/PM
// cells so every hit region is reachable.
var tpBounds = Rect{X: 5, Y: 3, W: 120, H: 24}

// clickAt sends a widget-local EventClick at absolute surface point (sx, sy).
func tpClickAt(tp *TimePicker, sx, sy int) {
	r := tp.Bounds()
	tp.OnEvent(Event{Kind: EventClick, X: sx - r.X, Y: sy - r.Y})
}

func TestNewTimePickerNormalises(t *testing.T) {
	tp := NewTimePicker(26, 65) // out of range on purpose
	if tp.Hour != 2 || tp.Minute != 5 {
		t.Fatalf("normalise: got %d:%d want 02:05", tp.Hour, tp.Minute)
	}
	if tp.MinuteStep != 1 {
		t.Fatalf("default MinuteStep: got %d want 1", tp.MinuteStep)
	}
}

func TestStepHourWrap(t *testing.T) {
	tp := NewTimePicker(23, 0)
	var got []int
	tp.OnChange = func(h, m int) { got = append(got, h) }
	tp.StepHour(1) // 23 -> 0
	if tp.Hour != 0 {
		t.Fatalf("23+1: got %d want 0", tp.Hour)
	}
	tp.StepHour(-1) // 0 -> 23
	if tp.Hour != 23 {
		t.Fatalf("0-1: got %d want 23", tp.Hour)
	}
	if len(got) != 2 || got[0] != 0 || got[1] != 23 {
		t.Fatalf("OnChange hours: got %v want [0 23]", got)
	}
}

func TestStepMinuteStepAndWrap(t *testing.T) {
	tp := NewTimePicker(10, 0)
	tp.MinuteStep = 15
	tp.StepMinute(-1) // 0 -> 45 (wrap, no carry)
	if tp.Minute != 45 {
		t.Fatalf("0-15: got %d want 45", tp.Minute)
	}
	if tp.Hour != 10 {
		t.Fatalf("hour must not carry: got %d want 10", tp.Hour)
	}
	tp.StepMinute(1) // 45 -> 0 (wrap)
	if tp.Minute != 0 {
		t.Fatalf("45+15: got %d want 0", tp.Minute)
	}
}

func TestStepMinuteDefaultStepWhenNonPositive(t *testing.T) {
	tp := NewTimePicker(0, 0)
	tp.MinuteStep = 0 // must be treated as 1
	tp.StepMinute(1)
	if tp.Minute != 1 {
		t.Fatalf("step<=0 default: got %d want 1", tp.Minute)
	}
}

func TestToggleAmPmShiftsHour(t *testing.T) {
	tp := NewTimePicker(9, 30) // AM
	tp.ToggleAmPm()            // -> PM
	if tp.Hour != 21 {
		t.Fatalf("9 AM->PM: got %d want 21", tp.Hour)
	}
	tp.ToggleAmPm() // -> AM
	if tp.Hour != 9 {
		t.Fatalf("21 PM->AM: got %d want 9", tp.Hour)
	}
}

func TestStringFormats(t *testing.T) {
	cases := []struct {
		h, m   int
		use12h bool
		want   string
	}{
		{15, 4, false, "15:04"},
		{0, 0, false, "00:00"},
		{15, 4, true, "3:04 PM"},
		{9, 5, true, "9:05 AM"},
		{0, 0, true, "12:00 AM"},  // midnight edge
		{12, 0, true, "12:00 PM"}, // noon edge
	}
	for _, c := range cases {
		tp := NewTimePicker(c.h, c.m)
		tp.Use12h = c.use12h
		if got := tp.String(); got != c.want {
			t.Errorf("String(%d:%d use12h=%v): got %q want %q", c.h, c.m, c.use12h, got, c.want)
		}
	}
}

// drawPaintsSomething renders tp into a fresh surface and reports whether any
// pixel differs from the 0xC8 sentinel — proof the widget painted.
func drawPaintsSomething(tp *TimePicker) bool {
	w, h := 140, 30
	buf := makeSurface(w, h)
	tp.SetBounds(tpBounds)
	tp.Draw(newP(buf, w), DefaultLight())
	for i := 0; i+3 < len(buf); i += 4 {
		if buf[i] != 0xC8 || buf[i+1] != 0xC8 || buf[i+2] != 0xC8 {
			return true
		}
	}
	return false
}

func TestDraw24h(t *testing.T) {
	tp := NewTimePicker(15, 4)
	if !drawPaintsSomething(tp) {
		t.Fatal("Draw (24h) painted nothing")
	}
}

func TestDraw12h(t *testing.T) {
	// PM hour (>=12) so meridiem "PM" branch + non-zero hour12 branch fire.
	tp := NewTimePicker(15, 4)
	tp.Use12h = true
	if !drawPaintsSomething(tp) {
		t.Fatal("Draw (12h PM) painted nothing")
	}
	// AM midnight so meridiem "AM" branch + hour12 h==0->12 branch fire.
	tp2 := NewTimePicker(0, 0)
	tp2.Use12h = true
	if !drawPaintsSomething(tp2) {
		t.Fatal("Draw (12h AM midnight) painted nothing")
	}
}

// regionCentres returns the absolute centre points of each hit region for the
// current bounds, mirroring the widget's own layout.
func regionCentres(tp *TimePicker) (hUp, hDown, mUp, mDown, ampm, colon [2]int) {
	hourCell, col, minCell, ampmCell := tp.layout()
	hu, hd := spinButtons(hourCell)
	mu, md := spinButtons(minCell)
	ctr := func(r Rect) [2]int { return [2]int{r.X + r.W/2, r.Y + r.H/2} }
	return ctr(hu), ctr(hd), ctr(mu), ctr(md), ctr(ampmCell), ctr(col)
}

func TestOnEventHitsEachRegion(t *testing.T) {
	tp := NewTimePicker(10, 30)
	tp.Use12h = true
	tp.SetBounds(tpBounds)
	fired := 0
	tp.OnChange = func(h, m int) { fired++ }
	hUp, hDown, mUp, mDown, ampm, colon := regionCentres(tp)

	tpClickAt(tp, hUp[0], hUp[1])
	if tp.Hour != 11 {
		t.Fatalf("hour up: got %d want 11", tp.Hour)
	}
	tpClickAt(tp, hDown[0], hDown[1])
	if tp.Hour != 10 {
		t.Fatalf("hour down: got %d want 10", tp.Hour)
	}
	tpClickAt(tp, mUp[0], mUp[1])
	if tp.Minute != 31 {
		t.Fatalf("minute up: got %d want 31", tp.Minute)
	}
	tpClickAt(tp, mDown[0], mDown[1])
	if tp.Minute != 30 {
		t.Fatalf("minute down: got %d want 30", tp.Minute)
	}
	tpClickAt(tp, ampm[0], ampm[1]) // AM -> PM
	if tp.Hour != 22 {
		t.Fatalf("ampm toggle: got %d want 22", tp.Hour)
	}
	if fired != 5 {
		t.Fatalf("OnChange fires: got %d want 5", fired)
	}

	// Click on the ":" separator: no region, no change, no fire.
	fired = 0
	before := *tp
	tpClickAt(tp, colon[0], colon[1])
	if tp.Hour != before.Hour || tp.Minute != before.Minute || fired != 0 {
		t.Fatalf("colon click changed state: %v -> %d:%d fired=%d", before, tp.Hour, tp.Minute, fired)
	}
}

func TestOnEventIgnoresNonClick(t *testing.T) {
	tp := NewTimePicker(10, 30)
	tp.SetBounds(tpBounds)
	tp.OnChange = func(h, m int) { t.Fatal("non-click event must not step") }
	tp.OnEvent(Event{Kind: EventKeyDown, X: 0, Y: 0, Code: "ArrowUp"})
}

func TestNilCallbackSafe(t *testing.T) {
	tp := NewTimePicker(23, 59) // OnChange nil
	tp.SetBounds(tpBounds)
	tp.StepHour(1)
	tp.StepMinute(1)
	tp.ToggleAmPm()
	hUp, _, _, _, _, _ := regionCentres(tp)
	tpClickAt(tp, hUp[0], hUp[1]) // click path with nil callback
}
