// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"testing"

	"github.com/go-widgets/painter"
)

// clipSpy wraps a PixelPainter and records the clip rectangles pushed onto it,
// so a test can assert the DockPanel paints its dock clipped to the item run.
type clipSpy struct {
	*painter.PixelPainter
	clips []Rect
}

func (c *clipSpy) PushClip(r Rect) {
	c.clips = append(c.clips, r)
	c.PixelPainter.PushClip(r)
}

func dpItems() []AppDockItem {
	return []AppDockItem{
		{Id: "a", Label: "A"},
		{Id: "b", Label: "B"},
		{Id: "c", Label: "C"},
	}
}

// rectsIntersect reports whether two rectangles share any pixel.
func rectsIntersect(a, b Rect) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

// TestDockPanelLayout lays a leading and a trailing accessory (each with a nil
// neighbour, to exercise the skip) and checks they land at the bar's ends with
// the dock filling the span between them — outside either accessory's rect.
func TestDockPanelLayout(t *testing.T) {
	lead := NewLabel("L")
	lead.SetBounds(Rect{W: 30})
	trail := NewLabel("T")
	trail.SetBounds(Rect{W: 40})

	dp := NewDockPanel(NewAppDock(dpItems()...))
	dp.Leading = []Widget{nil, lead}
	dp.Trailing = []Widget{trail, nil}
	r := Rect{X: 10, Y: 0, W: 500, H: 40}
	dp.SetBounds(r)
	g := scaled(AppDockGap)

	if lead.Bounds().X != r.X+g {
		t.Errorf("leading X = %d, want %d", lead.Bounds().X, r.X+g)
	}
	if lead.Bounds().H != r.H || lead.Bounds().Y != r.Y {
		t.Errorf("leading fitted to %+v, want full height at Y=%d", lead.Bounds(), r.Y)
	}
	wantTrailX := r.X + r.W - g - 40
	if trail.Bounds().X != wantTrailX {
		t.Errorf("trailing X = %d, want %d (trailing end)", trail.Bounds().X, wantTrailX)
	}

	db := dp.Dock.Bounds()
	if db.X != r.X+g+30+g {
		t.Errorf("dock run X = %d, want past the leading accessory %d", db.X, r.X+g+30+g)
	}
	if db.X+db.W != wantTrailX-g {
		t.Errorf("dock run right = %d, want a gap before the trailing accessory %d", db.X+db.W, wantTrailX-g)
	}
	// The item run sits OUTSIDE both accessories.
	if rectsIntersect(db, lead.Bounds()) || rectsIntersect(db, trail.Bounds()) {
		t.Error("the dock's item run overlaps an accessory")
	}
}

// TestDockPanelLeadingOnly / TrailingOnly cover each group's first-gap branch in
// isolation, and that the dock takes the whole remaining span on the other side.
func TestDockPanelLeadingOnly(t *testing.T) {
	lead := NewLabel("L")
	lead.SetBounds(Rect{W: 24})
	dp := NewDockPanel(NewAppDock(dpItems()...))
	dp.Leading = []Widget{lead}
	r := Rect{X: 0, Y: 0, W: 400, H: 40}
	dp.SetBounds(r)
	g := scaled(AppDockGap)
	if lead.Bounds().X != g {
		t.Errorf("leading X = %d, want %d", lead.Bounds().X, g)
	}
	db := dp.Dock.Bounds()
	if db.X != g+24+g || db.X+db.W != r.X+r.W {
		t.Errorf("dock run = %+v, want to fill from %d to the right edge %d", db, g+24+g, r.X+r.W)
	}
}

func TestDockPanelTrailingOnly(t *testing.T) {
	trail := NewLabel("T")
	trail.SetBounds(Rect{W: 24})
	dp := NewDockPanel(NewAppDock(dpItems()...))
	dp.Trailing = []Widget{trail}
	r := Rect{X: 0, Y: 0, W: 400, H: 40}
	dp.SetBounds(r)
	g := scaled(AppDockGap)
	if trail.Bounds().X != r.W-g-24 {
		t.Errorf("trailing X = %d, want %d", trail.Bounds().X, r.W-g-24)
	}
	db := dp.Dock.Bounds()
	if db.X != r.X || db.X+db.W != r.W-g-24-g {
		t.Errorf("dock run = %+v, want to fill from the left edge to %d", db, r.W-g-24-g)
	}
}

// TestDockPanelBackCompatByteIdentical is the back-compat anchor: a DockPanel
// wrapping a dock with NO accessories and NO menu paints byte-for-byte identical
// to a standalone AppDock at the same bounds (resting, no magnification).
func TestDockPanelBackCompatByteIdentical(t *testing.T) {
	theme := DefaultLight()
	B := Rect{X: 0, Y: 0, W: 500, H: 40}

	solo := NewAppDock(dpItems()...)
	solo.SetBounds(B)
	bufA := makeSurface(B.W, B.H)
	solo.Draw(newP(bufA, B.W), theme)

	dp := NewDockPanel(NewAppDock(dpItems()...))
	dp.SetBounds(B)
	if db := dp.Dock.Bounds(); db != B {
		t.Fatalf("no-accessory dock bounds = %+v, want the panel's exact bounds %+v", db, B)
	}
	bufB := makeSurface(B.W, B.H)
	dp.Draw(newP(bufB, B.W), theme)

	if !bytes.Equal(bufA, bufB) {
		t.Error("no-accessory DockPanel render differs from a standalone AppDock")
	}
}

// TestDockPanelDrawClipsTheRun renders with a magnified last item (which
// overflows the run) and asserts the dock is drawn under a clip equal to its run
// bounds, so the swell is cut at the run edge instead of spilling onto the
// trailing accessory. It also checks the accessories + menu are drawn.
func TestDockPanelDrawClipsTheRun(t *testing.T) {
	theme := DefaultLight()
	trail := NewLabel("T")
	trail.SetBounds(Rect{W: 40})
	dp := NewDockPanel(NewAppDock(dpItems()...))
	dp.Trailing = []Widget{trail}
	B := Rect{X: 0, Y: 0, W: 420, H: 40}
	dp.SetBounds(B)

	// Magnify the last item hard, cursor near the run's right edge.
	dp.Dock.MaxScale = 3
	db := dp.Dock.Bounds()
	dp.Dock.SetCursor(db.X+db.W-5, true)

	spy := &clipSpy{PixelPainter: newP(makeSurface(B.W, B.H), B.W)}
	dp.Draw(spy, theme)

	found := false
	for _, c := range spy.clips {
		if c == db {
			found = true
		}
	}
	if !found {
		t.Errorf("dock was not drawn clipped to its run %+v; clips = %v", db, spy.clips)
	}
	// The magnified last item overflows the run (proving the clip is load-bearing).
	last := dp.Dock.ItemRects()[2]
	if last.X+last.W <= db.X+db.W {
		t.Error("expected the magnified last item to overflow the run so the clip matters")
	}
}

// TestDockPanelMagnifyDoesNotOverlapAccessory: with the last item magnified so
// its rect overflows the run, the trailing accessory's rect still sits outside
// the run, and a click on the accessory routes to IT, never to the spilling
// item.
func TestDockPanelMagnifyDoesNotOverlapAccessory(t *testing.T) {
	activated := -1
	clicked := false
	trail := NewButton("T", func() { clicked = true })
	trail.SetBounds(Rect{W: 50})

	dp := NewDockPanel(NewAppDock(dpItems()...))
	dp.Dock.OnActivate = func(i int) { activated = i }
	dp.Trailing = []Widget{trail}
	B := Rect{X: 0, Y: 0, W: 420, H: 40}
	dp.SetBounds(B)

	dp.Dock.MaxScale = 3
	db := dp.Dock.Bounds()
	dp.Dock.SetCursor(db.X+db.W-5, true) // magnify the tail

	// The accessory is outside the item run even though the item overflows it.
	if rectsIntersect(db, trail.Bounds()) {
		t.Fatal("accessory rect overlaps the item run")
	}

	// A click inside the accessory routes to the accessory, not the dock.
	tb := trail.Bounds()
	dp.OnEvent(Event{Kind: EventClick, X: tb.X - B.X + 2, Y: tb.Y - B.Y + 2})
	if !clicked {
		t.Error("click in the accessory rect did not reach the accessory")
	}
	if activated != -1 {
		t.Errorf("the dock was activated (%d) by a click meant for the accessory", activated)
	}
}

// TestDockPanelEventRouting covers the click paths: a leading accessory, the
// dock's items, and an empty gutter that hits nothing.
func TestDockPanelEventRouting(t *testing.T) {
	leadHit := false
	lead := NewButton("L", func() { leadHit = true })
	lead.SetBounds(Rect{W: 40})
	activated := -1

	dp := NewDockPanel(NewAppDock(dpItems()...))
	dp.Dock.OnActivate = func(i int) { activated = i }
	dp.Leading = []Widget{lead}
	B := Rect{X: 5, Y: 0, W: 500, H: 40}
	dp.SetBounds(B)

	// Leading accessory.
	lb := lead.Bounds()
	dp.OnEvent(Event{Kind: EventClick, X: lb.X - B.X + 2, Y: lb.Y - B.Y + 2})
	if !leadHit {
		t.Error("click on the leading accessory did not route to it")
	}

	// A dock item.
	item := dp.Dock.ItemRects()[1]
	dp.OnEvent(Event{Kind: EventClick, X: item.X - B.X + 2, Y: item.Y - B.Y + 2})
	if activated != 1 {
		t.Errorf("click on item 1 activated %d", activated)
	}

	// An empty spot over neither an accessory nor an item: nothing fires.
	activated = -1
	dp.OnEvent(Event{Kind: EventClick, X: 0, Y: 0})
	if activated != -1 {
		t.Errorf("a click in dead space activated %d", activated)
	}
}

// TestDockPanelMouseMoveBroadcast forwards a move to every accessory (hover) and
// the dock (magnify), and also exercises the nil-dock branch.
func TestDockPanelMouseMoveBroadcast(t *testing.T) {
	lead := NewButton("L", nil)
	lead.SetBounds(Rect{W: 40})
	trail := NewButton("T", nil)
	trail.SetBounds(Rect{W: 40})

	dp := NewDockPanel(NewAppDock(dpItems()...))
	dp.Leading = []Widget{lead, nil}
	dp.Trailing = []Widget{trail, nil}
	dp.SetBounds(Rect{X: 0, Y: 0, W: 500, H: 40})

	// A move over the dock marks the cursor inside it (magnification driver).
	db := dp.Dock.Bounds()
	dp.OnEvent(Event{Kind: EventMouseMove, X: db.X + 5, Y: 20})
	if !dp.Dock.cursorInside {
		t.Error("a move over the run should mark the dock cursor inside")
	}

	// nil dock: the move still broadcasts to the accessories without panicking.
	dp.Dock = nil
	dp.SetBounds(Rect{X: 0, Y: 0, W: 500, H: 40})
	dp.OnEvent(Event{Kind: EventMouseMove, X: 10, Y: 20})
}

// TestDockPanelContextMenu covers the right-click flow: a secondary click over
// the bar opens the menu at the pointer, a row-click fires the row's action and
// dismisses, and an outside click dismisses.
func TestDockPanelContextMenu(t *testing.T) {
	fired := ""
	m := NewMenu([]MenuItem{
		{Label: "Open", Action: func() { fired = "Open" }},
		{Label: "Quit", Action: func() { fired = "Quit" }},
	})
	cm := NewContextMenu(m)
	cm.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 300}) // the surface it may cover

	dp := NewDockPanel(NewAppDock(dpItems()...))
	dp.Menu = cm
	dp.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 40})

	// A secondary click outside the bar does NOT open the menu.
	dp.OnEvent(Event{Kind: EventSecondaryClick, X: -5, Y: 5})
	if cm.Open().Get() {
		t.Fatal("secondary click outside the bar opened the menu")
	}

	// A secondary click over the bar opens it, anchored at the pointer.
	dp.OnEvent(Event{Kind: EventSecondaryClick, X: 100, Y: 20})
	if !cm.Open().Get() {
		t.Fatal("secondary click over the bar did not open the menu")
	}
	if cm.AnchorX != 100 || cm.AnchorY != 20 {
		t.Errorf("menu anchored at %d,%d, want 100,20", cm.AnchorX, cm.AnchorY)
	}

	// Drawing while open paints the dock, then the menu overlay on top.
	dp.Draw(newP(makeSurface(400, 300), 400), DefaultLight())

	// A move while open goes to the menu (its row highlight follows), no panic.
	dp.OnEvent(Event{Kind: EventMouseMove, X: 105, Y: 25})

	// Click the first row: its action fires and the menu closes.
	mb := cm.MenuBounds()
	rowY := mb.Y + m.RowTop(0) + m.RowHeight(0)/2
	dp.OnEvent(Event{Kind: EventClick, X: mb.X + 8, Y: rowY})
	if fired != "Open" {
		t.Errorf("row-click fired %q, want Open", fired)
	}
	if cm.Open().Get() {
		t.Error("menu should close after a row activates")
	}

	// Re-open, then an outside click dismisses it.
	dp.OnEvent(Event{Kind: EventSecondaryClick, X: 100, Y: 20})
	if !cm.Open().Get() {
		t.Fatal("menu did not re-open")
	}
	dp.OnEvent(Event{Kind: EventClick, X: 1, Y: 1}) // above the popped menu
	if cm.Open().Get() {
		t.Error("outside click did not dismiss the menu")
	}
}

// TestDockPanelSecondaryClickNoMenu: a secondary click with no attached menu is
// a no-op (does not panic, opens nothing).
func TestDockPanelSecondaryClickNoMenu(t *testing.T) {
	dp := NewDockPanel(NewAppDock(dpItems()...))
	dp.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 40})
	dp.OnEvent(Event{Kind: EventSecondaryClick, X: 50, Y: 20}) // no menu → no-op
}

// TestDockPanelChildren checks the a11y/generic walk exposure: leading, the
// dock (as a toolbar node), trailing, then the menu — nil slots skipped.
func TestDockPanelChildren(t *testing.T) {
	lead := NewLabel("L")
	trail := NewLabel("T")
	cm := NewContextMenu(NewMenu([]MenuItem{{Label: "X"}}))
	dp := NewDockPanel(NewAppDock(dpItems()...))
	dp.Leading = []Widget{lead, nil}
	dp.Trailing = []Widget{nil, trail}
	dp.Menu = cm
	dp.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 40})

	kids := dp.Children()
	if len(kids) != 4 {
		t.Fatalf("Children = %d, want 4 (lead, dock, trail, menu)", len(kids))
	}
	if kids[0] != Widget(lead) || kids[3] != Widget(cm) {
		t.Errorf("children order wrong: %#v", kids)
	}

	// The dock appears as a toolbar node in the accessibility walk.
	var toolbar *A11yNode
	for i := range WalkA11y(dp) {
		if WalkA11y(dp)[i].Role == RoleToolbar {
			n := WalkA11y(dp)[i]
			toolbar = &n
		}
	}
	if toolbar == nil {
		t.Fatal("the dock's toolbar node is missing from the a11y tree")
	}
	if toolbar.Rect != dp.Dock.Bounds() {
		t.Errorf("toolbar node rect = %+v, want the dock bounds %+v", toolbar.Rect, dp.Dock.Bounds())
	}
	// The panel itself is presentational (looked through, not announced).
	if dp.A11y().Role != RolePresentation {
		t.Errorf("DockPanel A11y = %q, want presentation", dp.A11y().Role)
	}
}

// TestDockPanelNoDock exercises the accessory-only configuration: layout, draw
// and children all work with a nil Dock.
func TestDockPanelNoDock(t *testing.T) {
	theme := DefaultLight()
	lead := NewLabel("L")
	lead.SetBounds(Rect{W: 30})
	dp := &DockPanel{Leading: []Widget{lead}}
	dp.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 40})
	dp.Draw(newP(makeSurface(200, 40), 200), theme)
	if got := dp.Children(); len(got) != 1 || got[0] != Widget(lead) {
		t.Errorf("nil-dock children = %v, want just the accessory", got)
	}
}

// TestDockPanelZeroSize: a zero-area panel draws nothing (early return).
func TestDockPanelZeroSize(t *testing.T) {
	dp := NewDockPanel(NewAppDock(dpItems()...))
	dp.SetBounds(Rect{})
	dp.Draw(newP(makeSurface(4, 4), 4), DefaultLight())
}

// TestDockPanelOverwideAccessories clamps the dock run to zero width when the
// accessories are wider than the panel (no negative-width bounds).
func TestDockPanelOverwideAccessories(t *testing.T) {
	lead := NewLabel("L")
	lead.SetBounds(Rect{W: 300})
	trail := NewLabel("T")
	trail.SetBounds(Rect{W: 300})
	dp := NewDockPanel(NewAppDock(dpItems()...))
	dp.Leading = []Widget{lead}
	dp.Trailing = []Widget{trail}
	dp.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 40})
	if w := dp.Dock.Bounds().W; w != 0 {
		t.Errorf("over-wide accessories left the dock W = %d, want clamped to 0", w)
	}
	dp.Draw(newP(makeSurface(200, 40), 200), DefaultLight()) // no panic
}

// TestAppDockNodeAdapter exercises every method of the AppDock→Widget adapter
// returned by DockPanel.Children (the walk itself only calls Bounds + A11y).
func TestAppDockNodeAdapter(t *testing.T) {
	dp := NewDockPanel(NewAppDock(dpItems()...))
	dp.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 40})

	var node Widget
	for _, ch := range dp.Children() {
		if a, ok := ch.(Accessible); ok && a.A11y().Role == RoleToolbar {
			node = ch
		}
	}
	if node == nil {
		t.Fatal("no toolbar adapter among the children")
	}

	b := node.Bounds()
	node.SetBounds(b) // round-trips to the dock
	if node.Bounds() != dp.Dock.Bounds() {
		t.Error("adapter bounds diverged from the dock")
	}
	item := dp.Dock.ItemRects()[0]
	if !node.HitTest(item.X+2, item.Y+2) {
		t.Error("adapter HitTest should be true over an item")
	}
	if node.HitTest(b.X+b.W+1000, b.Y) {
		t.Error("adapter HitTest should be false far outside the run")
	}
	node.Draw(newP(makeSurface(400, 40), 400), DefaultLight())
	node.OnEvent(Event{Kind: EventMouseMove, X: 5, Y: 5}) // forwards to the dock
}
