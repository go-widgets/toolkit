// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"testing"
)

// Every widget documented as an Animator must satisfy the interface; a missing
// or mis-typed method fails the build here instead of silently dropping the
// widget out of the tree walk.
var (
	_ Animator = (*Spinner)(nil)
	_ Animator = (*LoadMask)(nil)
	_ Animator = (*ProgressBar)(nil)
	_ Animator = (*Skeleton)(nil)
	_ Animator = (*SkeletonGroup)(nil)
)

const phaseEps = 1e-9

func nearly(a, b float64) bool { return math.Abs(a-b) < phaseEps }

// poison is an Animator whose Animating() records that it was consulted, so a
// test can prove TreeAnimating short-circuits before reaching it.
type poison struct {
	Base
	seen int
}

func (p *poison) Tick(float64)    {}
func (p *poison) Animating() bool { p.seen++; return true }
func (p *poison) touched() bool   { return p.seen > 0 }

func TestTreeAnimatingNilRoot(t *testing.T) {
	if TreeAnimating(nil) {
		t.Fatal("TreeAnimating(nil) = true, want false")
	}
}

func TestTickTreeNilRootNoPanic(t *testing.T) {
	TickTree(nil, 0.5) // must not panic
}

// A stopped animator at the root reports not-animating, and TickTree still
// advances its clock (Spinner ticks regardless of Active, matching its own
// long-standing contract).
func TestRootAnimatorItself(t *testing.T) {
	s := NewSpinner()
	if TreeAnimating(s) {
		t.Fatal("inactive spinner root: TreeAnimating = true, want false")
	}
	s.Active().Set(true)
	if !TreeAnimating(s) {
		t.Fatal("active spinner root: TreeAnimating = false, want true")
	}
	TickTree(s, 0.25)
	if !nearly(s.Phase, 0.25) {
		t.Fatalf("TickTree(spinner,0.25): Phase = %v, want 0.25", s.Phase)
	}
}

// An active Spinner buried under VBox → ScrollView is still found by the walk;
// an inactive one anywhere makes the whole tree report idle.
func TestTreeAnimatingNested(t *testing.T) {
	s := NewSpinner()
	sv := NewScrollView(s)
	box := NewVBox()
	box.Append(NewLabel("loading")) // a non-Animator sibling
	box.Append(sv)

	if TreeAnimating(box) {
		t.Fatal("nested inactive spinner: TreeAnimating = true, want false")
	}
	s.Active().Set(true)
	if !TreeAnimating(box) {
		t.Fatal("nested active spinner: TreeAnimating = false, want true")
	}
}

// TickTree advances every Animator by the exact dt and leaves a non-animating
// Animator (a determinate ProgressBar) at its zero Phase — proof it ticks the
// tree without perturbing widgets that declare no animation.
func TestTickTreeAdvancesExactlyAndSpares(t *testing.T) {
	s := NewSpinner()
	s.Active().Set(true)
	det := NewProgressBar() // determinate: Tick is a no-op, Phase must stay 0

	box := NewVBox()
	box.Append(NewScrollView(s))
	box.Append(det)
	box.Append(NewLabel("x")) // plain non-Animator, must be harmless

	TickTree(box, 0.4)
	if !nearly(s.Phase, 0.4) {
		t.Fatalf("spinner Phase = %v, want 0.4", s.Phase)
	}
	if det.Phase != 0 {
		t.Fatalf("determinate ProgressBar Phase = %v, want 0 (Tick must no-op)", det.Phase)
	}

	// A second tick past the cycle wraps modulo 1.
	TickTree(box, 0.8)
	if !nearly(s.Phase, 0.2) {
		t.Fatalf("spinner Phase after wrap = %v, want 0.2", s.Phase)
	}
}

// TreeAnimating stops at the first still-animating widget: the poison sibling,
// ordered after the active spinner, must never be consulted.
func TestTreeAnimatingShortCircuits(t *testing.T) {
	s := NewSpinner()
	s.Active().Set(true)
	p := &poison{}

	box := NewContainer(nil)
	box.AddWidget(s) // animating; visited first
	box.AddWidget(p) // must NOT be reached

	if !TreeAnimating(box) {
		t.Fatal("TreeAnimating = false, want true")
	}
	if p.touched() {
		t.Fatal("poison sibling was consulted — TreeAnimating did not short-circuit")
	}
}

// Spinner: Animating tracks Active.
func TestSpinnerAnimating(t *testing.T) {
	s := NewSpinner()
	if s.Animating() {
		t.Fatal("fresh spinner Animating = true, want false")
	}
	s.Active().Set(true)
	if !s.Animating() {
		t.Fatal("active spinner Animating = false, want true")
	}
}

// LoadMask: Animating tracks Active; Tick advances the internal spinner.
func TestLoadMaskAnimatingAndTick(t *testing.T) {
	m := NewLoadMask("Loading…")
	if m.Animating() {
		t.Fatal("fresh LoadMask Animating = true, want false")
	}
	m.Active = true
	if !m.Animating() {
		t.Fatal("active LoadMask Animating = false, want true")
	}
	m.Tick(0.3)
	if !nearly(m.spinner.Phase, 0.3) {
		t.Fatalf("LoadMask inner spinner Phase = %v, want 0.3", m.spinner.Phase)
	}
}

// ProgressBar: Animating tracks Indeterminate; Tick advances Phase only when
// indeterminate, and wraps.
func TestProgressBarAnimatingAndTick(t *testing.T) {
	pb := NewProgressBar()
	if pb.Animating() {
		t.Fatal("determinate ProgressBar Animating = true, want false")
	}
	pb.Tick(0.5)
	if pb.Phase != 0 {
		t.Fatalf("determinate Tick advanced Phase to %v, want 0", pb.Phase)
	}

	pb.Indeterminate = true
	if !pb.Animating() {
		t.Fatal("indeterminate ProgressBar Animating = false, want true")
	}
	pb.Tick(0.7)
	if !nearly(pb.Phase, 0.7) {
		t.Fatalf("indeterminate Tick: Phase = %v, want 0.7", pb.Phase)
	}
	pb.Tick(0.6) // 0.7+0.6 = 1.3 → wraps to 0.3
	if !nearly(pb.Phase, 0.3) {
		t.Fatalf("indeterminate Tick wrap: Phase = %v, want 0.3", pb.Phase)
	}
}

// Skeleton: Animating tracks Animated; Tick advances Phase only while animated,
// and wraps.
func TestSkeletonAnimatingAndTick(t *testing.T) {
	s := NewSkeleton(SkeletonText, 3)
	if s.Animating() {
		t.Fatal("static Skeleton Animating = true, want false")
	}
	s.Tick(0.5)
	if s.Phase != 0 {
		t.Fatalf("static Skeleton Tick advanced Phase to %v, want 0", s.Phase)
	}

	s.Animated = true
	if !s.Animating() {
		t.Fatal("animated Skeleton Animating = false, want true")
	}
	s.Tick(0.9)
	if !nearly(s.Phase, 0.9) {
		t.Fatalf("animated Skeleton Tick: Phase = %v, want 0.9", s.Phase)
	}
	s.Tick(0.4) // 1.3 → 0.3
	if !nearly(s.Phase, 0.3) {
		t.Fatalf("animated Skeleton Tick wrap: Phase = %v, want 0.3", s.Phase)
	}
}

// SkeletonGroup: Animating tracks Animated; Tick advances the group Phase only
// while animated, and wraps.
func TestSkeletonGroupAnimatingAndTick(t *testing.T) {
	g := &SkeletonGroup{}
	if g.Animating() {
		t.Fatal("static SkeletonGroup Animating = true, want false")
	}
	g.Tick(0.5)
	if g.Phase != 0 {
		t.Fatalf("static SkeletonGroup Tick advanced Phase to %v, want 0", g.Phase)
	}

	g.Animated = true
	if !g.Animating() {
		t.Fatal("animated SkeletonGroup Animating = false, want true")
	}
	g.Tick(0.8)
	if !nearly(g.Phase, 0.8) {
		t.Fatalf("animated SkeletonGroup Tick: Phase = %v, want 0.8", g.Phase)
	}
	g.Tick(0.5) // 1.3 → 0.3
	if !nearly(g.Phase, 0.3) {
		t.Fatalf("animated SkeletonGroup Tick wrap: Phase = %v, want 0.3", g.Phase)
	}
}
