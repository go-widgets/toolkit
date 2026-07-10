// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// DropZone is an inline "drag files here" target rendered as a
// bordered rectangle with a centred prompt string. It is the passive
// counterpart of FileChooser: FileChooser opens a modal directory
// browser, DropZone waits in place for the host to hand it dropped
// file paths (typically via a native HTML5 drag+drop listener the
// wasmbox compositor wires to the widget).
//
// The Hover flag toggles the dashed-border colour + surface fill so
// the user sees drag-over feedback before releasing the drop. DropZone
// is a DropTarget: it drives Hover from the formal drag lifecycle —
// EventDragStart / EventDragMove raise it, EventDragLeave clears it,
// and EventDrop delivers the payload (multiple paths newline-separated,
// recovered with SplitDropPayload) to OnDrop and clears Hover. As a
// convenience for demos and tests, EventClick also flips Hover in place.
type DropZone struct {
	Base
	Prompt string
	Hover  bool
	OnDrop func(paths []string)
}

// DropZone is a DropTarget.
var _ DropTarget = (*DropZone)(nil)

// AcceptsDrop reports whether a payload is droppable here. A DropZone is a
// generic file target, so it accepts any non-empty payload.
func (d *DropZone) AcceptsDrop(payload string) bool { return payload != "" }

// DropZone sizing constants. PadX / PadY are the outer insets from
// the bounds to where inner content (the prompt text) sits; DashLen
// + DashGap describe the stripe pattern of the dashed border, and
// BorderW is its per-edge pixel thickness. Kept generous so the
// dashed rectangle reads as a container, not a thin outline.
const (
	DropZonePadX    = 12
	DropZonePadY    = 12
	DropZoneDashLen = 4
	DropZoneDashGap = 4
	DropZoneBorderW = 2
)

// NewDropZone constructs a DropZone with the given prompt text. An
// empty prompt is replaced with the default "Drop files here" so a
// zero-argument caller still renders a legible target. Bounds default
// to zero; the caller positions the DropZone via SetBounds.
func NewDropZone(prompt string) *DropZone {
	if prompt == "" {
		prompt = "Drop files here"
	}
	return &DropZone{Prompt: prompt}
}

// Draw paints the surface fill, the four dashed edges + the centred
// prompt text. Fill + border colour swap on Hover so the drag-over
// state is visible without the caller having to swap in a different
// widget on drag-enter. Dashes are emitted as short filled rects so
// the toolkit stays on its two existing raster primitives (fillRect
// / strokeRect) rather than growing a Painter.DashedLine primitive.
func (d *DropZone) Draw(p painter.Painter, theme *Theme) {
	r := d.Bounds()
	face := theme.Surface
	border := theme.Border
	if d.Hover {
		face = theme.SurfaceAlt
		border = theme.Accent
	}
	fillRect(p, r.X, r.Y, r.W, r.H, face)
	step := DropZoneDashLen + DropZoneDashGap
	// Top + bottom edges: horizontal dashes.
	for x := r.X; x < r.X+r.W; x += step {
		w := DropZoneDashLen
		if x+w > r.X+r.W {
			w = r.X + r.W - x
		}
		fillRect(p, x, r.Y, w, DropZoneBorderW, border)
		fillRect(p, x, r.Y+r.H-DropZoneBorderW, w, DropZoneBorderW, border)
	}
	// Left + right edges: vertical dashes.
	for y := r.Y; y < r.Y+r.H; y += step {
		h := DropZoneDashLen
		if y+h > r.Y+r.H {
			h = r.Y + r.H - y
		}
		fillRect(p, r.X, y, DropZoneBorderW, h, border)
		fillRect(p, r.X+r.W-DropZoneBorderW, y, DropZoneBorderW, h, border)
	}
	tw := TextWidth(d.Prompt)
	tx := r.X + (r.W-tw)/2
	ty := r.Y + (r.H-GlyphHeight())/2
	DrawText(p, tx, ty, d.Prompt, theme.OnSurface)
}

// OnEvent implements the drag lifecycle: EventDragStart / EventDragMove raise
// Hover, EventDragLeave clears it, and EventDrop fires OnDrop with the payload's
// items (split from ev.Code) then clears Hover. EventClick flips Hover in place
// as a demo/test hook. All other event kinds are ignored so a keyboard event
// bound for a sibling widget does not accidentally trigger a drop.
func (d *DropZone) OnEvent(ev Event) {
	switch ev.Kind {
	case EventDragStart, EventDragMove:
		d.Hover = true
	case EventDragLeave:
		d.Hover = false
	case EventDrop:
		d.Hover = false
		if d.OnDrop != nil {
			d.OnDrop(SplitDropPayload(ev.Code))
		}
	case EventClick:
		d.Hover = !d.Hover
	}
}
