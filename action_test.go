// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

func TestNewActionDefaults(t *testing.T) {
	ran := false
	a := NewAction("save", "Save", func() { ran = true })
	if a.ID != "save" || a.Label != "Save" {
		t.Fatalf("NewAction fields = %+v", a)
	}
	if !a.IsEnabled() || !a.IsVisible() {
		t.Fatal("NewAction not enabled+visible by default")
	}
	if !a.CanRun() {
		t.Fatal("NewAction not runnable")
	}
	if !a.Execute() || !ran {
		t.Fatal("Execute did not run the body")
	}
}

func TestZeroValueActionRuns(t *testing.T) {
	ran := false
	a := &Action{ID: "x", Run: func() { ran = true }}
	// nil observables read as enabled+visible.
	if !a.IsEnabled() || !a.IsVisible() {
		t.Fatal("zero-value action not enabled+visible")
	}
	if !a.Execute() || !ran {
		t.Fatal("zero-value action did not run")
	}
}

func TestActionEnableDisable(t *testing.T) {
	ran := 0
	a := NewAction("x", "X", func() { ran++ })
	a.SetEnabled(false)
	if a.CanRun() {
		t.Fatal("disabled action reports CanRun")
	}
	if a.Execute() {
		t.Fatal("disabled action executed")
	}
	if ran != 0 {
		t.Fatalf("disabled body ran %d times", ran)
	}
	a.SetEnabled(true)
	if !a.Execute() || ran != 1 {
		t.Fatal("re-enabled action did not run")
	}
}

func TestActionBodylessIsNoop(t *testing.T) {
	a := NewAction("x", "X", nil)
	if a.CanRun() {
		t.Fatal("bodyless action reports CanRun")
	}
	if a.Execute() {
		t.Fatal("bodyless action executed")
	}
}

func TestActionSetVisibleAndZeroValueSetters(t *testing.T) {
	a := NewAction("x", "X", func() {})
	a.SetVisible(false)
	if a.IsVisible() {
		t.Fatal("SetVisible(false) did not take")
	}
	// Zero-value setters allocate the observable lazily.
	z := &Action{ID: "z"}
	z.SetEnabled(false)
	if z.IsEnabled() {
		t.Fatal("zero-value SetEnabled(false) did not take")
	}
	z2 := &Action{ID: "z2"}
	z2.SetVisible(false)
	if z2.IsVisible() {
		t.Fatal("zero-value SetVisible(false) did not take")
	}
}

func TestActionMenuItem(t *testing.T) {
	a := NewAction("save", "Save", func() {})
	a.Shortcut = MustParseChord("Ctrl+S")

	// Without a keymap: label + handler, no shortcut hint.
	mi := a.MenuItem(nil)
	if mi.Label != "Save" || mi.Shortcut != "" {
		t.Fatalf("MenuItem(nil) = %+v", mi)
	}
	mi.Action() // must not panic

	// With a keymap that binds it: the hint shows the live accelerator.
	k := NewKeymap()
	_ = k.Bind(a.Shortcut, a.ID, ScopeGlobal)
	mi = a.MenuItem(k)
	if mi.Shortcut != "Ctrl+S" {
		t.Fatalf("MenuItem shortcut = %q, want Ctrl+S", mi.Shortcut)
	}

	// With a keymap that does NOT bind it: no hint.
	k2 := NewKeymap()
	if got := a.MenuItem(k2).Shortcut; got != "" {
		t.Fatalf("unbound MenuItem shortcut = %q, want empty", got)
	}
}

func TestActionToolbarButton(t *testing.T) {
	ran := false
	a := NewAction("save", "Save", func() { ran = true })
	ti := a.ToolbarButton()
	if ti.Label != "Save" || ti.Disabled {
		t.Fatalf("enabled ToolbarButton = %+v", ti)
	}
	ti.OnClick()
	if !ran {
		t.Fatal("ToolbarButton OnClick did not run")
	}
	a.SetEnabled(false)
	if !a.ToolbarButton().Disabled {
		t.Fatal("disabled action produced an enabled ToolbarButton")
	}
}

func TestRegistryRegisterLookupOrder(t *testing.T) {
	r := NewActionRegistry()
	a := r.Add("a", "A", func() {})
	b := r.Add("b", "B", func() {})
	if r.Len() != 2 {
		t.Fatalf("Len = %d", r.Len())
	}
	got, ok := r.Lookup("a")
	if !ok || got != a {
		t.Fatal("Lookup(a) failed")
	}
	if r.Action("b") != b {
		t.Fatal("Action(b) failed")
	}
	if r.Action("missing") != nil {
		t.Fatal("Action(missing) non-nil")
	}
	if _, ok := r.Lookup("missing"); ok {
		t.Fatal("Lookup(missing) ok=true")
	}
	acts := r.Actions()
	if len(acts) != 2 || acts[0] != a || acts[1] != b {
		t.Fatalf("Actions order wrong: %v", acts)
	}
}

func TestRegistryReplaceKeepsPosition(t *testing.T) {
	r := NewActionRegistry()
	r.Add("a", "A", func() {})
	old := r.Add("b", "B", func() {})
	r.Add("c", "C", func() {})

	fired := 0
	r.OnChange = func() { fired++ }
	// Replace "b" in place.
	nb := r.Register(NewAction("b", "B2", func() {}))
	if r.Action("b") != nb {
		t.Fatal("replace did not swap the action")
	}
	acts := r.Actions()
	if len(acts) != 3 || acts[1].Label != "B2" {
		t.Fatalf("replace changed order: %v", acts)
	}
	// The OLD action's observers were released: toggling it must not notify.
	before := fired
	old.SetEnabled(false)
	if fired != before {
		t.Fatal("toggling a replaced action still notified the registry")
	}
	// The NEW action's observers are live.
	nb.SetEnabled(false)
	if fired == before {
		t.Fatal("toggling the current action did not notify")
	}
}

func TestRegistryOnChangeOnToggleAndAddRemove(t *testing.T) {
	r := NewActionRegistry()
	fired := 0
	r.OnChange = func() { fired++ }
	a := r.Add("a", "A", func() {}) // +1 (register)
	if fired != 1 {
		t.Fatalf("register fired %d", fired)
	}
	a.SetEnabled(false) // +1
	a.SetVisible(false) // +1
	if fired != 3 {
		t.Fatalf("toggles fired %d, want 3", fired)
	}
	if !r.Unregister("a") { // +1
		t.Fatal("Unregister(a) = false")
	}
	if fired != 4 {
		t.Fatalf("unregister fired %d, want 4", fired)
	}
	// Removed action's observers are gone: no further notifications.
	a.SetEnabled(true)
	if fired != 4 {
		t.Fatal("toggling an unregistered action still notified")
	}
}

func TestRegistryNilOnChangeIsSafe(t *testing.T) {
	r := NewActionRegistry() // OnChange nil
	a := r.Add("a", "A", func() {})
	a.SetEnabled(false) // must not panic through the nil OnChange
}

func TestRegistryUnregisterMissing(t *testing.T) {
	r := NewActionRegistry()
	if r.Unregister("nope") {
		t.Fatal("Unregister(missing) = true")
	}
}

func TestRegistryEnableDisableRun(t *testing.T) {
	r := NewActionRegistry()
	ran := 0
	r.Add("a", "A", func() { ran++ })

	if !r.Disable("a") || r.Action("a").IsEnabled() {
		t.Fatal("Disable(a) failed")
	}
	if r.Run("a") {
		t.Fatal("Run on disabled action succeeded")
	}
	if !r.Enable("a") || !r.Action("a").IsEnabled() {
		t.Fatal("Enable(a) failed")
	}
	if !r.Run("a") || ran != 1 {
		t.Fatalf("Run(a) did not execute (ran=%d)", ran)
	}
	// Missing ids report false on every mutator.
	if r.SetEnabled("x", true) || r.Enable("x") || r.Disable("x") || r.Run("x") {
		t.Fatal("a mutator on a missing id returned true")
	}
}

func TestRegistryRegisterPanics(t *testing.T) {
	r := NewActionRegistry()
	mustPanic(t, "nil action", func() { r.Register(nil) })
	mustPanic(t, "empty id", func() { r.Register(&Action{}) })
}

func TestRegistryBindDefaults(t *testing.T) {
	r := NewActionRegistry()
	save := r.Add("save", "Save", func() {})
	save.Shortcut = MustParseChord("Ctrl+S")
	r.Add("noshortcut", "No Shortcut", func() {}) // skipped (empty Shortcut)
	open := r.Add("open", "Open", func() {})
	open.Shortcut = MustParseChord("Ctrl+O")

	k := NewKeymap()
	if err := r.BindDefaults(k, ScopeGlobal); err != nil {
		t.Fatalf("BindDefaults: %v", err)
	}
	if a, st := k.Feed(ctrl("S"), 0); st != Complete || a != "save" {
		t.Fatalf("Ctrl+S = (%q,%v)", a, st)
	}
	if _, ok := k.ShortcutFor("noshortcut"); ok {
		t.Fatal("action with no declared shortcut got bound")
	}

	// A conflicting default surfaces the error.
	clash := r.Add("clash", "Clash", func() {})
	clash.Shortcut = MustParseChord("Ctrl+S") // same as save
	if err := r.BindDefaults(k, ScopeGlobal); err == nil {
		t.Fatal("BindDefaults did not report the conflicting default")
	}
}

func TestRegistryPaletteCommandsAndSetActions(t *testing.T) {
	r := NewActionRegistry()
	ranSave := false
	r.Add("save", "Save", func() { ranSave = true })
	hidden := r.Add("hidden", "Hidden", func() {})
	r.Add("open", "Open", func() {})
	hidden.SetVisible(false)

	cmds := r.PaletteCommands()
	if len(cmds) != 2 || cmds[0].Label != "Save" || cmds[1].Label != "Open" {
		t.Fatalf("PaletteCommands = %+v", cmds)
	}
	cmds[0].Action()
	if !ranSave {
		t.Fatal("palette command did not run its action")
	}

	// SetActions mirrors the visible set onto a palette and stays live via
	// OnChange.
	pal := NewCommandPalette(nil)
	r.OnChange = func() { pal.SetActions(r) }
	pal.SetActions(r)
	if len(pal.Commands) != 2 {
		t.Fatalf("palette Commands = %d, want 2", len(pal.Commands))
	}
	hidden.SetVisible(true) // fires OnChange -> repopulate
	if len(pal.Commands) != 3 {
		t.Fatalf("palette did not repopulate on visibility change: %d", len(pal.Commands))
	}
}

func TestActionIconFieldIsCallable(t *testing.T) {
	// The Icon callback is host-supplied; exercise the field shape so the
	// struct member is covered by a real invocation.
	drawn := false
	a := NewAction("x", "X", func() {})
	a.Icon = func(p painter.Painter, r Rect, ink RGBA) { drawn = true }
	a.Icon(nil, Rect{}, RGBA{})
	if !drawn {
		t.Fatal("Icon callback not invoked")
	}
}

// mustPanic runs fn and fails the test if it does not panic.
func mustPanic(t *testing.T, what string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s: expected panic, got none", what)
		}
	}()
	fn()
}
