// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// paintedCols returns the leftmost and rightmost painted column indices on the
// surface (or -1,-1 if nothing was painted), for asserting horizontal centring.
func paintedCols(buf []byte, w, h int) (lo, hi int) {
	lo, hi = -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px := pixelAt(buf, w, x, y)
			if px.R != 0xC8 || px.G != 0xC8 || px.B != 0xC8 {
				if lo < 0 || x < lo {
					lo = x
				}
				if x > hi {
					hi = x
				}
			}
		}
	}
	return lo, hi
}

// TestEmptyStateMessageIsReactive checks the primary message is exposed as an
// observable that drives the rendered text.
func TestEmptyStateMessageIsReactive(t *testing.T) {
	e := NewEmptyState("Folder is empty")
	if e.Message().Get() != "Folder is empty" {
		t.Fatalf("Message = %q, want the constructor text", e.Message().Get())
	}
	e.Message().Set("Nothing here")
	if e.msg.Text().Get() != "Nothing here" {
		t.Fatal("setting Message must update the underlying label")
	}
}

// TestEmptyStateCaption covers SetCaption (create then update) and the Caption
// observable (create-on-access then reuse).
func TestEmptyStateCaption(t *testing.T) {
	e := NewEmptyState("m")
	if e.caption != nil {
		t.Fatal("no caption until requested")
	}
	if got := e.SetCaption("first"); got != e {
		t.Fatal("SetCaption should return the widget for chaining")
	}
	if e.caption == nil || e.Caption().Get() != "first" {
		t.Fatalf("SetCaption must create + set the caption, got %q", e.Caption().Get())
	}
	e.SetCaption("second") // caption != nil branch: updates in place
	if e.Caption().Get() != "second" {
		t.Fatalf("SetCaption must update the existing caption, got %q", e.Caption().Get())
	}

	// Caption() on a fresh widget creates the label lazily.
	e2 := NewEmptyState("m")
	if e2.Caption().Get() != "" {
		t.Fatal("lazy Caption must start empty")
	}
	e2.Caption().Set("bound")
	if e2.Caption().Get() != "bound" {
		t.Fatal("second Caption() call must reuse the same observable")
	}
}

// TestEmptyStateGapAndIconSquare covers the gap and iconSquare helpers across
// their default and override branches, and the no-icon case.
func TestEmptyStateGapAndIconSquare(t *testing.T) {
	e := NewEmptyState("m")
	if e.gap() != scaled(emptyStateGap) {
		t.Fatalf("default gap = %d, want %d", e.gap(), scaled(emptyStateGap))
	}
	e.Gap = 12
	if e.gap() != scaled(12) {
		t.Fatalf("override gap = %d, want %d", e.gap(), scaled(12))
	}

	if e.iconSquare() != 0 {
		t.Fatalf("no icon → iconSquare 0, got %d", e.iconSquare())
	}
	e.Icon = &alignProbe{}
	if e.iconSquare() != scaled(emptyStateIcon) {
		t.Fatalf("default iconSquare = %d, want %d", e.iconSquare(), scaled(emptyStateIcon))
	}
	e.IconSize = 24
	if e.iconSquare() != scaled(24) {
		t.Fatalf("override iconSquare = %d, want %d", e.iconSquare(), scaled(24))
	}
}

// TestEmptyStateMessageOnlyLayout pins the message-only geometry: the single
// line is vertically centred at (H-glyphH)/2 and spans the full width.
func TestEmptyStateMessageOnlyLayout(t *testing.T) {
	const w, h = 100, 40
	e := NewEmptyState("Empty")
	e.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})

	gh := e.msg.faceFor().Height()
	wantTop := (h - gh) / 2
	if b := e.msg.Bounds(); b.X != 0 || b.W != w || b.Y != wantTop || b.H != gh {
		t.Fatalf("message bounds = %+v, want {0 %d %d %d}", b, wantTop, w, gh)
	}
}

// TestEmptyStateFullStackLayout pins the icon+message+caption stacked geometry
// with exact positions.
func TestEmptyStateFullStackLayout(t *testing.T) {
	const w, h = 100, 60
	e := NewEmptyState("Msg")
	e.Icon = &alignProbe{}
	e.IconSize = 20
	e.Gap = 4
	e.SetCaption("cap")
	e.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})

	gh := e.msg.faceFor().Height() // 7 for the bitmap font
	// total = gh + (20+4) + (4+gh); top = (60-total)/2.
	total := gh + (20 + 4) + (4 + gh)
	top := (h - total) / 2

	if b := e.Icon.Bounds(); b != (Rect{X: (w - 20) / 2, Y: top, W: 20, H: 20}) {
		t.Fatalf("icon bounds = %+v, want centred 20px square at Y=%d", b, top)
	}
	msgY := top + 20 + 4
	if b := e.msg.Bounds(); b != (Rect{X: 0, Y: msgY, W: w, H: gh}) {
		t.Fatalf("message bounds = %+v, want {0 %d %d %d}", b, msgY, w, gh)
	}
	capY := msgY + gh + 4
	if b := e.caption.Bounds(); b != (Rect{X: 0, Y: capY, W: w, H: gh}) {
		t.Fatalf("caption bounds = %+v, want {0 %d %d %d}", b, capY, w, gh)
	}
}

// TestEmptyStateDrawsCentredText is the pixel assertion: the message glyph run is
// horizontally centred (equal margins) and vertically centred in the region.
func TestEmptyStateDrawsCentredText(t *testing.T) {
	const w, h = 120, 40
	e := NewEmptyState("Empty")
	e.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	e.Draw(newP(buf, w), DefaultLight())

	lo, hi := paintedCols(buf, w, h)
	if lo < 0 {
		t.Fatal("centred message painted nothing")
	}
	leftMargin, rightMargin := lo, w-1-hi
	if diff := leftMargin - rightMargin; diff < -1 || diff > 1 {
		t.Fatalf("message not horizontally centred: left=%d right=%d", leftMargin, rightMargin)
	}
	gh := e.msg.faceFor().Height()
	if top := labelTopRow(buf, w, h); top != (h-gh)/2 {
		t.Fatalf("message top row = %d, want vertically centred %d", top, (h-gh)/2)
	}
}

// TestEmptyStateDrawFullStack renders icon+message+caption and checks the caption
// is tinted with the theme's muted ink.
func TestEmptyStateDrawFullStack(t *testing.T) {
	const w, h = 100, 60
	theme := DefaultLight()
	e := NewEmptyState("Msg")
	e.Icon = NewLabel("I") // a real drawing child so the icon branch paints
	e.IconSize = 12
	e.SetCaption("cap")
	e.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	e.Draw(newP(buf, w), theme)

	if labelTopRow(buf, w, h) < 0 {
		t.Fatal("full-stack EmptyState painted nothing")
	}
	if e.caption.Ink != mutedInk(theme) {
		t.Fatal("caption must be tinted with the muted ink")
	}
}

// TestEmptyStateChildren checks the accessibility surface across the icon and
// caption combinations, and the presentational role.
func TestEmptyStateChildren(t *testing.T) {
	msgOnly := NewEmptyState("m")
	if got := msgOnly.Children(); len(got) != 1 || got[0] != Widget(msgOnly.msg) {
		t.Fatalf("message-only Children = %v, want [msg]", got)
	}
	if msgOnly.A11y().Role != RolePresentation {
		t.Fatalf("A11y role = %q, want presentation", msgOnly.A11y().Role)
	}

	icon := &alignProbe{}
	full := NewEmptyState("m")
	full.Icon = icon
	full.SetCaption("c")
	got := full.Children()
	if len(got) != 3 || got[0] != Widget(icon) || got[1] != Widget(full.msg) || got[2] != Widget(full.caption) {
		t.Fatalf("full Children = %v, want [icon msg caption]", got)
	}
}
