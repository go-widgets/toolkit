// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"sort"
	"strings"

	"github.com/go-widgets/painter"
)

// This file gives the toolkit a generic, cross-widget TEXT SELECTION: the
// ability to drag-select arbitrary on-screen text — text that may span many
// widgets (several Labels, a heading and its paragraphs, …) — and read back the
// selected string for a copy. It is deliberately source-agnostic: it operates on
// a flat list of [TextRun]s (a string + its absolute pixel rect + the font it
// was drawn with), so a host can feed it runs gathered from toolkit widgets that
// implement [SelectableText] AND from any text the host draws by hand. The
// widget-buffer-oriented [Selection] (selection.go) stays the TextView's own
// line/col model; this is the screen-space, multi-widget complement.

// TextRun is one contiguous piece of drawn text at an absolute pixel position.
// Bounds is the run's rectangle in the same coordinate space the pointer events
// use (screen / surface pixels). Font measures per-character widths so a click
// resolves to a character boundary within the run.
type TextRun struct {
	Text   string
	Bounds Rect
	Font   Font
}

// SelectableText is implemented by widgets that expose their drawn text to the
// selection subsystem. Runs are returned in absolute coordinates (the widget
// offsets by its own Bounds), so a container can concatenate the runs of all its
// children into one [TextSelection].
type SelectableText interface {
	TextRuns() []TextRun
}

// textPos is a caret position: a run index plus a rune offset within that run's
// Text (0..len(runes)). The zero value is the start of the first run.
type textPos struct {
	run  int
	char int
}

// before reports whether p is earlier than q in document order (run first, then
// character).
func (p textPos) before(q textPos) bool {
	if p.run != q.run {
		return p.run < q.run
	}
	return p.char < q.char
}

// TextSelection is a screen-space, multi-run text selection driven by pointer
// drag. A host sets the current runs each frame (SetRuns), then routes
// press/drag/release (Begin/Drag/End) in the same coordinate space as the runs'
// Bounds. SelectedText returns the covered text; Draw paints the highlight.
//
// The zero value is an empty, inactive selection.
type TextSelection struct {
	runs           []TextRun // sorted into document order by SetRuns
	anchor, cursor textPos
	active         bool // a selection has been started (Begin) …
	dragging       bool // … and the pointer is still down (Begin→End)
}

// SetRuns replaces the selectable runs, sorted into document order (top-to-
// bottom, then left-to-right). Any run with empty Text or a zero-area rect is
// dropped. An active selection's endpoints are clamped to the new run set so a
// relayout (e.g. a resize that re-wraps text) can't leave them dangling.
func (s *TextSelection) SetRuns(runs []TextRun) {
	filtered := runs[:0:0]
	for _, r := range runs {
		if r.Text == "" || r.Bounds.W <= 0 || r.Bounds.H <= 0 {
			continue
		}
		filtered = append(filtered, r)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := filtered[i].Bounds, filtered[j].Bounds
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		return a.X < b.X
	})
	s.runs = filtered
	if s.active {
		s.anchor = s.clampPos(s.anchor)
		s.cursor = s.clampPos(s.cursor)
	}
}

// clampPos keeps a caret position valid against the current run set (positions
// only ever come from caretAt, so they are already non-negative; a shrink of the
// run set is the one thing that can push a run/char past the end).
func (s *TextSelection) clampPos(p textPos) textPos {
	if len(s.runs) == 0 {
		return textPos{}
	}
	if p.run >= len(s.runs) {
		p.run = len(s.runs) - 1
		p.char = len([]rune(s.runs[p.run].Text))
	}
	if n := len([]rune(s.runs[p.run].Text)); p.char > n {
		p.char = n
	}
	return p
}

// Begin starts a selection at the caret nearest (x, y): it sets the anchor and
// collapses the cursor onto it (an empty selection until the pointer drags).
func (s *TextSelection) Begin(x, y int) {
	if len(s.runs) == 0 {
		return
	}
	s.anchor = s.caretAt(x, y)
	s.cursor = s.anchor
	s.active = true
	s.dragging = true
}

// Drag extends the selection to the caret nearest (x, y). Ignored unless a drag
// is in progress (between Begin and End).
func (s *TextSelection) Drag(x, y int) {
	if !s.dragging {
		return
	}
	s.cursor = s.caretAt(x, y)
}

// End finishes the drag, leaving the selection in place (so SelectedText / Draw
// keep working until the next Begin or Clear).
func (s *TextSelection) End() { s.dragging = false }

// Clear discards the selection entirely.
func (s *TextSelection) Clear() {
	s.active = false
	s.dragging = false
	s.anchor = textPos{}
	s.cursor = textPos{}
}

// IsEmpty reports whether the selection covers zero characters (no drag, or the
// cursor still on the anchor).
func (s *TextSelection) IsEmpty() bool { return !s.active || s.anchor == s.cursor }

// caretAt resolves a pixel point to the nearest caret position: it picks the run
// closest to y (preferring one whose vertical band contains y), then the
// character boundary within that run nearest x.
func (s *TextSelection) caretAt(x, y int) textPos {
	best, bestDist := 0, 0
	for i, r := range s.runs {
		d := verticalDistance(r.Bounds, y)
		if i == 0 || d < bestDist {
			best, bestDist = i, d
			if d == 0 {
				// Among runs on this line, prefer the one whose X band is
				// closest to x, so a click between two runs lands on the right side.
				if hd := horizontalDistance(r.Bounds, x); hd == 0 {
					return textPos{i, charAt(r, x)}
				}
			}
		} else if d == bestDist && d == 0 {
			// Same line as the current best: switch if this run is horizontally
			// closer to x.
			if horizontalDistance(r.Bounds, x) < horizontalDistance(s.runs[best].Bounds, x) {
				best = i
			}
		}
	}
	return textPos{best, charAt(s.runs[best], x)}
}

// verticalDistance is 0 when y is inside r's vertical band, else the gap to the
// nearer edge.
func verticalDistance(r Rect, y int) int {
	if y < r.Y {
		return r.Y - y
	}
	if y >= r.Y+r.H {
		return y - (r.Y + r.H) + 1
	}
	return 0
}

// horizontalDistance is 0 when x is inside r's horizontal band, else the gap to
// the nearer edge.
func horizontalDistance(r Rect, x int) int {
	if x < r.X {
		return r.X - x
	}
	if x >= r.X+r.W {
		return x - (r.X + r.W) + 1
	}
	return 0
}

// charAt returns the rune offset in run whose caret x is nearest the pixel x.
// It measures growing prefixes with the run's font, so proportional fonts land
// on the visually closest gap. A click left of the run is offset 0; right of it
// is the full length.
func charAt(run TextRun, x int) int {
	runes := []rune(run.Text)
	if x <= run.Bounds.X || run.Font == nil {
		if x <= run.Bounds.X {
			return 0
		}
		return len(runes)
	}
	rel := x - run.Bounds.X
	prev := 0
	for i := 1; i <= len(runes); i++ {
		w := run.Font.Measure(string(runes[:i]))
		if rel <= (prev+w)/2 {
			return i - 1
		}
		prev = w
	}
	return len(runes)
}

// order returns the selection endpoints in document order (start ≤ end).
func (s *TextSelection) order() (start, end textPos) {
	if s.cursor.before(s.anchor) {
		return s.cursor, s.anchor
	}
	return s.anchor, s.cursor
}

// SelectedText returns the text the selection covers. Runs on the same visual
// line (equal Bounds.Y) are joined with a single space when the boundary is not
// already whitespace; a change of line inserts a newline. An empty selection
// returns "".
func (s *TextSelection) SelectedText() string {
	if s.IsEmpty() {
		return ""
	}
	start, end := s.order()
	var b strings.Builder
	for ri := start.run; ri <= end.run; ri++ {
		run := s.runs[ri]
		runes := []rune(run.Text)
		lo, hi := 0, len(runes)
		if ri == start.run {
			lo = start.char
		}
		if ri == end.run {
			hi = end.char
		}
		if ri > start.run {
			// Separator from the previous run: newline across lines, else a space
			// unless the seam already has whitespace.
			prev := s.runs[ri-1]
			switch {
			case prev.Bounds.Y != run.Bounds.Y:
				b.WriteByte('\n')
			case !endsOrStartsSpace(prev.Text, run.Text):
				b.WriteByte(' ')
			}
		}
		b.WriteString(string(runes[lo:hi]))
	}
	return b.String()
}

// endsOrStartsSpace reports whether joining a and b would already have
// whitespace at the seam (so no extra space is inserted).
func endsOrStartsSpace(a, b string) bool {
	if a != "" && a[len(a)-1] == ' ' {
		return true
	}
	return b != "" && b[0] == ' '
}

// Draw paints the selection highlight (a filled rect per covered run span) in
// col. Nothing is drawn for an empty selection. Runs are highlighted from their
// start-char x to their end-char x, full run height.
func (s *TextSelection) Draw(p painter.Painter, col RGBA) {
	if s.IsEmpty() {
		return
	}
	start, end := s.order()
	for ri := start.run; ri <= end.run; ri++ {
		run := s.runs[ri]
		runes := []rune(run.Text)
		lo, hi := 0, len(runes)
		if ri == start.run {
			lo = start.char
		}
		if ri == end.run {
			hi = end.char
		}
		x0 := run.Bounds.X + prefixWidth(run, runes, lo)
		x1 := run.Bounds.X + prefixWidth(run, runes, hi)
		if x1 <= x0 {
			// A zero-width span on an intermediate line still reads as "the whole
			// line is selected"; give it the run's full width so the highlight is
			// continuous across wrapped lines.
			if ri != start.run && ri != end.run {
				x0, x1 = run.Bounds.X, run.Bounds.X+run.Bounds.W
			} else {
				continue
			}
		}
		p.FillRect(painter.Rect{X: x0, Y: run.Bounds.Y, W: x1 - x0, H: run.Bounds.H}, col)
	}
}

// prefixWidth is the pixel width of the first n runes of run (0 when n<=0 or the
// font is nil). n is always within [0, len(runes)] at every call site.
func prefixWidth(run TextRun, runes []rune, n int) int {
	if n <= 0 || run.Font == nil {
		return 0
	}
	return run.Font.Measure(string(runes[:n]))
}

// CopySelection writes the selected text to the toolkit-wide clipboard (when
// non-empty) and returns it, so a host's copy chord is a one-liner. An empty
// selection leaves the clipboard untouched.
func (s *TextSelection) CopySelection() string {
	txt := s.SelectedText()
	if txt != "" {
		SetClipboardText(txt)
	}
	return txt
}

// childContainer is any container widget that exposes its children for generic
// traversal (Container, HBox, VBox, …), so CollectRuns can descend without
// knowing the concrete type.
type childContainer interface{ Children() []Widget }

// CollectRuns gathers the [TextRun]s of every widget in the tree rooted at w
// that implements [SelectableText], descending into any widget that exposes its
// children via [childContainer] (Container / HBox / VBox / …). It is the bridge
// a host uses to feed a widget tree into a [TextSelection].
func CollectRuns(w Widget) []TextRun {
	var out []TextRun
	var walk func(Widget)
	walk = func(x Widget) {
		if st, ok := x.(SelectableText); ok {
			out = append(out, st.TextRuns()...)
		}
		if c, ok := x.(childContainer); ok {
			for _, child := range c.Children() {
				if child != nil {
					walk(child)
				}
			}
		}
	}
	walk(w)
	return out
}
