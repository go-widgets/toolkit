// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Paned orientations.
const (
	PanedHorizontal = 0 // First left, Second right
	PanedVertical   = 1 // First top, Second bottom
)

// Paned splits its bounds into two child regions separated by a
// PanedHandleW-px draggable handle. Position is the handle's offset
// (in pixels) from the leading edge of First; orientation chooses
// whether that's measured along X (PanedHorizontal) or Y (PanedVertical).
//
// The toolkit's full event model is click-only in v0.2, so drag is
// exposed via direct OnDragHandle helpers callers wire to their own
// mouse-tracking state.
type Paned struct {
	Base
	First, Second     Widget
	Orientation       int
	Position          int
	OnPositionChanged func(pos int)
}

// PanedHandleW is the pixel thickness of the splitter handle.
const PanedHandleW = 6

// NewHPaned builds a horizontal Paned with a sensible default
// Position (mid-bounds, applied at first SetBounds).
func NewHPaned(first, second Widget) *Paned {
	return &Paned{First: first, Second: second, Orientation: PanedHorizontal}
}

// NewVPaned builds a vertical Paned with the same defaults as
// NewHPaned.
func NewVPaned(first, second Widget) *Paned {
	return &Paned{First: first, Second: second, Orientation: PanedVertical}
}

// SetBounds lays out First/Second around the handle.
func (p *Paned) SetBounds(r Rect) {
	p.Base.SetBounds(r)
	if p.Position == 0 { // first sizing -- centre.
		if p.Orientation == PanedHorizontal {
			p.Position = r.W / 2
		} else {
			p.Position = r.H / 2
		}
	}
	p.layout()
}

// MoveHandle slides the splitter to pos (clamped) and re-lays out
// children. Fires OnPositionChanged with the new value.
func (p *Paned) MoveHandle(pos int) {
	r := p.Bounds()
	total := r.W
	if p.Orientation == PanedVertical {
		total = r.H
	}
	if pos < 10 {
		pos = 10
	}
	if pos > total-10 {
		pos = total - 10
	}
	p.Position = pos
	p.layout()
	if p.OnPositionChanged != nil {
		p.OnPositionChanged(pos)
	}
}

// layout assigns Bounds to First / Second based on Orientation +
// Position.
func (p *Paned) layout() {
	r := p.Bounds()
	if p.First == nil || p.Second == nil {
		return
	}
	if p.Orientation == PanedHorizontal {
		p.First.SetBounds(Rect{X: r.X, Y: r.Y, W: p.Position, H: r.H})
		p.Second.SetBounds(Rect{
			X: r.X + p.Position + PanedHandleW,
			Y: r.Y,
			W: r.W - p.Position - PanedHandleW,
			H: r.H,
		})
	} else {
		p.First.SetBounds(Rect{X: r.X, Y: r.Y, W: r.W, H: p.Position})
		p.Second.SetBounds(Rect{
			X: r.X,
			Y: r.Y + p.Position + PanedHandleW,
			W: r.W,
			H: r.H - p.Position - PanedHandleW,
		})
	}
}

// Draw paints both children + the handle.
// drawSplitterBar paints a splitter / separator bar (a Paned handle or a Border
// split handle) with a DISTINCT background — a tone blended from SurfaceAlt
// toward Border so the bar stands out against the Surface/SurfaceAlt panels it
// sits between (the Sencha splitter look) — plus a 1px Border delineator on the
// bar's two long edges and a centred three-dot grip so it reads as draggable.
// vertical=true for a thin vertical bar (horizontal split), false for a thin
// horizontal bar (vertical split).
func drawSplitterBar(p painter.Painter, r Rect, vertical bool, theme *Theme) {
	fillRect(p, r.X, r.Y, r.W, r.H, blendRGBA(theme.SurfaceAlt, theme.Border, 0.45))
	if vertical {
		fillRect(p, r.X, r.Y, 1, r.H, theme.Border)
		fillRect(p, r.X+r.W-1, r.Y, 1, r.H, theme.Border)
	} else {
		fillRect(p, r.X, r.Y, r.W, 1, theme.Border)
		fillRect(p, r.X, r.Y+r.H-1, r.W, 1, theme.Border)
	}
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	for k := -1; k <= 1; k++ {
		if vertical {
			fillRect(p, cx-1, cy+k*4-1, 2, 2, theme.OnSurface)
		} else {
			fillRect(p, cx+k*4-1, cy-1, 2, 2, theme.OnSurface)
		}
	}
}

func (pd *Paned) Draw(p painter.Painter, theme *Theme) {
	if pd.First != nil {
		pd.First.Draw(p, theme)
	}
	if pd.Second != nil {
		pd.Second.Draw(p, theme)
	}
	r := pd.Bounds()
	if pd.Orientation == PanedHorizontal {
		drawSplitterBar(p, Rect{X: r.X + pd.Position, Y: r.Y, W: PanedHandleW, H: r.H}, true, theme)
	} else {
		drawSplitterBar(p, Rect{X: r.X, Y: r.Y + pd.Position, W: r.W, H: PanedHandleW}, false, theme)
	}
}

// OnEvent forwards to the appropriate child based on click position.
func (p *Paned) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	// ev is Paned-local; p.Position is a local offset, so the split test is
	// local-vs-local. The child, however, sits at a non-zero offset within the
	// Paned (First at the origin, Second past Position+handle), so the forwarded
	// event MUST be translated to child-local coordinates — otherwise a click in
	// the Second pane arrives with the Paned's coordinates and misroutes inside
	// the child (the bug this fixes; invisible only when Bounds is at 0,0).
	pos := ev.X
	if p.Orientation == PanedVertical {
		pos = ev.Y
	}
	pr := p.Bounds()
	if pos < p.Position && p.First != nil {
		p.First.OnEvent(translateEvent(ev, pr, p.First.Bounds()))
	} else if pos >= p.Position+PanedHandleW && p.Second != nil {
		p.Second.OnEvent(translateEvent(ev, pr, p.Second.Bounds()))
	}
}
