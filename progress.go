// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// ProgressBar is a bar with a filled portion proportional to Fraction in [0,1].
// Orientation picks the fill direction: Horizontal (default) fills left→right,
// Vertical fills bottom→top. An optional Label is centred over the bar in
// Theme.OnSurface ink (drawn for the horizontal orientation, where it fits).
type ProgressBar struct {
	Base
	Fraction    float64
	Label       string
	Orientation Orientation
}

// NewProgressBar builds an empty (Fraction=0) ProgressBar with no
// label.
func NewProgressBar() *ProgressBar { return &ProgressBar{} }

// SetFraction clamps + assigns Fraction. 0 = empty, 1 = full.
func (p *ProgressBar) SetFraction(f float64) {
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	p.Fraction = f
}

// Draw paints border + track + fill + optional centered label.
func (pb *ProgressBar) Draw(p painter.Painter, theme *Theme) {
	r := pb.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	f := pb.Fraction
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	if pb.Orientation == Vertical {
		// Fill from the bottom up.
		fillH := int(float64(r.H) * f)
		if fillH > 0 {
			fillRect(p, r.X, r.Y+r.H-fillH, r.W, fillH, theme.Accent)
		}
		strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
		return // a centred label doesn't fit a narrow vertical bar
	}
	fillW := int(float64(r.W) * f)
	if fillW > 0 {
		fillRect(p, r.X, r.Y, fillW, r.H, theme.Accent)
	}
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	if pb.Label != "" {
		tw := TextWidth(pb.Label)
		tx := r.X + (r.W-tw)/2
		ty := r.Y + (r.H-GlyphHeight())/2
		DrawText(p, tx, ty, pb.Label, theme.OnSurface)
	}
}

// LevelBar is the discrete cousin of ProgressBar: Max equal cells,
// the first Value cells filled in Accent + the rest in SurfaceAlt.
// Useful for battery / signal-strength / VU-meter style indicators.
// Orientation Horizontal (default) fills left→right; Vertical fills
// bottom→top.
type LevelBar struct {
	Base
	Value, Max  int
	Orientation Orientation
}

// NewLevelBar builds a LevelBar with the given Max (Value defaults
// to 0).
func NewLevelBar(max int) *LevelBar {
	if max < 1 {
		max = 1
	}
	return &LevelBar{Max: max}
}

// Draw paints Max cells with a 1-px gap; the first Value cells use
// Theme.Accent, the rest Theme.SurfaceAlt.
func (l *LevelBar) Draw(p painter.Painter, theme *Theme) {
	r := l.Bounds()
	if l.Max < 1 {
		return
	}
	if l.Orientation == Vertical {
		cellH := (r.H - (l.Max - 1)) / l.Max
		if cellH < 1 {
			cellH = 1
		}
		for i := 0; i < l.Max; i++ {
			// i counts from the bottom (the first cells to fill).
			y := r.Y + r.H - cellH - i*(cellH+1)
			if y < r.Y {
				break // no room above — stop (as do all higher cells)
			}
			fill := theme.SurfaceAlt
			if i < l.Value {
				fill = theme.Accent
			}
			fillRect(p, r.X, y, r.W, cellH, fill)
		}
		strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
		return
	}
	cellW := (r.W - (l.Max - 1)) / l.Max
	if cellW < 1 {
		cellW = 1
	}
	for i := 0; i < l.Max; i++ {
		x := r.X + i*(cellW+1)
		if x+cellW > r.X+r.W {
			break // this cell would spill past the widget — stop (as do all after)
		}
		fill := theme.SurfaceAlt
		if i < l.Value {
			fill = theme.Accent
		}
		fillRect(p, x, r.Y, cellW, r.H, fill)
	}
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
}
