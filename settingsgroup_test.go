// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func threeSwitchRows() (*SettingsGroup, []*Switch) {
	sws := []*Switch{NewSwitch(false), NewSwitch(false), NewSwitch(false)}
	g := NewSettingsGroup("Network",
		NewSettingRow("Wi-Fi", sws[0]),
		NewSettingRow("Bluetooth", sws[1]),
		NewSettingRow("Airplane mode", sws[2]),
	)
	return g, sws
}

// --- construction + measure ---------------------------------------------

func TestNewSettingsGroup(t *testing.T) {
	g, _ := threeSwitchRows()
	if g.Title != "Network" || len(g.Rows) != 3 {
		t.Fatalf("group = %q with %d rows, want Network/3", g.Title, len(g.Rows))
	}
}

func TestSettingsGroupHeaderH(t *testing.T) {
	g, _ := threeSwitchRows()
	if g.headerH() != GlyphHeight()+scaled(CardGapY) {
		t.Fatalf("titled headerH = %d", g.headerH())
	}
	g.Title = ""
	if g.headerH() != 0 {
		t.Fatalf("untitled headerH = %d, want 0", g.headerH())
	}
}

func TestSettingsGroupMeasure(t *testing.T) {
	g, _ := threeSwitchRows()
	const w = 320
	inner := w - 2*scaled(CardPadX)
	want := 2*scaled(CardPadY) + g.headerH()
	for _, row := range g.Rows {
		want += row.Measure(inner)
	}
	if got := g.Measure(w); got != want {
		t.Fatalf("Measure = %d, want %d", got, want)
	}
}

// --- draw: frame, caption, stacked rows, divider management --------------

func TestSettingsGroupDrawStacksRowsWithDividers(t *testing.T) {
	g, _ := threeSwitchRows()
	const w = 320
	h := g.Measure(w)
	theme := DefaultLight()
	g.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	g.Draw(newP(buf, w), theme)

	// Rows laid out top-to-bottom, full inner width, contiguous.
	inner := scaled(CardPadX)
	y := scaled(CardPadY) + g.headerH()
	for i, row := range g.Rows {
		rb := row.Bounds()
		if rb.X != inner || rb.Y != y {
			t.Fatalf("row %d bounds = %+v, want X=%d Y=%d", i, rb, inner, y)
		}
		if rb.W != w-2*scaled(CardPadX) {
			t.Fatalf("row %d width = %d, want inner width", i, rb.W)
		}
		y += rb.H
	}
	// Divider on for all but the last row.
	for i, row := range g.Rows {
		wantDiv := i < len(g.Rows)-1
		if row.Divider != wantDiv {
			t.Fatalf("row %d Divider = %v, want %v", i, row.Divider, wantDiv)
		}
	}
	// Caption ink drawn at the top-left in the dim tone.
	if !inkFound(buf, w, scaled(CardPadX), scaled(CardPadY), TextWidth("Network"), GlyphHeight(), dimInk(theme)) {
		t.Fatal("group caption ink missing")
	}
}

func TestSettingsGroupDrawUntitled(t *testing.T) {
	g, _ := threeSwitchRows()
	g.Title = ""
	const w = 320
	h := g.Measure(w)
	theme := DefaultLight()
	g.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	g.Draw(newP(buf, w), theme)
	// First row starts right at the top inset when there is no caption.
	if g.Rows[0].Bounds().Y != scaled(CardPadY) {
		t.Fatalf("untitled first row Y = %d, want %d",
			g.Rows[0].Bounds().Y, scaled(CardPadY))
	}
}

// --- events: routing a click into the right row's control ----------------

func TestSettingsGroupClickRoutesToRow(t *testing.T) {
	g, sws := threeSwitchRows()
	const w = 320
	h := g.Measure(w)
	theme := DefaultLight()
	g.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	g.Draw(newP(buf, w), theme)

	// Click the middle row's switch: only that switch toggles.
	sc := sws[1].Bounds()
	g.OnEvent(Event{Kind: EventClick, X: sc.X + sc.W/2, Y: sc.Y + sc.H/2})
	if sws[0].On().Get() || !sws[1].On().Get() || sws[2].On().Get() {
		t.Fatalf("routing wrong: %v/%v/%v, want only row 1 on",
			sws[0].On().Get(), sws[1].On().Get(), sws[2].On().Get())
	}
}

func TestSettingsGroupClickMissesAllRows(t *testing.T) {
	g, sws := threeSwitchRows()
	const w = 320
	h := g.Measure(w)
	g.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	g.Draw(newP(buf, w), DefaultLight())
	// A click in the caption strip (above every row) reaches nothing.
	g.OnEvent(Event{Kind: EventClick, X: scaled(CardPadX), Y: 1})
	for i, sw := range sws {
		if sw.On().Get() {
			t.Fatalf("row %d toggled on a miss", i)
		}
	}
}

func TestSettingsGroupIgnoresNonClick(t *testing.T) {
	g, sws := threeSwitchRows()
	g.SetBounds(Rect{X: 0, Y: 0, W: 320, H: 200})
	g.Draw(newP(makeSurface(320, 200), 320), DefaultLight())
	g.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	for i, sw := range sws {
		if sw.On().Get() {
			t.Fatalf("row %d toggled on a non-click", i)
		}
	}
}

// --- a11y ----------------------------------------------------------------

func TestSettingsGroupA11yAndChildren(t *testing.T) {
	g, _ := threeSwitchRows()
	if info := g.A11y(); info.Role != RoleGroup || info.Name != "Network" {
		t.Fatalf("A11y = %+v, want group named Network", info)
	}
	if kids := g.Children(); len(kids) != 3 {
		t.Fatalf("Children = %d, want 3 rows", len(kids))
	}
	// A nil row is skipped by Children (a group built incrementally).
	gg := &SettingsGroup{Rows: []*SettingRow{NewSettingRow("a", nil), nil}}
	if kids := gg.Children(); len(kids) != 1 {
		t.Fatalf("Children with a nil row = %d, want 1", len(kids))
	}
}

// WalkA11y descends group -> row -> control, so a screen reader hears the
// group caption, each row's label, and each control.
func TestSettingsGroupWalkA11y(t *testing.T) {
	g, _ := threeSwitchRows()
	g.SetBounds(Rect{X: 0, Y: 0, W: 320, H: g.Measure(320)})
	g.Draw(newP(makeSurface(320, 240), 320), DefaultLight())
	nodes := WalkA11y(g)
	// group(1) + 3 rows + 3 switches = 7 nodes.
	if len(nodes) != 7 {
		t.Fatalf("WalkA11y nodes = %d, want 7", len(nodes))
	}
	if nodes[0].Role != RoleGroup || nodes[0].Name != "Network" {
		t.Fatalf("root node = %+v, want group Network", nodes[0].A11yInfo)
	}
	switches := 0
	for _, n := range nodes {
		if n.Role == RoleSwitch {
			switches++
		}
	}
	if switches != 3 {
		t.Fatalf("switch nodes = %d, want 3", switches)
	}
}
