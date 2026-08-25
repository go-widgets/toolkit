// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"testing"
)

// layoutFR opens the bar, gives it a surface-sized bounds and draws it once so
// every child's Bounds is populated (relayout runs inside Draw), returning the
// painted surface for pixel checks.
func layoutFR(f *FindReplace, w, h int) []byte {
	f.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	f.Draw(newP(surf, w), DefaultLight())
	return surf
}

// clickFR sends an EventClick centred on a child widget (the bar's bounds sit at
// the origin, so surface coords equal widget-local coords).
func clickFR(f *FindReplace, w Widget) {
	cr := w.Bounds()
	f.OnEvent(Event{Kind: EventClick, X: cr.X + cr.W/2, Y: cr.Y + cr.H/2})
}

func TestFindReplaceDefaults(t *testing.T) {
	f := NewFindReplace()
	if f.Visible().Get() {
		t.Error("a new bar should be hidden")
	}
	if !f.Regex().Get() {
		t.Error("regex mode should default on")
	}
	if f.CaseSensitive().Get() || f.WholeWord().Get() {
		t.Error("case-sensitive and whole-word should default off")
	}
	if f.Query().Get() != "" || f.Replace().Get() != "" {
		t.Error("query and replacement should start empty")
	}
	if f.Total().Get() != 0 || f.Current().Get() != -1 || f.Invalid().Get() {
		t.Errorf("count state = (%d,%d,%v), want (0,-1,false)", f.Total().Get(), f.Current().Get(), f.Invalid().Get())
	}
	if opts := f.Options(); opts != (SearchOptions{Regex: true}) {
		t.Errorf("Options = %+v, want {Regex:true}", opts)
	}
	if a := f.A11y(); a.Role != RoleGroup || a.Name != "Find and replace" {
		t.Errorf("A11y = %+v, want group 'Find and replace'", a)
	}
	if got := f.Children(); len(got) != 11 {
		t.Errorf("Children count = %d, want 11", len(got))
	}
}

func TestFindReplaceFormatCount(t *testing.T) {
	cases := []struct {
		total, current int
		invalid        bool
		want           string
	}{
		{12, 2, false, "3 of 12"},
		{5, -1, false, "1 of 5"}, // current < 0 clamps to the first
		{5, 9, false, "5 of 5"},  // current past the end clamps to the last
		{0, 0, false, "No results"},
		{-1, 0, false, "No results"},
		{3, 1, true, "Bad pattern"}, // invalid wins over any tally
	}
	for _, c := range cases {
		if got := formatCount(c.total, c.current, c.invalid); got != c.want {
			t.Errorf("formatCount(%d,%d,%v) = %q, want %q", c.total, c.current, c.invalid, got, c.want)
		}
	}
}

func TestFindReplaceSetMatchesAndInvalid(t *testing.T) {
	f := NewFindReplace()
	f.SetMatches(12, 2)
	if f.CountText() != "3 of 12" {
		t.Errorf("CountText after SetMatches = %q, want '3 of 12'", f.CountText())
	}
	if f.Invalid().Get() {
		t.Error("SetMatches should clear the invalid flag")
	}
	// Invalid zeroes the tally and shows the bad-pattern readout.
	f.SetInvalid(true)
	if !f.Invalid().Get() || f.Total().Get() != 0 || f.Current().Get() != -1 {
		t.Errorf("after SetInvalid(true): invalid=%v total=%d current=%d", f.Invalid().Get(), f.Total().Get(), f.Current().Get())
	}
	if f.CountText() != "Bad pattern" {
		t.Errorf("CountText = %q, want 'Bad pattern'", f.CountText())
	}
	// Clearing invalid, with the tally still zero, reads "No results".
	f.SetInvalid(false)
	if f.CountText() != "No results" {
		t.Errorf("CountText after SetInvalid(false) = %q, want 'No results'", f.CountText())
	}
}

func TestFindReplaceOpenClose(t *testing.T) {
	f := NewFindReplace()
	f.Open()
	if !f.Visible().Get() {
		t.Fatal("Open did not show the bar")
	}
	if !f.query.Focused() || f.replace.Focused() {
		t.Error("Open should focus the query field, not the replacement")
	}
	f.Close()
	if f.Visible().Get() {
		t.Error("Close did not hide the bar")
	}

	// The ✕ button hides the bar AND fires OnClose.
	closed := 0
	f.OnClose = func() { closed++ }
	f.Open()
	layoutFR(f, 600, 200)
	clickFR(f, f.closeBtn)
	if f.Visible().Get() {
		t.Error("close button did not hide the bar")
	}
	if closed != 1 {
		t.Errorf("OnClose fired %d times, want 1", closed)
	}
}

func TestFindReplaceQueryChangeAndToggles(t *testing.T) {
	f := NewFindReplace()
	changes := 0
	f.OnQueryChange = func() { changes++ }

	// A query edit fires the change.
	f.Query().Set("foo")
	if changes != 1 {
		t.Fatalf("query change fired %d, want 1", changes)
	}
	// A programmatic toggle fires it too.
	f.CaseSensitive().Set(true)
	if changes != 2 {
		t.Fatalf("toggle change fired %d total, want 2", changes)
	}

	// Clicking the three toggle buttons flips their mode and fires the change.
	f.Open()
	layoutFR(f, 600, 200)
	clickFR(f, f.regexBtn)
	if f.Regex().Get() {
		t.Error("clicking the regex toggle did not turn it off")
	}
	clickFR(f, f.wordBtn)
	if !f.WholeWord().Get() {
		t.Error("clicking the whole-word toggle did not turn it on")
	}
	clickFR(f, f.caseBtn)
	if f.CaseSensitive().Get() {
		t.Error("clicking the case toggle did not flip it back off")
	}
	if changes < 5 {
		t.Errorf("toggle clicks fired %d changes total, want >= 5", changes)
	}
}

func TestFindReplaceButtonsFireCallbacks(t *testing.T) {
	f := NewFindReplace()
	var prev, next, rep, all int
	f.OnPrev = func() { prev++ }
	f.OnNext = func() { next++ }
	f.OnReplace = func() { rep++ }
	f.OnReplaceAll = func() { all++ }
	f.Open()
	layoutFR(f, 600, 200)

	clickFR(f, f.prevBtn)
	clickFR(f, f.nextBtn)
	clickFR(f, f.replaceBtn)
	clickFR(f, f.replaceAllBtn)
	if prev != 1 || next != 1 || rep != 1 || all != 1 {
		t.Errorf("callback counts = prev%d next%d rep%d all%d, want all 1", prev, next, rep, all)
	}

	// Clicking a field moves focus to it; typing then lands there.
	clickFR(f, f.replace)
	if !f.replace.Focused() || f.query.Focused() {
		t.Error("clicking the replacement field did not focus it")
	}
	clickFR(f, f.query)
	if !f.query.Focused() || f.replace.Focused() {
		t.Error("clicking the query field did not focus it")
	}

	// A click on the panel padding (no control there) is ignored — no callback,
	// no panic.
	pr := f.panelRect()
	f.OnEvent(Event{Kind: EventClick, X: pr.X + 1, Y: pr.Y + 1})
	if prev != 1 || next != 1 {
		t.Error("a click on empty panel padding triggered a control")
	}
}

func TestFindReplaceKeyboard(t *testing.T) {
	f := NewFindReplace()
	var next, prev, closed int
	f.OnNext = func() { next++ }
	f.OnPrev = func() { prev++ }
	f.OnClose = func() { closed++ }
	f.Open()

	// Typing goes to the focused query field; Backspace (the default key path)
	// trims it.
	f.OnEvent(Event{Kind: EventChar, Code: "a"})
	f.OnEvent(Event{Kind: EventChar, Code: "b"})
	if f.Query().Get() != "ab" {
		t.Fatalf("query after typing = %q, want 'ab'", f.Query().Get())
	}
	f.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if f.Query().Get() != "a" {
		t.Errorf("query after Backspace = %q, want 'a'", f.Query().Get())
	}

	// Enter / Shift+Enter step the match.
	f.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	f.OnEvent(Event{Kind: EventKeyDown, Code: "Enter", Shift: true})
	if next != 1 || prev != 1 {
		t.Errorf("Enter/Shift+Enter fired next=%d prev=%d, want 1/1", next, prev)
	}

	// Typing into the replacement field once it holds focus.
	f.replace.SetFocused(true)
	f.query.SetFocused(false)
	f.OnEvent(Event{Kind: EventChar, Code: "z"})
	if f.Replace().Get() != "z" {
		t.Errorf("replacement after typing = %q, want 'z'", f.Replace().Get())
	}

	// Escape closes the bar and fires OnClose.
	f.OnEvent(Event{Kind: EventKeyDown, Code: "Escape"})
	if f.Visible().Get() || closed != 1 {
		t.Errorf("Escape: visible=%v closed=%d, want hidden + 1", f.Visible().Get(), closed)
	}
}

func TestFindReplaceHiddenIgnoresEvents(t *testing.T) {
	f := NewFindReplace() // hidden
	fired := 0
	f.OnNext = func() { fired++ }
	// A hidden bar swallows events.
	f.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 200})
	f.OnEvent(Event{Kind: EventClick, X: 100, Y: 20})
	f.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if fired != 0 {
		t.Errorf("hidden bar handled events, fired %d", fired)
	}
	// A hidden bar paints nothing.
	const w, h = 600, 200
	surf := makeSurface(w, h)
	bare := makeSurface(w, h)
	f.Draw(newP(surf, w), DefaultLight())
	if !bytes.Equal(surf, bare) {
		t.Error("a hidden bar painted onto the surface")
	}
}

func TestFindReplaceInvalidInk(t *testing.T) {
	f := NewFindReplace()
	f.Open()
	f.SetInvalid(true)
	layoutFR(f, 600, 200)
	if f.countLabel.Ink != formFieldErrorInk {
		t.Errorf("invalid readout ink = %+v, want the error red %+v", f.countLabel.Ink, formFieldErrorInk)
	}
	// A valid state paints the count in the inherited (unset) ink.
	f.SetMatches(2, 0)
	layoutFR(f, 600, 200)
	if f.countLabel.Ink != (RGBA{}) {
		t.Errorf("valid readout ink = %+v, want unset (inherit)", f.countLabel.Ink)
	}
}

func TestFindReplaceNarrowClampsWidth(t *testing.T) {
	f := NewFindReplace()
	f.Open()
	// A bar far narrower than the packed controls: the query/replace field widths
	// would go negative and must clamp to 0 without panicking.
	f.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 200})
	surf := makeSurface(40, 200)
	f.Draw(newP(surf, 40), DefaultLight())
	if f.query.Bounds().W != 0 {
		t.Errorf("query field width = %d in a too-narrow bar, want clamped to 0", f.query.Bounds().W)
	}
}
