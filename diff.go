// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Diff renders a coloured, line-by-line unified diff view. Each line
// carries a kind (context / added / removed); Draw fills the row with
// a light green tint for added lines, a light red tint for removed
// lines, and the theme's Surface for context lines. A one-character
// prefix (' ', '+', '-') anchors the row at the left so the widget
// stays legible even when its background rows are omitted (as when a
// caller reuses this on top of a striped background).
//
// The widget is intentionally passive: it exposes no scroll, no
// selection, and no editing. Host apps that need those wrap Diff in a
// ScrollView + track selection externally.
type Diff struct {
	Base
	Lines []DiffLine
}

// DiffLine is one row in a Diff view: the raw text plus the change
// kind that colours it.
type DiffLine struct {
	Text string
	Kind DiffKind
}

// DiffKind enumerates the three per-line change categories a unified
// diff produces.
type DiffKind int

const (
	// DiffContext marks an unchanged, contextual line — rendered on
	// Theme.Surface with a leading space.
	DiffContext DiffKind = iota
	// DiffAdded marks a line inserted by the change — rendered on a
	// light green tint with a leading '+'.
	DiffAdded
	// DiffRemoved marks a line dropped by the change — rendered on a
	// light red tint with a leading '-'.
	DiffRemoved
)

// DiffLineH is the vertical stride between successive lines: one
// glyph tall plus two pixels of separation.
func DiffLineH() int { return GlyphHeight() + 2 }

// DiffPadX is the horizontal padding between the widget's outer
// border and the leading prefix glyph.
const DiffPadX = 4

// DiffPadY is the vertical padding above the first line and below
// the last line.
const DiffPadY = 2

// diffAddedFill is the row tint painted behind DiffAdded lines.
var diffAddedFill = RGBA{R: 200, G: 240, B: 200, A: 255}

// diffRemovedFill is the row tint painted behind DiffRemoved lines.
var diffRemovedFill = RGBA{R: 245, G: 210, B: 210, A: 255}

// diffAddedInk / diffRemovedInk are the FIXED text tones used on
// the (also fixed) Added/Removed row backgrounds. Fixed because the
// row backgrounds themselves are theme-independent (a diff view is
// a semantic display — the "added lines are green" convention is
// stronger than any per-theme adaptation), so pairing them with
// theme.OnSurface (light in dark themes) would sit unreadable light
// text on a light row. A dark ink pairs correctly with the light
// backgrounds in both light AND dark themes.
var (
	diffAddedInk   = RGBA{R: 20, G: 60, B: 20, A: 255}
	diffRemovedInk = RGBA{R: 90, G: 20, B: 20, A: 255}
)

// NewDiff builds a Diff view over the supplied lines. A nil slice is
// normalised to a zero-length slice so Draw never has to nil-guard.
func NewDiff(lines []DiffLine) *Diff {
	if lines == nil {
		lines = []DiffLine{}
	}
	return &Diff{Lines: lines}
}

// Draw paints the widget body, each row (with its per-kind tint and
// prefix glyph), and the outer border.
func (d *Diff) Draw(p painter.Painter, theme *Theme) {
	r := d.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	for i, line := range d.Lines {
		y := r.Y + DiffPadY + i*DiffLineH()
		fill := theme.Surface
		ink := theme.OnSurface
		prefix := " "
		switch line.Kind {
		case DiffAdded:
			fill = diffAddedFill
			ink = diffAddedInk
			prefix = "+"
		case DiffRemoved:
			fill = diffRemovedFill
			ink = diffRemovedInk
			prefix = "-"
		}
		fillRect(p, r.X+1, y, r.W-2, DiffLineH(), fill)
		DrawText(p, r.X+DiffPadX, y, prefix, ink)
		DrawText(p, r.X+DiffPadX+GlyphAdvance(), y, line.Text, ink)
	}
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
}
