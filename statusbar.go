// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Statusbar is a thin horizontal strip at the bottom of a window that
// shows N text segments (e.g. "Line 12, Col 4" + "UTF-8" + "Plain
// text" in an editor). Segments paint left-to-right with a 1-pixel
// divider between them; the LAST segment expands to fill any remaining
// width so an empty Statusbar still looks deliberate.
//
// Statusbar is the natural pairing for MenuBar + Toolbar above and a
// document area in the middle — together they assemble the "stock
// GTK" window frame.
type Statusbar struct {
	Base
	Segments []string

	// SegmentMinW is the minimum width any non-last segment takes. The
	// last segment ALWAYS fills the rest of the bar.
	SegmentMinW int // default StatusbarSegmentMinW
}

// Sizing constants.
const (
	StatusbarH           = 18
	StatusbarSegmentMinW = 80
	StatusbarPadX        = 6
)

// NewStatusbar builds a Statusbar with the given segments. SegmentMinW is left
// at zero — "use the default" — so the default minimum resolves through scaled at
// draw time and grows with HiDPI and touch density; a caller that wants a fixed
// minimum sets SegmentMinW explicitly (honoured verbatim). At compact/1x the
// resolved default is StatusbarSegmentMinW, byte-identical to before.
func NewStatusbar(segs []string) *Statusbar {
	return &Statusbar{Segments: segs}
}

// SetSegment replaces the i-th segment in place. Indexes out of range
// are appended (filling intermediate slots with "") so callers can
// grow the bar lazily.
func (s *Statusbar) SetSegment(i int, text string) {
	if i < 0 {
		return
	}
	for len(s.Segments) <= i {
		s.Segments = append(s.Segments, "")
	}
	s.Segments[i] = text
}

// Draw paints the strip + every segment.
func (s *Statusbar) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	// The default min-width resolves through scaled (an explicit SegmentMinW is
	// honoured verbatim); the horizontal pad routes through scaled too. Both equal
	// their logical constants at compact/1x, keeping the strip byte-identical.
	min := s.SegmentMinW
	if min <= 0 {
		min = scaled(StatusbarSegmentMinW)
	}
	padX := scaled(StatusbarPadX)
	x := r.X
	n := len(s.Segments)
	for i, seg := range s.Segments {
		var w int
		if i == n-1 {
			w = r.X + r.W - x
		} else {
			w = s.textWidth(seg) + 2*padX
			if w < min {
				w = min
			}
		}
		ty := r.Y + (r.H-s.glyphHeight())/2
		s.drawText(p, x+padX, ty, seg, theme.OnSurface)
		if i < n-1 {
			fillRect(p, x+w-1, r.Y+2, 1, r.H-4, theme.Border)
		}
		x += w
	}
}
