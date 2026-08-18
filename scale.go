// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Scale is a horizontal slider over a continuous Min..Max range.
// Click on the track jumps the thumb to that x-position and notifies
// the Value Observable's subscribers; dragging the thumb (or anywhere
// along the track with the button held) scrubs the value continuously
// through the same math. The 4-px track sits across the vertical
// midpoint in Theme.SurfaceAlt; the 10-px square thumb sits at the
// value's position in Theme.Accent.
//
// The reactive position is MVVM-only: it lives in an unexported
// [mvvm.Observable] exposed via [Scale.Value]. There is no settable Value
// field and no OnChange callback — a host binds Value() (Set / Subscribe /
// two-way) and a click or key adjustment Sets it (clamped to [Min, Max]),
// notifying subscribers on change.
type Scale struct {
	Base
	focusState
	Min, Max    float64
	Orientation Orientation
	// Step is the increment an arrow key applies to the value. When it is <= 0
	// the scale falls back to keyStep (1% of the range), so a caller that never
	// sets Step still gets sensible keyboard nudges. PageUp/PageDown always move a
	// whole page (keyPage, 10% of the range) regardless of Step.
	Step float64

	value *mvvm.Observable[float64]
}

// Value is the current position as a shared [mvvm.Observable]: a host binds it
// (Set / Subscribe / two-way) — there is no settable Value field. A click, a
// drag or a key adjustment Sets it (clamped to [Min, Max]); subscribers are
// notified on change.
func (s *Scale) Value() *mvvm.Observable[float64] {
	if s.value == nil {
		s.value = mvvm.NewObservable(0.0)
	}
	return s.value
}

// scaleThumbSize is the pixel side length of the thumb.
const scaleThumbSize = 16

// paintSliderThumb draws a slider/scale knob: a circular fill with a matching
// border — the Switch-knob shape shared by Scale and RangeSlider so every knob in
// the toolkit is one look. tx,ty is the knob's top-left; its side is the shared
// scaleThumbSize (scaled). The caller passes the fill/border already resolved for
// the enabled/disabled state.
func paintSliderThumb(p painter.Painter, tx, ty int, fill, border RGBA) {
	sz := scaled(scaleThumbSize)
	fillRoundRect(p, tx, ty, sz, sz, sz/2, fill)
	strokeRoundRect(p, tx, ty, sz, sz, sz/2, border)
}

// scaleTrackThickness is the groove's thickness in logical pixels. It is a
// metric like the thumb: a track that stayed four device pixels under a thumb
// that doubled would read as a thread.
const scaleTrackThickness = 4

// NewScale builds a Scale spanning [min, max] with the given initial
// value. Min == Max is allowed but renders a non-interactive track.
func NewScale(min, max, initial float64) *Scale {
	s := &Scale{Min: min, Max: max}
	s.SetValue(initial)
	return s
}

// SetValue clamps v to [Min, Max] and Sets the Value Observable — the shared
// mutate path for a click, a drag and every key adjustment. Subscribers are
// notified on change (an unchanged value is a no-op, per mvvm.Observable).
func (s *Scale) SetValue(v float64) {
	if v < s.Min {
		v = s.Min
	}
	if v > s.Max {
		v = s.Max
	}
	s.Value().Set(v)
}

// thumbRect returns the thumb's drawn rectangle for the current value, computed
// exactly as Draw places it (a scaled square centred across the track, its
// travel offset by pos along the active axis). ThumbHitRect derives the finger
// grab from it, so the grab always sits on the painted knob.
func (s *Scale) thumbRect() Rect {
	r := s.Bounds()
	sz := scaled(scaleThumbSize)
	var pos float64
	if s.Max > s.Min {
		pos = (s.Value().Get() - s.Min) / (s.Max - s.Min)
	}
	if s.Orientation == Vertical {
		ty := r.Y + int((1-pos)*float64(r.H-sz))
		return Rect{X: r.X + (r.W-sz)/2, Y: ty, W: sz, H: sz}
	}
	tx := r.X + int(pos*float64(r.W-sz))
	return Rect{X: tx, Y: r.Y + (r.H-sz)/2, W: sz, H: sz}
}

// ThumbHitRect is the finger grab for the slider knob: the drawn thumb rectangle
// clamped up to the touch minimum on each axis and centred over it. A knob only
// scaleThumbSize (16 logical px) across otherwise offers a 24px target under
// [DensityTouch]; this lifts it to the 44px floor without changing the painted
// thumb. At [DensityCompact] it equals the drawn thumb byte-for-byte.
func (s *Scale) ThumbHitRect() Rect { return touchHitRect(s.thumbRect()) }

// Draw paints a macOS-style slider: a rounded track whose filled portion (up
// to the thumb) is Accent and whose remainder is SurfaceAlt, with a circular
// white thumb -- matching the Switch's pill track + circular knob.
func (s *Scale) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	var pos float64
	if s.Max > s.Min {
		pos = (s.Value().Get() - s.Min) / (s.Max - s.Min)
	}
	// Track / fill / thumb / border colours. A disabled scale mutes them all so
	// it reads as inert; the enabled draw is byte-identical (branch only taken
	// when Disabled).
	trackC, fillC, thumbC, borderC := theme.SurfaceAlt, theme.Accent, theme.Surface, theme.Border
	if s.Disabled {
		trackC, fillC, thumbC, borderC = mutedFace(theme), mutedInk(theme), mutedFace(theme), mutedInk(theme)
	}
	if s.Orientation == Vertical {
		trackW := scaled(scaleTrackThickness)
		trackX := r.X + (r.W-trackW)/2
		trackR := trackW / 2
		fillRoundRect(p, trackX, r.Y, trackW, r.H, trackR, trackC)
		// pos=1 (Max) sits at the top; the fill runs from the thumb centre
		// down to the bottom (a fader reads "filled" below the knob).
		ty := r.Y + int((1-pos)*float64(r.H-scaled(scaleThumbSize)))
		centreY := ty + scaled(scaleThumbSize)/2
		fillRoundRect(p, trackX, centreY, trackW, r.Y+r.H-centreY, trackR, fillC)
		tx := r.X + (r.W-scaled(scaleThumbSize))/2
		paintSliderThumb(p, tx, ty, thumbC, borderC)
		s.drawFocusRing(p, theme, r)
		return
	}
	trackH := scaled(scaleTrackThickness)
	trackY := r.Y + (r.H-trackH)/2
	trackR := trackH / 2
	// Full (unfilled) track first, then the fill up to the thumb centre.
	fillRoundRect(p, r.X, trackY, r.W, trackH, trackR, trackC)
	// Position the thumb (pos computed above). When Max == Min, sit at the left.
	tx := r.X + int(pos*float64(r.W-scaled(scaleThumbSize)))
	fillRoundRect(p, r.X, trackY, tx+scaled(scaleThumbSize)/2-r.X, trackH, trackR, fillC)
	// Circular white thumb + border (same shape as the Switch knob).
	ty := r.Y + (r.H-scaled(scaleThumbSize))/2
	paintSliderThumb(p, tx, ty, thumbC, borderC)
	s.drawFocusRing(p, theme, r)
}

// keyStep is the arrow-key increment: Step when the caller set a positive one,
// else 1% of the range (so an unconfigured scale still nudges sensibly). Zero
// when the range is empty.
func (s *Scale) keyStep() float64 {
	if s.Step > 0 {
		return s.Step
	}
	return (s.Max - s.Min) / 100
}

// keyPage is the PageUp/PageDown increment: 10% of the range.
func (s *Scale) keyPage() float64 { return (s.Max - s.Min) / 10 }

// nudge adds delta to the value (clamped + Set via SetValue) -- the shared
// mutate path every arrow / Page key reuses, so a key move behaves exactly like
// a click that lands on the same value.
func (s *Scale) nudge(delta float64) { s.SetValue(s.Value().Get() + delta) }

// setTo Sets v (clamped via SetValue) -- used by Home/End.
func (s *Scale) setTo(v float64) { s.SetValue(v) }

// OnEvent: a click (or a drag while the button is held) moves the thumb to the
// pointer's position along the track and notifies the Value Observable; arrow /
// Home / End / Page keys move the value while focused. A single thumb needs no
// drag-grab state -- the position->SetValue math handles any coordinate
// identically, so a drag is just a click that keeps arriving.
func (s *Scale) OnEvent(ev Event) {
	if s.Disabled {
		return
	}
	if ev.Kind == EventKeyDown {
		if s.Max <= s.Min {
			return
		}
		switch ev.Code {
		case "ArrowRight", "ArrowUp":
			s.nudge(s.keyStep())
		case "ArrowLeft", "ArrowDown":
			s.nudge(-s.keyStep())
		case "PageUp":
			s.nudge(s.keyPage())
		case "PageDown":
			s.nudge(-s.keyPage())
		case "Home":
			s.setTo(s.Min)
		case "End":
			s.setTo(s.Max)
		}
		return
	}
	if ev.Kind != EventClick && ev.Kind != EventMouseDrag {
		return
	}
	r := s.Bounds()
	if s.Max <= s.Min {
		return
	}
	// Map the click across the track the THUMB centre actually travels (extent −
	// scaleThumbSize), offset by half a thumb — the inverse of Draw's placement.
	// Vertical is flipped so the top is Max (a fader reads up = more).
	var pos float64
	if s.Orientation == Vertical {
		span := r.H - scaled(scaleThumbSize)
		if span <= 0 {
			return
		}
		pos = 1 - float64(ev.Y-scaled(scaleThumbSize)/2)/float64(span)
	} else {
		span := r.W - scaled(scaleThumbSize)
		if span <= 0 {
			return
		}
		pos = float64(ev.X-scaled(scaleThumbSize)/2) / float64(span)
	}
	if pos < 0 {
		pos = 0
	}
	if pos > 1 {
		pos = 1
	}
	s.SetValue(s.Min + pos*(s.Max-s.Min))
}
