// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestScaledExported checks the exported [Scaled] seam sibling packages route
// their own metrics through: identity at 1x, doubled at 2x.
func TestScaledExported(t *testing.T) {
	defer SetMetricScale(1)
	if got := Scaled(10); got != 10 {
		t.Fatalf("Scaled(10) at 1x = %d, want 10 (identity)", got)
	}
	SetMetricScale(2)
	if got := Scaled(10); got != 20 {
		t.Fatalf("Scaled(10) at 2x = %d, want 20", got)
	}
}

// TestScrollGutterNormalized proves the one normalized scrollbar gutter — a
// scaled track PLUS a scaled gap — is applied identically by ScrollView, ListBox
// and TreeView, so scrolled content never sits flush against the thumb and the
// gap is the same in every scrollable reader widget.
func TestScrollGutterNormalized(t *testing.T) {
	defer SetMetricScale(1)
	// The gutter is the track thickness plus the normalized gap, and it is
	// strictly wider than the track, so content stops short of the track.
	if scrollbarTrack() != scrollbarWidth {
		t.Fatalf("scrollbarTrack at 1x = %d, want %d", scrollbarTrack(), scrollbarWidth)
	}
	if scrollGutter() != scrollbarWidth+scrollbarGap {
		t.Fatalf("scrollGutter at 1x = %d, want %d", scrollGutter(), scrollbarWidth+scrollbarGap)
	}
	if scrollGutter()-scrollbarTrack() != scrollbarGap || scrollbarGap <= 0 {
		t.Fatalf("gutter must leave a positive gap of %d before the track", scrollbarGap)
	}

	// ScrollView insets its content viewport by exactly the gutter.
	sv := NewScrollView(NewLabel("x"))
	sv.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	sv.SetContentSize(40, 400) // vertical overflow only
	if got := 100 - sv.viewport().W; got != scrollGutter() {
		t.Fatalf("ScrollView content inset = %d, want scrollGutter %d", got, scrollGutter())
	}

	// ListBox: the track column starts at W-track, and the row content stops one
	// gutter short of the right edge — a pixel probe proves content never reaches
	// the track, leaving the normalized gap.
	theme := DefaultLight()
	l := NewListBox(manyItems(50))
	l.RowHeight = 20
	l.Selected().Set(0)
	l.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100}) // 5 of 50 visible -> overflow
	if g, ok := l.scrollbarGeom(); !ok || g.cross0 != 100-scrollbarTrack() {
		t.Fatalf("ListBox track cross0 = %d (ok=%v), want %d", g.cross0, ok, 100-scrollbarTrack())
	}
	buf := makeSurface(128, 128)
	l.Draw(newP(buf, 128), theme)
	if got := pixelAt(buf, 128, 100-scrollGutter()-1, 5); got != theme.Accent {
		t.Fatalf("ListBox selected row should fill up to the gutter edge; got %+v want Accent", got)
	}
	if got := pixelAt(buf, 128, 100-scrollGutter()+1, 5); got == theme.Accent {
		t.Fatal("ListBox content must not extend into the scrollbar gutter gap")
	}
	// Probe the bare track well below the thumb (which sits at the top at ScrollRow 0).
	if got := pixelAt(buf, 128, 100-scrollbarTrack()+1, 90); got != theme.SurfaceAlt {
		t.Fatalf("ListBox track column should be SurfaceAlt; got %+v", got)
	}

	// TreeView uses the same track position and gutter, so its scrollbar lines up
	// with the others.
	root, _ := manyLeaves(40)
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	if g, ok := tv.scrollbarGeom(); !ok || g.cross0 != 100-scrollbarTrack() {
		t.Fatalf("TreeView track cross0 = %d (ok=%v), want %d", g.cross0, ok, 100-scrollbarTrack())
	}
}

// TestMetricScaleDoublesWidgetMetrics proves that SetMetricScale(2) doubles the
// pixel metrics of the reader's core widgets (with defer-reset to 1x). The
// package bitmap font does not scale, so cases are chosen so the doubled part is
// isolated and assertable exactly.
func TestMetricScaleDoublesWidgetMetrics(t *testing.T) {
	defer SetMetricScale(1)

	// A bare PostCard's height is pure metric — the pad top+bottom plus the
	// meta-spacer gap, with no text — so it doubles exactly.
	pc := &PostCard{}
	h1 := pc.Measure(200)
	SetMetricScale(2)
	h2 := pc.Measure(200)
	if h2 != 2*h1 {
		t.Fatalf("PostCard.Measure at 2x = %d, want %d (2×%d)", h2, 2*h1, h1)
	}

	// The scrollbar track and the content gutter both double.
	if scrollbarTrack() != 2*scrollbarWidth {
		t.Fatalf("scrollbarTrack at 2x = %d, want %d", scrollbarTrack(), 2*scrollbarWidth)
	}
	if scrollGutter() != 2*(scrollbarWidth+scrollbarGap) {
		t.Fatalf("scrollGutter at 2x = %d, want %d", scrollGutter(), 2*(scrollbarWidth+scrollbarGap))
	}

	// ScrollView's content inset doubles in lockstep.
	sv := NewScrollView(NewLabel("x"))
	sv.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	sv.SetContentSize(40, 400)
	if got := 100 - sv.viewport().W; got != scrollGutter() {
		t.Fatalf("ScrollView inset at 2x = %d, want gutter %d", got, scrollGutter())
	}

	// The default row heights of ListBox and TreeView double (18 -> 36).
	if l := NewListBox(nil); l.RowHeight != 36 {
		t.Fatalf("ListBox default RowHeight at 2x = %d, want 36", l.RowHeight)
	}
	if tv := NewTreeView(nil); tv.rowHeight() != 36 {
		t.Fatalf("TreeView default rowHeight at 2x = %d, want 36", tv.rowHeight())
	}

	// A Badge auto-sizes with 2× its horizontal padding (the text width is
	// font-fixed, so the padding delta is the whole scaled part).
	b := NewBadge("9")
	b.Draw(newP(makeSurface(64, 64), 64), DefaultLight())
	if pad := b.Bounds().W - b.textWidth("9"); pad != 2*scaled(BadgePadX) || pad != 4*BadgePadX {
		t.Fatalf("Badge horizontal padding at 2x = %d, want %d", pad, 4*BadgePadX)
	}

	// A Card's header strip gains 2× its vertical pad above the (font-fixed) glyph.
	if pad := CardHeaderH() - GlyphHeight(); pad != 2*scaled(CardPadY) || pad != 4*CardPadY {
		t.Fatalf("Card header pad at 2x = %d, want %d", pad, 4*CardPadY)
	}
}
