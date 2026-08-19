// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"testing"
)

// animatedDiagram returns a diagram whose single node carries the given animated
// icon, wired to a per-widget registry that has the "anim/*" icons installed.
func animatedDiagram(icon string) *IsoDiagram {
	reg := NewIsoIconRegistry()
	RegisterAnimatedIcons(reg)
	d := NewIsoDiagram(nil)
	d.Icons = reg
	d.Doc().PutNode(IsoNode{ID: "n", X: 4, Y: 4, Icon: icon, Color: RGBA{R: 60, G: 160, B: 220, A: 255}})
	return d
}

// TestAnimationStepDefaultPeriod: with AnimationPeriod left zero, the phase
// advances by dt/isoDefaultAnimPeriod.
func TestAnimationStepDefaultPeriod(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.AnimationStep(0.5) // 0.5 / 2.0 = 0.25
	if got := d.AnimationPhase(); !animApproxEq(got, 0.25) {
		t.Fatalf("phase = %v, want 0.25 (default period %v)", got, isoDefaultAnimPeriod)
	}
}

// TestAnimationStepCustomPeriod: a set AnimationPeriod scales dt.
func TestAnimationStepCustomPeriod(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.AnimationPeriod = 4
	d.AnimationStep(1) // 1 / 4 = 0.25
	if got := d.AnimationPhase(); !animApproxEq(got, 0.25) {
		t.Fatalf("phase = %v, want 0.25 (period 4)", got)
	}
}

// TestAnimationStepWraps: a dt longer than the period folds the phase into [0,1).
func TestAnimationStepWraps(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.AnimationPeriod = 2
	d.AnimationStep(3) // 1.5 -> wraps to 0.5
	if got := d.AnimationPhase(); !animApproxEq(got, 0.5) {
		t.Fatalf("phase = %v, want 0.5 (wrapped)", got)
	}
}

// TestAnimationStepInvalidatesOnlyWithAnimatedNode gates the repaint: stepping a
// diagram that carries an animated icon fires OnInvalidate; stepping one whose
// nodes are still-only (or icon-less, or unknown) advances the phase but never
// invalidates.
func TestAnimationStepInvalidatesOnlyWithAnimatedNode(t *testing.T) {
	// Animated node -> step invalidates.
	d := animatedDiagram("anim/cloud")
	count := 0
	d.OnInvalidate = func() { count++ }
	d.AnimationStep(0.1)
	if count == 0 {
		t.Fatal("animated node: AnimationStep did not invalidate")
	}
	if d.AnimationPhase() == 0 {
		t.Fatal("animated node: phase did not advance")
	}

	// Still-only doc: empty-icon node (continue branch) + unknown-icon node
	// (resolves to the non-animated fallback) -> no invalidate, phase still moves.
	still := NewIsoDiagram(nil)
	still.Icons = NewIsoIconRegistry()                                // nothing animated registered
	still.Doc().PutNode(IsoNode{ID: "blank", X: 1, Y: 1})             // Icon == ""
	still.Doc().PutNode(IsoNode{ID: "unk", X: 3, Y: 3, Icon: "nope"}) // unknown -> fallback
	sc := 0
	still.OnInvalidate = func() { sc++ }
	still.AnimationStep(0.3)
	if sc != 0 {
		t.Fatalf("still-only doc invalidated %d times, want 0", sc)
	}
	if !animApproxEq(still.AnimationPhase(), 0.15) {
		t.Fatalf("still-only doc phase = %v, want 0.15 (phase must still advance)", still.AnimationPhase())
	}
}

// TestSeamByteIdenticalForStillIcons is the control run: advancing the phase must
// NOT change the pixels of a diagram whose icons are all still — the animation
// seam is invisible to non-animated content.
func TestSeamByteIdenticalForStillIcons(t *testing.T) {
	theme := DefaultLight()
	d := NewIsoDiagram(nil)
	d.Doc().PutNode(IsoNode{ID: "s", X: 4, Y: 4, Icon: "server", Color: RGBA{R: 230, G: 120, B: 20, A: 255}})
	before, err := RenderImage(d, 400, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	d.AnimationStep(0.37) // phase advances, but no icon here is animated
	after, err := RenderImage(d, 400, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before.Pix, after.Pix) {
		t.Fatal("still-icon render changed after AnimationStep (seam is not transparent)")
	}
}

// TestAnimatedFullCycleReturnsToRest proves determinism + cycle wrap at the pixel
// level: stepping an animated diagram by exactly one full period returns it to the
// phase-0 rest frame, byte-for-byte.
func TestAnimatedFullCycleReturnsToRest(t *testing.T) {
	theme := DefaultLight()
	d := animatedDiagram("anim/cloud")
	rest, err := RenderImage(d, 400, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	d.AnimationStep(isoDefaultAnimPeriod) // one full cycle -> phase wraps to 0
	if !animApproxEq(d.AnimationPhase(), 0) {
		t.Fatalf("phase after a full cycle = %v, want 0", d.AnimationPhase())
	}
	back, err := RenderImage(d, 400, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rest.Pix, back.Pix) {
		t.Fatal("a full-cycle step did not return the animated render to its rest frame")
	}
}

// TestAnimatedPixelsMove is the toothed pixel proof: at two distinct phases an
// animated icon renders visibly different pixels (here the cloud's vertical bob).
func TestAnimatedPixelsMove(t *testing.T) {
	theme := DefaultLight()
	d := animatedDiagram("anim/cloud")
	at0, err := RenderImage(d, 400, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	d.AnimationStep(0.5) // -> phase 0.25, cloud at its highest
	at1, err := RenderImage(d, 400, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(at0.Pix, at1.Pix) {
		t.Fatal("animated icon rendered identically at phase 0 and 0.25 (nothing moved)")
	}
}
