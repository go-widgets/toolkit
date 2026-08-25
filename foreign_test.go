// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// foreignRender draws w onto a freshly zeroed w×h buffer and reports whether
// anything was painted. An untouched pixel keeps A=0, so painted ground is
// distinguishable from untouched ground.
func foreignRender(w Widget, width, height int, theme *Theme) bool {
	buf := make([]byte, 4*width*height)
	w.Draw(painter.NewPixelPainter(buf, width, height), theme)
	for _, b := range buf {
		if b != 0 {
			return true
		}
	}
	return false
}

// An unclaimed region shows its Fallback, so the same tree still works where no
// host answers for its Kind — a native window, a terminal, a snapshot test.
func TestForeignFallbackRendersUntilClaimed(t *testing.T) {
	lbl := NewLabel("fallback")
	f := &Foreign{Key: "doc", Kind: "html", Fallback: lbl}
	f.SetBounds(Rect{X: 4, Y: 6, W: 60, H: 24})
	th := DefaultLight()

	if !foreignRender(f, 80, 40, th) {
		t.Error("an unclaimed region must render its fallback")
	}
	if lbl.Bounds() != f.Bounds() {
		t.Errorf("fallback bounds = %+v, want the region's %+v", lbl.Bounds(), f.Bounds())
	}

	f.Claimed().Set(true)
	if foreignRender(f, 80, 40, th) {
		t.Error("a claimed region must paint nothing: the host object is above it")
	}
}

// A region with no Fallback paints nothing and swallows no input, claimed or not.
func TestForeignWithoutFallback(t *testing.T) {
	f := &Foreign{Kind: "video"}
	f.SetBounds(Rect{W: 10, H: 10})
	if foreignRender(f, 10, 10, DefaultLight()) {
		t.Error("no fallback means nothing to paint")
	}
	f.OnEvent(Event{Kind: EventClick}) // must not panic
	f.Claimed().Set(true)
	f.OnEvent(Event{Kind: EventClick})
}

// Input reaches the fallback while it is the thing on screen, and stops once the
// host's own object is above the canvas receiving events directly.
func TestForeignEventRouting(t *testing.T) {
	clicks := 0
	btn := NewButton("b", func() { clicks++ })
	f := &Foreign{Fallback: btn}
	f.SetBounds(Rect{W: 40, H: 24})
	btn.SetBounds(f.Bounds())

	f.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if clicks != 1 {
		t.Fatalf("unclaimed region: fallback got %d clicks, want 1", clicks)
	}
	f.Claimed().Set(true)
	f.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if clicks != 1 {
		t.Errorf("claimed region must not forward input: %d clicks", clicks)
	}
}

// The region is presentational: unclaimed, its fallback speaks for itself;
// claimed, the host's object is already in the platform's accessibility tree.
func TestForeignA11yIsPresentational(t *testing.T) {
	f := &Foreign{Fallback: NewLabel("inner")}
	if got := f.A11y().Role; got != RolePresentation {
		t.Errorf("role = %v, want RolePresentation", got)
	}
	if len(f.Children()) != 1 {
		t.Error("the fallback must stay visible to the tree walks")
	}
	if len((&Foreign{}).Children()) != 0 {
		t.Error("a region with no fallback has no children")
	}
}

// WalkForeign finds regions wherever they are composed, in visual order, and
// reports absolute surface coordinates.
func TestWalkForeignFindsNestedRegions(t *testing.T) {
	a := &Foreign{Key: "a", Kind: "html", Content: "<p>x</p>"}
	a.SetBounds(Rect{X: 10, Y: 20, W: 100, H: 50})
	b := &Foreign{Key: "b", Kind: "video"}
	b.SetBounds(Rect{X: 0, Y: 80, W: 100, H: 60})
	box := NewContainer(nil).AddWidget(a).AddWidget(NewLabel("between")).AddWidget(b)
	box.SetBounds(Rect{W: 200, H: 200})

	got := WalkForeign(box)
	if len(got) != 2 {
		t.Fatalf("found %d regions, want 2: %+v", len(got), got)
	}
	if got[0].Key != "a" || got[0].Kind != "html" || got[0].Content != "<p>x</p>" {
		t.Errorf("first region = %+v", got[0])
	}
	if got[0].Rect != (Rect{X: 10, Y: 20, W: 100, H: 50}) {
		t.Errorf("rect = %+v, want the widget's own bounds", got[0].Rect)
	}
	if !got[0].Visible || got[0].Clip != got[0].Rect {
		t.Errorf("an unclipped region must report its whole rect visible: %+v", got[0])
	}
	if got[1].Key != "b" {
		t.Errorf("regions must come back in visual order: %+v", got)
	}
	if WalkForeign(nil) != nil {
		t.Error("walking nothing must find nothing")
	}
}

// A region inside a scrolled viewport reports the visible part, with the scroll
// offset applied — and reports nothing visible once scrolled away. A host that
// ignores this paints its object over the widgets around the viewport.
func TestWalkForeignClipsToViewport(t *testing.T) {
	f := &Foreign{Key: "doc", Kind: "html"}
	f.SetBounds(Rect{X: 5, Y: 5, W: 100, H: 400})
	inner := NewContainer(nil).AddWidget(f)
	inner.SetBounds(Rect{X: 5, Y: 5, W: 100, H: 400})
	sv := &ScrollView{Child: inner}
	sv.SetBounds(Rect{X: 5, Y: 5, W: 100, H: 60})
	sv.SetContentSize(100, 400)

	got := WalkForeign(sv)
	if len(got) != 1 {
		t.Fatalf("found %d regions, want 1", len(got))
	}
	vp := sv.ChildClip()
	if !got[0].Visible || got[0].Clip != vp {
		t.Errorf("clip = %+v, want the viewport %+v", got[0].Clip, vp)
	}
	if vp.W >= sv.Bounds().W || vp.H >= sv.Bounds().H {
		t.Errorf("the clip must exclude the scrollbar gutters: viewport %+v vs bounds %+v", vp, sv.Bounds())
	}
	if got[0].Rect.H != 400 {
		t.Errorf("Rect must stay the region's full extent: %+v", got[0].Rect)
	}

	sv.Scroll(0, 1000) // scrolled far past the region, clamped at the end
	got = WalkForeign(sv)
	if len(got) != 1 {
		t.Fatalf("found %d regions after scrolling, want 1", len(got))
	}
	if got[0].Rect.Y >= 5 {
		t.Errorf("a scrolled region must report where it is PAINTED: %+v", got[0].Rect)
	}
	if got[0].Clip != vp {
		t.Errorf("the visible band stays the viewport %+v: %+v", vp, got[0].Clip)
	}

	sv.SetBounds(Rect{X: 5, Y: 5, W: 100, H: 0}) // a viewport with no height
	got = WalkForeign(sv)
	if len(got) != 1 || got[0].Visible || got[0].Clip != (Rect{}) {
		t.Errorf("a region with nothing on screen must report so: %+v", got)
	}
}

// A region inside a scrolled panel inside a scrolled page is visible only where
// BOTH viewports agree — the intersection, not the innermost one. A host that
// takes the innermost clip alone paints its object outside the outer viewport.
func TestWalkForeignIntersectsNestedViewports(t *testing.T) {
	f := &Foreign{Key: "doc", Kind: "html"}
	f.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 300})
	content := NewContainer(nil).AddWidget(f)
	content.SetBounds(Rect{W: 300, H: 300})

	inner := &ScrollView{Child: content}
	inner.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	inner.SetContentSize(300, 300)
	panel := NewContainer(nil).AddWidget(inner)
	panel.SetBounds(Rect{W: 200, H: 200})

	outer := &ScrollView{Child: panel}
	outer.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 90})
	outer.SetContentSize(200, 200)

	got := WalkForeign(outer)
	if len(got) != 1 {
		t.Fatalf("found %d regions, want 1", len(got))
	}
	want := intersectRect(outer.ChildClip(), inner.ChildClip())
	if got[0].Clip != want {
		t.Errorf("clip = %+v, want the two viewports' overlap %+v", got[0].Clip, want)
	}
	if got[0].Clip.W >= inner.ChildClip().W || got[0].Clip.H >= inner.ChildClip().H {
		t.Errorf("the outer viewport must bite: %+v is not smaller than %+v", got[0].Clip, inner.ChildClip())
	}
}
