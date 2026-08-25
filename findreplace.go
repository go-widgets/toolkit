// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// FindReplace is a compact, floating find-and-replace bar for a code editor: a
// two-row panel anchored to the TOP-RIGHT of its bounds so it never covers the
// document — the whole point being that the user watches the highlighted matches
// (see CodeEditor.SetMatchHighlights) while they type. It is the docked
// find-bar idiom (VS Code / a browser's Ctrl-F), not a full-screen modal, which
// is why it draws over a corner rather than a dimming scrim.
//
// It is UI ONLY: it holds the query, the replacement, the search toggles and the
// count readout, and it fires callbacks — but it never runs a regexp itself. A
// host wires OnQueryChange (and the toggles, which also fire it) to run
// [FindMatches] over its buffer, then pushes the result back with SetMatches and
// drives the editor's highlight API; Prev/Next/Replace/Replace-all are likewise
// the host's to implement over its own buffer. Keeping the engine outside makes
// the bar reusable over any editor, not just this toolkit's.
//
// Every visible element is a toolkit widget — a [Backdrop] ground, two
// [SearchEntry] fields, [Button] toggles/actions and a [Label] readout — composed
// and laid out here; the bar hand-draws nothing.
//
// # Reactive surface
//
// The query and replacement text are the two SearchEntry Text() Observables,
// exposed as Query() and Replace(); the three search toggles are Button
// Selected() Observables, exposed as Regex() / CaseSensitive() / WholeWord();
// and the match count, current index, invalid-pattern flag and visibility are
// this widget's own Observables (Total / Current / Invalid / Visible). A host
// binds or subscribes to any of them. The count READOUT ("3 of 12", "No
// results", "Bad pattern") is derived from Total/Current/Invalid and refreshed
// on each Draw, so a host only sets the numbers (SetMatches / SetInvalid) and
// the label follows.
type FindReplace struct {
	Base

	// OnQueryChange fires whenever the query text OR a search toggle changes —
	// the host's cue to re-run the search and push the new count + highlights
	// back. OnPrev / OnNext step the current match; OnReplace replaces the
	// current match and OnReplaceAll every match; OnClose is the user dismissing
	// the bar (the ✕ button or Escape). Any may be nil.
	OnQueryChange func()
	OnPrev        func()
	OnNext        func()
	OnReplace     func()
	OnReplaceAll  func()
	OnClose       func()

	// panel is the rounded ground; the interactive children sit on top of it.
	panel *Backdrop
	// query / replace are the two text fields; countLabel is the readout.
	query      *SearchEntry
	replace    *SearchEntry
	countLabel *Label
	// prev / next step matches; replaceBtn / replaceAllBtn apply the replacement;
	// closeBtn dismisses. regexBtn / caseBtn / wordBtn are the search toggles,
	// whose Selected() Observable is the mode state.
	prevBtn       *Button
	nextBtn       *Button
	replaceBtn    *Button
	replaceAllBtn *Button
	closeBtn      *Button
	regexBtn      *Button
	caseBtn       *Button
	wordBtn       *Button

	// visible / total / current / invalid are this widget's own reactive state,
	// reached through the Visible()/Total()/Current()/Invalid() accessors.
	visible *mvvm.Observable[bool]
	total   *mvvm.Observable[int]
	current *mvvm.Observable[int]
	invalid *mvvm.Observable[bool]
}

// Find-bar layout metrics (LOGICAL units, routed through scaled at use so the
// bar grows with HiDPI / touch density). frPanelW is the fixed panel width;
// frRowH each row's height; frBtnW a square icon-button; frWideBtnW a text
// action button ("Replace"/"All"); frCountW the count readout column.
const (
	frPanelW   = 360
	frMargin   = 8
	frPad      = 6
	frRowH     = 22
	frGap      = 4
	frBtnW     = 22
	frWideBtnW = 56
	frCountW   = 54
)

// NewFindReplace builds a hidden find-and-replace bar with the regex toggle on
// (the query field is a regular expression by default), case-sensitivity and
// whole-word off, an empty query/replacement and a "No results" readout. Call
// Open to show it and wire the OnX callbacks to a host that owns the buffer.
func NewFindReplace() *FindReplace {
	f := &FindReplace{
		panel:         &Backdrop{Radius: scaled(6), StrokeWidth: 1},
		query:         NewSearchEntry(""),
		replace:       NewSearchEntry(""),
		countLabel:    NewLabel(""),
		prevBtn:       NewButton("<", nil),
		nextBtn:       NewButton(">", nil),
		replaceBtn:    NewButton("Replace", nil),
		replaceAllBtn: NewButton("All", nil),
		closeBtn:      NewButton("x", nil),
		regexBtn:      NewButton(".*", nil),
		caseBtn:       NewButton("Aa", nil),
		wordBtn:       NewButton("W", nil),
		visible:       mvvm.NewObservable(false),
		total:         mvvm.NewObservable(0),
		current:       mvvm.NewObservable(-1),
		invalid:       mvvm.NewObservable(false),
	}
	f.regexBtn.Selected().Set(true) // regex mode on by default
	f.countLabel.Align = AlignCenter
	f.countLabel.VAlign = VMiddle

	// Query text and every toggle re-run the host's search.
	f.query.Text().SubscribeChanged(f.fireQueryChange)
	f.regexBtn.Selected().SubscribeChanged(f.fireQueryChange)
	f.caseBtn.Selected().SubscribeChanged(f.fireQueryChange)
	f.wordBtn.Selected().SubscribeChanged(f.fireQueryChange)

	// Toggle buttons flip their own Selected() (the subscription above then fires
	// the query change); action buttons forward to the host callbacks.
	f.regexBtn.OnClick = func() { toggleObs(f.regexBtn.Selected()) }
	f.caseBtn.OnClick = func() { toggleObs(f.caseBtn.Selected()) }
	f.wordBtn.OnClick = func() { toggleObs(f.wordBtn.Selected()) }
	f.prevBtn.OnClick = func() { fire(f.OnPrev) }
	f.nextBtn.OnClick = func() { fire(f.OnNext) }
	f.replaceBtn.OnClick = func() { fire(f.OnReplace) }
	f.replaceAllBtn.OnClick = func() { fire(f.OnReplaceAll) }
	f.closeBtn.OnClick = f.closeFromUser
	return f
}

// fire calls fn when it is non-nil — the guard every button callback shares.
func fire(fn func()) {
	if fn != nil {
		fn()
	}
}

// toggleObs flips a bool Observable, the toggle buttons' one action.
func toggleObs(o *mvvm.Observable[bool]) { o.Set(!o.Get()) }

// fireQueryChange invokes OnQueryChange when set — the shared observer for the
// query text and the three toggles.
func (f *FindReplace) fireQueryChange() { fire(f.OnQueryChange) }

// closeFromUser hides the bar and fires OnClose — the ✕ button and Escape path.
func (f *FindReplace) closeFromUser() {
	f.Close()
	fire(f.OnClose)
}

// --- reactive accessors ----------------------------------------------------

// Visible reports (and drives) whether the bar is shown. Draw and OnEvent are
// no-ops while it is false.
func (f *FindReplace) Visible() *mvvm.Observable[bool] { return f.visible }

// Query is the search text — the query SearchEntry's own Text() Observable. A
// host subscribes to it (via OnQueryChange, or directly) and reads it to run the
// regexp.
func (f *FindReplace) Query() *mvvm.Observable[string] { return f.query.Text() }

// Replace is the replacement text — the replace SearchEntry's Text() Observable.
func (f *FindReplace) Replace() *mvvm.Observable[string] { return f.replace.Text() }

// Regex reports whether the query is a regular expression (true, the default) or
// a literal string — the regex toggle's Selected() Observable.
func (f *FindReplace) Regex() *mvvm.Observable[bool] { return f.regexBtn.Selected() }

// CaseSensitive reports whether matching is case-sensitive — the "Aa" toggle.
func (f *FindReplace) CaseSensitive() *mvvm.Observable[bool] { return f.caseBtn.Selected() }

// WholeWord reports whether matches are constrained to whole words — the "W"
// toggle.
func (f *FindReplace) WholeWord() *mvvm.Observable[bool] { return f.wordBtn.Selected() }

// Total is the number of matches the host found for the current query.
func (f *FindReplace) Total() *mvvm.Observable[int] { return f.total }

// Current is the 0-based index of the active match within Total, or -1 when none
// is current (an empty result set, or before the host selects one).
func (f *FindReplace) Current() *mvvm.Observable[int] { return f.current }

// Invalid reports whether the current query failed to compile — the bar shows a
// "Bad pattern" readout and the host paints no highlights.
func (f *FindReplace) Invalid() *mvvm.Observable[bool] { return f.invalid }

// Options snapshots the three toggles into a [SearchOptions] a host hands to
// [FindMatches] — so the host reads the mode in one call instead of three.
func (f *FindReplace) Options() SearchOptions {
	return SearchOptions{
		Regex:         f.Regex().Get(),
		CaseSensitive: f.CaseSensitive().Get(),
		WholeWord:     f.WholeWord().Get(),
	}
}

// SetMatches records a fresh search result: total matches and the 0-based index
// of the current one (-1 for none). It clears any invalid-pattern state, since a
// result set means the query compiled.
func (f *FindReplace) SetMatches(total, current int) {
	f.invalid.Set(false)
	f.total.Set(total)
	f.current.Set(current)
}

// SetInvalid marks the current query as (not) compilable. Marking it invalid
// also zeroes the count so the readout shows "Bad pattern" rather than a stale
// tally.
func (f *FindReplace) SetInvalid(bad bool) {
	f.invalid.Set(bad)
	if bad {
		f.total.Set(0)
		f.current.Set(-1)
	}
}

// CountText is the readout string derived from the current Total / Current /
// Invalid: "Bad pattern" for an invalid query, "No results" for zero matches,
// else "<index+1> of <total>". It is what the Label shows, exposed so a host can
// mirror the readout elsewhere (a status bar) without re-deriving it.
func (f *FindReplace) CountText() string {
	return formatCount(f.total.Get(), f.current.Get(), f.invalid.Get())
}

// formatCount renders the match readout. Invalid wins over everything; then a
// non-positive total is "No results"; otherwise the 1-based current position
// (clamped into [1, total] so a -1 or overshoot never prints nonsense) of the
// total.
func formatCount(total, current int, invalid bool) string {
	if invalid {
		return "Bad pattern"
	}
	if total <= 0 {
		return "No results"
	}
	idx := current
	if idx < 0 {
		idx = 0
	}
	if idx >= total {
		idx = total - 1
	}
	return strconv.Itoa(idx+1) + " of " + strconv.Itoa(total)
}

// Open shows the bar and moves keyboard focus to the query field.
func (f *FindReplace) Open() {
	f.visible.Set(true)
	f.query.SetFocused(true)
	f.replace.SetFocused(false)
}

// Close hides the bar without firing OnClose (that is the user-intent path — see
// closeFromUser). Idempotent.
func (f *FindReplace) Close() { f.visible.Set(false) }

// --- composition -----------------------------------------------------------

// controls are the interactive + label children on top of the panel, in reading
// (left-to-right, top row then bottom) order — the order Draw paints them, the
// order a click is dispatched, and the order a screen reader announces them.
func (f *FindReplace) controls() []Widget {
	return []Widget{
		f.query, f.regexBtn, f.caseBtn, f.wordBtn, f.countLabel,
		f.prevBtn, f.nextBtn, f.closeBtn,
		f.replace, f.replaceBtn, f.replaceAllBtn,
	}
}

// Children exposes the composed controls so a generic walk (accessibility)
// reaches the fields and buttons inside the bar — the childContainer contract
// enforced by TestEveryContainerExposesItsChildren.
func (f *FindReplace) Children() []Widget { return f.controls() }

// A11y reports the bar as a group named for its purpose; the query, buttons and
// readout inside describe themselves through the child walk.
func (f *FindReplace) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Name: "Find and replace"}
}

// panelRect is the top-right anchored panel rectangle within the bar's bounds.
// The panel is frPanelW wide, but a bar narrower than that shrinks it to fit
// (never wider than the bounds less its margins) so a compact editor does not
// get a panel hanging off its edge; the field widths inside then clamp to 0.
func (f *FindReplace) panelRect() Rect {
	b := f.Bounds()
	m := scaled(frMargin)
	pw := scaled(frPanelW)
	if avail := b.W - 2*m; pw > avail {
		pw = avail
	}
	pw = clampW(pw)
	ph := scaled(frPad)*2 + scaled(frRowH)*2 + scaled(frGap)
	return Rect{X: b.X + b.W - pw - m, Y: b.Y + m, W: pw, H: ph}
}

// relayout positions the panel and every child. It runs at the top of Draw and
// OnEvent so a click always hit-tests the same rectangles Draw painted.
func (f *FindReplace) relayout() {
	pr := f.panelRect()
	f.panel.SetBounds(pr)
	pad := scaled(frPad)
	rowH := scaled(frRowH)
	gap := scaled(frGap)
	btn := scaled(frBtnW)
	wide := scaled(frWideBtnW)
	countW := scaled(frCountW)

	y1 := pr.Y + pad
	y2 := y1 + rowH + gap
	left := pr.X + pad
	right := pr.X + pr.W - pad

	// Row 1, packed from the right: close, next, prev, count, word, case, regex;
	// the query field fills what is left on the row's left.
	rx := right
	rx = place(f.closeBtn, rx, y1, btn, rowH, gap)
	rx = place(f.nextBtn, rx, y1, btn, rowH, gap)
	rx = place(f.prevBtn, rx, y1, btn, rowH, gap)
	rx = place(f.countLabel, rx, y1, countW, rowH, gap)
	rx = place(f.wordBtn, rx, y1, btn, rowH, gap)
	rx = place(f.caseBtn, rx, y1, btn, rowH, gap)
	rx = place(f.regexBtn, rx, y1, btn, rowH, gap)
	f.query.SetBounds(Rect{X: left, Y: y1, W: clampW(rx - left), H: rowH})

	// Row 2, packed from the right: replace-all, replace; the replacement field
	// fills the rest.
	r2 := right
	r2 = place(f.replaceAllBtn, r2, y2, wide, rowH, gap)
	r2 = place(f.replaceBtn, r2, y2, wide, rowH, gap)
	f.replace.SetBounds(Rect{X: left, Y: y2, W: clampW(r2 - left), H: rowH})
}

// place positions w as a width-wide, h-tall slot whose RIGHT edge is at rightX,
// and returns the right edge for the NEXT control packed to its left — the slot's
// left edge minus one inter-control gap.
func place(w Widget, rightX, y, width, h, gap int) int {
	x := rightX - width
	w.SetBounds(Rect{X: x, Y: y, W: width, H: h})
	return x - gap
}

// clampW floors a computed field width at 0 so a very narrow bar never hands a
// negative width to a text field.
func clampW(w int) int {
	if w < 0 {
		return 0
	}
	return w
}

// Draw paints the panel ground and the composed controls, refreshing the count
// readout first. Nothing is drawn while the bar is hidden.
func (f *FindReplace) Draw(p painter.Painter, theme *Theme) {
	if !f.visible.Get() {
		return
	}
	f.relayout()
	f.countLabel.Text().Set(f.CountText())
	if f.invalid.Get() {
		f.countLabel.Ink = formFieldErrorInk
	} else {
		f.countLabel.Ink = RGBA{}
	}
	f.panel.Fill = theme.Surface
	f.panel.Stroke = theme.Border
	f.panel.Draw(p, theme)
	for _, w := range f.controls() {
		w.Draw(p, theme)
	}
}

// OnEvent routes input while the bar is visible: a click is dispatched to the
// control under it (focusing the field clicked), Escape closes the bar, Enter /
// Shift+Enter step to the next / previous match, and every other key or
// character goes to the focused text field. Events while hidden are ignored.
func (f *FindReplace) OnEvent(ev Event) {
	if !f.visible.Get() {
		return
	}
	f.relayout()
	switch ev.Kind {
	case EventClick:
		f.dispatchClick(ev)
	case EventKeyDown:
		f.handleKey(ev)
	case EventChar, EventCompositionStart, EventCompositionUpdate, EventCompositionEnd:
		f.focusedEntry().OnEvent(ev)
	}
}

// handleKey runs the bar-level keys (Escape to close, Enter/Shift+Enter to step
// matches) and forwards anything else — Backspace, etc. — to the focused field.
func (f *FindReplace) handleKey(ev Event) {
	switch ev.Code {
	case "Escape":
		f.closeFromUser()
	case "Enter":
		if ev.Shift {
			fire(f.OnPrev)
		} else {
			fire(f.OnNext)
		}
	default:
		f.focusedEntry().OnEvent(ev)
	}
}

// focusedEntry is the text field that receives typed input: the replacement
// field when it holds focus, else the query field (the default).
func (f *FindReplace) focusedEntry() *SearchEntry {
	if f.replace.Focused() {
		return f.replace
	}
	return f.query
}

// dispatchClick forwards a click to the control under it (in surface
// coordinates) and, when that control is one of the two text fields, moves
// keyboard focus to it. A click on the panel padding — or outside the bar
// entirely — is ignored (the bar is docked, not modal: an outside click does not
// dismiss it).
func (f *FindReplace) dispatchClick(ev Event) {
	b := f.Bounds()
	ax, ay := ev.X+b.X, ev.Y+b.Y
	for _, w := range f.controls() {
		cr := w.Bounds()
		if !cr.Contains(ax, ay) {
			continue
		}
		switch w {
		case Widget(f.query):
			f.query.SetFocused(true)
			f.replace.SetFocused(false)
		case Widget(f.replace):
			f.replace.SetFocused(true)
			f.query.SetFocused(false)
		}
		w.OnEvent(translateEvent(ev, b, cr))
		return
	}
}
