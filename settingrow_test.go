// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- API surface ---------------------------------------------------------

func TestSettingRowConstants(t *testing.T) {
	if SettingRowPadX != 12 || SettingRowPadY != 8 ||
		SettingRowSubtitleGap != 2 || SettingRowControlW != 48 ||
		SettingRowControlH != 24 {
		t.Fatalf("constants drifted: PadX=%d PadY=%d Gap=%d CtrlW=%d CtrlH=%d",
			SettingRowPadX, SettingRowPadY, SettingRowSubtitleGap,
			SettingRowControlW, SettingRowControlH)
	}
}

func TestNewSettingRowDefaults(t *testing.T) {
	sw := NewSwitch(false)
	r := NewSettingRow("Wi-Fi", sw)
	if r.Title != "Wi-Fi" {
		t.Fatalf("Title = %q, want Wi-Fi", r.Title)
	}
	if r.Control != sw {
		t.Fatal("Control not stored")
	}
	if r.Subtitle != "" {
		t.Fatalf("Subtitle = %q, want empty", r.Subtitle)
	}
	if !r.Divider {
		t.Fatal("NewSettingRow should default Divider on")
	}
}

// --- controlSize: nil / preset / default branches ------------------------

func TestSettingRowControlSizeNil(t *testing.T) {
	r := NewSettingRow("t", nil)
	if cw, ch := r.controlSize(); cw != 0 || ch != 0 {
		t.Fatalf("nil control size = (%d,%d), want (0,0)", cw, ch)
	}
}

func TestSettingRowControlSizePreset(t *testing.T) {
	c := &arChild{}
	c.SetBounds(Rect{W: 40, H: 20})
	r := NewSettingRow("t", c)
	if cw, ch := r.controlSize(); cw != 40 || ch != 20 {
		t.Fatalf("preset control size = (%d,%d), want (40,20)", cw, ch)
	}
}

func TestSettingRowControlSizeDefault(t *testing.T) {
	// A control with zero bounds falls back to the slot defaults.
	c := &arChild{}
	r := NewSettingRow("t", c)
	if cw, ch := r.controlSize(); cw != scaled(SettingRowControlW) || ch != scaled(SettingRowControlH) {
		t.Fatalf("default control size = (%d,%d), want (%d,%d)",
			cw, ch, scaled(SettingRowControlW), scaled(SettingRowControlH))
	}
}

// --- Measure: text-tall vs control-tall, subtitle on/off -----------------

func TestSettingRowMeasureTextDominant(t *testing.T) {
	// No control: height is the single text line plus the vertical inset.
	r := NewSettingRow("Only text", nil)
	want := GlyphHeight() + 2*scaled(SettingRowPadY)
	if got := r.Measure(300); got != want {
		t.Fatalf("Measure = %d, want %d", got, want)
	}
}

func TestSettingRowMeasureWithSubtitle(t *testing.T) {
	r := NewSettingRow("Title", nil)
	r.Subtitle = "desc"
	want := 2*GlyphHeight() + scaled(SettingRowSubtitleGap) + 2*scaled(SettingRowPadY)
	if got := r.Measure(300); got != want {
		t.Fatalf("Measure(subtitle) = %d, want %d", got, want)
	}
}

func TestSettingRowMeasureControlDominant(t *testing.T) {
	// A tall control drives the height when it exceeds the text block.
	c := &arChild{}
	c.SetBounds(Rect{W: 40, H: 100})
	r := NewSettingRow("t", c)
	want := 100 + 2*scaled(SettingRowPadY)
	if got := r.Measure(300); got != want {
		t.Fatalf("Measure(tall control) = %d, want %d", got, want)
	}
}

// --- Draw: body, control placement, divider ------------------------------

func TestSettingRowDrawBodyControlAndDivider(t *testing.T) {
	const w, h = 200, 44
	theme := DefaultLight()
	c := &arChild{}
	c.SetBounds(Rect{W: 40, H: 24})
	r := NewSettingRow("Title", c)
	r.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	r.Draw(newP(buf, w), theme)

	// Surface body somewhere with no glyph/control ink.
	if got := pixelAt(buf, w, w/2, h/2); got != theme.Surface {
		t.Fatalf("body pixel = %+v, want Surface", got)
	}
	// Bottom divider in Border.
	if got := pixelAt(buf, w, w/2, h-1); got != theme.Border {
		t.Fatalf("divider = %+v, want Border", got)
	}
	// Control drawn once and placed right-aligned, vertically centred.
	if c.draws != 1 {
		t.Fatalf("control draws = %d, want 1", c.draws)
	}
	cb := c.Bounds()
	wantX := w - scaled(SettingRowPadX) - 40
	if cb.X != wantX || cb.W != 40 || cb.H != 24 || cb.Y != (h-24)/2 {
		t.Fatalf("control bounds = %+v, want X=%d Y=%d W=40 H=24",
			cb, wantX, (h-24)/2)
	}
	// Title ink present at the leading edge.
	blockH := GlyphHeight()
	ty := (h - blockH) / 2
	if !inkFound(buf, w, scaled(SettingRowPadX), ty, TextWidth("Title"), GlyphHeight(), theme.OnSurface) {
		t.Fatal("title ink missing at leading edge")
	}
}

func TestSettingRowDrawWithSubtitle(t *testing.T) {
	const w, h = 200, 52
	theme := DefaultLight()
	r := NewSettingRow("Title", nil)
	r.Subtitle = "sub"
	r.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	r.Draw(newP(buf, w), theme)

	blockH := 2*GlyphHeight() + scaled(SettingRowSubtitleGap)
	ty := (h - blockH) / 2
	sy := ty + GlyphHeight() + scaled(SettingRowSubtitleGap)
	if !inkFound(buf, w, scaled(SettingRowPadX), sy, TextWidth("sub"), GlyphHeight(), dimInk(theme)) {
		t.Fatal("subtitle ink missing at expected location")
	}
}

func TestSettingRowDrawNoDivider(t *testing.T) {
	const w, h = 200, 44
	theme := DefaultLight()
	r := NewSettingRow("Title", nil)
	r.Divider = false
	r.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	r.Draw(newP(buf, w), theme)
	// Bottom row stays Surface with the divider off.
	if got := pixelAt(buf, w, w/2, h-1); got != theme.Surface {
		t.Fatalf("no-divider bottom = %+v, want Surface", got)
	}
}

func TestSettingRowDrawNoControlDarkTheme(t *testing.T) {
	const w, h = 160, 44
	theme := DefaultDark()
	r := NewSettingRow("Dark", nil)
	r.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	r.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, w/2, h/2); got != theme.Surface {
		t.Fatalf("dark body = %+v, want Surface", got)
	}
}

// --- OnEvent: forwarding into the trailing control -----------------------

func TestSettingRowClickTogglesSwitch(t *testing.T) {
	const w, h = 200, 44
	theme := DefaultLight()
	sw := NewSwitch(false)
	r := NewSettingRow("Wi-Fi", sw)
	r.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	r.Draw(newP(buf, w), theme) // positions the switch

	cr := sw.Bounds()
	// Click the centre of the switch slot: it must toggle.
	r.OnEvent(Event{Kind: EventClick, X: cr.X + cr.W/2, Y: cr.Y + cr.H/2})
	if !sw.On().Get() {
		t.Fatal("click in the switch rect did not toggle it")
	}
	// A click on the label column is dropped (switch state unchanged).
	r.OnEvent(Event{Kind: EventClick, X: scaled(SettingRowPadX), Y: h / 2})
	if !sw.On().Get() {
		t.Fatal("label-column click reached the switch")
	}
}

func TestSettingRowForwardsKeyboardToControl(t *testing.T) {
	sw := NewSwitch(false)
	r := NewSettingRow("Wi-Fi", sw)
	r.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 44})
	// Non-click events are forwarded unconditionally; Space flips the switch.
	r.OnEvent(Event{Kind: EventKeyDown, Code: "Space"})
	if !sw.On().Get() {
		t.Fatal("keyboard event not forwarded to the control")
	}
}

func TestSettingRowClickScaleScrubs(t *testing.T) {
	const w, h = 200, 44
	theme := DefaultLight()
	sc := NewScale(0, 100, 0)
	sc.SetBounds(Rect{W: 120, H: 24})
	r := NewSettingRow("Volume", sc)
	r.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	r.Draw(newP(buf, w), theme)

	cr := sc.Bounds()
	// Click near the right end of the track: the value must move off zero.
	r.OnEvent(Event{Kind: EventClick, X: cr.X + cr.W - 4, Y: cr.Y + cr.H/2})
	if sc.Value().Get() <= 0 {
		t.Fatalf("scale value = %v, want > 0 after a right-end click", sc.Value().Get())
	}
}

func TestSettingRowNilControlEventsAreNoOp(t *testing.T) {
	r := NewSettingRow("t", nil)
	r.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 44})
	// No control: neither a click nor a key press panics or does anything.
	r.OnEvent(Event{Kind: EventClick, X: 10, Y: 20})
	r.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
}

// --- a11y ----------------------------------------------------------------

func TestSettingRowA11yAndChildren(t *testing.T) {
	sw := NewSwitch(true)
	r := NewSettingRow("Wi-Fi", sw)
	if info := r.A11y(); info.Role != RoleGroup || info.Name != "Wi-Fi" {
		t.Fatalf("A11y = %+v, want group named Wi-Fi", info)
	}
	kids := r.Children()
	if len(kids) != 1 || kids[0] != sw {
		t.Fatalf("Children = %v, want [switch]", kids)
	}
	// A label-only row exposes no children.
	if kids := NewSettingRow("t", nil).Children(); len(kids) != 0 {
		t.Fatalf("label-only Children = %v, want empty", kids)
	}
}

// inkFound reports whether ink appears anywhere in the [x,x+w)×[y,y+h) box.
func inkFound(buf []byte, stride, x, y, w, h int, ink RGBA) bool {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			if pixelAt(buf, stride, xx, yy) == ink {
				return true
			}
		}
	}
	return false
}
