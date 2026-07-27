// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// AccordionSection is one titled, collapsible slot in an Accordion:
// a header row showing Title + a body Widget shown only while the
// section is expanded.
type AccordionSection struct {
	Title string
	Body  Widget
}

// Accordion is a vertical stack of AccordionHeaderH-tall header rows,
// each owning a collapsible body below it. By default the sections
// are mutually exclusive (exclusive-accordion behaviour): clicking a
// header expands it + collapses any other expanded section. Setting
// Multiple lets every header toggle independently instead.
//
// Expanded (used when Multiple is false) is the index of the single
// expanded section, or -1 when every section is collapsed. In
// Multiple mode Expanded is ignored + each section's open/closed
// state is tracked independently.
//
// The remaining vertical space below all headers (Bounds().H minus
// every header's height) is shared evenly among the currently
// expanded sections, mirroring how Expander gives its Content the
// full remaining space when there is only one section.
type Accordion struct {
	Base
	Sections []AccordionSection
	Expanded int
	Multiple bool

	// multiExpanded tracks per-section expanded state in Multiple
	// mode. Lazily allocated on first toggle; nil means "nothing
	// expanded yet".
	multiExpanded map[int]bool
}

// NewAccordion builds an Accordion with every section collapsed.
func NewAccordion(sections []AccordionSection) *Accordion {
	return &Accordion{Sections: sections, Expanded: -1}
}

// isExpanded reports whether section i is currently open.
func (a *Accordion) isExpanded(i int) bool {
	if a.Multiple {
		return a.multiExpanded[i]
	}
	return a.Expanded == i
}

// toggle flips section i's expanded state: independently in Multiple
// mode, or exclusively (collapsing any other open section, and
// collapsing to -1 when i was already the open one) otherwise.
func (a *Accordion) toggle(i int) {
	if a.Multiple {
		if a.multiExpanded == nil {
			a.multiExpanded = make(map[int]bool)
		}
		a.multiExpanded[i] = !a.multiExpanded[i]
		return
	}
	if a.Expanded == i {
		a.Expanded = -1
	} else {
		a.Expanded = i
	}
}

// sectionRects lays out every header + body rect in surface
// coordinates (i.e. anchored at Bounds().X/Y, matching the frame
// Draw + translateEvent expect). bodies[i] is the zero Rect for any
// section that isn't currently expanded.
func (a *Accordion) sectionRects() (headers, bodies []Rect) {
	r := a.Bounds()
	n := len(a.Sections)
	headers = make([]Rect, n)
	bodies = make([]Rect, n)

	remaining := r.H - ExpanderHeaderH*n
	if remaining < 0 {
		remaining = 0
	}
	expandedCount := 0
	for i := 0; i < n; i++ {
		if a.isExpanded(i) {
			expandedCount++
		}
	}
	perBody, extra := 0, 0
	if expandedCount > 0 {
		perBody = remaining / expandedCount
		extra = remaining - perBody*expandedCount
	}

	y := r.Y
	seen := 0
	for i := 0; i < n; i++ {
		headers[i] = Rect{X: r.X, Y: y, W: r.W, H: ExpanderHeaderH}
		y += ExpanderHeaderH
		if a.isExpanded(i) {
			h := perBody
			seen++
			if seen == expandedCount {
				// The last expanded section absorbs the remainder so
				// the sections collectively fill Bounds().H exactly.
				h += extra
			}
			bodies[i] = Rect{X: r.X, Y: y, W: r.W, H: h}
			y += h
		}
	}
	return headers, bodies
}

// Draw paints every section's header (title + disclosure chevron)
// + the body of every currently expanded section, clipped to its
// share of the remaining space.
func (a *Accordion) Draw(p painter.Painter, theme *Theme) {
	headers, bodies := a.sectionRects()
	for i, sec := range a.Sections {
		hr := headers[i]
		fillRect(p, hr.X, hr.Y, hr.W, hr.H, theme.SurfaceAlt)
		cx := hr.X + 6
		cy := hr.Y + ExpanderHeaderH/2
		drawDisclosureChevron(p, cx, cy, a.isExpanded(i), theme.OnSurface)
		textY := hr.Y + (ExpanderHeaderH-a.glyphHeight())/2
		a.drawText(p, hr.X+16, textY, sec.Title, theme.OnSurface)
		if sec.Body == nil {
			continue
		}
		if br := bodies[i]; br.H > 0 {
			sec.Body.SetBounds(br)
			sec.Body.Draw(p, theme)
		}
	}
}

// OnEvent: a click on header i toggles it (see toggle); a click
// inside an expanded section's body forwards to that Body via
// translated coordinates. Anything else (non-click events, or a
// click that lands on neither a header nor an open body) is a
// no-op.
func (a *Accordion) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	r := a.Bounds()
	ax, ay := ev.X+r.X, ev.Y+r.Y
	headers, bodies := a.sectionRects()
	for i := range a.Sections {
		hr := headers[i]
		if ay >= hr.Y && ay < hr.Y+hr.H && ax >= hr.X && ax < hr.X+hr.W {
			a.toggle(i)
			return
		}
		br := bodies[i]
		if br.H > 0 && ay >= br.Y && ay < br.Y+br.H && ax >= br.X && ax < br.X+br.W {
			if sec := a.Sections[i]; sec.Body != nil {
				sec.Body.SetBounds(br)
				sec.Body.OnEvent(translateEvent(ev, r, br))
			}
			return
		}
	}
}

// drawDisclosureChevron paints a small ▶ (collapsed) / ▼ (expanded)
// triangle at (cx, cy), matching Expander's chevron so an Accordion
// section looks identical to a standalone Expander header.
func drawDisclosureChevron(p painter.Painter, cx, cy int, expanded bool, ink RGBA) {
	if expanded {
		// ▼ : flat top (widest row), point at bottom (narrow tip).
		for t := 0; t < 5; t++ {
			fillRect(p, cx-t, cy+2-t, 1+2*t, 1, ink)
		}
	} else {
		// ▶ : flat left (tallest column), point at right (narrow tip).
		for t := 0; t < 5; t++ {
			fillRect(p, cx+2-t, cy-t, 1, 1+2*t, ink)
		}
	}
}
