// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/mvvm"
)

func TestButtonA11yReportsDisabled(t *testing.T) {
	b := NewButton("x", nil)
	if b.A11y().Disabled {
		t.Error("an enabled button is announced disabled")
	}
	b.Disabled().Set(true)
	if !b.A11y().Disabled {
		t.Error("a disabled button is not announced disabled")
	}
}

// emptyChooser is a ForgeChooser with no forge to offer yet.
type emptyChooser struct{ ForgeModel }

func (emptyChooser) Forges() []string             { return nil }
func (emptyChooser) Forge() *mvvm.Observable[int] { return mvvm.NewObservable(0) }

func TestForgeChooserWithNoForgesShowsAGreyedDropDown(t *testing.T) {
	m := newFakeForge()
	f := laidOutForge(emptyChooser{m}, 300, 1200)
	defer f.Close()
	r, ok := f.Parts()["forge"]
	if !ok || r.W <= 0 || r.H <= 0 {
		t.Fatalf("a chooser present in a not-yet-right state is shown: %v %v", r, ok)
	}
	d := f.client.forge
	if !d.Disabled().Get() {
		t.Fatal("the empty drop-down is not greyed")
	}
	if !d.A11y().Disabled {
		t.Error("a screen reader is not told the drop-down is greyed")
	}
	// no gesture can open it or select an index that does not exist
	d.OnEvent(Event{Kind: EventClick})
	for _, k := range []string{"ArrowDown", "ArrowUp", "Enter", " "} {
		d.OnEvent(Event{Kind: EventKeyDown, Code: k})
	}
	if d.Open().Get() || d.Selected().Get() != 0 || d.PopoverBounds().H != 0 || d.Current() != "" {
		t.Errorf("open %v selected %d popover %v current %q", d.Open().Get(), d.Selected().Get(), d.PopoverBounds(), d.Current())
	}
	if d.PopoverClick(r.X, r.Y+r.H+1) {
		t.Error("a closed empty drop-down consumed a click")
	}
	d.Select(0)
	d.Select(-1)
	if d.Selected().Get() != 0 || d.Open().Get() {
		t.Error("Select reached a nonexistent index")
	}
	f.Draw(newP(makeSurface(300, 1200), 300), DefaultLight()) // never panics

	// a chooser with forges is not greyed
	g := NewForgePane(m)
	defer g.Close()
	if g.client.forge.Disabled().Get() {
		t.Error("a drop-down with forges is greyed")
	}
}
