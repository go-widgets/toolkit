// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"
	"unicode"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// CompletionItemKind classifies a completion candidate so the popup can draw a
// small, colour-coded kind glyph beside it. The set mirrors the subset of the
// LSP CompletionItemKind enumeration that a code/markup editor actually needs,
// so a host adapting language-server results maps its lsp.CompletionItemKind
// onto these 1:1 (loom does exactly that); a static provider — the go-tex
// playground's LaTeX list — picks the closest kind for each entry.
type CompletionItemKind int

const (
	// CompletionText is a plain textual match with no richer classification —
	// the fallback kind, and the zero value.
	CompletionText CompletionItemKind = iota
	// CompletionMethod / CompletionFunction / CompletionConstructor are
	// callables.
	CompletionMethod
	CompletionFunction
	CompletionConstructor
	// CompletionField / CompletionVariable / CompletionProperty are values.
	CompletionField
	CompletionVariable
	CompletionProperty
	// CompletionClass / CompletionInterface / CompletionModule / CompletionEnum
	// are types and namespaces.
	CompletionClass
	CompletionInterface
	CompletionModule
	CompletionEnum
	// CompletionKeyword / CompletionSnippet / CompletionConstant round out the
	// language-level kinds (a LaTeX command is a Keyword, an environment
	// template a Snippet, a math symbol a Constant, say).
	CompletionKeyword
	CompletionSnippet
	CompletionConstant
	// completionKindEnd is the exclusive upper bound of valid kinds — not a
	// kind itself. It lets a test assert every kind has a glyph + colour, so a
	// newly appended kind fails loudly instead of rendering blank.
	completionKindEnd
)

// CompletionItem is one candidate the popup can offer, shaped after the LSP
// CompletionItem so a host adapts language-server results field-for-field. Only
// Label is required; every other field has a sensible empty-value fallback.
type CompletionItem struct {
	// Label is the text shown in the list and the fallback insert text (used
	// when InsertText is empty).
	Label string
	// Kind drives the row's kind glyph + colour.
	Kind CompletionItemKind
	// Detail is a short, dim, right-aligned hint (a signature or type).
	Detail string
	// Documentation is a longer description shown for the selected item in the
	// optional doc panel below the list. Empty hides the panel.
	Documentation string
	// InsertText is what accepting the item inserts, replacing the current
	// word. Empty falls back to Label. It may carry a single final-cursor
	// "$0" tab-stop marker (the caret lands there after insertion); "${0:x}"
	// is honoured too, and any other "$n"/"${n:default}" marker is stripped
	// gracefully, keeping only its default text.
	InsertText string
	// FilterText is what the prefix filter matches the current word against.
	// Empty falls back to Label — so a host that wants the user to type "\se"
	// to reach a "\section" whose Label reads differently sets FilterText.
	FilterText string
}

// Completion popup metrics, in LOGICAL pixels (routed through scaled at use).
const (
	// CompletionRowH is the pixel height of one candidate row — matched to the
	// DropDown/palette popover row so the completion list reads as the same
	// family of overlay.
	CompletionRowH = 18
	// CompletionMaxRows caps how many rows the popup shows at once; a longer
	// filtered list is reachable by scrolling (see compScroll).
	CompletionMaxRows = 10
	// CompletionMinW is the floor on the popup width so a list of short labels
	// still reads as a panel.
	CompletionMinW = 120
	// completionPadX is the horizontal breathing space inside the popup, and
	// completionGlyphCells the number of glyph advances reserved for the kind
	// glyph + its trailing gap at the left of every row.
	completionPadX        = 6
	completionGlyphCells  = 2
	completionDetailGapPx = 12
)

// CompletionOpen is the popup-shown flag as a shared [mvvm.Observable]: a host
// binds it (Get / Set / Subscribe) to learn when completion opens or closes —
// there is no settable Open field. Typing, an explicit trigger, accept, Escape
// and a caret move all Set it; subscribers are notified on change. A bare
// &CodeEditor{} lazy-inits it to false.
func (c *CodeEditor) CompletionOpen() *mvvm.Observable[bool] {
	if c.compOpen == nil {
		c.compOpen = mvvm.NewObservable(false)
	}
	return c.compOpen
}

// CompletionSelection is the highlighted row within the FILTERED list as a
// shared [mvvm.Observable]: a host binds it exactly like a ListBox's Selected.
// Key navigation, a row click and every refresh Set it (always re-clamped into
// the current list). A bare &CodeEditor{} lazy-inits it to 0.
func (c *CodeEditor) CompletionSelection() *mvvm.Observable[int] {
	if c.compSel == nil {
		c.compSel = mvvm.NewObservable(0)
	}
	return c.compSel
}

// CompletionActive reports whether the popup is currently shown — the plain
// convenience read over CompletionOpen for a headless host or a test.
func (c *CodeEditor) CompletionActive() bool { return c.CompletionOpen().Get() }

// CompletionSelected is the highlighted row index within the filtered list —
// the plain convenience read over CompletionSelection.
func (c *CodeEditor) CompletionSelected() int { return c.CompletionSelection().Get() }

// CompletionItems returns a copy of the current filtered candidate list — the
// exact rows the popup renders, in display order. A host reads it to mirror the
// editor's filtering (a live count, its own overlay) without duplicating the
// match logic. The copy is the caller's to mutate.
func (c *CodeEditor) CompletionItems() []CompletionItem {
	return append([]CompletionItem(nil), c.compItems...)
}

// wordCharFn resolves the effective word-character predicate: the host's
// override if set, else defaultWordChar.
func (c *CodeEditor) wordCharFn() func(rune) bool {
	if c.CompletionWordChar != nil {
		return c.CompletionWordChar
	}
	return defaultWordChar
}

// defaultWordChar is the identifier rule used when no override is set: a rune
// belongs to the current word if it is a letter, a digit, or an underscore.
func defaultWordChar(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// currentWord returns the word immediately before the caret — the maximal run
// of word-characters on the caret's line ending at CursorCol — and the column
// it starts at. An empty word (the caret sits after a non-word char) returns
// ("", CursorCol), which prefix-matches every candidate.
func (c *CodeEditor) currentWord() (word string, start int) {
	isWord := c.wordCharFn()
	line, col := c.CursorLine().Get(), c.CursorCol().Get()
	runes := []rune(c.lines[line])
	start = col
	for start > 0 && isWord(runes[start-1]) {
		start--
	}
	return string(runes[start:col]), start
}

// completionFilterText is the text an item filters against: FilterText when
// set, else its Label.
func completionFilterText(it CompletionItem) string {
	if it.FilterText != "" {
		return it.FilterText
	}
	return it.Label
}

// filterCompletions keeps the items whose filter text has word as a
// case-insensitive PREFIX (the IntelliSense/LSP default; an empty word matches
// everything), preserving the source order. It is the single match rule the
// popup uses, kept prefix-based so "\se" narrows to the commands that start
// "\se" — consistent, predictable filtering as the user types.
func filterCompletions(items []CompletionItem, word string) []CompletionItem {
	w := strings.ToLower(word)
	out := make([]CompletionItem, 0, len(items))
	for _, it := range items {
		if w == "" || strings.HasPrefix(strings.ToLower(completionFilterText(it)), w) {
			out = append(out, it)
		}
	}
	return out
}

// openOrRefreshCompletion (re)computes the filtered candidate list for the
// caret context and shows the popup. It is the one entry point every trigger
// routes through: an identifier keystroke, an explicit Ctrl/Cmd+Space, and a
// Backspace that stays within a word. With no source, or when the filter
// narrows to nothing, it closes instead — so the popup is shown only when it
// has something to offer. Opening from closed resets the selection + scroll to
// the top; refreshing while already open clamps the prior selection into the
// new list.
func (c *CodeEditor) openOrRefreshCompletion() {
	if c.CompletionSource == nil {
		c.closeCompletion()
		return
	}
	word, start := c.currentWord()
	filtered := filterCompletions(c.CompletionSource(c.lines, c.CursorLine().Get(), c.CursorCol().Get()), word)
	if len(filtered) == 0 {
		c.closeCompletion()
		return
	}
	if !c.CompletionActive() {
		c.CompletionSelection().Set(0)
		c.compScroll = 0
	}
	c.compItems = filtered
	c.compWordStart = start
	c.CompletionOpen().Set(true)
	c.clampCompSel()
}

// closeCompletion hides the popup and clears its derived state. Safe to call
// when already closed (Set is change-gated, so no spurious notification).
func (c *CodeEditor) closeCompletion() {
	c.CompletionOpen().Set(false)
	c.compItems = nil
	c.compWordStart = 0
	c.compScroll = 0
	c.CompletionSelection().Set(0)
}

// clampCompSel keeps the selection inside [0, len(compItems)-1] (pinning to 0
// on an empty list) and scrolls it into the visible window — the popup analogue
// of CommandPalette.clampSelected, called after every selection or filter
// change so a shrinking list never leaves the highlight dangling.
func (c *CodeEditor) clampCompSel() {
	n := len(c.compItems)
	sel := c.CompletionSelection().Get()
	if n == 0 {
		c.CompletionSelection().Set(0)
		return
	}
	if sel < 0 {
		sel = 0
	}
	if sel >= n {
		sel = n - 1
	}
	c.CompletionSelection().Set(sel)
	c.scrollCompSelIntoView()
}

// maxCompScroll is the highest compScroll that still fills the popup window:
// len(compItems) - CompletionMaxRows, floored at 0.
func (c *CodeEditor) maxCompScroll() int {
	m := len(c.compItems) - CompletionMaxRows
	if m < 0 {
		m = 0
	}
	return m
}

// clampedCompScroll returns compScroll clamped to [0, maxCompScroll] without
// mutating the field, so a stale value after the list shrank never paints or
// hit-tests outside the valid window.
func (c *CodeEditor) clampedCompScroll() int {
	s := c.compScroll
	if s < 0 {
		s = 0
	}
	if m := c.maxCompScroll(); s > m {
		s = m
	}
	return s
}

// scrollCompSelIntoView nudges compScroll so the selected row stays within the
// CompletionMaxRows-tall window, keeping arrow/page navigation visible past the
// fold.
func (c *CodeEditor) scrollCompSelIntoView() {
	sel := c.CompletionSelection().Get()
	if sel < c.compScroll {
		c.compScroll = sel
	} else if sel >= c.compScroll+CompletionMaxRows {
		c.compScroll = sel - CompletionMaxRows + 1
	}
	c.compScroll = c.clampedCompScroll()
}

// moveCompSel shifts the selection by delta rows within the filtered list,
// clamped at both ends (no wraparound — matching DropDown/ListBox), and scrolls
// to keep it visible.
func (c *CodeEditor) moveCompSel(delta int) {
	c.CompletionSelection().Set(c.CompletionSelection().Get() + delta)
	c.clampCompSel()
}

// scrollCompletion shifts the popup's scroll window by delta rows (wheel over
// the open popup), clamped to the valid range.
func (c *CodeEditor) scrollCompletion(delta int) {
	c.compScroll += delta
	c.compScroll = c.clampedCompScroll()
}

// acceptCompletion inserts the selected item — replacing the current word
// [compWordStart, CursorCol) with its InsertText (or Label) — places the caret
// at the "$0" marker if present else after the inserted text, and closes the
// popup. A no-op on an empty list.
func (c *CodeEditor) acceptCompletion() {
	if len(c.compItems) == 0 {
		c.closeCompletion()
		return
	}
	it := c.compItems[c.CompletionSelection().Get()]
	insert := it.InsertText
	if insert == "" {
		insert = it.Label
	}
	body, caretOff := parseSnippet(insert)
	c.pushUndo()
	line := c.CursorLine().Get()
	runes := []rune(c.lines[line])
	before := runes[:c.compWordStart]
	after := runes[c.CursorCol().Get():]
	merged := append(append(append([]rune{}, before...), []rune(body)...), after...)
	c.lines[line] = string(merged)
	c.CursorCol().Set(c.compWordStart + caretOff)
	c.sync() // publish the edit onto the Text() Observable (OnChange's successor)
	c.closeCompletion()
}

// parseSnippet expands an InsertText's tab-stop markers into the literal text
// to insert plus the caret offset (in runes) within it. It honours a single
// final-cursor stop — bare "$0" or the braced "${0:default}" — placing the
// caret where the stop sat (after its default text, if any). Every other
// numbered marker ("$1", "${2:name}") is a placeholder this minimal expander
// does not step through: it is stripped to just its default text, so unknown
// markers degrade gracefully instead of leaking "$" syntax into the buffer.
// With no stop at all the caret lands after the whole body.
func parseSnippet(s string) (body string, caret int) {
	runes := []rune(s)
	var out []rune
	caret = -1
	i := 0
	for i < len(runes) {
		if runes[i] != '$' {
			out = append(out, runes[i])
			i++
			continue
		}
		// A "$" begins a marker. Two forms: "$<digits>" and "${<digits>[:def]}".
		if i+1 < len(runes) && runes[i+1] == '{' {
			num, def, end, ok := parseBraceStop(runes, i)
			if ok {
				if num == 0 && caret < 0 {
					caret = len(out)
				}
				out = append(out, []rune(def)...)
				i = end
				continue
			}
		}
		if num, end, ok := parseBareStop(runes, i); ok {
			if num == 0 && caret < 0 {
				caret = len(out)
			}
			i = end
			continue
		}
		// A lone "$" not starting a marker is a literal dollar sign.
		out = append(out, runes[i])
		i++
	}
	if caret < 0 {
		caret = len(out)
	}
	return string(out), caret
}

// parseBareStop parses a "$<digits>" marker starting at runes[i] (runes[i] is
// '$'). It returns the stop number, the index just past the marker, and whether
// a bare numbered stop was actually there (a "$" followed by no digit is not).
func parseBareStop(runes []rune, i int) (num, end int, ok bool) {
	j := i + 1
	for j < len(runes) && unicode.IsDigit(runes[j]) {
		num = num*10 + int(runes[j]-'0')
		j++
	}
	if j == i+1 {
		return 0, 0, false // no digits after '$'
	}
	return num, j, true
}

// parseBraceStop parses a "${<digits>[:default]}" marker starting at runes[i]
// (runes[i] is '$', runes[i+1] is '{'). It returns the stop number, its default
// text (empty when the ":default" part is absent), the index just past the
// closing '}', and whether a well-formed braced stop was there (a missing digit
// or a missing '}' is not).
func parseBraceStop(runes []rune, i int) (num int, def string, end int, ok bool) {
	j := i + 2 // past "${"
	digits := j
	for j < len(runes) && unicode.IsDigit(runes[j]) {
		num = num*10 + int(runes[j]-'0')
		j++
	}
	if j == digits {
		return 0, "", 0, false // no digits after "${"
	}
	if j < len(runes) && runes[j] == ':' {
		j++
		defStart := j
		for j < len(runes) && runes[j] != '}' {
			j++
		}
		if j >= len(runes) {
			return 0, "", 0, false // unterminated
		}
		def = string(runes[defStart:j])
	}
	if j >= len(runes) || runes[j] != '}' {
		return 0, "", 0, false // unterminated
	}
	return num, def, j + 1, true
}

// isCompletionTrigger reports whether ev is the explicit "open completion"
// chord — Ctrl+Space or Cmd(Meta)+Space — however the host encodes it: as a
// combined Code ("Ctrl+Space", "Meta+ ", …) or as a plain Space with the Ctrl
// or Meta modifier flag set.
func isCompletionTrigger(ev Event) bool {
	switch ev.Code {
	case "Ctrl+Space", "Meta+Space", "Ctrl+ ", "Meta+ ":
		return true
	case " ", "Space":
		return ev.Ctrl || ev.Meta
	}
	return false
}

// OnEvent overrides the promoted TextView handler to run the completion popup.
// While the popup is open it consumes the keys that drive it (Up/Down/PageUp/
// PageDown navigate, Enter/Tab accept, Escape closes) so they never reach the
// buffer, scrolls on the wheel, and accepts a click on a row; a caret-moving
// key or an off-popup click closes it and falls through. Typing an identifier
// character opens/refreshes the popup (after the character is inserted); the
// explicit Ctrl/Cmd+Space chord opens it anywhere. Everything else is delegated
// unchanged to TextView, so an editor with no CompletionSource behaves exactly
// as before.
func (c *CodeEditor) OnEvent(ev Event) {
	if ev.Kind == EventKeyDown && isCompletionTrigger(ev) {
		c.openOrRefreshCompletion()
		return
	}
	if c.CompletionActive() && c.handleCompletionEvent(ev) {
		return
	}
	switch ev.Kind {
	case EventChar:
		c.TextView.OnEvent(ev)
		c.onCharTyped(ev.Code)
	default:
		c.TextView.OnEvent(ev)
	}
}

// handleCompletionEvent runs one event against the OPEN popup and reports
// whether the popup fully handled it (so OnEvent stops). A consumed key never
// reaches the buffer; a caret-moving key or an off-popup click returns false
// AFTER closing the popup, so OnEvent still delivers the event to TextView.
func (c *CodeEditor) handleCompletionEvent(ev Event) bool {
	switch ev.Kind {
	case EventScroll:
		c.scrollCompletion(ev.Delta)
		return true
	case EventClick:
		return c.completionClick(ev)
	case EventChar:
		return false // handled by OnEvent's insert-then-refresh path
	case EventKeyDown:
		return c.completionKey(ev)
	default:
		return false
	}
}

// completionKey handles a key while the popup is open. It returns true when the
// key was consumed by the popup. Backspace is a special case: it is forwarded
// to the buffer and then the popup is refreshed against the shortened word
// (closing if the word emptied or no candidate survives) — reported consumed so
// OnEvent does not also forward it. A caret-moving key (arrows left/right,
// Home/End) closes the popup and returns false so OnEvent forwards it.
func (c *CodeEditor) completionKey(ev Event) bool {
	switch ev.Code {
	case "ArrowDown":
		c.moveCompSel(1)
	case "ArrowUp":
		c.moveCompSel(-1)
	case "PageDown":
		c.moveCompSel(CompletionMaxRows)
	case "PageUp":
		c.moveCompSel(-CompletionMaxRows)
	case "Enter", "Tab":
		c.acceptCompletion()
	case "Escape":
		c.closeCompletion()
	case "Backspace":
		c.TextView.OnEvent(ev)
		c.openOrRefreshCompletion()
	default:
		c.closeCompletion()
		return false
	}
	return true
}

// completionClick routes a click while the popup is open: a click inside the
// popup selects that row (mapped back through the scroll offset) and accepts
// it; a click anywhere else closes the popup and returns false so OnEvent
// forwards the click to the buffer (moving the caret).
func (c *CodeEditor) completionClick(ev Event) bool {
	pb := c.CompletionBounds()
	if ev.X < pb.X || ev.X >= pb.X+pb.W || ev.Y < pb.Y || ev.Y >= pb.Y+pb.H {
		c.closeCompletion()
		return false
	}
	// The popup is exactly len(compItems) (capped at CompletionMaxRows) rows
	// tall, so a click within its bounds always lands on a real row.
	row := c.clampedCompScroll() + (ev.Y-pb.Y)/scaled(CompletionRowH)
	c.CompletionSelection().Set(row)
	c.acceptCompletion()
	return true
}

// onCharTyped runs after a printable character reached the buffer: an
// identifier character opens/refreshes the popup, any other character closes it
// (unless there is no source, in which case there is nothing to do). code is
// the one-rune EventChar payload; an empty payload — which TextView ignored —
// is ignored here too.
func (c *CodeEditor) onCharTyped(code string) {
	if c.CompletionSource == nil || code == "" {
		return
	}
	r := []rune(code)
	last := r[len(r)-1]
	if c.wordCharFn()(last) {
		c.openOrRefreshCompletion()
	} else {
		c.closeCompletion()
	}
}

// CompletionBounds is the popup's rectangle, anchored so its top-left sits just
// below the start of the word being completed (via CaretPixel, the shared caret
// geometry) and flipped ABOVE the caret line when a below-placement would
// overflow the editor's bottom edge. Width fits the widest visible row (kind
// glyph + label + detail), floored at CompletionMinW and clamped within the
// editor; height is one row per visible candidate. A closed or empty popup
// returns the zero Rect.
func (c *CodeEditor) CompletionBounds() Rect {
	rows := len(c.compItems)
	if rows == 0 {
		return Rect{}
	}
	if rows > CompletionMaxRows {
		rows = CompletionMaxRows
	}
	rowH := scaled(CompletionRowH)
	h := rows * rowH
	w := c.completionWidth()
	r := c.Bounds()
	cx, cy := c.CaretPixel(c.CursorLine().Get(), c.compWordStart)
	lineH := c.glyphHeight() + 4
	x, y := cx, cy+lineH
	if y+h > r.Y+r.H {
		y = cy - h // flip above the caret line
	}
	if x+w > r.X+r.W {
		x = r.X + r.W - w
	}
	if x < r.X {
		x = r.X
	}
	return Rect{X: x, Y: y, W: w, H: h}
}

// completionWidth measures the popup width: the widest row's kind-glyph column
// plus label plus (when any item carries one) a detail column, plus padding —
// floored at CompletionMinW and capped at the editor width so it never spills
// past the frame.
func (c *CodeEditor) completionWidth() int {
	glyphCol := completionGlyphCells * c.glyphAdvance()
	w := CompletionMinW
	for _, it := range c.compItems {
		row := glyphCol + c.textWidth(it.Label)
		if it.Detail != "" {
			row += scaled(completionDetailGapPx) + c.textWidth(it.Detail)
		}
		if row += 2 * scaled(completionPadX); row > w {
			w = row
		}
	}
	if bw := c.Bounds().W; bw > 0 && w > bw {
		w = bw
	}
	return w
}

// drawCompletion paints the popup at CompletionBounds when open: it reuses a
// ListBox for the row layout, selection highlight and virtual scrolling (fed
// compScroll), with an ItemRenderer that draws each row's kind glyph, label and
// dim right-aligned detail. The optional doc panel for the selected item is
// painted just below. A no-op while closed or empty.
func (c *CodeEditor) drawCompletion(p painter.Painter, theme *Theme) {
	if !c.CompletionActive() || len(c.compItems) == 0 {
		return
	}
	pb := c.CompletionBounds()
	labels := make([]string, len(c.compItems))
	for i, it := range c.compItems {
		labels[i] = it.Label
	}
	lb := NewListBox(labels)
	lb.RowHeight = scaled(CompletionRowH)
	lb.Selected().Set(c.CompletionSelection().Get())
	lb.ScrollRow().Set(c.clampedCompScroll())
	lb.ItemRenderer = func(pp painter.Painter, th *Theme, rc Rect, index int, _ string, selected bool, ink RGBA) {
		c.drawCompletionRow(pp, th, rc, c.compItems[index], selected, ink)
	}
	lb.SetBounds(pb)
	lb.Draw(p, theme)
	c.drawCompletionDoc(p, theme, pb)
}

// drawCompletionRow renders one candidate inside the ListBox row rectangle rc:
// a colour-coded kind glyph, the label in the row ink, and — right-aligned and
// dimmed — the detail hint. On the selected (accent) row the glyph switches to
// the row ink so it stays legible against the highlight.
func (c *CodeEditor) drawCompletionRow(p painter.Painter, theme *Theme, rc Rect, it CompletionItem, selected bool, ink RGBA) {
	ty := rc.Y + (rc.H-c.glyphHeight())/2
	glyphInk := completionKindColor(it.Kind, theme)
	if selected {
		glyphInk = ink
	}
	x := rc.X + scaled(completionPadX)
	c.drawText(p, x, ty, completionKindGlyph(it.Kind), glyphInk)
	x = rc.X + scaled(completionPadX) + completionGlyphCells*c.glyphAdvance()
	c.drawText(p, x, ty, it.Label, ink)
	if it.Detail != "" {
		dw := c.textWidth(it.Detail)
		dx := rc.X + rc.W - scaled(completionPadX) - dw
		c.drawText(p, dx, ty, it.Detail, blendRGBA(ink, theme.Surface, 0.45))
	}
}

// drawCompletionDoc paints the selected item's Documentation, when it has any,
// in a one-row panel directly below the list (or above it, mirroring a list
// that itself flipped above the caret). A selected item with no Documentation
// paints nothing, so the panel appears only when it has something to show.
func (c *CodeEditor) drawCompletionDoc(p painter.Painter, theme *Theme, list Rect) {
	sel := c.CompletionSelection().Get()
	if sel < 0 || sel >= len(c.compItems) {
		return
	}
	doc := c.compItems[sel].Documentation
	if doc == "" {
		return
	}
	rowH := scaled(CompletionRowH)
	y := list.Y + list.H
	if editor := c.Bounds(); y+rowH > editor.Y+editor.H {
		y = list.Y - rowH // flip above, matching a list that opened upward
	}
	fillRect(p, list.X, y, list.W, rowH, theme.SurfaceAlt)
	strokeRect(p, list.X, y, list.W, rowH, theme.Border)
	ty := y + (rowH-c.glyphHeight())/2
	c.drawText(p, list.X+scaled(completionPadX), ty, doc, dimInk(theme))
}

// completionKindGlyph is the one- or two-character badge drawn beside a
// candidate to signal its kind at a glance — a compact, font-agnostic cue (the
// toolkit's bitmap font has no icon set), consistent across every kind so the
// column reads as a legend.
func completionKindGlyph(k CompletionItemKind) string {
	switch k {
	case CompletionMethod:
		return "m"
	case CompletionFunction:
		return "fn"
	case CompletionConstructor:
		return "ct"
	case CompletionField:
		return "fd"
	case CompletionVariable:
		return "v"
	case CompletionProperty:
		return "pr"
	case CompletionClass:
		return "C"
	case CompletionInterface:
		return "I"
	case CompletionModule:
		return "M"
	case CompletionEnum:
		return "E"
	case CompletionKeyword:
		return "kw"
	case CompletionSnippet:
		return "sn"
	case CompletionConstant:
		return "cn"
	default: // CompletionText
		return "ab"
	}
}

// completionKindColor is the kind glyph's colour, derived from the theme so it
// reads under both light and dark palettes: callables in the accent, types in
// OnSurface, keywords/snippets/constants in a blend, and plain text dimmed.
func completionKindColor(k CompletionItemKind, theme *Theme) RGBA {
	switch k {
	case CompletionMethod, CompletionFunction, CompletionConstructor:
		return theme.Accent
	case CompletionClass, CompletionInterface, CompletionModule, CompletionEnum:
		return theme.OnSurface
	case CompletionField, CompletionVariable, CompletionProperty:
		return blendRGBA(theme.Accent, theme.OnSurface, 0.5)
	case CompletionKeyword, CompletionSnippet, CompletionConstant:
		return blendRGBA(theme.OnSurface, theme.Accent, 0.5)
	default: // CompletionText
		return dimInk(theme)
	}
}
