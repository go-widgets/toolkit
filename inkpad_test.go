// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"errors"
	"image/png"
	"io"
	"testing"

	"github.com/go-gfx/gfx/raster"
)

// stroke draws one stroke through the pad's own event path: a press, a drag to
// each point and the release.
func stroke(p *InkPad, pts ...[2]int) {
	p.OnEvent(Event{Kind: EventClick, X: pts[0][0], Y: pts[0][1]})
	for _, q := range pts[1:] {
		p.OnEvent(Event{Kind: EventMouseDrag, X: q[0], Y: q[1]})
	}
	p.OnEvent(Event{Kind: EventMouseUp, X: pts[len(pts)-1][0], Y: pts[len(pts)-1][1]})
}

func sizedPad(w, h int) *InkPad {
	p := NewInkPad()
	p.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	return p
}

func TestInkPadStrokeLifecycle(t *testing.T) {
	p := sizedPad(100, 50)
	if !p.Empty().Get() || p.Strokes().Len() != 0 || p.PointerGrabbed() {
		t.Fatal("a new pad must be empty and hold no pointer")
	}
	var changes int
	p.Strokes().SubscribeChanged(func() { changes++ })

	p.OnEvent(Event{Kind: EventClick, X: 10, Y: 25})
	if !p.PointerGrabbed() || p.Empty().Get() || p.Strokes().Len() != 1 {
		t.Fatalf("a press must open a stroke: grabbed=%v empty=%v n=%d", p.PointerGrabbed(), p.Empty().Get(), p.Strokes().Len())
	}
	p.OnEvent(Event{Kind: EventMouseDrag, X: 50, Y: 25})
	p.OnEvent(Event{Kind: EventMouseUp, X: 50, Y: 25})
	if p.PointerGrabbed() {
		t.Fatal("the release must lift the pen")
	}
	got := p.Strokes().At(0)
	want := InkStroke{{0.1, 0.5}, {0.5, 0.5}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("stroke = %v, want %v (normalised across the box)", got, want)
	}
	if changes != 2 {
		t.Fatalf("subscribers told %d times, want 2 (the press, the kept point)", changes)
	}
	// A drag with no press open is nothing.
	p.OnEvent(Event{Kind: EventMouseDrag, X: 90, Y: 40})
	if len(p.Strokes().At(0)) != 2 {
		t.Fatal("a drag after the release must not extend the stroke")
	}
}

func TestInkPadDropsPointsUnderAPixelApart(t *testing.T) {
	p := sizedPad(100, 50)
	stroke(p, [2]int{10, 10}, [2]int{10, 10}, [2]int{11, 10}, [2]int{11, 11})
	// (10,10) press; (10,10) is 0 apart; (11,10) is exactly one pixel: kept;
	// (11,11) is one pixel from that: kept.
	if n := len(p.Strokes().At(0)); n != 3 {
		t.Fatalf("kept %d points, want 3", n)
	}
	q := sizedPad(100, 50)
	stroke(q, [2]int{10, 10}, [2]int{10, 10})
	if n := len(q.Strokes().At(0)); n != 1 {
		t.Fatalf("a repeated point was kept: %d points", n)
	}
	// The step is measured in pixels, not in normalised units: 0.5 px apart on a
	// 100 px box is a normalised 0.005.
	r := sizedPad(1000, 500)
	stroke(r, [2]int{10, 10}, [2]int{10, 10})
	r.OnEvent(Event{Kind: EventClick, X: 20, Y: 20})
	r.move(20, 20)
	if n := len(r.Strokes().At(1)); n != 1 {
		t.Fatalf("zero movement kept a point: %d", n)
	}
}

func TestInkPadClampsToTheBox(t *testing.T) {
	p := sizedPad(100, 50)
	stroke(p, [2]int{-30, -30}, [2]int{500, 500})
	s := p.Strokes().At(0)
	if s[0] != (InkPoint{0, 0}) || s[1] != (InkPoint{1, 1}) {
		t.Fatalf("an out-of-box pointer must end at the edge, got %v", s)
	}
}

func TestInkPadBeforeLayoutUsesANominalBox(t *testing.T) {
	p := NewInkPad()
	stroke(p, [2]int{150, 60})
	if got := p.Strokes().At(0)[0]; got != (InkPoint{0.5, 0.5}) {
		t.Fatalf("nominal 300x120 box: point = %v, want {0.5 0.5}", got)
	}
}

func TestInkPadUndoOnAnEmptyPadIsARefusal(t *testing.T) {
	p := sizedPad(100, 50)
	var told int
	p.OnFull = func(InkLimit) { told++ }
	refusesWithoutPanic(t, "Undo on an empty pad", func() {
		if p.Undo() {
			t.Error("Undo on an empty pad must answer false")
		}
	})
	if !p.Empty().Get() || p.Strokes().Len() != 0 || told != 0 {
		t.Fatal("Undo on an empty pad must change nothing")
	}
}

func TestInkPadUndoTakesTheLastStrokeBack(t *testing.T) {
	p := sizedPad(100, 50)
	stroke(p, [2]int{10, 10}, [2]int{30, 10})
	stroke(p, [2]int{10, 30}, [2]int{30, 30}, [2]int{50, 30})
	if p.points != 5 {
		t.Fatalf("points = %d, want 5", p.points)
	}
	if !p.Undo() {
		t.Fatal("Undo with ink must answer true")
	}
	if p.points != 2 || p.Strokes().Len() != 1 || p.Empty().Get() {
		t.Fatalf("after one Undo: points=%d strokes=%d empty=%v", p.points, p.Strokes().Len(), p.Empty().Get())
	}
	if !p.Undo() || !p.Empty().Get() || p.points != 0 {
		t.Fatal("the last Undo must leave the pad empty")
	}
}

func TestInkPadUndoLiftsAnOpenPen(t *testing.T) {
	p := sizedPad(100, 50)
	p.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	p.Undo()
	if p.PointerGrabbed() {
		t.Fatal("undoing the stroke being drawn must lift the pen")
	}
	p.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: 40})
	if p.Strokes().Len() != 0 {
		t.Fatal("a drag after the undo resurrected the stroke")
	}
}

func TestInkPadClear(t *testing.T) {
	p := sizedPad(100, 50)
	stroke(p, [2]int{10, 10}, [2]int{30, 10})
	p.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	p.Clear()
	if !p.Empty().Get() || p.Strokes().Len() != 0 || p.points != 0 || p.PointerGrabbed() {
		t.Fatal("Clear must empty the pad and lift the pen")
	}
	p.Clear() // clearing an empty pad is harmless
	if !p.Empty().Get() {
		t.Fatal("Clear on an empty pad broke Empty")
	}
}

func TestInkPadStrokeCapRefusesOnceThenUndoRearms(t *testing.T) {
	p := sizedPad(100, 50)
	p.MaxStrokes = 2
	var told []InkLimit
	p.OnFull = func(l InkLimit) { told = append(told, l) }
	stroke(p, [2]int{10, 10}, [2]int{20, 10})
	stroke(p, [2]int{10, 20}, [2]int{20, 20})
	stroke(p, [2]int{10, 30}, [2]int{20, 30})
	stroke(p, [2]int{10, 40}, [2]int{20, 40})
	if p.Strokes().Len() != 2 {
		t.Fatalf("the cap let %d strokes in, want 2", p.Strokes().Len())
	}
	if len(told) != 1 || told[0] != InkStrokesFull {
		t.Fatalf("OnFull = %v, want one InkStrokesFull", told)
	}
	if p.PointerGrabbed() {
		t.Fatal("a refused press must not hold the pointer")
	}
	p.Undo()
	stroke(p, [2]int{10, 30}, [2]int{20, 30})
	stroke(p, [2]int{10, 40}, [2]int{20, 40})
	if len(told) != 2 {
		t.Fatalf("Undo must re-arm the callback: told %d times", len(told))
	}
	p.Clear()
	if p.warned {
		t.Fatal("Clear must re-arm the callback")
	}
}

func TestInkPadPointCapOnMoveAndOnPress(t *testing.T) {
	p := sizedPad(100, 50)
	p.MaxPoints = 3
	var told []InkLimit
	p.OnFull = func(l InkLimit) { told = append(told, l) }
	stroke(p, [2]int{10, 10}, [2]int{20, 10}, [2]int{30, 10}, [2]int{40, 10}, [2]int{50, 10})
	if p.points != 3 || len(p.Strokes().At(0)) != 3 {
		t.Fatalf("points = %d, want the cap 3", p.points)
	}
	if len(told) != 1 || told[0] != InkPointsFull {
		t.Fatalf("OnFull = %v, want one InkPointsFull", told)
	}
	stroke(p, [2]int{10, 30}, [2]int{20, 30})
	if p.Strokes().Len() != 1 || len(told) != 1 {
		t.Fatal("a press on a pad full of points must be refused, and say so once")
	}
	q := sizedPad(100, 50)
	q.MaxPoints = 1
	stroke(q, [2]int{10, 10})
	stroke(q, [2]int{20, 20}) // no callback set: nothing to call, nothing to crash
	if q.Strokes().Len() != 1 {
		t.Fatal("a second press past the point cap was accepted")
	}
}

func TestInkPadDefaultCapsAndColours(t *testing.T) {
	p := NewInkPad()
	if p.maxStrokes() != InkMaxStrokes || p.maxPoints() != InkMaxPoints {
		t.Fatalf("defaults = %d/%d", p.maxStrokes(), p.maxPoints())
	}
	p.MaxStrokes, p.MaxPoints = 5, 7
	if p.maxStrokes() != 5 || p.maxPoints() != 7 {
		t.Fatal("configured caps ignored")
	}
	if p.ink() != defaultInk || p.paper() != defaultPaper {
		t.Fatal("default colours")
	}
	p.Ink, p.Paper = RGB(1, 2, 3), RGB(4, 5, 6)
	if p.ink() != RGB(1, 2, 3) || p.paper() != RGB(4, 5, 6) {
		t.Fatal("configured colours ignored")
	}
	if InkMaxStrokes != 64 || InkMaxPoints != 8192 {
		t.Fatal("the documented defaults moved")
	}
}

func TestInkPadTouchEvents(t *testing.T) {
	p := sizedPad(100, 50)
	p.OnEvent(Event{Kind: EventTouchStart, X: 10, Y: 10})
	p.OnEvent(Event{Kind: EventTouchMove, X: 40, Y: 10})
	if !p.PointerGrabbed() || len(p.Strokes().At(0)) != 2 {
		t.Fatal("touch start and move must draw")
	}
	p.OnEvent(Event{Kind: EventTouchEnd, X: 40, Y: 10})
	if p.PointerGrabbed() {
		t.Fatal("touch end must lift the pen")
	}
	p.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"}) // not a pointer event
	if p.Strokes().Len() != 1 {
		t.Fatal("a key drew")
	}
}

func TestInkPadDisabledTakesNothing(t *testing.T) {
	p := sizedPad(100, 50)
	p.Disabled().Set(true)
	stroke(p, [2]int{10, 10}, [2]int{30, 10})
	if !p.Empty().Get() {
		t.Fatal("a disabled pad drew")
	}
	if a := p.A11y(); !a.Disabled {
		t.Fatal("a disabled pad must announce itself disabled")
	}
}

func TestInkPadA11y(t *testing.T) {
	p := sizedPad(100, 50)
	if a := p.A11y(); a.Role != RoleGroup || a.Value != "empty" || a.Disabled {
		t.Fatalf("empty pad a11y = %+v", a)
	}
	stroke(p, [2]int{10, 10}, [2]int{30, 10})
	if a := p.A11y(); a.Value != "drawn" {
		t.Fatalf("drawn pad a11y = %+v", a)
	}
}

// --- drawing -------------------------------------------------------------

func TestInkPadDrawPaintsTheInkOnAPathPainter(t *testing.T) {
	theme := DefaultLight()
	p := NewInkPad()
	p.SetBounds(Rect{X: 5, Y: 5, W: 100, H: 50})
	stroke(p, [2]int{10, 20}, [2]int{30, 20}, [2]int{60, 20})
	surf := makeSurface(120, 70)
	p.Draw(newP(surf, 120), theme)
	if got := pixelAt(surf, 120, 5+30, 5+20); got != defaultInk {
		t.Fatalf("ink pixel = %+v, want the pen %+v", got, defaultInk)
	}
	if got := pixelAt(surf, 120, 50, 12); got != defaultPaper {
		t.Fatalf("paper pixel = %+v, want %+v", got, defaultPaper)
	}
	if got := pixelAt(surf, 120, 30, 5+50*3/4); got != theme.Border {
		t.Fatalf("baseline pixel = %+v, want the theme border %+v", got, theme.Border)
	}
}

func TestInkPadDrawDisabledIsMuted(t *testing.T) {
	p := NewInkPad()
	p.SetBounds(Rect{X: 5, Y: 5, W: 100, H: 50})
	stroke(p, [2]int{10, 20}, [2]int{60, 20})
	p.Disabled().Set(true)
	surf := makeSurface(120, 70)
	p.Draw(newP(surf, 120), DefaultLight())
	got := pixelAt(surf, 120, 5+30, 5+20)
	if got == defaultInk || got == defaultPaper {
		t.Fatalf("a disabled pad must draw a muted ink, got %+v", got)
	}
}

// refusesWithoutPanic runs f and fails the test, by an assertion, if it panics:
// the guards below are what keep these calls from crashing, and a crash must
// read as a broken guard rather than as an aborted test binary.
func refusesWithoutPanic(t *testing.T, what string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s panicked: %v", what, r)
		}
	}()
	f()
}

func TestInkPadGuardsAnswerInsteadOfPanicking(t *testing.T) {
	refusesWithoutPanic(t, "Undo on an empty pad", func() {
		p := NewInkPad()
		if p.Undo() {
			t.Error("Undo on an empty pad answered true")
		}
		if !p.Empty().Get() || p.Strokes().Len() != 0 {
			t.Error("Undo on an empty pad changed it")
		}
	})
	refusesWithoutPanic(t, "PNG of an empty pad", func() {
		if data, err := sizedPad(100, 50).PNG(); !errors.Is(err, ErrInkEmpty) || data != nil {
			t.Errorf("PNG of an empty pad = %d bytes, %v; want a refusal", len(data), err)
		}
	})
	refusesWithoutPanic(t, "Draw on a painter without paths", func() {
		p := NewInkPad()
		p.SetBounds(Rect{X: 5, Y: 5, W: 100, H: 50})
		stroke(p, [2]int{10, 20}, [2]int{30, 20})
		p.Draw(&nonPathPainter{buf: makeSurface(120, 70), w: 120, h: 70}, DefaultLight())
	})
}

func TestInkPadDrawWithoutPathPainterDrawsTheFrameOnly(t *testing.T) {
	theme := DefaultLight()
	p := NewInkPad()
	p.SetBounds(Rect{X: 5, Y: 5, W: 100, H: 50})
	stroke(p, [2]int{10, 20}, [2]int{30, 20}, [2]int{60, 20})
	np := &nonPathPainter{buf: makeSurface(120, 70), w: 120, h: 70}
	p.Draw(np, theme)
	px := func(x, y int) RGBA {
		o := (y*np.w + x) * 4
		return RGBA{R: np.buf[o], G: np.buf[o+1], B: np.buf[o+2], A: np.buf[o+3]}
	}
	if got := px(5+30, 5+20); got != defaultPaper {
		t.Fatalf("a painter without paths drew ink: %+v", got)
	}
	if got := px(30, 5+50*3/4); got != theme.Border {
		t.Fatalf("the frame's baseline = %+v, want %+v", got, theme.Border)
	}
}

// --- the picture ---------------------------------------------------------

func TestInkPadExportRefusesAnEmptyPad(t *testing.T) {
	p := sizedPad(100, 50)
	if _, err := p.Image(); !errors.Is(err, ErrInkEmpty) {
		t.Fatalf("Image on an empty pad: %v", err)
	}
	if data, err := p.PNG(); !errors.Is(err, ErrInkEmpty) || data != nil {
		t.Fatalf("PNG on an empty pad: %v, %d bytes", err, len(data))
	}
}

func TestInkPadPNGIsCroppedToTheInkOnATransparentGround(t *testing.T) {
	p := sizedPad(300, 120)
	stroke(p, [2]int{100, 40}, [2]int{150, 60}, [2]int{200, 40})
	data, err := p.PNG()
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("not a PNG: %v", err)
	}
	b := img.Bounds()
	// The ink spans 100/300 of the 900 px picture across and 20/120 of 360 down,
	// so the cropped picture is about 300 x 60 px, not 900 x 360.
	if b.Dx() < 300 || b.Dx() > 320 || b.Dy() < 60 || b.Dy() > 80 {
		t.Fatalf("picture is %dx%d, want about 300x60 (cropped to the ink)", b.Dx(), b.Dy())
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Fatalf("the ground must be transparent, corner alpha = %d", a)
	}
	var ink bool
	for y := b.Min.Y; y < b.Max.Y && !ink; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a == 0xffff {
				ink = true
				break
			}
		}
	}
	if !ink {
		t.Fatal("no opaque ink pixel in the picture")
	}
}

func TestInkPadImageIsBounded(t *testing.T) {
	// A very tall box: the picture's height is capped, however the pad was sized.
	p := sizedPad(100, 5000)
	stroke(p, [2]int{1, 1}, [2]int{99, 4999})
	img, err := p.Image()
	if err != nil {
		t.Fatal(err)
	}
	if img.W > InkRenderW || img.H > InkRenderMaxH {
		t.Fatalf("picture %dx%d exceeds %dx%d", img.W, img.H, InkRenderW, InkRenderMaxH)
	}
}

// The stated bound holds margin included, for every shape of box and drawing.
func TestInkPadImageNeverExceedsTheBoundWithItsMargin(t *testing.T) {
	for _, c := range []struct{ w, h int }{{300, 300}, {300, 120}, {2000, 100}, {100, 2000}, {10, 10}, {900, 600}, {5000, 5000}} {
		p := sizedPad(c.w, c.h)
		stroke(p, [2]int{0, 0}, [2]int{c.w, c.h}, [2]int{0, c.h}, [2]int{c.w, 0})
		img, err := p.Image()
		if err != nil {
			t.Fatal(err)
		}
		if img.W > InkRenderW || img.H > InkRenderMaxH {
			t.Errorf("%dx%d pad: picture %dx%d exceeds %dx%d", c.w, c.h, img.W, img.H, InkRenderW, InkRenderMaxH)
		}
	}
}

// One scale for both axes: a square drawn on a square pad is a square picture.
func TestInkPadImageKeepsTheDrawingsProportions(t *testing.T) {
	p := sizedPad(300, 300)
	stroke(p, [2]int{50, 50}, [2]int{250, 50}, [2]int{250, 250}, [2]int{50, 250}, [2]int{50, 50})
	img, err := p.Image()
	if err != nil {
		t.Fatal(err)
	}
	if d := img.W - img.H; d < -2 || d > 2 {
		t.Fatalf("a square exported as %dx%d", img.W, img.H)
	}
}

// The pen is 2.5 logical pixels of the pad it was drawn on, whatever its width.
func TestInkPadPenWidthFollowsTheRealBox(t *testing.T) {
	p := sizedPad(600, 100)
	stroke(p, [2]int{100, 50}, [2]int{500, 50})
	img, err := p.Image()
	if err != nil {
		t.Fatal(err)
	}
	// 600 px of box scale to 900: the factor is 1.5, the pen 3.75 px.
	x, rows := img.W/2, 0
	for y := 0; y < img.H; y++ {
		if img.Pix[4*(y*img.W+x)+3] > 127 {
			rows++
		}
	}
	if rows < 3 || rows > 5 {
		t.Fatalf("the line is %d px thick, want about 3.75", rows)
	}
}

// A host that edits the strokes list leaves the pad consistent.
func TestInkPadSurvivesAHostEditingTheList(t *testing.T) {
	p := sizedPad(100, 50)
	p.MaxStrokes, p.MaxPoints = 3, 3
	stroke(p, [2]int{10, 10}, [2]int{20, 20}, [2]int{30, 30})
	p.Strokes().Clear()
	if !p.Empty().Get() || p.points != 0 {
		t.Fatalf("after the host's Clear: empty=%v points=%d", p.Empty().Get(), p.points)
	}
	stroke(p, [2]int{10, 10}, [2]int{20, 20})
	if p.Strokes().Len() != 1 || p.Empty().Get() || len(p.Strokes().At(0)) != 2 {
		t.Fatal("a press after the host's Clear was refused")
	}
	// The host removes or empties the stroke the pen is drawing.
	p.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	refusesWithoutPanic(t, "a drag after the host emptied the list", func() {
		p.Strokes().Clear()
		p.OnEvent(Event{Kind: EventMouseDrag, X: 50, Y: 20})
	})
	if p.PointerGrabbed() {
		t.Fatal("the pen stayed down on a list with no stroke")
	}
	p.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	p.Strokes().Set(0, InkStroke{})
	refusesWithoutPanic(t, "a drag on an emptied stroke", func() {
		p.OnEvent(Event{Kind: EventMouseDrag, X: 50, Y: 20})
	})
	if p.PointerGrabbed() {
		t.Fatal("the pen stayed down on an emptied stroke")
	}
}

// A zero-value pad that a host filled through the list exports at the nominal box.
func TestInkPadZeroValueExportsAtTheNominalBox(t *testing.T) {
	p := &InkPad{}
	p.Strokes().Append(InkStroke{{0.2, 0.2}, {0.8, 0.5}})
	img, err := p.Image()
	if err != nil || img.W == 0 || img.H == 0 || p.Empty().Get() {
		t.Fatalf("image %v, err %v, empty %v", img, err, p.Empty().Get())
	}
}

// Disabling the pad mid-stroke must not leave the pen down.
func TestInkPadReleaseWhileDisabledLiftsThePen(t *testing.T) {
	p := sizedPad(100, 50)
	p.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	p.Disabled().Set(true)
	p.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: 10})
	if p.PointerGrabbed() || len(p.Strokes().At(0)) != 1 {
		t.Fatal("a drag while disabled must lift the pen and add nothing")
	}
	p.Disabled().Set(false)
	p.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	p.Disabled().Set(true)
	p.OnEvent(Event{Kind: EventMouseUp, X: 10, Y: 10})
	if p.PointerGrabbed() {
		t.Fatal("a release while disabled left the pen down")
	}
	p.Disabled().Set(false)
	p.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	p.CancelPointer()
	if p.PointerGrabbed() {
		t.Fatal("CancelPointer must lift the pen")
	}
}

// inkColumn is a VBox{Label, InkPad} in a ScrollView, the pad below the label.
func inkColumn() (*ScrollView, *InkPad) {
	pad := NewInkPad()
	v := NewVBox()
	v.Spacing = 0
	v.AddFixed(NewLabel("x"), 40)
	v.AddFixed(pad, 100)
	sv := NewScrollView(v)
	sv.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	v.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 140})
	sv.SetContentSize(100, 140)
	return sv, pad
}

func TestScrollViewOnlyGrabsTheWidgetThePressLandedOn(t *testing.T) {
	sv, pad := inkColumn()
	sv.OnEvent(Event{Kind: EventClick, X: 10, Y: 60}) // on the pad
	if !pad.PointerGrabbed() {
		t.Fatal("test needs the pad to take the press")
	}
	pad.Disabled().Set(true)
	sv.OnEvent(Event{Kind: EventMouseUp, X: 10, Y: 60})
	pad.Disabled().Set(false)
	// A press on the label, then a drag: the view pans, the pad draws nothing.
	sv.OnEvent(Event{Kind: EventClick, X: 10, Y: 30})
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 10, Y: -10})
	if sv.OffsetY().Get() != 40 {
		t.Fatalf("offset %d, want 40: the pad stole the gesture", sv.OffsetY().Get())
	}
	if pad.Strokes().Len() != 1 || len(pad.Strokes().At(0)) != 1 {
		t.Fatalf("stray ink: %v", pad.Strokes().Slice())
	}
	sv.OnEvent(Event{Kind: EventMouseUp, X: 10, Y: -10})
}

// The pen is genuinely down on the pad, and a press lands on the label.
func TestScrollViewDoesNotGrabAPenDownElsewhere(t *testing.T) {
	sv, pad := inkColumn()
	sv.OnEvent(Event{Kind: EventClick, X: 10, Y: 60}) // the pad's pen goes down
	sv.OnEvent(Event{Kind: EventClick, X: 10, Y: 30}) // the label
	if !pad.PointerGrabbed() {
		t.Fatal("test needs the pad's pen still down")
	}
	if sv.PointerGrabbed() {
		t.Fatal("the view grabbed a pad the press did not land on")
	}
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 10, Y: -10})
	if sv.OffsetY().Get() != 40 {
		t.Fatalf("offset %d, want the pan to 40", sv.OffsetY().Get())
	}
}

func TestScrollViewForgetsAGrabAnotherPressSupersedes(t *testing.T) {
	sv, pad := inkColumn()
	sv.OnEvent(Event{Kind: EventClick, X: 10, Y: 60}) // grabbed, and the release is lost
	pad.CancelPointer()                               // the pad lets go on its own
	sv.OnEvent(Event{Kind: EventClick, X: 10, Y: 30})
	if sv.PointerGrabbed() {
		t.Fatal("a press on the label must end the old grab")
	}
}

func TestPointerRouterCancelLetsTheGrabbedPadGo(t *testing.T) {
	sv, pad := inkColumn()
	r := &PointerRouter{Targets: func() []Widget { return []Widget{sv} }}
	r.Press(10, 60, "")
	if !pad.PointerGrabbed() || !sv.PointerGrabbed() {
		t.Fatal("test needs the press to be grabbed")
	}
	r.Cancel()
	if pad.PointerGrabbed() || sv.PointerGrabbed() || r.Grabbed() != nil {
		t.Fatal("Cancel must let the pad, the view and the router go")
	}
	r.Press(10, 30, "")
	r.Drag(10, -10)
	if sv.OffsetY().Get() != 40 || len(pad.Strokes().At(0)) != 1 {
		t.Fatalf("after Cancel: offset %d, ink %v; want the pan and no ink", sv.OffsetY().Get(), pad.Strokes().Slice())
	}
	r.Release(10, -10)
}

func TestInkPadAFullStopIsAMark(t *testing.T) {
	p := sizedPad(100, 50)
	stroke(p, [2]int{50, 25})
	img, err := p.Image()
	if err != nil {
		t.Fatal(err)
	}
	var alpha int
	for i := 3; i < len(img.Pix); i += 4 {
		alpha += int(img.Pix[i])
	}
	if alpha == 0 {
		t.Fatal("a one-point stroke drew nothing in the picture")
	}
}

func TestInkPadPNGReportsAnEncoderFailure(t *testing.T) {
	p := sizedPad(100, 50)
	stroke(p, [2]int{10, 10}, [2]int{30, 30})
	boom := errors.New("disk full")
	old := inkEncode
	inkEncode = func(_ io.Writer, _ *raster.Image) error { return boom }
	defer func() { inkEncode = old }()
	if _, err := p.PNG(); !errors.Is(err, boom) {
		t.Fatalf("PNG = %v, want the encoder's error", err)
	}
}

// --- the pointer grab through a ScrollView -------------------------------

// scrolledPad is a pad in a ScrollView taller than its viewport, the viewport
// placed away from the origin so a missing translation shows.
func scrolledPad() (*ScrollView, *InkPad) {
	pad := NewInkPad()
	sv := NewScrollView(pad)
	sv.SetBounds(Rect{X: 20, Y: 30, W: 100, H: 100})
	pad.SetBounds(Rect{X: 20, Y: 30, W: 100, H: 400})
	sv.SetContentSize(100, 400)
	return sv, pad
}

func TestScrollViewHandsTheDragToAGrabbingChild(t *testing.T) {
	sv, pad := scrolledPad()
	sv.OnEvent(Event{Kind: EventClick, X: 10, Y: 20})
	if !pad.PointerGrabbed() || !sv.PointerGrabbed() {
		t.Fatalf("the press must start a stroke (pad=%v) the view reports it holds (view=%v)", pad.PointerGrabbed(), sv.PointerGrabbed())
	}
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 10, Y: 60})
	if sv.OffsetY().Get() != 0 {
		t.Fatalf("a drag that draws scrolled the view to %d", sv.OffsetY().Get())
	}
	sv.OnEvent(Event{Kind: EventMouseUp, X: 10, Y: 60})
	if pad.PointerGrabbed() || sv.PointerGrabbed() {
		t.Fatal("the release must let the pointer go")
	}
	s := pad.Strokes().At(0)
	if len(s) != 2 || s[0] != (InkPoint{0.1, 0.05}) || s[1] != (InkPoint{0.1, 0.15}) {
		t.Fatalf("stroke = %v, want the drag in the pad's own frame", s)
	}
	// With the pointer let go, a press on empty content pans again.
	pad.MaxStrokes = 1
	sv.OnEvent(Event{Kind: EventClick, X: 10, Y: 80}) // refused: the pad is full
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 10, Y: 40})
	if sv.OffsetY().Get() != 40 {
		t.Fatalf("a refused press must leave the drag to the pan: offset %d, want 40", sv.OffsetY().Get())
	}
	sv.OnEvent(Event{Kind: EventMouseUp, X: 10, Y: 40})
}

func TestScrollViewGrabFollowsTheScrolledContent(t *testing.T) {
	sv, pad := scrolledPad()
	sv.Scroll(0, 50)
	// Content point (10, 60) of the pad is under view point (10, 10).
	sv.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 30, Y: 10})
	sv.OnEvent(Event{Kind: EventMouseUp, X: 30, Y: 10})
	s := pad.Strokes().At(0)
	if len(s) != 2 || s[0] != (InkPoint{0.1, 0.15}) || s[1] != (InkPoint{0.3, 0.15}) {
		t.Fatalf("stroke = %v, want the scrolled offset applied to every sample", s)
	}
	if sv.OffsetY().Get() != 50 {
		t.Fatalf("offset moved to %d", sv.OffsetY().Get())
	}
}

func TestScrollViewFindsAGrabberInsideAContainer(t *testing.T) {
	pad := NewInkPad()
	box := NewVBox()
	box.AddFixed(NewLabel("Sign here"), 30)
	box.AddFixed(pad, 200)
	sv := NewScrollView(box)
	sv.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	box.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 400})
	sv.SetContentSize(100, 400)
	pb := pad.Bounds()
	sv.OnEvent(Event{Kind: EventClick, X: pb.X + 10, Y: pb.Y + 10})
	if !pad.PointerGrabbed() {
		t.Fatal("the press did not reach the pad through the VBox")
	}
	sv.OnEvent(Event{Kind: EventMouseDrag, X: pb.X + 50, Y: pb.Y + 10})
	sv.OnEvent(Event{Kind: EventMouseUp, X: pb.X + 50, Y: pb.Y + 10})
	if len(pad.Strokes().At(0)) != 2 || sv.OffsetY().Get() != 0 {
		t.Fatal("the drag did not draw through the container")
	}
}

func TestNestedScrollViewsForwardTheGrab(t *testing.T) {
	pad := NewInkPad()
	inner := NewScrollView(pad)
	inner.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	pad.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 400})
	inner.SetContentSize(100, 400)
	inner.Scroll(0, 40)
	outer := NewScrollView(inner)
	outer.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	inner.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	outer.SetContentSize(100, 100)

	outer.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	outer.OnEvent(Event{Kind: EventMouseDrag, X: 10, Y: 30})
	outer.OnEvent(Event{Kind: EventMouseUp, X: 10, Y: 30})
	s := pad.Strokes().At(0)
	if len(s) != 2 || s[0] != (InkPoint{0.1, 0.125}) || s[1] != (InkPoint{0.1, 0.175}) {
		t.Fatalf("stroke = %v, want the inner view's offset applied once", s)
	}
}

func TestScrollViewKeepsPanningForAChildThatDoesNotGrab(t *testing.T) {
	sv := newPanScrollView()
	sv.OnEvent(Event{Kind: EventClick, X: 40, Y: 60})
	if sv.PointerGrabbed() {
		t.Fatal("a view around a Label holds nothing")
	}
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: 40})
	if sv.OffsetY().Get() != 20 {
		t.Fatalf("pan broke: offset %d", sv.OffsetY().Get())
	}
	sv.OnEvent(Event{Kind: EventMouseUp, X: 40, Y: 40})
}

// A PointerRouter that grabbed the ScrollView drives the pad through it.
func TestPointerRouterDrivesAPadInAScrollView(t *testing.T) {
	sv, pad := scrolledPad()
	r := &PointerRouter{Targets: func() []Widget { return []Widget{sv} }}
	r.Press(30, 50, "")
	r.Drag(60, 50)
	r.Release(60, 50)
	if pad.Empty().Get() || sv.OffsetY().Get() != 0 {
		t.Fatalf("empty=%v offset=%d", pad.Empty().Get(), sv.OffsetY().Get())
	}
}

// --- fuzz ----------------------------------------------------------------

// FuzzInkPadEvents feeds the stroke-ingest path what a pointer (or a script
// pretending to be one) can send: any event kind at any coordinate, in any order,
// against tight caps. It asserts the invariants the bounds exist for.
func FuzzInkPadEvents(f *testing.F) {
	f.Add([]byte{0, 10, 0, 10, 0, 1, 40, 0, 10, 0, 2, 40, 0, 10, 0})
	f.Add([]byte{0, 255, 255, 255, 255, 1, 0, 128, 0, 128, 1, 1, 1, 1, 1})
	f.Add(bytes.Repeat([]byte{0, 1, 0, 1, 0, 1, 7, 0, 9, 0, 2, 0, 0, 0, 0}, 40))
	f.Fuzz(func(t *testing.T, data []byte) {
		p := NewInkPad()
		p.MaxStrokes, p.MaxPoints = 4, 32
		if len(data)%2 == 1 {
			p.SetBounds(Rect{W: 120, H: 60})
		}
		var full int
		p.OnFull = func(InkLimit) { full++ }
		for len(data) >= 5 {
			kind := []EventKind{EventClick, EventMouseDrag, EventMouseUp, EventTouchStart, EventTouchMove, EventTouchEnd}[int(data[0])%6]
			x := int(int16(uint16(data[1]) | uint16(data[2])<<8))
			y := int(int16(uint16(data[3]) | uint16(data[4])<<8))
			data = data[5:]
			p.OnEvent(Event{Kind: kind, X: x, Y: y})
			if len(data)%7 == 0 {
				refusesWithoutPanic(t, "Undo", func() { p.Undo() })
			}
		}
		n, points := p.Strokes().Len(), 0
		for _, s := range p.Strokes().Slice() {
			points += len(s)
			if len(s) == 0 {
				t.Fatal("an empty stroke was kept")
			}
			for _, q := range s {
				if q.X < 0 || q.X > 1 || q.Y < 0 || q.Y > 1 {
					t.Fatalf("point %v outside the box", q)
				}
			}
		}
		if n > 4 || points > 32 || points != p.points {
			t.Fatalf("strokes=%d points=%d (counted %d): a cap was exceeded or the count drifted", n, points, p.points)
		}
		if p.Empty().Get() != (n == 0) {
			t.Fatal("Empty disagrees with the strokes")
		}
		if p.PointerGrabbed() && n == 0 {
			t.Fatal("holding the pointer with no stroke")
		}
		if _, err := p.Image(); (err == nil) != (n > 0) {
			t.Fatalf("Image err=%v with %d strokes", err, n)
		}
		_ = full
	})
}
