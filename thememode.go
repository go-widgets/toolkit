// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"sort"
	"sync"
)

// A ThemeFamily is a theme in its two variants: what a Dark/Light/System choice
// picks between.
type ThemeFamily struct {
	Name        string
	Light, Dark func() *Theme
}

var (
	themeFamiliesMu sync.RWMutex
	themeFamilies   = map[string]ThemeFamily{}
)

func init() {
	RegisterThemeFamily(ThemeFamily{Name: "default", Light: DefaultLight, Dark: DefaultDark})
	RegisterThemeFamily(ThemeFamily{Name: "adwaita", Light: AdwaitaLight, Dark: AdwaitaDark})
	RegisterThemeFamily(ThemeFamily{Name: "fluent", Light: FluentLight, Dark: FluentDark})
	RegisterThemeFamily(ThemeFamily{Name: "whitesur", Light: WhiteSurLight, Dark: WhiteSurDark})
}

// RegisterThemeFamily adds f to the registry, replacing a family of the same
// name (an application registers its own, such as a Slate with its brand). It
// panics on an empty name or a missing variant: both are programming errors.
func RegisterThemeFamily(f ThemeFamily) {
	if f.Name == "" || f.Light == nil || f.Dark == nil {
		panic("toolkit: a ThemeFamily needs a Name, a Light and a Dark")
	}
	themeFamiliesMu.Lock()
	themeFamilies[f.Name] = f
	themeFamiliesMu.Unlock()
}

// ThemeFamilyNamed is the registered family called name.
func ThemeFamilyNamed(name string) (ThemeFamily, bool) {
	themeFamiliesMu.RLock()
	defer themeFamiliesMu.RUnlock()
	f, ok := themeFamilies[name]
	return f, ok
}

// ThemeFamilies are the registered families, sorted by name.
func ThemeFamilies() []ThemeFamily {
	themeFamiliesMu.RLock()
	out := make([]ThemeFamily, 0, len(themeFamilies))
	for _, f := range themeFamilies {
		out = append(out, f)
	}
	themeFamiliesMu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ThemeMode is the viewer's choice between the two variants of a family.
type ThemeMode int

const (
	// ThemeSystem follows the system's preference (prefers-color-scheme in a
	// browser), live. It is the zero value: the default.
	ThemeSystem ThemeMode = iota
	ThemeLight
	ThemeDark
)

// ThemeModes are the three choices, in the order a selector lists them.
var ThemeModes = []ThemeMode{ThemeDark, ThemeLight, ThemeSystem}

// String is "system", "light" or "dark": the form that is stored.
func (m ThemeMode) String() string {
	switch m {
	case ThemeLight:
		return "light"
	case ThemeDark:
		return "dark"
	}
	return "system"
}

// ParseThemeMode reads what String wrote; ok is false for anything else.
func ParseThemeMode(s string) (ThemeMode, bool) {
	for _, m := range ThemeModes {
		if m.String() == s {
			return m, true
		}
	}
	return ThemeSystem, false
}

// A ColorScheme is where the system's light/dark preference comes from: a
// browser host answers with matchMedia("(prefers-color-scheme: dark)"), a test
// with a fake.
type ColorScheme interface {
	// Dark reports whether the system prefers dark now.
	Dark() bool
	// Watch calls f with the new preference each time it changes, until stop.
	Watch(f func(dark bool)) (stop func())
}

// A ThemeManager holds the family and the mode, and answers with the theme in
// effect. In ThemeSystem it follows its ColorScheme (Follow) and says so to
// its subscribers when the preference changes, so a host repaints without a
// reload. It is safe for use from several goroutines; subscribers are called
// outside its lock.
type ThemeManager struct {
	mu         sync.Mutex
	family     ThemeFamily
	mode       ThemeMode
	systemDark bool
	stop       func()
	subs       map[int]func()
	next       int
}

// NewThemeManager is a manager of family in ThemeSystem mode, system light
// until Follow or SetSystemDark says otherwise.
func NewThemeManager(family ThemeFamily) *ThemeManager {
	return &ThemeManager{family: family, subs: map[int]func(){}}
}

// OnChange registers f, called after the mode, the family or the effective
// variant changed; the result unsubscribes.
func (m *ThemeManager) OnChange(f func()) (unsubscribe func()) {
	m.mu.Lock()
	id := m.next
	m.next++
	m.subs[id] = f
	m.mu.Unlock()
	return func() {
		m.mu.Lock()
		delete(m.subs, id)
		m.mu.Unlock()
	}
}

// notify calls the subscribers, in registration order, with the lock released.
func (m *ThemeManager) notify() {
	m.mu.Lock()
	ids := make([]int, 0, len(m.subs))
	for id := range m.subs {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	fs := make([]func(), len(ids))
	for i, id := range ids {
		fs[i] = m.subs[id]
	}
	m.mu.Unlock()
	for _, f := range fs {
		f()
	}
}

// Mode is the viewer's choice.
func (m *ThemeManager) Mode() ThemeMode {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mode
}

// SetMode changes the choice and tells the subscribers when it is a change.
func (m *ThemeManager) SetMode(mode ThemeMode) {
	m.mu.Lock()
	changed := m.mode != mode
	m.mode = mode
	m.mu.Unlock()
	if changed {
		m.notify()
	}
}

// SetFamily changes the family and tells the subscribers when it is a change.
func (m *ThemeManager) SetFamily(f ThemeFamily) {
	m.mu.Lock()
	changed := m.family.Name != f.Name
	m.family = f
	m.mu.Unlock()
	if changed {
		m.notify()
	}
}

// Family is the family in use.
func (m *ThemeManager) Family() ThemeFamily {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.family
}

// SetSystemDark records the system's preference. Subscribers are told only if
// it changes what is shown (the mode is ThemeSystem).
func (m *ThemeManager) SetSystemDark(dark bool) {
	m.mu.Lock()
	changed := m.systemDark != dark && m.mode == ThemeSystem
	m.systemDark = dark
	m.mu.Unlock()
	if changed {
		m.notify()
	}
}

// Follow reads cs now and subscribes to its changes (replacing a previous
// Follow), so ThemeSystem tracks the system live. Close ends it.
func (m *ThemeManager) Follow(cs ColorScheme) {
	m.Close()
	stop := cs.Watch(m.SetSystemDark)
	m.mu.Lock()
	m.stop = stop
	m.mu.Unlock()
	m.SetSystemDark(cs.Dark())
}

// Close stops following the system's preference.
func (m *ThemeManager) Close() {
	m.mu.Lock()
	stop := m.stop
	m.stop = nil
	m.mu.Unlock()
	if stop != nil {
		stop()
	}
}

// Dark reports whether the variant in effect is the dark one.
func (m *ThemeManager) Dark() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.darkLocked()
}

func (m *ThemeManager) darkLocked() bool {
	switch m.mode {
	case ThemeDark:
		return true
	case ThemeLight:
		return false
	}
	return m.systemDark
}

// Theme builds the theme in effect: the family's dark or light variant.
func (m *ThemeManager) Theme() *Theme {
	m.mu.Lock()
	f, dark := m.family, m.darkLocked()
	m.mu.Unlock()
	if dark {
		return f.Dark()
	}
	return f.Light()
}
