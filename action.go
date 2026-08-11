// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Action is a single named command — the one source of truth that a menu item,
// a toolbar button, a command-palette entry and a keyboard shortcut all point
// at. Define it once, then surface it in every affordance via the converters
// ([Action.MenuItem], [Action.ToolbarButton]) and register it with an
// [ActionRegistry] (which feeds a [CommandPalette]) and a [Keymap] (which binds
// its accelerator); flipping [Action.Enabled] or [Action.Visible] then updates
// every surface at once.
//
// Enabled and Visible are [mvvm.Observable] values so app state can drive them
// through the go-widgets MVVM layer — an Observable[bool] on a view model binds
// straight onto them — and any surface observing them repaints on change.
// [NewAction] initialises both to true; the fields also default to a usable nil
// (treated as "enabled"/"visible") so a zero-value literal still runs.
type Action struct {
	// ID is the stable identifier a [Keymap] binds and an [ActionRegistry]
	// keys on. It must be non-empty to register.
	ID string
	// Label is the human-readable text shown in menus, buttons and the
	// palette.
	Label string
	// Icon, when set, draws the action's glyph into rect r in colour ink —
	// the same callback shape as [Button.Icon]. Optional (nil draws no
	// glyph).
	Icon func(p painter.Painter, r Rect, ink RGBA)
	// Shortcut is the action's declared default accelerator (single- or
	// multi-stroke). Register it into a [Keymap] with
	// [ActionRegistry.BindDefaults] or [Keymap.Bind]; the Keymap, not this
	// field, is the live source of truth once rebinding is allowed.
	Shortcut Chord
	// Run is the command body, invoked by [Action.Execute] when the action
	// is enabled. Nil makes the action a non-running placeholder.
	Run func()

	// Enabled gates whether the action runs and reads as active; Visible
	// gates whether surfaces show it at all. Both are observable.
	Enabled *mvvm.Observable[bool]
	Visible *mvvm.Observable[bool]
}

// NewAction builds an enabled, visible action with the given id, label and
// command body.
func NewAction(id, label string, run func()) *Action {
	return &Action{
		ID:      id,
		Label:   label,
		Run:     run,
		Enabled: mvvm.NewObservable(true),
		Visible: mvvm.NewObservable(true),
	}
}

// normalize lazily allocates the Enabled/Visible observables (defaulting true)
// so a zero-value Action gains real observables the moment a registry needs to
// subscribe to them.
func (a *Action) normalize() {
	if a.Enabled == nil {
		a.Enabled = mvvm.NewObservable(true)
	}
	if a.Visible == nil {
		a.Visible = mvvm.NewObservable(true)
	}
}

// IsEnabled reports the action's enabled state, treating a nil observable as
// enabled.
func (a *Action) IsEnabled() bool { return a.Enabled == nil || a.Enabled.Get() }

// IsVisible reports the action's visible state, treating a nil observable as
// visible.
func (a *Action) IsVisible() bool { return a.Visible == nil || a.Visible.Get() }

// SetEnabled sets the enabled state, allocating the observable if needed.
func (a *Action) SetEnabled(b bool) {
	a.normalize()
	a.Enabled.Set(b)
}

// SetVisible sets the visible state, allocating the observable if needed.
func (a *Action) SetVisible(b bool) {
	a.normalize()
	a.Visible.Set(b)
}

// CanRun reports whether Execute would run: the action has a body and is
// enabled.
func (a *Action) CanRun() bool { return a.Run != nil && a.IsEnabled() }

// Execute runs the action's body if [Action.CanRun], returning whether it ran.
// A disabled or bodyless action is a safe no-op — every surface routes through
// this guard, so a stale menu click or palette pick on a just-disabled action
// does nothing.
func (a *Action) Execute() bool {
	if !a.CanRun() {
		return false
	}
	a.Run()
	return true
}

// MenuItem projects the action onto a [MenuItem], filling Label, an Execute
// handler and — when keymap is non-nil and the action is bound — the shortcut
// hint from the keymap's most specific binding, so the menu always shows the
// live accelerator.
func (a *Action) MenuItem(keymap *Keymap) MenuItem {
	mi := MenuItem{Label: a.Label, Action: func() { a.Execute() }}
	if keymap != nil {
		if ch, ok := keymap.ShortcutFor(a.ID); ok {
			mi.Shortcut = ch.String()
		}
	}
	return mi
}

// ToolbarButton projects the action onto a [ToolbarItem], filling Label, an
// OnClick handler and Disabled from the action's enabled state. (ToolbarItem
// takes a raster Icon, not a draw callback, so [Action.Icon] is not copied
// here; a host that wants the glyph renders it into an Icon buffer itself.)
func (a *Action) ToolbarButton() ToolbarItem {
	return ToolbarItem{Label: a.Label, OnClick: func() { a.Execute() }, Disabled: !a.IsEnabled()}
}

// ActionRegistry is the lookup + iteration index for [Action]s: one place to
// register a command, find it by id, toggle it, run it, and enumerate the set
// (in registration order) to build a menu, a toolbar or a [CommandPalette].
//
// It observes each registered action's Enabled/Visible observables and fires
// OnChange when any of them changes (or when an action is added/removed), so a
// palette rebuilt from [ActionRegistry.PaletteCommands] stays current without
// the app polling.
type ActionRegistry struct {
	byID  map[string]*Action
	order []string
	unsub map[string][]func()

	// OnChange, when set, fires when the action set changes or any
	// registered action's Enabled/Visible flips.
	OnChange func()
}

// NewActionRegistry returns an empty registry.
func NewActionRegistry() *ActionRegistry {
	return &ActionRegistry{
		byID:  map[string]*Action{},
		unsub: map[string][]func(){},
	}
}

// notify fires OnChange if set.
func (r *ActionRegistry) notify() {
	if r.OnChange != nil {
		r.OnChange()
	}
}

// releaseSubs unsubscribes and forgets the observers installed for id.
func (r *ActionRegistry) releaseSubs(id string) {
	for _, u := range r.unsub[id] {
		u()
	}
	delete(r.unsub, id)
}

// Register adds (or replaces, by id) an action and returns it for chaining.
// Registering a new id appends it to the iteration order; re-registering an
// existing id keeps its position and swaps the action. The registry subscribes
// to the action's Enabled/Visible observables so later toggles fan out through
// OnChange. Panics on a nil action or an empty ID (programmer errors).
func (r *ActionRegistry) Register(a *Action) *Action {
	if a == nil {
		panic("toolkit: ActionRegistry.Register(nil)")
	}
	if a.ID == "" {
		panic("toolkit: ActionRegistry.Register: empty Action.ID")
	}
	a.normalize()
	if _, exists := r.byID[a.ID]; exists {
		r.releaseSubs(a.ID)
	} else {
		r.order = append(r.order, a.ID)
	}
	r.byID[a.ID] = a
	u1 := a.Enabled.SubscribeChanged(r.notify)
	u2 := a.Visible.SubscribeChanged(r.notify)
	r.unsub[a.ID] = []func(){u1, u2}
	r.notify()
	return a
}

// Add is a convenience that builds an action via [NewAction] and registers it,
// returning the action so the caller can set Icon/Shortcut.
func (r *ActionRegistry) Add(id, label string, run func()) *Action {
	return r.Register(NewAction(id, label, run))
}

// Unregister removes the action with id (unsubscribing its observers),
// returning whether one was present.
func (r *ActionRegistry) Unregister(id string) bool {
	if _, ok := r.byID[id]; !ok {
		return false
	}
	r.releaseSubs(id)
	delete(r.byID, id)
	for i, o := range r.order {
		if o == id {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	r.notify()
	return true
}

// Lookup returns the action for id and whether it was found.
func (r *ActionRegistry) Lookup(id string) (*Action, bool) {
	a, ok := r.byID[id]
	return a, ok
}

// Action returns the action for id, or nil if it is not registered.
func (r *ActionRegistry) Action(id string) *Action { return r.byID[id] }

// Actions returns the registered actions in registration order.
func (r *ActionRegistry) Actions() []*Action {
	out := make([]*Action, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.byID[id])
	}
	return out
}

// Len reports how many actions are registered.
func (r *ActionRegistry) Len() int { return len(r.order) }

// SetEnabled toggles the enabled state of action id, returning whether it was
// found.
func (r *ActionRegistry) SetEnabled(id string, enabled bool) bool {
	a, ok := r.byID[id]
	if !ok {
		return false
	}
	a.SetEnabled(enabled)
	return true
}

// Enable enables action id, returning whether it was found.
func (r *ActionRegistry) Enable(id string) bool { return r.SetEnabled(id, true) }

// Disable disables action id, returning whether it was found.
func (r *ActionRegistry) Disable(id string) bool { return r.SetEnabled(id, false) }

// Run executes action id (through its enabled guard), returning whether it both
// existed and ran.
func (r *ActionRegistry) Run(id string) bool {
	a, ok := r.byID[id]
	if !ok {
		return false
	}
	return a.Execute()
}

// BindDefaults binds every registered action that declares a non-empty
// [Action.Shortcut] into keymap at the given scope, so a single pass wires the
// declared accelerators. It stops and returns the first conflict error
// ([ErrConflict]); on success it returns nil.
func (r *ActionRegistry) BindDefaults(keymap *Keymap, scope Scope) error {
	for _, id := range r.order {
		a := r.byID[id]
		if len(a.Shortcut) == 0 {
			continue
		}
		if err := keymap.Bind(a.Shortcut, a.ID, scope); err != nil {
			return err
		}
	}
	return nil
}

// PaletteCommands projects the visible actions (in registration order) onto
// [PaletteCommand]s for a [CommandPalette]. Each command's handler runs the
// action through its enabled guard, so a disabled-but-visible action shows in
// the palette yet does nothing when chosen. Pair with [CommandPalette.SetActions]
// and the registry's OnChange to keep the palette live.
func (r *ActionRegistry) PaletteCommands() []PaletteCommand {
	var out []PaletteCommand
	for _, id := range r.order {
		a := r.byID[id]
		if !a.IsVisible() {
			continue
		}
		act := a
		out = append(out, PaletteCommand{Label: a.Label, Action: func() { act.Execute() }})
	}
	return out
}

// SetActions replaces the palette's commands with the registry's current
// visible actions. Wire it to the registry's OnChange (c.SetActions(r)) so the
// palette rebuilds whenever an action's visibility flips or the set changes.
func (c *CommandPalette) SetActions(r *ActionRegistry) {
	c.Commands = r.PaletteCommands()
	c.clampSelected()
}
