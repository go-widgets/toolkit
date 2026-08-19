// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	stdcolor "image/color"
	"math"
	"reflect"
	"testing"

	gfxcolor "github.com/go-gfx/gfx/color"
	"github.com/go-gfx/gfx/iso"
)

// animEps is the tolerance for comparing procedurally computed grid coordinates.
const animEps = 1e-9

func animApproxEq(a, b float64) bool { return math.Abs(a-b) <= animEps }

func animApproxVec(a, b iso.Vec3) bool {
	return animApproxEq(a.X, b.X) && animApproxEq(a.Y, b.Y) && animApproxEq(a.Z, b.Z)
}

// --- phase folding + curves ---------------------------------------------------

func TestIsoWrapPhase(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{0, 0},
		{0.25, 0.25},
		{0.999, 0.999},
		{1, 0},
		{1.25, 0.25},
		{2, 0},
		{-0.25, 0.75},
		{-1.25, 0.75},
	}
	for _, c := range cases {
		if got := isoWrapPhase(c.in); !animApproxEq(got, c.want) {
			t.Fatalf("isoWrapPhase(%v) = %v, want %v", c.in, got, c.want)
		}
	}
	// A tiny negative input folds to exactly 1.0 before the guard clamps it to 0:
	// -1e-20 is below the ULP of 1.0, so (-1e-20) - floor(-1e-20) == 1.0. This is
	// the only path that reaches the `p >= 1` clamp.
	if fold := -1e-20 - math.Floor(-1e-20); fold != 1 {
		t.Fatalf("precondition: fold = %v, want exactly 1.0 to exercise the clamp", fold)
	}
	if got := isoWrapPhase(-1e-20); got != 0 {
		t.Fatalf("isoWrapPhase(-1e-20) = %v, want 0 (cycle-edge clamp)", got)
	}
}

func TestIsoBobPulse(t *testing.T) {
	// isoBob: sin(2*pi*phase).
	for _, c := range []struct{ p, want float64 }{{0, 0}, {0.25, 1}, {0.5, 0}, {0.75, -1}} {
		if got := isoBob(c.p); !animApproxEq(got, c.want) {
			t.Fatalf("isoBob(%v) = %v, want %v", c.p, got, c.want)
		}
	}
	// isoPulse: (1-cos(2*pi*phase))/2 — 0 at rest, 1 at half cycle.
	for _, c := range []struct{ p, want float64 }{{0, 0}, {0.25, 0.5}, {0.5, 1}, {0.75, 0.5}} {
		if got := isoPulse(c.p); !animApproxEq(got, c.want) {
			t.Fatalf("isoPulse(%v) = %v, want %v", c.p, got, c.want)
		}
	}
}

// TestIsoTranslateZ exercises every branch of the type switch, including the
// default (an unhandled primitive is returned unchanged).
func TestIsoTranslateZ(t *testing.T) {
	col := stdcolor.RGBA{R: 1, G: 2, B: 3, A: 255}
	dz := 0.5

	if got := isoTranslateZ(iso.Cube{Pos: iso.V(1, 2, 3), Size: 1, Color: col}, dz).(iso.Cube); !animApproxVec(got.Pos, iso.V(1, 2, 3.5)) {
		t.Fatalf("cube lift = %+v", got.Pos)
	}
	if got := isoTranslateZ(iso.Brick{Pos: iso.V(1, 2, 3), Dim: iso.Dimension{W: 1, H: 1, D: 1}, Color: col}, dz).(iso.Brick); !animApproxVec(got.Pos, iso.V(1, 2, 3.5)) {
		t.Fatalf("brick lift = %+v", got.Pos)
	}
	if got := isoTranslateZ(iso.Pyramid{Pos: iso.V(1, 2, 3), Dim: iso.Dimension{W: 1, H: 1, D: 1}, Color: col}, dz).(iso.Pyramid); !animApproxVec(got.Pos, iso.V(1, 2, 3.5)) {
		t.Fatalf("pyramid lift = %+v", got.Pos)
	}
	line := isoTranslateZ(iso.Line{From: iso.V(0, 0, 1), To: iso.V(1, 1, 2), Color: col, Width: 2}, dz).(iso.Line)
	if !animApproxVec(line.From, iso.V(0, 0, 1.5)) || !animApproxVec(line.To, iso.V(1, 1, 2.5)) {
		t.Fatalf("line lift = %+v -> %+v", line.From, line.To)
	}
	// default branch: a Slope is not handled and comes back byte-identical.
	slope := iso.Slope{Pos: iso.V(1, 2, 3), Dim: iso.Dimension{W: 1, H: 1, D: 1}, Color: col}
	if got := isoTranslateZ(slope, dz); got != iso.Shape(slope) {
		t.Fatalf("default branch mutated the shape: %+v", got)
	}
}

// --- procedural animated icon contract ---------------------------------------

// TestProceduralRenderEqualsPhaseZero pins the retro-compat contract: the
// IsoIcon.Render of a procedural animated icon is exactly its RenderAt at phase 0.
func TestProceduralRenderEqualsPhaseZero(t *testing.T) {
	base := stdcolor.RGBA{R: 200, G: 50, B: 90, A: 255}
	for _, id := range IsoAnimatedIconIDs {
		reg := NewIsoIconRegistry()
		RegisterAnimatedIcons(reg)
		icon, ok := reg.Resolve(id)
		if !ok {
			t.Fatalf("animated icon %q not registered", id)
		}
		anim := icon.(IsoAnimatedIcon)
		if !reflect.DeepEqual(anim.Render(2, 3, base), anim.RenderAt(2, 3, base, 0)) {
			t.Fatalf("%q: Render != RenderAt(phase 0)", id)
		}
	}
}

// TestProceduralPhaseWraps proves the cycle folds: phase p and p+k render alike.
func TestProceduralPhaseWraps(t *testing.T) {
	base := stdcolor.RGBA{R: 200, G: 50, B: 90, A: 255}
	icon := IsoProceduralAnimIcon{Frame: isoAnimGearFrame}
	// 0.25 and its integer offsets are exact binary fractions, so the fold is
	// bit-identical (unlike a value such as 1.3-1 which drifts by an ULP).
	at := icon.RenderAt(1, 1, base, 0.25)
	for _, p := range []float64{1.25, 2.25, -0.75, -1.75} {
		if !reflect.DeepEqual(icon.RenderAt(1, 1, base, p), at) {
			t.Fatalf("RenderAt(%v) != RenderAt(0.25): cycle did not wrap", p)
		}
	}
}

// TestRegisterAnimatedIcons checks every advertised id installs an IsoAnimatedIcon.
func TestRegisterAnimatedIcons(t *testing.T) {
	reg := NewIsoIconRegistry()
	RegisterAnimatedIcons(reg)
	if len(IsoAnimatedIconIDs) == 0 {
		t.Fatal("no animated icon ids advertised")
	}
	for _, id := range IsoAnimatedIconIDs {
		icon, ok := reg.Resolve(id)
		if !ok {
			t.Fatalf("%q did not register", id)
		}
		if _, isAnim := icon.(IsoAnimatedIcon); !isAnim {
			t.Fatalf("%q is %T, not an IsoAnimatedIcon", id, icon)
		}
	}
}

// --- toothed per-frame assertions (exact expected values) ---------------------

// TestAnimCloudBob asserts the cloud's exact vertical offset: at phase 0.25 the
// bob is +isoCloudBobAmplitude (sin=1), so every puff brick rises by exactly that.
func TestAnimCloudBob(t *testing.T) {
	base := stdcolor.RGBA{R: 60, G: 160, B: 220, A: 255}
	rest := isoAnimCloudFrame(2, 3, base, 0).Shapes
	up := isoAnimCloudFrame(2, 3, base, 0.25).Shapes
	if len(rest) != len(up) || len(rest) == 0 {
		t.Fatalf("cloud shape counts: rest=%d up=%d", len(rest), len(up))
	}
	for i := range rest {
		z0 := rest[i].(iso.Brick).Pos.Z
		z1 := up[i].(iso.Brick).Pos.Z
		if !animApproxEq(z1-z0, isoCloudBobAmplitude) {
			t.Fatalf("puff %d rise = %v, want %v", i, z1-z0, isoCloudBobAmplitude)
		}
	}
	// At phase 0 the frame equals the still cloud (bob = 0).
	if !reflect.DeepEqual(rest, isoCloudShapes(2, 3, base)) {
		t.Fatal("cloud rest frame != still cloud")
	}
}

// TestAnimDatabasePulse asserts the exact Shade factor at a chosen phase.
func TestAnimDatabasePulse(t *testing.T) {
	base := stdcolor.RGBA{R: 200, G: 50, B: 90, A: 255}
	still := isoDatabaseShapes(2, 3, base)
	// phase 0.25: factor = 1 + amp*sin(pi/2) = 1 + amp.
	got := isoAnimDatabaseFrame(2, 3, base, 0.25).Shapes
	wantFactor := 1 + isoDatabasePulseAmp
	for i := range still {
		orig := still[i].(iso.Brick).Color
		want := gfxcolor.Shade(orig, wantFactor)
		if g := got[i].(iso.Brick).Color; g != want {
			t.Fatalf("band %d colour = %v, want Shade(%v,%v)=%v", i, g, orig, wantFactor, want)
		}
	}
	// phase 0 (factor exactly 1) equals the still database.
	if !reflect.DeepEqual(isoAnimDatabaseFrame(2, 3, base, 0).Shapes, still) {
		t.Fatal("database rest frame != still database")
	}
}

// TestAnimGearRotates asserts the tooth-0 world position at two phases.
func TestAnimGearRotates(t *testing.T) {
	base := stdcolor.RGBA{R: 200, G: 50, B: 90, A: 255}
	x, y := 3, 4
	cx, cy := float64(x)+0.5, float64(y)+0.5
	shapes := isoAnimGearFrame(x, y, base, 0).Shapes
	if len(shapes) != isoGearTeeth+1 {
		t.Fatalf("gear shape count = %d, want %d", len(shapes), isoGearTeeth+1)
	}
	// tooth 0 at phase 0: ang 0 -> +X of the hub.
	want0 := iso.V(cx+isoGearRadius-isoGearTooth/2, cy-isoGearTooth/2, 0.35)
	if got := shapes[1].(iso.Cube).Pos; !animApproxVec(got, want0) {
		t.Fatalf("tooth0@phase0 = %+v, want %+v", got, want0)
	}
	// A quarter turn moves tooth 0 to +Y (a genuinely different position).
	rot := isoAnimGearFrame(x, y, base, 0.25).Shapes
	wantQ := iso.V(cx-isoGearTooth/2, cy+isoGearRadius-isoGearTooth/2, 0.35)
	if got := rot[1].(iso.Cube).Pos; !animApproxVec(got, wantQ) {
		t.Fatalf("tooth0@phase0.25 = %+v, want %+v", got, wantQ)
	}
}

// TestAnimServerLEDBlinks asserts the LED colour at rest vs. full brightness.
func TestAnimServerLEDBlinks(t *testing.T) {
	base := stdcolor.RGBA{R: 200, G: 50, B: 90, A: 255}
	rest := isoAnimServerFrame(2, 3, base, 0).Shapes
	lit := isoAnimServerFrame(2, 3, base, 0.5).Shapes
	// The LED is the last appended shape; the server body precedes it unchanged.
	ledRest := rest[len(rest)-1].(iso.Cube)
	ledLit := lit[len(lit)-1].(iso.Cube)
	if ledRest.Color != gfxcolor.Shade(base, 0.5) {
		t.Fatalf("LED@phase0 = %v, want Shade(base,0.5)", ledRest.Color)
	}
	if ledLit.Color != gfxcolor.Shade(base, 0.5+0.9) {
		t.Fatalf("LED@phase0.5 = %v, want Shade(base,1.4)", ledLit.Color)
	}
	if ledRest.Color == ledLit.Color {
		t.Fatal("LED did not change brightness across the cycle")
	}
	// The tower body (all but the LED) is the still server, phase-independent.
	if !reflect.DeepEqual(rest[:len(rest)-1], isoServerShapes(2, 3, base)) {
		t.Fatal("server body drifted from the still server")
	}
}

// TestAnimSpinnerHighlight asserts dot-0 brightness as the highlight orbits.
func TestAnimSpinnerHighlight(t *testing.T) {
	base := stdcolor.RGBA{R: 200, G: 50, B: 90, A: 255}
	shapes := isoAnimSpinnerFrame(1, 1, base, 0).Shapes
	if len(shapes) != isoSpinnerDots {
		t.Fatalf("spinner dot count = %d, want %d", len(shapes), isoSpinnerDots)
	}
	// phase 0: dot 0 is under the highlight (t=0) -> brightest, Shade(base,1.2).
	if got := shapes[0].(iso.Cube).Color; got != gfxcolor.Shade(base, 0.5+0.7) {
		t.Fatalf("dot0@phase0 = %v, want Shade(base,1.2)", got)
	}
	// phase 0.5: highlight is opposite dot 0 (t=0.5) -> Shade(base,0.85).
	half := isoAnimSpinnerFrame(1, 1, base, 0.5).Shapes
	if got := half[0].(iso.Cube).Color; got != gfxcolor.Shade(base, 0.5+0.7*0.5) {
		t.Fatalf("dot0@phase0.5 = %v, want Shade(base,0.85)", got)
	}
}
