// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// arChild is a Widget stub for ActionRow tests. Records Draw + OnEvent
// calls with the local event so tests can assert coordinate translation.
type arChild struct {
	Base
	draws  int
	events []Event
}

func (c *arChild) Draw(p painter.Painter, theme *Theme) {
	_, _ = p, theme
	c.draws++
}

func (c *arChild) OnEvent(ev Event) { c.events = append(c.events, ev) }

// --- API surface ---------------------------------------------------------

func TestActionRowConstants(t *testing.T) {
	if ActionRowPadX != 12 || ActionRowPadY != 8 ||
		ActionRowSubtitleGap != 2 || ActionRowSlotW != 32 {
		t.Fatalf("constants drifted: PadX=%d PadY=%d Gap=%d SlotW=%d",
			ActionRowPadX, ActionRowPadY, ActionRowSubtitleGap, ActionRowSlotW)
	}
}

func TestNewActionRowDefaults(t *testing.T) {
	a := NewActionRow("Wi-Fi")
	if a.Title != "Wi-Fi" {
		t.Fatalf("Title = %q, want Wi-Fi", a.Title)
	}
	if a.Subtitle != "" || a.Prefix != nil || a.Suffix != nil {
		t.Fatal("NewActionRow left non-zero optional fields")
	}
}

// --- Draw: body + divider ------------------------------------------------

func TestActionRowDrawBodyAndDivider(t *testing.T) {
	const w, h = 200, 40
	theme := DefaultLight()
	a := NewActionRow("Title")
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	a.Draw(newP(buf, w), theme)

	// Body interior: Surface fill.
	if got := pixelAt(buf, w, 100, 20); got != theme.Surface {
		t.Fatalf("body pixel = %+v, want Surface", got)
	}
	// Bottom row: divider in Border tone.
	if got := pixelAt(buf, w, 100, h-1); got != theme.Border {
		t.Fatalf("bottom divider = %+v, want Border", got)
	}
	// Title text present in OnSurface.
	found := false
	for y := ActionRowPadY; y < ActionRowPadY+GlyphHeight() && !found; y++ {
		for x := ActionRowPadX; x < ActionRowPadX+TextWidth("Title") && !found; x++ {
			if pixelAt(buf, w, x, y) == theme.OnSurface {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("title ink missing")
	}
}

// --- Draw: dark theme ----------------------------------------------------

func TestActionRowDrawDarkTheme(t *testing.T) {
	const w, h = 160, 40
	theme := DefaultDark()
	a := NewActionRow("Dark")
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	a.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, 100, 20); got != theme.Surface {
		t.Fatalf("dark body pixel = %+v, want Surface", got)
	}
}

// --- Draw: zero-width bounds ---------------------------------------------

func TestActionRowDrawZeroWidthBoundsNoPanic(t *testing.T) {
	const w, h = 40, 40
	theme := DefaultLight()
	a := NewActionRow("X")
	a.SetBounds(Rect{X: 0, Y: 0, W: 0, H: h})
	buf := makeSurface(w, h)
	a.Draw(newP(buf, w), theme)
}

// --- Draw: subtitle ------------------------------------------------------

func TestActionRowDrawWithSubtitle(t *testing.T) {
	const w, h = 200, 44
	theme := DefaultLight()
	a := NewActionRow("Title")
	a.Subtitle = "sub"
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	a.Draw(newP(buf, w), theme)

	// Subtitle ink is drawn in dimInk tone below the title.
	sy := ActionRowPadY + GlyphHeight() + ActionRowSubtitleGap
	found := false
	for y := sy; y < sy+GlyphHeight() && !found; y++ {
		for x := ActionRowPadX; x < ActionRowPadX+TextWidth("sub") && !found; x++ {
			if pixelAt(buf, w, x, y) == dimInk(theme) {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("subtitle ink missing at expected location")
	}
}

// --- Draw: prefix + suffix ----------------------------------------------

func TestActionRowDrawWithPrefixAndSuffix(t *testing.T) {
	const w, h = 200, 40
	theme := DefaultLight()
	a := NewActionRow("Wi-Fi")
	pfx := &arChild{}
	sfx := &arChild{}
	a.Prefix = pfx
	a.Suffix = sfx
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	a.Draw(newP(buf, w), theme)

	if pfx.draws != 1 || sfx.draws != 1 {
		t.Fatalf("child draws = %d/%d, want 1/1", pfx.draws, sfx.draws)
	}
	// Prefix bounds fill the left slot.
	pb := pfx.Bounds()
	if pb.X != 0 || pb.Y != 0 || pb.W != ActionRowSlotW || pb.H != h {
		t.Fatalf("Prefix bounds = %+v, want (0,0,%d,%d)", pb, ActionRowSlotW, h)
	}
	// Suffix bounds fill the right slot.
	sb := sfx.Bounds()
	if sb.X != w-ActionRowSlotW || sb.Y != 0 || sb.W != ActionRowSlotW || sb.H != h {
		t.Fatalf("Suffix bounds = %+v, want (%d,0,%d,%d)",
			sb, w-ActionRowSlotW, ActionRowSlotW, h)
	}
}

// --- OnEvent: routing to Prefix / Suffix / middle ------------------------

func TestActionRowClickPrefixForwards(t *testing.T) {
	a := NewActionRow("t")
	pfx := &arChild{}
	a.Prefix = pfx
	a.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 40})
	a.OnEvent(Event{Kind: EventClick, X: 4, Y: 20})
	if len(pfx.events) != 1 {
		t.Fatalf("Prefix events = %d, want 1", len(pfx.events))
	}
	if pfx.events[0].X != 4 || pfx.events[0].Y != 20 {
		t.Fatalf("Prefix event coords = %+v, want (4,20)", pfx.events[0])
	}
}

func TestActionRowClickPrefixSlotNilPrefix(t *testing.T) {
	a := NewActionRow("t")
	a.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 40})
	// Nil Prefix: no panic, no forwarding.
	a.OnEvent(Event{Kind: EventClick, X: 4, Y: 20})
}

func TestActionRowClickSuffixForwards(t *testing.T) {
	a := NewActionRow("t")
	sfx := &arChild{}
	a.Suffix = sfx
	a.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 40})
	// X = 190 lands in the last SlotW-wide slot; translated = 190 - (200-32) = 22.
	a.OnEvent(Event{Kind: EventClick, X: 190, Y: 20})
	if len(sfx.events) != 1 {
		t.Fatalf("Suffix events = %d, want 1", len(sfx.events))
	}
	if sfx.events[0].X != 22 || sfx.events[0].Y != 20 {
		t.Fatalf("Suffix event coords = %+v, want (22,20)", sfx.events[0])
	}
}

func TestActionRowClickSuffixSlotNilSuffix(t *testing.T) {
	a := NewActionRow("t")
	a.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 40})
	// Nil Suffix: no panic, no forwarding.
	a.OnEvent(Event{Kind: EventClick, X: 190, Y: 20})
}

func TestActionRowClickMiddleIsNoOp(t *testing.T) {
	a := NewActionRow("t")
	pfx := &arChild{}
	sfx := &arChild{}
	a.Prefix = pfx
	a.Suffix = sfx
	a.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 40})
	a.OnEvent(Event{Kind: EventClick, X: 100, Y: 20})
	if len(pfx.events) != 0 || len(sfx.events) != 0 {
		t.Fatalf("middle click reached a child: pfx=%d sfx=%d",
			len(pfx.events), len(sfx.events))
	}
}

func TestActionRowIgnoresNonClick(t *testing.T) {
	a := NewActionRow("t")
	pfx := &arChild{}
	sfx := &arChild{}
	a.Prefix = pfx
	a.Suffix = sfx
	a.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 40})
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if len(pfx.events) != 0 || len(sfx.events) != 0 {
		t.Fatal("KeyDown should not route through ActionRow")
	}
}
