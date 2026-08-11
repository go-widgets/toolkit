// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"errors"
	"testing"
)

// keydown builds a plain key-down event for a single key (no modifiers).
func keydown(code string) Event { return Event{Kind: EventKeyDown, Code: code} }

// ctrl builds a Ctrl+<code> key-down event.
func ctrl(code string) Event { return Event{Kind: EventKeyDown, Code: code, Ctrl: true} }

func TestScopeString(t *testing.T) {
	cases := map[Scope]string{
		ScopeGlobal: "global",
		ScopeWindow: "window",
		ScopeWidget: "widget",
		Scope(9):    "Scope(9)",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("Scope(%d).String() = %q, want %q", int(s), got, want)
		}
	}
}

func TestMatchStateString(t *testing.T) {
	cases := map[MatchState]string{
		NoMatch:       "no-match",
		Partial:       "partial",
		Complete:      "complete",
		MatchState(9): "MatchState(9)",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("MatchState.String() = %q, want %q", got, want)
		}
	}
}

func TestActiveScopesAndHas(t *testing.T) {
	m := ActiveScopes(ScopeWidget)
	if !m.has(ScopeGlobal) {
		t.Error("global not implicitly active")
	}
	if !m.has(ScopeWidget) {
		t.Error("widget not active")
	}
	if m.has(ScopeWindow) {
		t.Error("window unexpectedly active")
	}
	// A zero mask still resolves global.
	if !ScopeMask(0).has(ScopeGlobal) {
		t.Error("zero mask does not treat global as active")
	}
}

func TestBindConflictAndIdempotent(t *testing.T) {
	k := NewKeymap()
	save := MustParseChord("Ctrl+S")

	if err := k.Bind(save, "save", ScopeGlobal); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	// Idempotent: same chord, same action, same scope.
	if err := k.Bind(save, "save", ScopeGlobal); err != nil {
		t.Fatalf("idempotent Bind: %v", err)
	}
	if n := len(k.Bindings()); n != 1 {
		t.Fatalf("idempotent Bind grew the map to %d", n)
	}
	// Conflict: same chord+scope, different action.
	err := k.Bind(save, "saveAll", ScopeGlobal)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict Bind = %v, want ErrConflict", err)
	}
	// Same chord in a different scope is allowed (shadowing).
	if err := k.Bind(save, "saveWidget", ScopeWidget); err != nil {
		t.Fatalf("cross-scope Bind: %v", err)
	}
}

func TestBindValidation(t *testing.T) {
	k := NewKeymap()
	if !errors.Is(k.Bind(nil, "x", ScopeGlobal), ErrEmptyChord) {
		t.Error("empty chord not rejected")
	}
	if !errors.Is(k.Bind(MustParseChord("a"), "", ScopeGlobal), ErrEmptyAction) {
		t.Error("empty action not rejected")
	}
}

func TestOnChangeFires(t *testing.T) {
	k := NewKeymap()
	n := 0
	k.OnChange = func() { n++ }
	if err := k.Bind(MustParseChord("a"), "x", ScopeGlobal); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("OnChange fired %d times after Bind, want 1", n)
	}
	// A no-op idempotent bind must not fire.
	_ = k.Bind(MustParseChord("a"), "x", ScopeGlobal)
	if n != 1 {
		t.Fatalf("OnChange fired on idempotent Bind (%d)", n)
	}
}

func TestRebindHot(t *testing.T) {
	k := NewKeymap()
	_ = k.Bind(MustParseChord("Ctrl+S"), "save", ScopeGlobal)
	_ = k.Bind(MustParseChord("Ctrl+O"), "open", ScopeGlobal)

	// Rebind save to Ctrl+W, live.
	if err := k.Rebind("save", MustParseChord("Ctrl+W"), ScopeGlobal); err != nil {
		t.Fatalf("Rebind: %v", err)
	}
	if ch, ok := k.ShortcutFor("save"); !ok || ch.String() != "Ctrl+W" {
		t.Fatalf("after rebind ShortcutFor(save) = %v ok=%v", ch, ok)
	}
	// The old chord no longer resolves save.
	if _, st := k.Feed(ctrl("S"), 0); st != NoMatch {
		t.Errorf("old Ctrl+S still matched after rebind (state=%v)", st)
	}
	// The new chord resolves save.
	if a, st := k.Feed(ctrl("W"), 0); st != Complete || a != "save" {
		t.Errorf("new Ctrl+W = (%q,%v), want (save,complete)", a, st)
	}
	// open is untouched.
	if a, st := k.Feed(ctrl("O"), 0); st != Complete || a != "open" {
		t.Errorf("open = (%q,%v)", a, st)
	}

	// Rebinding onto a chord held by a DIFFERENT action conflicts, no change.
	err := k.Rebind("save", MustParseChord("Ctrl+O"), ScopeGlobal)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting Rebind = %v, want ErrConflict", err)
	}
	if ch, _ := k.ShortcutFor("save"); ch.String() != "Ctrl+W" {
		t.Errorf("save moved despite conflicting rebind: %v", ch)
	}
}

func TestRebindValidation(t *testing.T) {
	k := NewKeymap()
	if !errors.Is(k.Rebind("x", nil, ScopeGlobal), ErrEmptyChord) {
		t.Error("empty chord not rejected")
	}
	if !errors.Is(k.Rebind("", MustParseChord("a"), ScopeGlobal), ErrEmptyAction) {
		t.Error("empty action not rejected")
	}
}

func TestUnbind(t *testing.T) {
	k := NewKeymap()
	_ = k.Bind(MustParseChord("Ctrl+S"), "save", ScopeGlobal)
	if !k.Unbind(MustParseChord("Ctrl+S"), ScopeGlobal) {
		t.Fatal("Unbind returned false for an existing binding")
	}
	if k.Unbind(MustParseChord("Ctrl+S"), ScopeGlobal) {
		t.Fatal("Unbind returned true for a removed binding")
	}
}

func TestUnbindAction(t *testing.T) {
	k := NewKeymap()
	_ = k.Bind(MustParseChord("Ctrl+S"), "save", ScopeGlobal)
	_ = k.Bind(MustParseChord("Ctrl+Shift+S"), "save", ScopeWidget)
	_ = k.Bind(MustParseChord("Ctrl+O"), "open", ScopeGlobal)

	if n := k.UnbindAction("save"); n != 2 {
		t.Fatalf("UnbindAction(save) removed %d, want 2", n)
	}
	if n := k.UnbindAction("save"); n != 0 {
		t.Fatalf("second UnbindAction(save) removed %d, want 0", n)
	}
	if len(k.Bindings()) != 1 {
		t.Fatalf("open binding was collateral-removed")
	}
}

func TestConflictQuery(t *testing.T) {
	k := NewKeymap()
	_ = k.Bind(MustParseChord("Ctrl+S"), "save", ScopeGlobal)
	if a, c := k.Conflict(MustParseChord("Ctrl+S"), ScopeGlobal); !c || a != "save" {
		t.Fatalf("Conflict = (%q,%v), want (save,true)", a, c)
	}
	if _, c := k.Conflict(MustParseChord("Ctrl+S"), ScopeWidget); c {
		t.Error("Conflict reported a cross-scope collision")
	}
	if _, c := k.Conflict(MustParseChord("Ctrl+Q"), ScopeGlobal); c {
		t.Error("Conflict reported a free chord as taken")
	}
}

func TestShortcutForPrefersSpecificScope(t *testing.T) {
	k := NewKeymap()
	if _, ok := k.ShortcutFor("save"); ok {
		t.Error("ShortcutFor found a binding for an unbound action")
	}
	_ = k.Bind(MustParseChord("Ctrl+S"), "save", ScopeGlobal)
	_ = k.Bind(MustParseChord("Ctrl+Shift+S"), "save", ScopeWidget)
	ch, ok := k.ShortcutFor("save")
	if !ok || ch.String() != "Ctrl+Shift+S" {
		t.Fatalf("ShortcutFor(save) = %v, want the widget-scope Ctrl+Shift+S", ch)
	}
}

func TestPendingAndReset(t *testing.T) {
	k := NewKeymap()
	_ = k.Bind(MustParseChord("g d"), "goDefs", ScopeGlobal)
	if _, st := k.Feed(keydown("g"), 0); st != Partial {
		t.Fatalf("g = %v, want Partial", st)
	}
	if p := k.Pending(); p.String() != "G" {
		t.Fatalf("Pending = %q, want G", p.String())
	}
	k.Reset()
	if len(k.Pending()) != 0 {
		t.Fatalf("Reset left pending = %v", k.Pending())
	}
	// After reset, a lone d resolves nothing (chord abandoned).
	if _, st := k.Feed(keydown("d"), 0); st != NoMatch {
		t.Fatalf("d after reset = %v, want NoMatch", st)
	}
}

func TestFeedNonKeyEventIsInert(t *testing.T) {
	k := NewKeymap()
	_ = k.Bind(MustParseChord("g d"), "goDefs", ScopeGlobal)
	_, _ = k.Feed(keydown("g"), 0) // pending = g
	// A mouse move must not disturb the pending chord.
	if a, st := k.Feed(Event{Kind: EventMouseMove}, 0); st != NoMatch || a != "" {
		t.Fatalf("non-key Feed = (%q,%v)", a, st)
	}
	if k.Pending().String() != "G" {
		t.Fatalf("non-key event cleared the pending chord: %v", k.Pending())
	}
	// Completing the chord still works.
	if a, st := k.Feed(keydown("d"), 0); st != Complete || a != "goDefs" {
		t.Fatalf("g,(move),d = (%q,%v), want (goDefs,complete)", a, st)
	}
}

func TestFeedChordComplete(t *testing.T) {
	k := NewKeymap()
	_ = k.Bind(MustParseChord("Ctrl+K Ctrl+S"), "keyboardShortcuts", ScopeGlobal)

	if a, st := k.Feed(ctrl("K"), 0); st != Partial || a != "" {
		t.Fatalf("Ctrl+K = (%q,%v), want partial", a, st)
	}
	if a, st := k.Feed(ctrl("S"), 0); st != Complete || a != "keyboardShortcuts" {
		t.Fatalf("Ctrl+S = (%q,%v), want (keyboardShortcuts,complete)", a, st)
	}
	// Pending cleared after completion.
	if len(k.Pending()) != 0 {
		t.Fatalf("pending not cleared after Complete: %v", k.Pending())
	}
}

func TestFeedSingleKeyComplete(t *testing.T) {
	k := NewKeymap()
	_ = k.Bind(MustParseChord("/"), "search", ScopeGlobal)
	if a, st := k.Feed(Event{Kind: EventChar, Code: "/"}, 0); st != Complete || a != "search" {
		t.Fatalf("/ = (%q,%v), want (search,complete)", a, st)
	}
}

func TestFeedNoMatchWithNoPending(t *testing.T) {
	k := NewKeymap()
	_ = k.Bind(MustParseChord("g d"), "goDefs", ScopeGlobal)
	if a, st := k.Feed(keydown("z"), 0); st != NoMatch || a != "" {
		t.Fatalf("z = (%q,%v), want NoMatch", a, st)
	}
}

func TestFeedMidChordRetriesFresh(t *testing.T) {
	k := NewKeymap()
	_ = k.Bind(MustParseChord("g d"), "goDefs", ScopeGlobal)
	_ = k.Bind(MustParseChord("x"), "cut", ScopeGlobal)
	_ = k.Bind(MustParseChord("g g"), "goTop", ScopeGlobal)

	// g -> partial; then x is not "g x" but IS a fresh binding -> Complete cut.
	_, _ = k.Feed(keydown("g"), 0)
	if a, st := k.Feed(keydown("x"), 0); st != Complete || a != "cut" {
		t.Fatalf("g then x = (%q,%v), want (cut,complete) via fresh retry", a, st)
	}

	// g -> partial; then g again: "g g" completes goTop (the retry is not even
	// needed here — "g g" is a real chord — but exercises another path).
	_, _ = k.Feed(keydown("g"), 0)
	if a, st := k.Feed(keydown("g"), 0); st != Complete || a != "goTop" {
		t.Fatalf("g g = (%q,%v), want (goTop,complete)", a, st)
	}
}

func TestFeedMidChordRetryStartsNewPartial(t *testing.T) {
	k := NewKeymap()
	_ = k.Bind(MustParseChord("g d"), "goDefs", ScopeGlobal)
	_ = k.Bind(MustParseChord("d w"), "delWord", ScopeGlobal)

	// g -> partial; d is not "g d"?  It IS "g d" -> would complete. Use a key
	// that fails "g <k>" but starts its own chord: press g, then d... that
	// completes goDefs. Instead press g then 'd' would complete. So to hit the
	// "retry begins a new Partial" path we need: pending=g, next key k where
	// "g k" is no binding but "k ..." is a chord prefix.
	// Bind e as no "g e" but "e f" chord.
	_ = k.Bind(MustParseChord("e f"), "endFile", ScopeGlobal)
	_, _ = k.Feed(keydown("g"), 0) // pending g
	if a, st := k.Feed(keydown("e"), 0); st != Partial || a != "" {
		t.Fatalf("g then e = (%q,%v), want Partial (fresh chord e f)", a, st)
	}
	if k.Pending().String() != "E" {
		t.Fatalf("pending after fresh-retry Partial = %q, want E", k.Pending().String())
	}
	if a, st := k.Feed(keydown("f"), 0); st != Complete || a != "endFile" {
		t.Fatalf("e f = (%q,%v)", a, st)
	}
}

func TestFeedMidChordRetryNoMatch(t *testing.T) {
	k := NewKeymap()
	_ = k.Bind(MustParseChord("g d"), "goDefs", ScopeGlobal)
	_, _ = k.Feed(keydown("g"), 0)   // pending g
	a, st := k.Feed(keydown("z"), 0) // "g z" no; "z" no -> NoMatch, reset
	if st != NoMatch || a != "" {
		t.Fatalf("g then z = (%q,%v), want NoMatch", a, st)
	}
	if len(k.Pending()) != 0 {
		t.Fatalf("pending not cleared after retry NoMatch: %v", k.Pending())
	}
}

func TestFeedScopePriority(t *testing.T) {
	k := NewKeymap()
	sel := MustParseChord("Ctrl+A")
	_ = k.Bind(sel, "selectAllItems", ScopeGlobal)
	_ = k.Bind(sel, "selectAllText", ScopeWidget)

	// Widget scope inactive: the global binding resolves.
	if a, st := k.Feed(ctrl("A"), 0); st != Complete || a != "selectAllItems" {
		t.Fatalf("Ctrl+A (no widget scope) = (%q,%v), want selectAllItems", a, st)
	}
	// Widget scope active: it shadows the global binding.
	if a, st := k.Feed(ctrl("A"), ActiveScopes(ScopeWidget)); st != Complete || a != "selectAllText" {
		t.Fatalf("Ctrl+A (widget active) = (%q,%v), want selectAllText", a, st)
	}
}

func TestFeedInactiveScopeIsSkipped(t *testing.T) {
	k := NewKeymap()
	// A widget-only binding does not fire when its scope is inactive.
	_ = k.Bind(MustParseChord("Ctrl+D"), "duplicateLine", ScopeWidget)
	if _, st := k.Feed(ctrl("D"), 0); st != NoMatch {
		t.Fatalf("widget-only Ctrl+D fired with no active widget scope (state=%v)", st)
	}
	if a, st := k.Feed(ctrl("D"), ActiveScopes(ScopeWidget)); st != Complete || a != "duplicateLine" {
		t.Fatalf("Ctrl+D (widget active) = (%q,%v)", a, st)
	}
}

func TestBindingsSnapshotIsIndependent(t *testing.T) {
	k := NewKeymap()
	_ = k.Bind(MustParseChord("Ctrl+S"), "save", ScopeGlobal)
	snap := k.Bindings()
	snap[0].Chord[0].Key = "MUTATED"
	if ch, _ := k.ShortcutFor("save"); ch[0].Key != "S" {
		t.Fatalf("mutating the snapshot changed the live binding: %v", ch)
	}
}
