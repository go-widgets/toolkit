// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

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
	position          *mvvm.Observable[int] // reactive via Position()
	OnPositionChanged func(pos int)

	// resizing is true between grabbing the handle (EventClick on it) and
	// releasing it, so EventMouseDrag moves the split.
	resizing bool
}

// Position is reactive state as a shared [mvvm.Observable]; edits Set it. Lazily created.
func (p *Paned) Position() *mvvm.Observable[int] {
	if p.position == nil {
		p.position = mvvm.NewObservable[int](0)
	}
	return p.position
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
	if p.Position().Get() == 0 { // first sizing -- centre.
		if p.Orientation == PanedHorizontal {
			p.Position().Set(r.W / 2)
		} else {
			p.Position().Set(r.H / 2)
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
	p.Position().Set(pos)
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
		p.First.SetBounds(Rect{X: r.X, Y: r.Y, W: p.Position().Get(), H: r.H})
		p.Second.SetBounds(Rect{
			X: r.X + p.Position().Get() + scaled(PanedHandleW),
			Y: r.Y,
			W: r.W - p.Position().Get() - scaled(PanedHandleW),
			H: r.H,
		})
	} else {
		p.First.SetBounds(Rect{X: r.X, Y: r.Y, W: r.W, H: p.Position().Get()})
		p.Second.SetBounds(Rect{
			X: r.X,
			Y: r.Y + p.Position().Get() + scaled(PanedHandleW),
			W: r.W,
			H: r.H - p.Position().Get() - scaled(PanedHandleW),
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
	// The edge hairlines and the grip dots are metrics like the bar itself: left
	// at one device pixel they vanish on a HiDPI screen where the bar around
	// them doubled.
	edge := scaled(1)
	if vertical {
		fillRect(p, r.X, r.Y, edge, r.H, theme.Border)
		fillRect(p, r.X+r.W-edge, r.Y, edge, r.H, theme.Border)
	} else {
		fillRect(p, r.X, r.Y, r.W, edge, theme.Border)
		fillRect(p, r.X, r.Y+r.H-edge, r.W, edge, theme.Border)
	}
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	dot, gap := scaled(2), scaled(4)
	for k := -1; k <= 1; k++ {
		if vertical {
			fillRect(p, cx-dot/2, cy+k*gap-dot/2, dot, dot, theme.OnSurface)
		} else {
			fillRect(p, cx+k*gap-dot/2, cy-dot/2, dot, dot, theme.OnSurface)
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
		drawSplitterBar(p, Rect{X: r.X + pd.Position().Get(), Y: r.Y, W: scaled(PanedHandleW), H: r.H}, true, theme)
	} else {
		drawSplitterBar(p, Rect{X: r.X, Y: r.Y + pd.Position().Get(), W: r.W, H: scaled(PanedHandleW)}, false, theme)
	}
}

// OnEvent forwards to the appropriate child based on click position.
func (p *Paned) OnEvent(ev Event) {
	// ev is Paned-local; p.Position().Get() is a local offset, so the split test is
	// local-vs-local. The child, however, sits at a non-zero offset within the
	// Paned (First at the origin, Second past Position+handle), so the forwarded
	// event MUST be translated to child-local coordinates — otherwise a click in
	// the Second pane arrives with the Paned's coordinates and misroutes inside
	// the child (the bug this fixes; invisible only when Bounds is at 0,0).
	pos := ev.X
	if p.Orientation == PanedVertical {
		pos = ev.Y
	}
	switch ev.Kind {
	case EventClick:
		// A press on the handle grabs it for a resize; otherwise route to the
		// pane under the pointer.
		if pos >= p.Position().Get() && pos < p.Position().Get()+scaled(PanedHandleW) {
			p.resizing = true
			return
		}
		pr := p.Bounds()
		if pos < p.Position().Get() && p.First != nil {
			p.First.OnEvent(translateEvent(ev, pr, p.First.Bounds()))
		} else if pos >= p.Position().Get()+scaled(PanedHandleW) && p.Second != nil {
			p.Second.OnEvent(translateEvent(ev, pr, p.Second.Bounds()))
		}
	case EventMouseDrag:
		if p.resizing {
			p.MoveHandle(pos) // drag the grip → move the split
		}
	case EventMouseUp:
		p.resizing = false
	}
}
