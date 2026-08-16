// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// GroupCard draws in the default bitmap font: glyph height 7, advance 6. Card
// constants: CardPadX=8, CardPadY=6, CardGapX=6, CardLineSpacing=2, BadgePadX=4,
// BadgePadY=1. GroupChevronW=16, GroupMemberH=20, GroupCheckSize=18. Derived:
// badgeRowH=9, titleSlot=9, metaH=7, headerContentH=25. Every geometry below is
// worked out by hand from those so a layout shift is caught.

func TestGroupCardMeasure(t *testing.T) {
	c := NewGroupCard("SRC", "base", "12 parts")
	// Collapsed: 2*CardPadY(6) + headerContentH(25) = 37.
	if got := c.Measure(300); got != 37 {
		t.Fatalf("collapsed measure = %d, want 37", got)
	}
	// Expanded with 3 members: 37 + 3*GroupMemberH(20) = 97.
	c.Expanded = true
	c.Members = []string{"a", "b", "c"}
	if got := c.Measure(300); got != 97 {
		t.Fatalf("expanded measure = %d, want 97", got)
	}
}

func TestGroupCardRects(t *testing.T) {
	c := NewGroupCard("Usenet", "release.base", "12 parts · 3 files · 40 MB")
	c.SetBounds(Rect{X: 0, Y: 0, W: 300, H: c.Measure(300)})

	// Chevron: {inner.X, inner.Y+(25-16)/2, 16, 16} = {8, 10, 16, 16}.
	if got := c.ChevronRect(); got != (Rect{X: 8, Y: 10, W: 16, H: 16}) {
		t.Fatalf("ChevronRect = %+v, want {8 10 16 16}", got)
	}

	// Not actionable: no action pill, no checkbox.
	if got := c.ActionRect(); got != (Rect{}) {
		t.Fatalf("ActionRect (inert) = %+v, want empty", got)
	}
	if got := c.CheckRect(); got != (Rect{}) {
		t.Fatalf("CheckRect (inert) = %+v, want empty", got)
	}

	// Member 0 row: {contentX=24, 31, W=268, 20}.
	if got := c.MemberRect(0); got != (Rect{X: 24, Y: 31, W: 268, H: 20}) {
		t.Fatalf("MemberRect(0) = %+v, want {24 31 268 20}", got)
	}
	// Member 1 sits one GroupMemberH lower.
	if got := c.MemberRect(1); got.Y != 51 {
		t.Fatalf("MemberRect(1).Y = %d, want 51", got.Y)
	}
}

func TestGroupCardActionableRects(t *testing.T) {
	c := NewGroupCard("Usenet", "base", "meta")
	c.Actionable = true
	c.Action = "Reconstruct"
	c.SetBounds(Rect{X: 0, Y: 0, W: 300, H: c.Measure(300)})

	// actionW = 6*11 + 2*4 + 8 = 82; actionH = 7 + 2 + 4 = 13.
	// ActionRect.X = inner.X(8) + inner.W(284) - 82 = 210; Y = 6+(25-13)/2 = 12.
	if got := c.ActionRect(); got != (Rect{X: 210, Y: 12, W: 82, H: 13}) {
		t.Fatalf("ActionRect = %+v, want {210 12 82 13}", got)
	}
	// CheckRect: right = ActionRect.X - CardPadX = 202; X = 202-18 = 184;
	// Y = 6 + (25-18)/2 = 9.
	if got := c.CheckRect(); got != (Rect{X: 184, Y: 9, W: 18, H: 18}) {
		t.Fatalf("CheckRect = %+v, want {184 9 18 18}", got)
	}

	// Actionable but no Action pill: the checkbox right-aligns to the content edge.
	c.Action = ""
	if got := c.ActionRect(); got != (Rect{}) {
		t.Fatalf("ActionRect (no action text) = %+v, want empty", got)
	}
	// right = inner.X+inner.W = 292; X = 292-18 = 274.
	if got := c.CheckRect(); got != (Rect{X: 274, Y: 9, W: 18, H: 18}) {
		t.Fatalf("CheckRect (no action) = %+v, want {274 9 18 18}", got)
	}
}

func TestBadgeInk(t *testing.T) {
	set := RGBA{R: 1, G: 2, B: 3, A: 0xFF}
	if got := badgeInk(set, RGBA{}); got != set {
		t.Fatalf("explicit ink = %+v, want %+v", got, set)
	}
	light := RGBA{R: 0xEE, G: 0xEE, B: 0xEE, A: 0xFF}
	if got := badgeInk(RGBA{}, light); got != readableInk(light) {
		t.Fatalf("derived ink on a light fill = %+v", got)
	}
	if got := badgeInk(RGBA{}, RGBA{}); got.A != 0 {
		t.Fatalf("no ink and no fill should be zero, got %+v", got)
	}
}

func TestGroupCardPillWidth(t *testing.T) {
	c := NewGroupCard("", "", "")
	if got := c.pillWidth(""); got != 0 {
		t.Fatalf("empty pill width = %d, want 0", got)
	}
	// "AB" => 6*2 + 2*BadgePadX(4) = 20.
	if got := c.pillWidth("AB"); got != 20 {
		t.Fatalf("pill width = %d, want 20", got)
	}
}

func TestGroupCardChildren(t *testing.T) {
	// Collapsed full header: title + meta, no members.
	c := NewGroupCard("Usenet", "base", "meta")
	if kids := c.Children(); len(kids) != 2 {
		t.Fatalf("collapsed children = %d, want 2 (title, meta)", len(kids))
	}
	// Expanded: title + meta + each member.
	c.Expanded = true
	c.Members = []string{"p1", "p2"}
	if kids := c.Children(); len(kids) != 4 {
		t.Fatalf("expanded children = %d, want 4", len(kids))
	}
	// No title, no meta, collapsed => nil.
	empty := NewGroupCard("Usenet", "", "")
	if kids := empty.Children(); kids != nil {
		t.Fatalf("empty card children = %v, want nil", kids)
	}
	// Meta only.
	metaOnly := NewGroupCard("Usenet", "", "just meta")
	if kids := metaOnly.Children(); len(kids) != 1 {
		t.Fatalf("meta-only children = %d, want 1", len(kids))
	}
}

func TestGroupCardChildrenRuns(t *testing.T) {
	c := NewGroupCard("Usenet", "release.base", "12 parts · 3 files")
	c.Expanded = true
	c.Members = []string{"file.part1 (1/2) 1 MB", "file.part2 (2/2) 1 MB"}
	r := Rect{X: 20, Y: 20, W: 260, H: c.Measure(260)}
	c.SetBounds(r)
	buf := makeSurface(r.X+r.W+20, r.Y+r.H+20)
	c.Draw(newP(buf, r.X+r.W+20), DefaultLight())

	runs := CollectRuns(c)
	want := []string{"release.base", "12 parts · 3 files", "file.part1 (1/2) 1 MB", "file.part2 (2/2) 1 MB"}
	if len(runs) != len(want) {
		t.Fatalf("want %d runs %v, got %d %v", len(want), want, len(runs), runs)
	}
	for i, w := range want {
		if runs[i].Text != w {
			t.Fatalf("run %d = %q, want %q", i, runs[i].Text, w)
		}
		b := runs[i].Bounds
		if b.X < r.X || b.Y < r.Y || b.X+b.W > r.X+r.W || b.Y+b.H > r.Y+r.H {
			t.Fatalf("run %d (%q) bounds %+v escape the card %+v", i, w, b, r)
		}
	}
}

func TestGroupCardNarrowClamps(t *testing.T) {
	// A card narrower than the chevron column drives both the title and meta widths
	// negative; assemble clamps them to 1 rather than panicking.
	c := NewGroupCard("U", "a very long title that cannot fit", "a very long meta line")
	c.SetBounds(Rect{X: 0, Y: 0, W: 20, H: c.Measure(20)})
	buf := makeSurface(60, 60)
	c.Draw(newP(buf, 60), DefaultLight()) // must not panic
	if c.titleLbl.Bounds().W < 1 || c.metaLbl.Bounds().W < 1 {
		t.Fatalf("clamped widths must stay >= 1, got title=%d meta=%d", c.titleLbl.Bounds().W, c.metaLbl.Bounds().W)
	}
}

func TestGroupCardA11y(t *testing.T) {
	got := NewGroupCard("Usenet", "The Base", "meta").A11y()
	if got != (A11yInfo{Role: RoleGroup, Name: "The Base"}) {
		t.Fatalf("A11y = %+v, want a group named by the title", got)
	}
}

func TestGroupCardDrawFull(t *testing.T) {
	// A fully-featured card (source + status pills, checkbox + action pill, expanded
	// members) paints its accent action pill and stays within bounds.
	accent := DefaultLight().Accent
	c := &GroupCard{
		Pill:        "Usenet",
		PillColor:   RGBA{R: 0x20, G: 0x60, B: 0xC0, A: 0xFF},
		Status:      "complete",
		StatusColor: RGBA{R: 0x2E, G: 0x7D, B: 0x32, A: 0xFF},
		Title:       "release.base.name",
		Meta:        "12 parts · 3 files · 40 MB",
		Expanded:    true,
		Members:     []string{"part one", "part two"},
		Actionable:  true,
		Action:      "Reconstruct",
		Checked:     true,
	}
	const pad = 12
	h := c.Measure(280)
	surfW, surfH := 280+2*pad, h+2*pad
	buf := makeSurface(surfW, surfH)
	c.SetBounds(Rect{X: pad, Y: pad, W: 280, H: h})
	c.Draw(newP(buf, surfW), DefaultLight())

	// The action pill fills in the theme accent, so those pixels appear.
	if !hasColor(buf, surfW, accent) {
		t.Fatal("the accent action pill was never painted")
	}
	// The source pill fill appears too.
	if !hasColor(buf, surfW, RGBA{R: 0x20, G: 0x60, B: 0xC0, A: 0xFF}) {
		t.Fatal("the source pill was never painted")
	}
	// Nothing paints outside the surface's card region.
	minX, minY, maxX, maxY := nbPaintedBBox(buf, surfW, surfH)
	if maxX < 0 {
		t.Fatal("card painted nothing")
	}
	if minX < pad || minY < pad || maxX >= pad+280 || maxY >= pad+h {
		t.Fatalf("painting escaped the card: bbox (%d,%d)-(%d,%d) card (%d,%d)-(%d,%d)", minX, minY, maxX, maxY, pad, pad, pad+280, pad+h)
	}
}

func TestGroupCardDrawMinimal(t *testing.T) {
	// A bare collapsed card (no pill, no status, not actionable) still paints its
	// frame + chevron and does not panic.
	c := &GroupCard{Title: "just a title", Meta: "just meta"}
	h := c.Measure(200)
	buf := makeSurface(220, h+20)
	c.SetBounds(Rect{X: 10, Y: 10, W: 200, H: h})
	c.Draw(newP(buf, 220), DefaultLight())
	if _, _, maxX, _ := nbPaintedBBox(buf, 220, h+20); maxX < 0 {
		t.Fatal("a minimal card should still paint its frame + chevron")
	}
}
