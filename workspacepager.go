// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// WorkspacePager is a compact one-of-N cell strip for a desktop shell's
// workspace switcher: Count same-size cells laid end to end, each carrying its
// 1-based number (or a custom Labels caption), with exactly one — the Current
// workspace — highlighted in Theme.Accent. Clicking a cell switches to it,
// notifying the Current Observable's subscribers; the arrow / Home / End keys
// move the selection.
//
// This is the "N cells, the current one highlighted, click to switch" indicator
// a dock or panel would otherwise hand-draw. It differs from ViewSwitcher (a
// stretch-to-fill segmented tab strip of text views) and Pagination (a page
// navigator with prev/next chrome and an ellipsis window) in three ways a
// pager needs: fixed compact numbered cells rather than a widened text strip,
// no navigation chrome, and an optional per-cell Occupied dot marking which
// workspaces currently hold windows.
//
// Orientation lays the cells left-to-right (Horizontal, the zero value — a
// panel indicator) or top-to-bottom (Vertical — a side dock). A WorkspacePager
// with Count <= 0 paints nothing and ignores events, so a caller can assemble
// it before it knows the workspace count.
//
// The reactive selection is MVVM-only: the current index lives in an unexported
// Observable exposed via [WorkspacePager.Current]. Count, Labels, Occupied and
// Orientation are set-once layout/content config and stay plain fields.
type WorkspacePager struct {
	Base
	focusState
	// Count is the number of workspace cells (config).
	Count int
	// Labels optionally overrides a cell's caption. When Labels is nil, shorter
	// than Count, or an entry is "", that cell shows its 1-based number instead
	// (config).
	Labels []string
	// Occupied optionally marks which workspaces hold windows: a true entry
	// draws a small occupancy dot in that cell. A nil or shorter slice leaves
	// the trailing cells dot-less (config).
	Occupied []bool
	// Orientation lays the cells left-to-right (Horizontal, the zero value) or
	// top-to-bottom (Vertical) (config).
	Orientation Orientation

	current *mvvm.Observable[int]
}

// Current is the selected workspace index (0-based) as a shared
// [mvvm.Observable]: a host binds it (Set / Subscribe / two-way) — there is no
// settable Current field. A click on a cell, or an arrow / Home / End key, Sets
// it (clamped to [0, Count-1]); subscribers are notified on change. A bare
// &WorkspacePager{} lazily initialises the Observable to 0 on first access.
func (wp *WorkspacePager) Current() *mvvm.Observable[int] {
	if wp.current == nil {
		wp.current = mvvm.NewObservable(0)
	}
	return wp.current
}

// WorkspacePager sizing constants (device pixels at MetricScale 1, compact
// density). Chosen so a numbered cell fits inside a slim panel/dock strip.
const (
	// WorkspacePagerCellW is the pixel width of each workspace cell.
	WorkspacePagerCellW = 22
	// WorkspacePagerCellH is the pixel height of each workspace cell.
	WorkspacePagerCellH = 18
	// WorkspacePagerGap is the pixel spacer between successive cells.
	WorkspacePagerGap = 2
	// workspacePagerDotD is the diameter of the per-cell occupancy dot.
	workspacePagerDotD = 4
)

// NewWorkspacePager constructs a WorkspacePager over count workspaces with the
// initial selection at current. current is clamped into [0, count-1], or forced
// to 0 when count <= 0, so the widget is never in an out-of-range state.
func NewWorkspacePager(count, current int) *WorkspacePager {
	switch {
	case count <= 0:
		current = 0
	case current < 0:
		current = 0
	case current >= count:
		current = count - 1
	}
	wp := &WorkspacePager{Count: count}
	wp.current = mvvm.NewObservable(current)
	return wp
}

// cellLabel returns cell i's caption: its Labels entry when that is present and
// non-empty, else its 1-based number.
func (wp *WorkspacePager) cellLabel(i int) string {
	if i < len(wp.Labels) && wp.Labels[i] != "" {
		return wp.Labels[i]
	}
	return strconv.Itoa(i + 1)
}

// occupied reports whether cell i carries an occupancy dot.
func (wp *WorkspacePager) occupied(i int) bool {
	return i < len(wp.Occupied) && wp.Occupied[i]
}

// Draw paints each cell — the Current one filled in Theme.Accent, the rest in
// Theme.SurfaceAlt — with its centred number/label and, where Occupied, a small
// dot in the top-right corner. Count <= 0 paints nothing.
func (wp *WorkspacePager) Draw(p painter.Painter, theme *Theme) {
	if wp.Count <= 0 {
		return
	}
	r := wp.Bounds()
	vertical := wp.Orientation == Vertical
	cellW, cellH := scaled(WorkspacePagerCellW), scaled(WorkspacePagerCellH)
	gap := scaled(WorkspacePagerGap)
	dotD := scaled(workspacePagerDotD)
	pad := max(1, scaled(1))
	cur := wp.Current().Get()

	// The cell strip is pinned to one edge; the layout axis advances the other
	// coordinate. Horizontal centres the row vertically in a tall strip; vertical
	// centres the column horizontally in a wide strip.
	x, y := r.X, r.Y
	if !vertical && r.H > cellH {
		y = r.Y + (r.H-cellH)/2
	}
	if vertical && r.W > cellW {
		x = r.X + (r.W-cellW)/2
	}

	for i := 0; i < wp.Count; i++ {
		cx, cy := x, y
		if vertical {
			cy = y + i*(cellH+gap)
		} else {
			cx = x + i*(cellW+gap)
		}
		fill := theme.SurfaceAlt
		ink := theme.OnSurface
		dot := theme.Accent
		if i == cur {
			fill = theme.Accent
			ink = accentInk(theme)
			dot = accentInk(theme)
		}
		fillRect(p, cx, cy, cellW, cellH, fill)
		strokeRect(p, cx, cy, cellW, cellH, theme.Border)
		lab := wp.cellLabel(i)
		tx := cx + (cellW-wp.textWidth(lab))/2
		ty := cy + (cellH-wp.glyphHeight())/2
		wp.drawText(p, tx, ty, lab, ink)
		if wp.occupied(i) {
			fillRoundRect(p, cx+cellW-dotD-pad, cy+pad, dotD, dotD, dotD/2, dot)
		}
	}
	wp.drawFocusRing(p, theme, r)
}

// OnEvent switches workspace on a click or a key. A click Sets Current to the
// cell it lands on; ArrowLeft/Up step back, ArrowRight/Down step forward, Home
// selects the first workspace and End the last (all clamped). Count <= 0, a
// key while Disabled, and a click that misses every cell are no-ops.
func (wp *WorkspacePager) OnEvent(ev Event) {
	if wp.Count <= 0 {
		return
	}
	if ev.Kind == EventKeyDown {
		if wp.Disabled().Get() {
			return
		}
		cur := wp.Current().Get()
		switch ev.Code {
		case "ArrowLeft", "ArrowUp":
			wp.goTo(cur - 1)
		case "ArrowRight", "ArrowDown":
			wp.goTo(cur + 1)
		case "Home":
			wp.goTo(0)
		case "End":
			wp.goTo(wp.Count - 1)
		}
		return
	}
	if ev.Kind != EventClick {
		return
	}
	vertical := wp.Orientation == Vertical
	r := wp.Bounds()
	cellW, cellH := scaled(WorkspacePagerCellW), scaled(WorkspacePagerCellH)
	gap := scaled(WorkspacePagerGap)
	// Each cell is a tap target: its hit box clamps UP to the density minimum on
	// each axis and centres over the drawn cell (the Steps/Switch idiom), without
	// changing what is painted. At compact density the clamp is a pass-through.
	hitW, hitH := TouchTarget(cellW), TouchTarget(cellH)
	xOff, yOff := 0, 0
	if !vertical && r.H > cellH {
		yOff = (r.H - cellH) / 2
	}
	if vertical && r.W > cellW {
		xOff = (r.W - cellW) / 2
	}
	for i := 0; i < wp.Count; i++ {
		cx, cy := xOff, yOff
		if vertical {
			cy = yOff + i*(cellH+gap)
		} else {
			cx = xOff + i*(cellW+gap)
		}
		hx := cx - (hitW-cellW)/2
		hy := cy - (hitH-cellH)/2
		if ev.X >= hx && ev.X < hx+hitW && ev.Y >= hy && ev.Y < hy+hitH {
			wp.Current().Set(i)
			return
		}
	}
}

// goTo clamps i to [0, Count-1] and Sets the Current Observable — the shared
// mutate path a click and every key reuse. Subscribers are notified on change;
// an unchanged index is a no-op (per mvvm.Observable).
func (wp *WorkspacePager) goTo(i int) {
	if i < 0 {
		i = 0
	}
	if i >= wp.Count {
		i = wp.Count - 1
	}
	wp.Current().Set(i)
}

// A11y reports the WorkspacePager as a tablist named by its current
// workspace's caption — the same role ViewSwitcher and Notebook use for a
// one-of-N selector.
func (wp *WorkspacePager) A11y() A11yInfo {
	name := ""
	if cur := wp.Current().Get(); cur >= 0 && cur < wp.Count {
		name = wp.cellLabel(cur)
	}
	return A11yInfo{Role: RoleTablist, Name: name}
}
