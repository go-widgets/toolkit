// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

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
func (p *Paned) Draw(surface []byte, surfaceW int, theme *Theme) {
	if p.First != nil {
		p.First.Draw(surface, surfaceW, theme)
	}
	if p.Second != nil {
		p.Second.Draw(surface, surfaceW, theme)
	}
	r := p.Bounds()
	if p.Orientation == PanedHorizontal {
		fillRect(surface, surfaceW, r.X+p.Position, r.Y, PanedHandleW, r.H, theme.SurfaceAlt)
	} else {
		fillRect(surface, surfaceW, r.X, r.Y+p.Position, r.W, PanedHandleW, theme.SurfaceAlt)
	}
}

// OnEvent forwards to the appropriate child based on click position.
func (p *Paned) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	if p.Orientation == PanedHorizontal {
		if ev.X < p.Position && p.First != nil {
			p.First.OnEvent(ev)
		} else if ev.X >= p.Position+PanedHandleW && p.Second != nil {
			p.Second.OnEvent(ev)
		}
	} else {
		if ev.Y < p.Position && p.First != nil {
			p.First.OnEvent(ev)
		} else if ev.Y >= p.Position+PanedHandleW && p.Second != nil {
			p.Second.OnEvent(ev)
		}
	}
}
