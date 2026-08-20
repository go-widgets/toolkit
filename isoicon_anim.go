// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	stdcolor "image/color"
	"math"

	gfxcolor "github.com/go-gfx/gfx/color"
	"github.com/go-gfx/gfx/iso"
)

// IsoAnimatedIcon is an [IsoIcon] whose drawing varies with a time PHASE — a
// procedural animation authored from the very same isometric primitives as the
// still icons, never from external art. It renders one frame for a given phase;
// the diagram drives that phase from the host's clock (see
// [IsoDiagram.AnimationStep]).
//
// The phase is a wrap-around cycle position: it is normalised into [0, 1) before
// a frame is composed, so phase p, p+1 and p-1 all render identically. A still
// (never-driven) diagram holds phase 0, and the interface's own [IsoIcon.Render]
// MUST equal [IsoAnimatedIcon.RenderAt] at phase 0 — the rest frame — so an
// animated icon that is never stepped draws exactly like an authored still.
type IsoAnimatedIcon interface {
	IsoIcon
	// RenderAt returns the drawing for cell (x, y) shaded from base at animation
	// phase, a cycle position normalised into [0, 1). The implementation must not
	// retain x, y or base.
	RenderAt(x, y int, base stdcolor.RGBA, phase float64) IsoIconDrawing
}

// IsoProceduralAnimIcon is the pure-Go, code-authored [IsoAnimatedIcon]: a single
// Frame closure composes the icon's [iso.Shape]s (and/or sprite) at a phase. It
// is to [IsoAnimatedIcon] what [IsoPrimitiveIcon] is to [IsoIcon]. The phase
// handed to Frame is always in [0, 1) (RenderAt wraps it first), so a frame
// author never has to fold the cycle themselves, and Render is Frame at phase 0.
type IsoProceduralAnimIcon struct {
	// Frame composes the icon at grid cell (x, y) from base colour base for the
	// wrapped phase in [0, 1).
	Frame func(x, y int, base stdcolor.RGBA, phase float64) IsoIconDrawing
}

// RenderAt satisfies [IsoAnimatedIcon]: it folds phase into [0, 1) and calls
// Frame. RenderAt(...,phase) and RenderAt(...,phase+k) for any integer k return
// the same drawing.
func (i IsoProceduralAnimIcon) RenderAt(x, y int, base stdcolor.RGBA, phase float64) IsoIconDrawing {
	return i.Frame(x, y, base, isoWrapPhase(phase))
}

// Render satisfies [IsoIcon] by rendering the rest frame (phase 0), so a
// procedural animated icon that is never driven draws exactly like a still.
func (i IsoProceduralAnimIcon) Render(x, y int, base stdcolor.RGBA) IsoIconDrawing {
	return i.RenderAt(x, y, base, 0)
}

// isoWrapPhase folds a continuous phase into the half-open cycle [0, 1); negative
// inputs wrap forward (e.g. -0.25 -> 0.75). A value that folds to exactly 1
// (which math.Floor cannot produce here) would map to 0.
func isoWrapPhase(phase float64) float64 {
	// A non-finite phase (NaN / ±Inf a host may hand RenderAt directly) has no
	// place on the cycle; fold it to the rest frame so an animated icon renders
	// its phase-0 still rather than emitting NaN coordinates.
	if math.IsNaN(phase) || math.IsInf(phase, 0) {
		return 0
	}
	p := phase - math.Floor(phase)
	if p >= 1 { // guards a float edge where phase-Floor rounds up to 1.0
		p = 0
	}
	return p
}

// isoBob is the signed breathing curve sin(2*pi*phase) in [-1, 1]; it is 0 at
// phase 0 (the rest frame) and drives symmetric offsets/brightening.
func isoBob(phase float64) float64 { return math.Sin(2 * math.Pi * phase) }

// isoPulse is the unsigned breathing curve (1-cos(2*pi*phase))/2 in [0, 1]; it is
// 0 at phase 0, rises to 1 at phase 0.5 and returns to 0 — a clean pulse whose
// rest value is 0.
func isoPulse(phase float64) float64 { return 0.5 - 0.5*math.Cos(2*math.Pi*phase) }

// isoTranslateZ returns shape lifted by dz along +Z. It handles exactly the
// primitive kinds the animated icons compose (cube, brick, pyramid, line); any
// other shape is returned unchanged.
func isoTranslateZ(shape iso.Shape, dz float64) iso.Shape {
	switch s := shape.(type) {
	case iso.Cube:
		s.Pos.Z += dz
		return s
	case iso.Brick:
		s.Pos.Z += dz
		return s
	case iso.Pyramid:
		s.Pos.Z += dz
		return s
	case iso.Line:
		s.From.Z += dz
		s.To.Z += dz
		return s
	default:
		return shape
	}
}

// --- built-in animated icon ids ----------------------------------------------

// IsoAnimatedIconIDs are the ids of the built-in procedural animated icons that
// [RegisterAnimatedIcons] installs. They are namespaced under "anim/" so they
// never collide with the still built-ins, and are opt-in: the default registry
// does NOT carry them until a host registers them.
var IsoAnimatedIconIDs = []string{
	"anim/cloud",    // floats up and down (vertical bob)
	"anim/database", // breathes brighter/darker (hue pulse)
	"anim/gear",     // rotates its teeth about the hub
	"anim/server",   // tower with a blinking status LED
	"anim/spinner",  // ring of dots with an orbiting highlight
}

// RegisterAnimatedIcons installs the built-in animated icons ([IsoAnimatedIconIDs])
// into r. It is opt-in — a host calls it on the default registry
// ([IsoDefaultIcons]) or on a per-widget one to make the "anim/*" ids resolvable.
// Registering is idempotent (it replaces under the same ids).
func RegisterAnimatedIcons(r *IsoIconRegistry) {
	r.Register("anim/cloud", IsoProceduralAnimIcon{Frame: isoAnimCloudFrame})
	r.Register("anim/database", IsoProceduralAnimIcon{Frame: isoAnimDatabaseFrame})
	r.Register("anim/gear", IsoProceduralAnimIcon{Frame: isoAnimGearFrame})
	r.Register("anim/server", IsoProceduralAnimIcon{Frame: isoAnimServerFrame})
	r.Register("anim/spinner", IsoProceduralAnimIcon{Frame: isoAnimSpinnerFrame})
}

// animation shape constants — grid-unit amplitudes and counts, kept named so the
// tests assert against the same values the frames use.
const (
	isoCloudBobAmplitude = 0.15 // grid units the cloud rises/falls
	isoDatabasePulseAmp  = 0.25 // Shade-factor swing of the database breathing
	isoGearTeeth         = 6    // teeth around the rotating gear
	isoGearRadius        = 0.32 // orbit radius of a gear tooth, grid units
	isoGearTooth         = 0.18 // gear tooth cube size, grid units
	isoSpinnerDots       = 8    // dots in the spinner ring
	isoSpinnerRadius     = 0.34 // spinner ring radius, grid units
	isoSpinnerDot        = 0.16 // spinner dot cube size, grid units
	isoServerLEDSize     = 0.18 // blinking status LED cube size, grid units
)

// --- animated icon frames ----------------------------------------------------

// isoAnimCloudFrame is the still cloud lifted by a sinusoidal vertical bob: the
// whole puff rises and falls by isoCloudBobAmplitude over one cycle, resting at
// its authored height at phase 0.
func isoAnimCloudFrame(x, y int, c stdcolor.RGBA, phase float64) IsoIconDrawing {
	bob := isoCloudBobAmplitude * isoBob(phase)
	shapes := isoCloudShapes(x, y, c)
	for i := range shapes {
		shapes[i] = isoTranslateZ(shapes[i], bob)
	}
	return IsoIconDrawing{Shapes: shapes}
}

// isoAnimDatabaseFrame is the still stacked database whose every band breathes
// brighter then darker: each brick's colour is Shade-scaled by 1+amp*sin, so at
// phase 0 (sin 0) the factor is exactly 1 and the frame equals the still
// database.
func isoAnimDatabaseFrame(x, y int, c stdcolor.RGBA, phase float64) IsoIconDrawing {
	factor := 1 + isoDatabasePulseAmp*isoBob(phase)
	shapes := isoDatabaseShapes(x, y, c)
	for i := range shapes {
		if b, ok := shapes[i].(iso.Brick); ok {
			b.Color = gfxcolor.Shade(b.Color, factor)
			shapes[i] = b
		}
	}
	return IsoIconDrawing{Shapes: shapes}
}

// isoAnimGearFrame is a gear: a lightened hub cube with isoGearTeeth tooth cubes
// orbiting it. The whole ring rotates by one full turn per cycle (angle
// 2*pi*phase), so at phase 0 the teeth sit on the cardinal start angles and every
// other phase turns them.
func isoAnimGearFrame(x, y int, c stdcolor.RGBA, phase float64) IsoIconDrawing {
	fx, fy := float64(x), float64(y)
	cx, cy := fx+0.5, fy+0.5
	hub := gfxcolor.Shade(c, 1.1)
	tooth := gfxcolor.Shade(c, 0.7)
	shapes := make([]iso.Shape, 0, isoGearTeeth+1)
	shapes = append(shapes, iso.Cube{Pos: iso.V(cx-0.2, cy-0.2, 0.3), Size: 0.4, Color: hub})
	for i := 0; i < isoGearTeeth; i++ {
		ang := 2 * math.Pi * (phase + float64(i)/isoGearTeeth)
		px := cx + isoGearRadius*math.Cos(ang) - isoGearTooth/2
		py := cy + isoGearRadius*math.Sin(ang) - isoGearTooth/2
		shapes = append(shapes, iso.Cube{Pos: iso.V(px, py, 0.35), Size: isoGearTooth, Color: tooth})
	}
	return IsoIconDrawing{Shapes: shapes}
}

// isoAnimServerFrame is the still server tower topped by a status LED that blinks:
// the LED cube's colour is Shade-scaled by a pulse (dim at phase 0, brightest at
// phase 0.5), so the tower reads as a live, working machine.
func isoAnimServerFrame(x, y int, c stdcolor.RGBA, phase float64) IsoIconDrawing {
	fx, fy := float64(x), float64(y)
	shapes := isoServerShapes(x, y, c)
	led := gfxcolor.Shade(c, 0.5+0.9*isoPulse(phase))
	shapes = append(shapes, iso.Cube{
		Pos:   iso.V(fx+0.15, fy+0.15, 1.75),
		Size:  isoServerLEDSize,
		Color: led,
	})
	return IsoIconDrawing{Shapes: shapes}
}

// isoAnimSpinnerFrame is a ring of isoSpinnerDots dot cubes at fixed positions
// whose brightness follows a highlight orbiting the ring once per cycle: the dot
// the highlight just passed is brightest. The positions never move; only the
// per-dot Shade factor changes with phase, so the ring reads as a spinning
// progress indicator.
func isoAnimSpinnerFrame(x, y int, c stdcolor.RGBA, phase float64) IsoIconDrawing {
	fx, fy := float64(x), float64(y)
	cx, cy := fx+0.5, fy+0.5
	shapes := make([]iso.Shape, 0, isoSpinnerDots)
	for i := 0; i < isoSpinnerDots; i++ {
		ang := 2 * math.Pi * float64(i) / isoSpinnerDots
		px := cx + isoSpinnerRadius*math.Cos(ang) - isoSpinnerDot/2
		py := cy + isoSpinnerRadius*math.Sin(ang) - isoSpinnerDot/2
		// t is 0 at the dot under the highlight, rising to ~1 just behind it.
		t := isoWrapPhase(phase - float64(i)/isoSpinnerDots)
		bright := gfxcolor.Shade(c, 0.5+0.7*(1-t))
		shapes = append(shapes, iso.Cube{Pos: iso.V(px, py, 0.3), Size: isoSpinnerDot, Color: bright})
	}
	return IsoIconDrawing{Shapes: shapes}
}
