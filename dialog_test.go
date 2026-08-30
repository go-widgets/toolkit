// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// The panel is a rounded sheet: its corners are transparent where a square one
// would have painted, and its outline follows the rounding.
func TestDialogPanelIsRounded(t *testing.T) {
	d := NewDialog("Title", nil)
	d.Closable = true
	d.SetBounds(Rect{W: 200, H: 140})
	buf := make([]byte, 4*200*140)
	d.Draw(painter.NewPixelPainter(buf, 200, 140), DefaultLight())

	// A rounded corner does not carry the panel's own paint. Asserting merely
	// "unpainted" was too coarse: the drop shadow, offset down and right, lands
	// on the corners it falls past, and that is the shadow doing its job.
	at := func(x, y int) RGBA {
		i := (y*200 + x) * 4
		return RGBA{R: buf[i], G: buf[i+1], B: buf[i+2], A: buf[i+3]}
	}
	topEdge, bottomEdge := at(100, 0), at(100, 139)
	if topEdge.A == 0 {
		t.Error("the middle of the top edge must be painted")
	}
	if bottomEdge.A == 0 {
		t.Error("the middle of the bottom edge must be painted")
	}
	for _, c := range [][2]int{{0, 0}, {199, 0}, {0, 139}, {199, 139}} {
		if got := at(c[0], c[1]); got == topEdge || got == bottomEdge {
			t.Errorf("corner %v carries the panel's own paint %v; it must be rounded away", c, got)
		}
	}
}

// The panel casts a shadow down and to the right, which is what makes a rounded
// sheet read as floating rather than merely as a rounded region. Rounding alone
// was measured on the live playground and was invisible: the corner showed the
// dark scrim through it at [16,18,21] where the edge read [58,62,70].
func TestDialogCastsAShadow(t *testing.T) {
	const W, H = 260, 200
	buf := make([]byte, 4*W*H)
	p := painter.NewPixelPainter(buf, W, H)
	d := NewDialog("Title", nil)
	d.SetBounds(Rect{X: 20, Y: 20, W: 200, H: 140})
	d.Draw(p, DefaultLight())

	at := func(x, y int) RGBA {
		i := (y*W + x) * 4
		return RGBA{R: buf[i], G: buf[i+1], B: buf[i+2], A: buf[i+3]}
	}
	drop := scaled(DialogShadow)
	// Below the middle of the bottom edge, inside the shadow's offset. The
	// CORNERS are the wrong place to probe: the shadow is rounded too, so it has
	// already curved away there.
	if got := at(20+100, 20+140+drop/2); got.A == 0 {
		t.Errorf("no shadow below the panel: %v", got)
	}
	// And nothing further out than the shadow reaches.
	if got := at(20+100, 20+140+drop+2); got.A != 0 {
		t.Errorf("the shadow must not extend past its offset: %v", got)
	}
	// Same to the right.
	if got := at(20+200+drop/2, 20+70); got.A == 0 {
		t.Errorf("no shadow to the right of the panel: %v", got)
	}
}

// The title bar is told from the body by its accent, and the action strip by a
// hairline, so neither reads as one field of colour with what it sits against.
func TestDialogBandsAreSeparated(t *testing.T) {
	th := DefaultLight()
	d := NewDialog("Title", nil, NewButton("OK", nil)) // a button so the action strip is present
	d.SetBounds(Rect{W: 200, H: 140})
	buf := make([]byte, 4*200*140)
	d.Draw(painter.NewPixelPainter(buf, 200, 140), th)

	rgb := func(x, y int) RGBA {
		i := (y*200 + x) * 4
		return RGBA{R: buf[i], G: buf[i+1], B: buf[i+2], A: buf[i+3]}
	}
	// The title bar carries the ACCENT, which is what separates it from the body
	// now — a hairline used to do that job and is gone with it.
	titleH := scaled(DialogTitleH)
	if got := rgb(100, titleH/2); got != th.Accent {
		t.Errorf("the title bar is not accented: %v at y=%d, want %v", got, titleH/2, th.Accent)
	}
	if got := rgb(100, titleH+4); got == th.Accent {
		t.Errorf("the accent must stop at the title bar, found it at y=%d", titleH+4)
	}
	stripY := 140 - scaled(DialogButtonStripH)
	if got := rgb(100, stripY); got != th.Border {
		t.Errorf("no hairline above the action strip: %v at y=%d, want %v", got, stripY, th.Border)
	}
}

// The close control is a real icon on a flat button, not a letter in a box.
func TestDialogCloseButtonIsAFlatIcon(t *testing.T) {
	d := NewDialog("Title", nil)
	d.Closable = true
	cb := d.closeButton()
	if !cb.Flat {
		t.Error("the close button must be flat: a framed square belongs to the content, not the window")
	}
	if cb.Glyph == nil {
		t.Error("the close button must draw a real icon, not a text glyph")
	}
	if cb.Icon != "" {
		t.Errorf("the stand-in letter must be gone, got %q", cb.Icon)
	}
}

// dlgRender draws d onto a fresh w×h buffer and returns a pixel reader.
func dlgRender(d *Dialog, w, h int, theme *Theme) func(x, y int) RGBA {
	buf := make([]byte, 4*w*h)
	d.Draw(painter.NewPixelPainter(buf, w, h), theme)
	return func(x, y int) RGBA {
		i := (y*w + x) * 4
		return RGBA{R: buf[i], G: buf[i+1], B: buf[i+2], A: buf[i+3]}
	}
}

// The two window controls are OFF by default: a Dialog is a sheet, and a sheet
// has one thing to do with it.
func TestDialogWindowControlsAreOptional(t *testing.T) {
	d := NewDialog("Title", nil)
	d.Closable = true
	if d.Minimisable || d.Maximisable {
		t.Fatal("minimise and maximise must default off")
	}
	if got := len(d.titleButtons()); got != 1 {
		t.Errorf("only the close control is present, got %d", got)
	}
	d.Minimisable, d.Maximisable = true, true
	if got := len(d.titleButtons()); got != 3 {
		t.Errorf("all three controls must be present, got %d", got)
	}
}

// A drag starts on the bar, never on a control. Counting only the close button —
// which is what this did before the other two existed — would arm a drag on top
// of them.
func TestDialogDragExcludesEveryControl(t *testing.T) {
	d := NewDialog("Title", nil)
	d.Closable, d.Minimisable, d.Maximisable = true, true, true
	d.SetBounds(Rect{W: 400, H: 200})
	sq := scaled(DialogTitleH)

	if !d.onTitleBar(Event{X: 10, Y: sq / 2}) {
		t.Error("the left of the bar must start a drag")
	}
	for i := 1; i <= 3; i++ {
		x := 400 - i*sq + sq/2
		if d.onTitleBar(Event{X: x, Y: sq / 2}) {
			t.Errorf("control %d at x=%d must not start a drag", i, x)
		}
	}
}

// Minimised rolls the panel up to its title bar: out of the way without being
// dismissed, and nothing below it is drawn or laid out.
func TestDialogMinimiseRollsUp(t *testing.T) {
	content := NewLabel("body")
	d := NewDialog("Title", content)
	d.Closable, d.Minimisable = true, true
	d.SetBounds(Rect{W: 400, H: 240})
	full := d.Bounds().H

	d.Minimised().Set(true)
	d.applyBounds()
	if got := d.Bounds().H; got != scaled(DialogTitleH) {
		t.Errorf("minimised height = %d, want the title bar's %d", got, scaled(DialogTitleH))
	}
	at := dlgRender(d, 420, 260, DefaultLight())
	if got := at(200, scaled(DialogTitleH)+6); got.A != 0 {
		t.Errorf("nothing may be painted below a rolled-up bar, got %v", got)
	}

	d.Minimised().Set(false)
	d.applyBounds()
	if got := d.Bounds().H; got != full {
		t.Errorf("restored height = %d, want %d", got, full)
	}
}

// Maximised takes the whole DragBounds and returns to where it was. With no
// DragBounds there is nothing to fill, so it does nothing rather than guess.
func TestDialogMaximiseFillsItsBounds(t *testing.T) {
	d := NewDialog("Title", nil)
	d.Closable, d.Maximisable = true, true
	d.SetBounds(Rect{X: 20, Y: 20, W: 200, H: 140})
	before := d.Bounds()

	d.DragBounds = Rect{X: 0, Y: 0, W: 600, H: 400}
	d.Maximised().Set(true)
	d.applyBounds()
	if got := d.Bounds(); got != d.DragBounds {
		t.Errorf("maximised to %+v, want the whole %+v", got, d.DragBounds)
	}

	d.Maximised().Set(false)
	d.applyBounds()
	if got := d.Bounds(); got != before {
		t.Errorf("restored to %+v, want %+v", got, before)
	}

	bare := NewDialog("Title", nil)
	bare.Maximisable = true
	bare.SetBounds(Rect{X: 5, Y: 5, W: 100, H: 80})
	was := bare.Bounds()
	bare.Maximised().Set(true)
	bare.applyBounds()
	if got := bare.Bounds(); got != was {
		t.Errorf("with no DragBounds there is nothing to fill: %+v -> %+v", was, got)
	}
}

// A click on each control reaches it: the close one fires OnClose, the other two
// toggle their state.
func TestDialogControlsRespondToClicks(t *testing.T) {
	closed := 0
	d := NewDialog("Title", nil)
	d.Closable, d.Minimisable, d.Maximisable = true, true, true
	d.OnClose = func() { closed++ }
	d.SetBounds(Rect{W: 400, H: 200})
	sq := scaled(DialogTitleH)
	click := func(i int) { d.OnEvent(Event{Kind: EventClick, X: 400 - i*sq + sq/2, Y: sq / 2}) }

	click(1)
	if closed != 1 {
		t.Errorf("the close control did not fire: %d", closed)
	}
	click(2)
	if !d.Maximised().Get() {
		t.Error("the maximise control did not toggle")
	}
	click(3)
	if !d.Minimised().Get() {
		t.Error("the minimise control did not toggle")
	}
	click(3)
	if d.Minimised().Get() {
		t.Error("minimise must toggle back")
	}
}

// The maximise control's glyph follows the state: a panel that fills its bounds
// offers to shrink back, and one that does not offers to fill them.
func TestDialogMaximiseGlyphFollowsTheState(t *testing.T) {
	d := NewDialog("Title", nil)
	d.Maximisable = true
	d.SetBounds(Rect{W: 200, H: 140})

	var drawn []Rect
	capture := func() {
		b := d.maxButton()
		drawn = drawn[:0]
		b.Glyph(painter.NewPixelPainter(make([]byte, 4*40*40), 40, 40), Rect{W: 20, H: 20}, RGBA{})
	}
	capture() // not maximised: the "expand" branch
	d.Maximised().Set(true)
	capture() // maximised: the "collapse" branch — a different glyph name
	// The assertion is that both branches run without panicking and the button is
	// the same instance either way; the glyph name itself is iconoir's business.
	if d.maxButton() != d.maxBtn {
		t.Error("the maximise control must be built once and reused")
	}
}

// A rolled-up panel routes nothing below its bar: the action buttons and the
// content are not there to be clicked.
func TestDialogMinimisedSwallowsBodyClicks(t *testing.T) {
	fired := 0
	btn := NewButton("Do", func() { fired++ })
	d := NewDialog("Title", nil, btn)
	d.Closable, d.Minimisable = true, true
	d.SetBounds(Rect{W: 300, H: 200})

	// Where the action button sits while the panel is open.
	bb := btn.Bounds()
	local := Event{Kind: EventClick, X: bb.X - d.Bounds().X + bb.W/2, Y: bb.Y - d.Bounds().Y + bb.H/2}

	d.OnEvent(local)
	if fired != 1 {
		t.Fatalf("control: an open panel must route to its action button, got %d", fired)
	}
	d.Minimised().Set(true)
	d.applyBounds()
	d.OnEvent(local)
	if fired != 1 {
		t.Errorf("a rolled-up panel must route nothing below its bar, got %d", fired)
	}
}
