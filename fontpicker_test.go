// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

func TestFontPickerKeepsAnUnknownFamily(t *testing.T) {
	f := NewFontPickerFull([]string{"Inter", "Fira Code"}, FontSpec{Family: "Inter", Weight: 400, Size: 18})
	if got := f.Spec().Get(); got != (FontSpec{Family: "Inter", Weight: 400, Size: 18}) {
		t.Fatalf("initial = %+v", got)
	}
	var heard []FontSpec
	f.Spec().Subscribe(func(s FontSpec) { heard = append(heard, s) })

	f.Spec().Set(FontSpec{Family: "Acme Sans", Weight: 650, Size: 20}) // neither listed
	if !contains(f.Families, "Acme Sans") {
		t.Error("an unlisted family must be added, not replaced")
	}
	if f.family.Current() != "Acme Sans" || f.weight.Current() != "650" || f.size.Value().Get() != 20 {
		t.Errorf("parts = %q %q %d", f.family.Current(), f.weight.Current(), f.size.Value().Get())
	}
	if len(heard) != 1 {
		t.Errorf("one host Set must notify once, got %d: %+v", len(heard), heard)
	}

	f.family.Select(0) // the user picks another family: the spec follows
	if got := f.Spec().Get().Family; got != "Inter" {
		t.Errorf("after the user chose Inter, spec family = %q", got)
	}
	f.size.SetValue(1000) // clamped to SizeMax
	if got := f.Spec().Get().Size; got != 400 {
		t.Errorf("size clamps to SizeMax: %d", got)
	}
	f.AddFamily("") // ignored
	f.AddFamily("Inter")
	if n := len(f.Families); n != 3 {
		t.Errorf("families = %v", f.Families)
	}
}

func TestFontPickerFamilyOnlyReportsNoWeightOrSize(t *testing.T) {
	f := NewFontPicker([]string{"A", "B"})
	f.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 24})
	if got := f.Spec().Get(); got != (FontSpec{Family: "A"}) {
		t.Fatalf("spec = %+v", got)
	}
	if n := len(f.Children()); n != 1 {
		t.Fatalf("a family-only picker has one control, got %d", n)
	}
	f.Draw(newP(makeSurface(200, 24), 200), DefaultLight())
	f.OnEvent(Event{Kind: EventClick, X: 10, Y: 10}) // opens the family list
	if !f.family.Open().Get() {
		t.Error("a click on the family opens its list")
	}
	if got := PopoverOwners(f); len(got) != 1 {
		t.Errorf("the open list must be reachable by the popover walk, got %d", len(got))
	}
	if f.A11y().Name != "A" {
		t.Error("the group is named by its family")
	}
}

func TestFontPickerLayoutOfAllThreeControls(t *testing.T) {
	f := NewFontPickerFull([]string{"A"}, FontSpec{Weight: 700})
	f.SetBounds(Rect{X: 10, Y: 5, W: 300, H: 24})
	fam, wt, sz := f.family.Bounds(), f.weight.Bounds(), f.size.Bounds()
	if !(fam.X == 10 && fam.X+fam.W <= wt.X && wt.X+wt.W <= sz.X && sz.X+sz.W == 310) {
		t.Errorf("one row, family first: %v %v %v", fam, wt, sz)
	}
	f.SetBounds(Rect{W: 40, H: 24}) // narrower than the weight and size: the family shrinks to zero, never negative
	if f.family.Bounds().W < 0 {
		t.Error("negative width")
	}
	if f.Spec().Get().Weight != 700 {
		t.Errorf("weight = %d", f.Spec().Get().Weight)
	}
}
