// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"errors"
	"fmt"
)

// Scope is where a key binding applies, and therefore its resolution priority.
// A binding on a more specific scope shadows a less specific one that shares
// the same chord: the focused widget's bindings win over the window's, which
// win over the application-global ones. This is what lets a text field bind
// Ctrl+A to "select all" while the app keeps Ctrl+A as "select all items"
// elsewhere — same chord, different active scope.
type Scope int

const (
	// ScopeGlobal applies application-wide and is ALWAYS active during
	// resolution regardless of the active mask; it has the lowest priority.
	ScopeGlobal Scope = iota
	// ScopeWindow applies while a particular window/view is focused; it
	// overrides ScopeGlobal.
	ScopeWindow
	// ScopeWidget applies while a particular widget is focused; it has the
	// highest priority and overrides both ScopeWindow and ScopeGlobal.
	ScopeWidget
)

// String returns the scope's name for hints and diagnostics.
func (s Scope) String() string {
	switch s {
	case ScopeGlobal:
		return "global"
	case ScopeWindow:
		return "window"
	case ScopeWidget:
		return "widget"
	default:
		return fmt.Sprintf("Scope(%d)", int(s))
	}
}

// ScopeMask is the set of scopes that are active for one resolution — the
// window and/or widget contexts currently focused. ScopeGlobal is implicitly
// always active, so a zero mask still resolves global bindings.
type ScopeMask uint8

const (
	// MaskGlobal marks ScopeGlobal active (implied by every resolution).
	MaskGlobal ScopeMask = 1 << ScopeGlobal
	// MaskWindow marks ScopeWindow active.
	MaskWindow ScopeMask = 1 << ScopeWindow
	// MaskWidget marks ScopeWidget active.
	MaskWidget ScopeMask = 1 << ScopeWidget
)

// ActiveScopes builds a [ScopeMask] from the given scopes, always including
// ScopeGlobal so global bindings resolve even when no window/widget context is
// supplied.
func ActiveScopes(scopes ...Scope) ScopeMask {
	m := MaskGlobal
	for _, s := range scopes {
		m |= 1 << uint(s)
	}
	return m
}

// has reports whether scope s is active in the mask, with ScopeGlobal always
// active.
func (m ScopeMask) has(s Scope) bool {
	if s == ScopeGlobal {
		return true
	}
	return m&(1<<uint(s)) != 0
}

// Binding pairs a chord with the action it triggers and the scope it applies
// in. Returned by [Keymap.Bindings] as an immutable snapshot.
type Binding struct {
	Chord  Chord
	Action string
	Scope  Scope
}

// MatchState is the outcome of feeding one keystroke to a [Keymap].
type MatchState int

const (
	// NoMatch means the keystroke completed no binding and continues no
	// pending chord; the chord state is reset.
	NoMatch MatchState = iota
	// Partial means the keystroke is a valid prefix of one or more longer
	// chords; the [Keymap] is now awaiting the next stroke.
	Partial
	// Complete means the keystroke completed a binding; the returned action
	// id should be run and the chord state is reset.
	Complete
)

// String returns the state's name for diagnostics.
func (s MatchState) String() string {
	switch s {
	case NoMatch:
		return "no-match"
	case Partial:
		return "partial"
	case Complete:
		return "complete"
	default:
		return fmt.Sprintf("MatchState(%d)", int(s))
	}
}

// ErrConflict is returned by [Keymap.Bind] / [Keymap.Rebind] when the target
// chord is already bound to a different action in the same scope.
var ErrConflict = errors.New("toolkit: chord already bound in this scope")

// ErrEmptyChord is returned when a bind is attempted with a zero-length chord.
var ErrEmptyChord = errors.New("toolkit: empty chord")

// ErrEmptyAction is returned when a bind is attempted with an empty action id.
var ErrEmptyAction = errors.New("toolkit: empty action id")

// Keymap maps chords to action ids across scopes, resolves keystrokes to
// actions one stroke at a time (so multi-stroke chords work), detects binding
// conflicts, and supports live rebinding. It holds no reference to an
// [ActionRegistry]: it stores action ids, and the caller runs the resolved id
// against a registry. This keeps the binding layer independent of the actions
// themselves — the same map can be inspected, serialised or rebound without
// the actions existing yet.
//
// A Keymap is not safe for concurrent use; drive it from the UI goroutine.
type Keymap struct {
	bindings []Binding
	pending  Chord

	// OnChange, when set, fires after any mutation to the bindings
	// (Bind/Rebind/Unbind), so a menu or palette can refresh the shortcut
	// hints it shows for its actions.
	OnChange func()
}

// NewKeymap returns an empty [Keymap].
func NewKeymap() *Keymap { return &Keymap{} }

// changed fires OnChange if set.
func (k *Keymap) changed() {
	if k.OnChange != nil {
		k.OnChange()
	}
}

// Bind binds chord to action in scope. Binding the same chord to the same
// action in the same scope is idempotent; binding it to a DIFFERENT action in
// the same scope returns [ErrConflict] (the existing binding is left intact).
// Binding the same chord in a different scope is always allowed — that is how
// a widget-scope binding shadows a global one.
func (k *Keymap) Bind(chord Chord, action string, scope Scope) error {
	if len(chord) == 0 {
		return ErrEmptyChord
	}
	if action == "" {
		return ErrEmptyAction
	}
	for _, b := range k.bindings {
		if b.Scope == scope && b.Chord.equal(chord) {
			if b.Action == action {
				return nil
			}
			return fmt.Errorf("%w: %s in %s already bound to %q", ErrConflict, chord, scope, b.Action)
		}
	}
	k.bindings = append(k.bindings, Binding{Chord: cloneChord(chord), Action: action, Scope: scope})
	k.changed()
	return nil
}

// Rebind changes the chord bound to action in scope, live: it removes every
// existing binding for that action in that scope and installs the new chord.
// If the new chord is already bound to a DIFFERENT action in the same scope it
// returns [ErrConflict] and makes no change. This is the hot-rebinding entry
// point a "customise shortcuts" UI calls.
func (k *Keymap) Rebind(action string, chord Chord, scope Scope) error {
	if len(chord) == 0 {
		return ErrEmptyChord
	}
	if action == "" {
		return ErrEmptyAction
	}
	for _, b := range k.bindings {
		if b.Scope == scope && b.Chord.equal(chord) && b.Action != action {
			return fmt.Errorf("%w: %s in %s already bound to %q", ErrConflict, chord, scope, b.Action)
		}
	}
	out := k.bindings[:0:0]
	for _, b := range k.bindings {
		if b.Scope == scope && b.Action == action {
			continue
		}
		out = append(out, b)
	}
	out = append(out, Binding{Chord: cloneChord(chord), Action: action, Scope: scope})
	k.bindings = out
	k.changed()
	return nil
}

// Unbind removes the binding for chord in scope, returning whether one was
// found and removed.
func (k *Keymap) Unbind(chord Chord, scope Scope) bool {
	for i, b := range k.bindings {
		if b.Scope == scope && b.Chord.equal(chord) {
			k.bindings = append(k.bindings[:i], k.bindings[i+1:]...)
			k.changed()
			return true
		}
	}
	return false
}

// UnbindAction removes every binding for action across all scopes, returning
// how many were removed.
func (k *Keymap) UnbindAction(action string) int {
	n := 0
	out := k.bindings[:0:0]
	for _, b := range k.bindings {
		if b.Action == action {
			n++
			continue
		}
		out = append(out, b)
	}
	if n > 0 {
		k.bindings = out
		k.changed()
	}
	return n
}

// Conflict reports the action already bound to chord in scope, if any — the
// query a rebinding dialog runs to warn "this key is already used by X" before
// committing. conflict is false when the chord is free in that scope.
func (k *Keymap) Conflict(chord Chord, scope Scope) (action string, conflict bool) {
	for _, b := range k.bindings {
		if b.Scope == scope && b.Chord.equal(chord) {
			return b.Action, true
		}
	}
	return "", false
}

// ShortcutFor returns the chord bound to action at its most specific scope
// (widget over window over global), for a menu or palette to display as the
// action's current shortcut. It reflects live rebinds. ok is false when the
// action has no binding.
func (k *Keymap) ShortcutFor(action string) (Chord, bool) {
	best := -1
	var chord Chord
	for _, b := range k.bindings {
		if b.Action == action && int(b.Scope) > best {
			best = int(b.Scope)
			chord = b.Chord
		}
	}
	if best < 0 {
		return nil, false
	}
	return cloneChord(chord), true
}

// Bindings returns an independent snapshot of every binding in registration
// order (chords deep-copied), for inspection or serialisation.
func (k *Keymap) Bindings() []Binding {
	out := make([]Binding, len(k.bindings))
	for i, b := range k.bindings {
		out[i] = Binding{Chord: cloneChord(b.Chord), Action: b.Action, Scope: b.Scope}
	}
	return out
}

// Pending returns the chord typed so far but not yet resolved — the prefix a
// status line shows as "waiting for the next key". It is empty except between
// the strokes of a multi-stroke chord.
func (k *Keymap) Pending() Chord { return cloneChord(k.pending) }

// Reset clears any pending chord, abandoning a half-typed multi-stroke binding
// (e.g. on focus loss or Escape).
func (k *Keymap) Reset() { k.pending = nil }

// Feed resolves one input event against the active scopes, tracking
// multi-stroke chords across calls:
//
//   - Complete: the event finished a binding; the returned id is the action to
//     run and the pending chord is cleared.
//   - Partial: the event is a valid prefix of a longer chord; "" is returned
//     and the pending chord is retained for the next Feed.
//   - NoMatch: the event matched nothing; the pending chord is cleared. If a
//     chord was in progress it is abandoned and the event is retried as a fresh
//     first stroke, so a stray key mid-chord can itself begin a new binding.
//
// Non-keyboard events return NoMatch without disturbing the pending chord.
// ScopeGlobal is always considered active; the mask adds window/widget scopes.
func (k *Keymap) Feed(ev Event, active ScopeMask) (action string, state MatchState) {
	acc, ok := AcceleratorFromEvent(ev)
	if !ok {
		return "", NoMatch
	}
	active |= MaskGlobal
	cand := append(cloneChord(k.pending), acc)
	action, state = k.match(cand, active)
	switch state {
	case Complete:
		k.pending = nil
		return action, Complete
	case Partial:
		k.pending = cand
		return "", Partial
	default:
		if len(k.pending) > 0 {
			// Abandon the in-progress chord and retry this key on its own.
			k.pending = nil
			fresh := Chord{acc}
			action, state = k.match(fresh, active)
			switch state {
			case Complete:
				return action, Complete
			case Partial:
				k.pending = fresh
				return "", Partial
			}
		}
		k.pending = nil
		return "", NoMatch
	}
}

// match resolves a candidate chord against the active bindings. An exact match
// wins immediately (fired on completion), choosing the highest-priority active
// scope; otherwise, if the candidate is a prefix of any active binding it is
// Partial; otherwise NoMatch.
func (k *Keymap) match(cand Chord, active ScopeMask) (string, MatchState) {
	bestScope := -1
	var bestAction string
	exact, prefix := false, false
	for _, b := range k.bindings {
		if !active.has(b.Scope) {
			continue
		}
		switch {
		case b.Chord.equal(cand):
			exact = true
			if int(b.Scope) > bestScope {
				bestScope = int(b.Scope)
				bestAction = b.Action
			}
		case b.Chord.hasPrefix(cand):
			prefix = true
		}
	}
	if exact {
		return bestAction, Complete
	}
	if prefix {
		return "", Partial
	}
	return "", NoMatch
}
