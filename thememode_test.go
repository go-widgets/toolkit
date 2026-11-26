// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"sync"
	"testing"
)

type fakeScheme struct {
	mu      sync.Mutex
	dark    bool
	watcher func(bool)
	stopped int
}

func (f *fakeScheme) Dark() bool { return f.dark }
func (f *fakeScheme) Watch(fn func(bool)) func() {
	f.watcher = fn
	return func() { f.stopped++ }
}
func (f *fakeScheme) set(dark bool) { f.dark = dark; f.watcher(dark) }

func TestThemeRegistry(t *testing.T) {
	names := map[string]bool{}
	for _, f := range ThemeFamilies() {
		names[f.Name] = true
	}
	for _, n := range []string{"default", "adwaita", "fluent", "whitesur"} {
		if !names[n] {
			t.Errorf("family %q is not registered", n)
		}
	}
	fs := ThemeFamilies()
	for i := 1; i < len(fs); i++ {
		if fs[i-1].Name >= fs[i].Name {
			t.Fatalf("not sorted: %s, %s", fs[i-1].Name, fs[i].Name)
		}
	}
	f, ok := ThemeFamilyNamed("default")
	if !ok || f.Light().Background == f.Dark().Background {
		t.Fatalf("default family: ok=%v", ok)
	}
	if _, ok := ThemeFamilyNamed("nope"); ok {
		t.Fatal("unknown family found")
	}
	RegisterThemeFamily(ThemeFamily{Name: "zz-test", Light: DefaultLight, Dark: DefaultDark})
	if _, ok := ThemeFamilyNamed("zz-test"); !ok {
		t.Fatal("registered family not found")
	}
	for _, bad := range []ThemeFamily{{}, {Name: "x", Light: DefaultLight}, {Name: "x", Dark: DefaultDark}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%+v should panic", bad)
				}
			}()
			RegisterThemeFamily(bad)
		}()
	}
}

func TestThemeModeStrings(t *testing.T) {
	for _, m := range ThemeModes {
		got, ok := ParseThemeMode(m.String())
		if !ok || got != m {
			t.Errorf("%v does not round-trip: %v %v", m, got, ok)
		}
	}
	if ThemeMode(0) != ThemeSystem {
		t.Fatal("the zero mode must be System")
	}
	if _, ok := ParseThemeMode("blue"); ok {
		t.Fatal("garbage parsed")
	}
	if m, _ := ParseThemeMode(""); m != ThemeSystem {
		t.Fatal("garbage must read as System")
	}
	if ThemeModes[0] != ThemeDark || ThemeModes[1] != ThemeLight || ThemeModes[2] != ThemeSystem {
		t.Fatalf("selector order: %v", ThemeModes)
	}
}

func TestThemeManagerModes(t *testing.T) {
	f, _ := ThemeFamilyNamed("default")
	m := NewThemeManager(f)
	n := 0
	unsub := m.OnChange(func() { n++ })
	if m.Mode() != ThemeSystem || m.Dark() || m.Theme().Background != f.Light().Background {
		t.Fatal("starts System over a light system")
	}
	m.SetMode(ThemeDark)
	if !m.Dark() || m.Theme().Background != f.Dark().Background || n != 1 {
		t.Fatalf("Dark: dark=%v n=%d", m.Dark(), n)
	}
	m.SetMode(ThemeDark) // no change, no notice
	m.SetSystemDark(false)
	m.SetSystemDark(true) // the system moves while the mode is Dark: nothing shown changes
	if n != 1 {
		t.Fatalf("a system change under a fixed mode must be silent: n=%d", n)
	}
	m.SetMode(ThemeLight)
	if m.Dark() || n != 2 {
		t.Fatalf("Light: dark=%v n=%d", m.Dark(), n)
	}
	m.SetMode(ThemeSystem)
	if !m.Dark() || n != 3 { // the system said dark above
		t.Fatalf("System reads the recorded preference: dark=%v n=%d", m.Dark(), n)
	}
	m.SetSystemDark(false)
	if m.Dark() || n != 4 {
		t.Fatalf("System follows: dark=%v n=%d", m.Dark(), n)
	}
	m.SetFamily(f) // same family: silent
	a, _ := ThemeFamilyNamed("adwaita")
	m.SetFamily(a)
	if m.Family().Name != "adwaita" || m.Theme().Background != a.Light().Background || n != 5 {
		t.Fatalf("family: %s n=%d", m.Family().Name, n)
	}
	unsub()
	m.SetMode(ThemeDark)
	if n != 5 {
		t.Fatal("an unsubscribed callback was called")
	}
}

func TestThemeManagerFollowsTheSystemLive(t *testing.T) {
	f, _ := ThemeFamilyNamed("default")
	m := NewThemeManager(f)
	cs := &fakeScheme{dark: true}
	n := 0
	m.OnChange(func() { n++ })
	m.Follow(cs)
	if !m.Dark() || n != 1 {
		t.Fatalf("Follow reads the current preference: dark=%v n=%d", m.Dark(), n)
	}
	cs.set(false)
	if m.Dark() || n != 2 {
		t.Fatalf("live change: dark=%v n=%d", m.Dark(), n)
	}
	cs.set(true)
	if !m.Dark() || n != 3 {
		t.Fatalf("live change back: dark=%v n=%d", m.Dark(), n)
	}
	m.Follow(cs) // replaces the earlier watch
	if cs.stopped != 1 {
		t.Fatalf("the previous watch must be stopped: %d", cs.stopped)
	}
	m.Close()
	m.Close() // idempotent
	if cs.stopped != 2 {
		t.Fatalf("Close stops the watch once: %d", cs.stopped)
	}
}

func TestThemeManagerSubscribersMayCallBack(t *testing.T) {
	f, _ := ThemeFamilyNamed("default")
	m := NewThemeManager(f)
	var order []int
	m.OnChange(func() { order = append(order, 1); _ = m.Dark(); _ = m.Theme() }) // no deadlock
	m.OnChange(func() { order = append(order, 2) })
	m.SetMode(ThemeDark)
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("registration order: %v", order)
	}
}
