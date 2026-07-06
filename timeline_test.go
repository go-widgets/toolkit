// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- Constructor ---------------------------------------------------------

// NewTimeline stores the events slice verbatim + wires the Base.
func TestNewTimelineStoresEvents(t *testing.T) {
	evs := []TimelineEvent{{Title: "one"}, {Title: "two"}}
	tl := NewTimeline(evs)
	if len(tl.Events) != 2 || tl.Events[0].Title != "one" {
		t.Fatalf("NewTimeline round-trip broken: %+v", tl.Events)
	}
}

// nil events collapses to a non-nil empty slice so range loops
// stay safe.
func TestNewTimelineNilEventsBecomesEmptySlice(t *testing.T) {
	tl := NewTimeline(nil)
	if tl.Events == nil {
		t.Fatal("NewTimeline(nil) must leave Events non-nil")
	}
	if len(tl.Events) != 0 {
		t.Fatalf("len(Events) = %d, want 0", len(tl.Events))
	}
}

// --- timelineMarkerInk (pure helper) -------------------------------------

// Every Kind must map to a distinct ink so an app that reads the
// timeline at a glance can tell success from warning from error.
func TestTimelineMarkerInkKindsAreDistinct(t *testing.T) {
	theme := DefaultLight()
	inks := []RGBA{
		timelineMarkerInk(TimelineDefault, theme),
		timelineMarkerInk(TimelineSuccess, theme),
		timelineMarkerInk(TimelineWarning, theme),
		timelineMarkerInk(TimelineError, theme),
	}
	for i := 0; i < len(inks); i++ {
		for j := i + 1; j < len(inks); j++ {
			if inks[i] == inks[j] {
				t.Fatalf("kinds %d and %d share ink %+v", i, j, inks[i])
			}
		}
	}
}

// TimelineDefault defers to Theme.Accent so a neutral event matches
// the app's accent palette.
func TestTimelineMarkerInkDefaultIsAccent(t *testing.T) {
	theme := DefaultLight()
	if got := timelineMarkerInk(TimelineDefault, theme); got != theme.Accent {
		t.Fatalf("Default ink = %+v, want theme.Accent %+v", got, theme.Accent)
	}
}

// TimelineSuccess / Warning / Error lock in the exact RGB tuples
// Alert already ships — otherwise a Timeline row and an Alert
// banner of the same severity read as different colours.
func TestTimelineMarkerInkFixedTuplesMatchAlert(t *testing.T) {
	theme := DefaultLight()
	if got := timelineMarkerInk(TimelineSuccess, theme); got != RGB(0x2E, 0x8B, 0x57) {
		t.Fatalf("Success ink = %+v, want (0x2E,0x8B,0x57)", got)
	}
	if got := timelineMarkerInk(TimelineWarning, theme); got != RGB(0xE0, 0xA0, 0x30) {
		t.Fatalf("Warning ink = %+v, want (0xE0,0xA0,0x30)", got)
	}
	if got := timelineMarkerInk(TimelineError, theme); got != RGB(0xC0, 0x30, 0x30) {
		t.Fatalf("Error ink = %+v, want (0xC0,0x30,0x30)", got)
	}
}

// Out-of-range Kind falls back to Default (theme.Accent).
func TestTimelineMarkerInkDefaultBranch(t *testing.T) {
	theme := DefaultLight()
	if got := timelineMarkerInk(TimelineKind(999), theme); got != theme.Accent {
		t.Fatalf("default-arm ink = %+v, want theme.Accent %+v", got, theme.Accent)
	}
}

// --- Draw branches -------------------------------------------------------

// Empty events: Surface fill + rail line paint, but no marker
// squares land. Detects a regression where the marker loop
// silently ran once with an empty event.
func TestTimelineDrawEmptyEventsFillAndRailOnly(t *testing.T) {
	const w, h = 120, 80
	theme := DefaultLight()
	tl := NewTimeline(nil)
	tl.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 80})
	buf := makeSurface(w, h)
	tl.Draw(newP(buf, w), theme)

	// Surface fill lands: corner pixel (past the rail column) is Surface.
	if got := pixelAt(buf, w, w-2, 2); got != theme.Surface {
		t.Fatalf("corner fill = %+v, want Surface %+v", got, theme.Surface)
	}
	// Rail line: at railX, inside the padded band, ink is Border.
	railX := TimelinePadX + TimelineMarkerW/2
	if got := pixelAt(buf, w, railX, TimelinePadY+2); got != theme.Border {
		t.Fatalf("rail pixel = %+v, want Border %+v", got, theme.Border)
	}
	// No Accent-inked square marker anywhere (empty events).
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == theme.Accent {
				t.Fatalf("empty timeline painted Accent at (%d,%d)", x, y)
			}
		}
	}
}

// One default-kind event: marker fills Accent, Title paints in
// OnSurface ink to the right of the marker column.
func TestTimelineDrawSingleDefaultEvent(t *testing.T) {
	const w, h = 160, 60
	theme := DefaultLight()
	tl := NewTimeline([]TimelineEvent{{Title: "Hello", Kind: TimelineDefault}})
	tl.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 60})
	buf := makeSurface(w, h)
	tl.Draw(newP(buf, w), theme)

	// Marker centre pixel is Accent.
	railX := TimelinePadX + TimelineMarkerW/2
	markerY := TimelinePadY + (GlyphHeight-TimelineMarkerSize)/2
	// Sample the marker's interior (avoid the rail-intersect edge case).
	if got := pixelAt(buf, w, railX+1, markerY+1); got != theme.Accent {
		t.Fatalf("marker interior = %+v, want Accent %+v", got, theme.Accent)
	}
	// Title text lands: scan the title row for OnSurface pixels.
	textX := TimelinePadX + TimelineMarkerW
	inked := 0
	for y := TimelinePadY; y < TimelinePadY+GlyphHeight; y++ {
		for x := textX; x < w; x++ {
			if pixelAt(buf, w, x, y) == theme.OnSurface {
				inked++
			}
		}
	}
	if inked == 0 {
		t.Fatal("single event painted 0 OnSurface title pixels")
	}
}

// Multiple events, one per Kind, with + without Detail. Exercises
// the marker-colour switch for all four Kinds AND both sides of the
// `if ev.Detail != ""` branch in a single Draw.
func TestTimelineDrawEveryKindWithAndWithoutDetail(t *testing.T) {
	const w, h = 200, 200
	theme := DefaultLight()
	tl := NewTimeline([]TimelineEvent{
		{Title: "Default", Kind: TimelineDefault},
		{Title: "OK", Detail: "deploy green", Kind: TimelineSuccess},
		{Title: "Warn", Kind: TimelineWarning},
		{Title: "Err", Detail: "build red", Kind: TimelineError},
	})
	tl.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	buf := makeSurface(w, h)
	tl.Draw(newP(buf, w), theme)

	railX := TimelinePadX + TimelineMarkerW/2
	// Walk each event and probe the marker interior + look for its ink.
	y := TimelinePadY
	events := tl.Events
	for i, ev := range events {
		markerY := y + (GlyphHeight-TimelineMarkerSize)/2
		wantInk := timelineMarkerInk(ev.Kind, theme)
		got := pixelAt(buf, w, railX+1, markerY+1)
		if got != wantInk {
			t.Fatalf("event %d (%v) marker = %+v, want %+v",
				i, ev.Kind, got, wantInk)
		}
		blockH := TimelineEventH
		if ev.Detail != "" {
			// Detail row should carry dim ink (dimInk helper) somewhere.
			detailY := y + GlyphHeight + TimelineDetailGap
			inked := 0
			wantDim := dimInk(theme)
			for dy := detailY; dy < detailY+GlyphHeight; dy++ {
				for dx := TimelinePadX + TimelineMarkerW; dx < w; dx++ {
					if pixelAt(buf, w, dx, dy) == wantDim {
						inked++
					}
				}
			}
			if inked == 0 {
				t.Fatalf("event %d Detail row painted 0 dim ink pixels", i)
			}
			blockH += TimelineDetailGap + GlyphHeight
		}
		y += blockH
	}
}

// Empty-Detail branch: with a single event whose Detail is "", no
// text lands in the Detail row band. Combined with the previous
// test this covers both sides of the `if Detail != ""` guard.
func TestTimelineDrawEmptyDetailSkipsSecondRow(t *testing.T) {
	const w, h = 160, 60
	theme := DefaultLight()
	tl := NewTimeline([]TimelineEvent{{Title: "T"}})
	tl.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 60})
	buf := makeSurface(w, h)
	tl.Draw(newP(buf, w), theme)
	detailY := TimelinePadY + GlyphHeight + TimelineDetailGap
	textX := TimelinePadX + TimelineMarkerW
	// Dim ink is used by Detail text. Rail line uses theme.Border so
	// restrict the scan to the text column past the rail.
	wantDim := dimInk(theme)
	for y := detailY; y < detailY+GlyphHeight; y++ {
		for x := textX; x < w; x++ {
			if pixelAt(buf, w, x, y) == wantDim {
				t.Fatalf("empty Detail painted dim ink at (%d,%d)", x, y)
			}
		}
	}
}

// Dark theme: swapping Theme.Surface / Border / Accent flips the
// pixel colours without changing the widget code.
func TestTimelineDrawDarkTheme(t *testing.T) {
	const w, h = 160, 80
	theme := DefaultDark()
	tl := NewTimeline([]TimelineEvent{{Title: "T"}})
	tl.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 80})
	buf := makeSurface(w, h)
	tl.Draw(newP(buf, w), theme)
	// Corner fill = dark Surface.
	if got := pixelAt(buf, w, w-2, 2); got != theme.Surface {
		t.Fatalf("dark corner = %+v, want dark Surface %+v", got, theme.Surface)
	}
	// Rail = dark Border. Sample well below the marker's y range
	// (marker is 6 px tall starting at TimelinePadY) so the marker
	// fill doesn't shadow the rail pixel.
	railX := TimelinePadX + TimelineMarkerW/2
	railProbeY := TimelinePadY + TimelineMarkerSize + 4
	if got := pixelAt(buf, w, railX, railProbeY); got != theme.Border {
		t.Fatalf("dark rail = %+v, want dark Border %+v", got, theme.Border)
	}
	// Marker = dark Accent.
	markerY := TimelinePadY + (GlyphHeight-TimelineMarkerSize)/2
	if got := pixelAt(buf, w, railX+1, markerY+1); got != theme.Accent {
		t.Fatalf("dark marker = %+v, want dark Accent %+v", got, theme.Accent)
	}
}

// Zero-width + zero-height bounds: fillRect / rail no-op via their
// own guards; per-event marker + text still fire but the assertion
// is non-panic.
func TestTimelineDrawZeroSizedBoundsNoPanic(t *testing.T) {
	const w, h = 20, 20
	theme := DefaultLight()
	tl := NewTimeline([]TimelineEvent{{Title: "T", Detail: "D"}})
	tl.SetBounds(Rect{X: 5, Y: 5, W: 0, H: 0})
	buf := makeSurface(w, h)
	tl.Draw(newP(buf, w), theme)
	// Zero-size bounds -> the rail's height = 0 - 2*TimelinePadY (negative)
	// so fillRect exits early via w<=0 || h<=0 guard. Just prove no panic.
	_ = pixelAt(buf, w, 5, 5)
}

// theme.Extra["OnAccent"] fallback: Timeline renders on Surface,
// not Accent, so populating Extra["OnAccent"] MUST NOT change any
// drawn pixel.
func TestTimelineDrawIgnoresExtraOnAccent(t *testing.T) {
	const w, h = 160, 80
	base := DefaultLight()
	withExtra := &Theme{
		Background: base.Background, Surface: base.Surface,
		SurfaceAlt: base.SurfaceAlt, OnBackground: base.OnBackground,
		OnSurface: base.OnSurface, Accent: base.Accent, Border: base.Border,
		Extra: map[string]RGBA{"OnAccent": RGB(0xFF, 0x00, 0xFF)},
	}
	tl := NewTimeline([]TimelineEvent{{Title: "T", Detail: "D", Kind: TimelineDefault}})
	tl.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 80})
	bufA := makeSurface(w, h)
	bufB := makeSurface(w, h)
	tl.Draw(newP(bufA, w), base)
	tl.Draw(newP(bufB, w), withExtra)
	for i := range bufA {
		if bufA[i] != bufB[i] {
			t.Fatalf("Extra[OnAccent] changed pixel byte %d: base=%d extra=%d",
				i, bufA[i], bufB[i])
		}
	}
}
