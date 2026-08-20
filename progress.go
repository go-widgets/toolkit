// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// ProgressBar is a bar with a filled portion proportional to Fraction in [0,1].
// Orientation picks the fill direction: Horizontal (default) fills left→right,
// Vertical fills bottom→top. An optional Label is centred over the bar in
// Theme.OnSurface ink (drawn for the horizontal orientation, where it fits).
//
// When Indeterminate is set the bar ignores Fraction and instead animates a
// short chunk sliding along the track, driven by Phase (0..1, advance it from
// the host frame loop like a Spinner) — for work whose completion is unknown
// (a page fetch, an open-ended request).
type ProgressBar struct {
	Base
	Label         string
	Orientation   Orientation
	Indeterminate bool
	Phase         float64 // 0..1, only used when Indeterminate

	// fraction is the reactive determinate fill in [0,1]; see [ProgressBar.Fraction].
	fraction *mvvm.Observable[float64]
}

// NewProgressBar builds an empty (Fraction=0) ProgressBar with no
// label.
func NewProgressBar() *ProgressBar { return &ProgressBar{} }

// Fraction is the determinate fill level in [0,1] as a shared [mvvm.Observable]:
// SetFraction (or a direct Set) drives it and Draw reads it live. Lazily created,
// defaulting to 0 (empty). Ignored while Indeterminate.
func (pb *ProgressBar) Fraction() *mvvm.Observable[float64] {
	if pb.fraction == nil {
		pb.fraction = mvvm.NewObservable(0.0)
	}
	return pb.fraction
}

// Tick advances the indeterminate sweep by deltaSeconds, wrapping Phase modulo
// 1 so it stays bounded. A determinate bar (the default) has no animation, so
// Tick is a no-op for it — matching what Animating reports. Together they make
// an indeterminate ProgressBar an [Animator], driven by [TickTree] /
// [TreeAnimating].
func (pb *ProgressBar) Tick(deltaSeconds float64) {
	if !pb.Indeterminate {
		return
	}
	pb.Phase += deltaSeconds
	pb.Phase -= math.Floor(pb.Phase)
}

// Animating reports whether the bar still needs frames: true exactly when it is
// Indeterminate (a determinate bar is a static fill and needs no repaint).
func (pb *ProgressBar) Animating() bool { return pb.Indeterminate }

// SetFraction clamps + assigns Fraction. 0 = empty, 1 = full.
func (p *ProgressBar) SetFraction(f float64) {
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	p.Fraction().Set(f)
}

// indetSpan returns the visible offset+length of the sliding chunk within a
// track of trackLen units at phase ph (wrapped to [0,1)). The chunk is 30% of
// the track and travels from fully off the near edge to fully off the far edge;
// length 0 means the chunk is entirely off-track at this phase.
func indetSpan(trackLen int, ph float64) (offset, length int) {
	ph -= math.Floor(ph)
	chunk := trackLen * 3 / 10
	if chunk < 1 {
		chunk = 1
	}
	pos := -chunk + int(ph*float64(trackLen+chunk)) // chunk start relative to origin
	s := pos
	if s < 0 {
		s = 0
	}
	e := pos + chunk
	if e > trackLen {
		e = trackLen
	}
	if e <= s {
		return 0, 0
	}
	return s, e - s
}

// drawIndeterminate paints the track, the sliding Accent chunk, the border and
// an optional label (horizontal only).
func (pb *ProgressBar) drawIndeterminate(p painter.Painter, theme *Theme, r Rect) {
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	if pb.Orientation == Vertical {
		if off, ln := indetSpan(r.H, pb.Phase); ln > 0 {
			fillRect(p, r.X, r.Y+off, r.W, ln, theme.Accent)
		}
		strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
		return
	}
	if off, ln := indetSpan(r.W, pb.Phase); ln > 0 {
		fillRect(p, r.X+off, r.Y, ln, r.H, theme.Accent)
	}
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	if pb.Label != "" {
		tw := pb.textWidth(pb.Label)
		pb.drawText(p, r.X+(r.W-tw)/2, r.Y+(r.H-pb.glyphHeight())/2, pb.Label, theme.OnSurface)
	}
}

// Draw paints border + track + fill + optional centered label.
func (pb *ProgressBar) Draw(p painter.Painter, theme *Theme) {
	r := pb.Bounds()
	if pb.Indeterminate {
		pb.drawIndeterminate(p, theme, r)
		return
	}
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	f := pb.Fraction().Get()
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
		tw := pb.textWidth(pb.Label)
		tx := r.X + (r.W-tw)/2
		ty := r.Y + (r.H-pb.glyphHeight())/2
		pb.drawText(p, tx, ty, pb.Label, theme.OnSurface)
	}
}

// LevelBar is the discrete cousin of ProgressBar: Max equal cells,
// the first Value cells filled + the rest in SurfaceAlt. Useful for
// battery / signal-strength / VU-meter style indicators. Orientation
// Horizontal (default) fills left→right; Vertical fills bottom→top.
//
// Two optional refinements layer on without changing the default look
// (no Label, no Thresholds renders exactly as before, filling in Accent):
//
//   - Label: a caption centred over the bar (horizontal orientation only,
//     where it fits), in Theme.OnSurface ink — e.g. "72%".
//   - Thresholds: value bands that recolour the filled cells (e.g. red when
//     low, amber mid, green high). The band whose Min is the greatest value
//     not exceeding Value wins; with no matching band (or none configured)
//     the fill stays Theme.Accent.
type LevelBar struct {
	Base
	Max         int
	Orientation Orientation
	// Label, when non-empty, is centred over the bar (horizontal only) in
	// Theme.OnSurface ink. The zero value draws no caption (the original look).
	Label string
	// Thresholds recolour the filled cells by value band. Empty (the default)
	// keeps the Accent fill, so an unset LevelBar is byte-identical to before.
	Thresholds []LevelThreshold

	// value is the reactive number of lit cells; see [LevelBar.Value].
	value *mvvm.Observable[int]
}

// Value is the number of lit cells as a shared [mvvm.Observable]; edits Set it
// and Draw reads it live. Lazily created, defaulting to 0 (empty).
func (l *LevelBar) Value() *mvvm.Observable[int] {
	if l.value == nil {
		l.value = mvvm.NewObservable(0)
	}
	return l.value
}

// LevelThreshold recolours a LevelBar's fill once Value reaches Min. Several
// thresholds partition the range into coloured bands (e.g. {0,red}, {4,amber},
// {8,green}); the band with the greatest Min not exceeding Value is applied.
type LevelThreshold struct {
	Min   int
	Color RGBA
}

// NewLevelBar builds a LevelBar with the given Max (Value defaults
// to 0).
func NewLevelBar(max int) *LevelBar {
	if max < 1 {
		max = 1
	}
	return &LevelBar{Max: max}
}

// fillColor resolves the colour of the filled cells: the Color of the highest
// Threshold whose Min does not exceed Value, or Theme.Accent when no threshold
// matches (or none are configured) — preserving the original default fill.
func (l *LevelBar) fillColor(theme *Theme) RGBA {
	fill := theme.Accent
	chosen := false
	bestMin := 0
	for _, th := range l.Thresholds {
		if l.Value().Get() >= th.Min && (!chosen || th.Min >= bestMin) {
			fill, bestMin, chosen = th.Color, th.Min, true
		}
	}
	return fill
}

// Draw paints Max cells with a 1-px gap; the first Value cells use the
// threshold fill (Theme.Accent by default), the rest Theme.SurfaceAlt. For the
// horizontal orientation an optional Label is centred over the bar.
func (l *LevelBar) Draw(p painter.Painter, theme *Theme) {
	r := l.Bounds()
	if l.Max < 1 {
		return
	}
	lit := l.fillColor(theme)
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
			if i < l.Value().Get() {
				fill = lit
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
		if i < l.Value().Get() {
			fill = lit
		}
		fillRect(p, x, r.Y, cellW, r.H, fill)
	}
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	if l.Label != "" {
		tx := r.X + (r.W-l.textWidth(l.Label))/2
		ty := r.Y + (r.H-l.glyphHeight())/2
		l.drawText(p, tx, ty, l.Label, theme.OnSurface)
	}
}
