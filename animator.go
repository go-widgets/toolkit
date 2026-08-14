// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// Animator is an optional Widget capability: a widget that animates (a spinner,
// an indeterminate progress bar, a skeleton shimmer) implements it so a host
// present loop can advance it each frame and learn whether it still needs
// frames — so an idle UI stops repainting and a stopped animation costs nothing.
//
// The contract is the same manual-clock one the rest of the toolkit uses: the
// widget owns no goroutine and no timer. The host calls Tick(dt) once per frame
// with the elapsed wall-clock seconds, then consults Animating to decide whether
// to schedule another frame. This retires the hand-rolled "am I still spinning?"
// bookkeeping that applications otherwise keep beside every spinner — the source
// of the classic frozen-spinner bug where the flag and the animation drift apart.
//
// A container does not implement Animator: [TickTree] and [TreeAnimating]
// descend the widget tree through [childContainer] (the same convention
// [WalkA11y] and [CollectRuns] use) and apply the capability to whichever leaves
// carry it, so a host drives a whole composed UI with one call and never wires a
// spinner through by hand.
type Animator interface {
	// Tick advances the animation by dt seconds (the elapsed wall-clock time
	// since the previous frame).
	Tick(dt float64)
	// Animating reports whether the widget still needs fresh frames. When every
	// Animator in a tree returns false the host can stop repainting until the
	// next interaction restarts one.
	Animating() bool
}

// TickTree advances every [Animator] in the tree rooted at root by dt seconds.
//
// It descends into any widget exposing its children via [childContainer] — the
// same walk [WalkA11y] and [CollectRuns] use — so one call drives a whole
// composed UI. The root itself is ticked when it is an Animator, and the walk
// still descends into it (a widget can be both an Animator and a container).
// A nil root (or a nil child a container might yield) is skipped, so callers
// need not guard the tree they hand in.
func TickTree(root Widget, dt float64) {
	if root == nil {
		return
	}
	if a, ok := root.(Animator); ok {
		a.Tick(dt)
	}
	if c, ok := root.(childContainer); ok {
		for _, child := range c.Children() {
			TickTree(child, dt)
		}
	}
}

// TreeAnimating reports whether at least one [Animator] in the tree rooted at
// root still needs frames (its Animating returns true).
//
// It descends through [childContainer] exactly like [TickTree], and
// short-circuits: the first still-animating widget found ends the walk, because
// a host only needs to know that *something* wants another frame, not how many.
// A nil root (or a nil child) contributes nothing, so an empty or partly-built
// tree simply reports false.
func TreeAnimating(root Widget) bool {
	if root == nil {
		return false
	}
	if a, ok := root.(Animator); ok && a.Animating() {
		return true
	}
	if c, ok := root.(childContainer); ok {
		for _, child := range c.Children() {
			if TreeAnimating(child) {
				return true
			}
		}
	}
	return false
}
