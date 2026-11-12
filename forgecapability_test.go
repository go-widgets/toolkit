// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// Embedding the ForgeModel INTERFACE promotes only its methods, so these wrap
// the full fake and leave out Dir / Forges / Forge.
type noOptional struct{ ForgeModel }

type chooserOnly struct {
	ForgeModel
	ForgeChooser
}

type locatableOnly struct {
	ForgeModel
	ForgeLocatable
}

func TestForgeFullModelKeepsForgeAndFolder(t *testing.T) {
	m := newFakeForge()
	f := laidOutForge(m, 300, 1200)
	defer f.Close()
	if f.chooser == nil || f.locatable == nil {
		t.Fatal("a model that has the methods is found")
	}
	for _, mode := range []ForgeMode{ForgeClient, ForgeFrame, ForgeServer} {
		m.mode.Set(mode)
		f.Draw(newP(makeSurface(300, 1200), 300), DefaultLight())
		if _, ok := f.Parts()["dir"]; !ok {
			t.Errorf("mode %d lost the folder", mode)
		}
	}
	m.mode.Set(ForgeClient)
	if _, ok := f.Parts()["forge"]; !ok {
		t.Error("the forge drop-down is gone")
	}
	f.client.dir.Text().Set("sub")
	f.client.forge.Selected().Set(1)
	if m.dir.Get() != "sub" || m.forge.Get() != 1 {
		t.Error("the folder and forge are not bound")
	}
}

func TestForgeWithoutChooserHasNoDropDown(t *testing.T) {
	m := newFakeForge()
	full := laidOutForge(m, 300, 1200)
	defer full.Close()
	f := laidOutForge(locatableOnly{m, m}, 300, 1200)
	defer f.Close()
	parts := f.Parts()
	if _, ok := parts["forge"]; ok {
		t.Fatal("a model without ForgeChooser still shows the drop-down")
	}
	if _, ok := parts["dir"]; !ok {
		t.Error("the folder went with it")
	}
	if parts["repo"].Y >= full.Parts()["repo"].Y {
		t.Errorf("the layout did not close up: repo at %d, was %d", parts["repo"].Y, full.Parts()["repo"].Y)
	}
	if got, was := len(WalkA11y(f)), len(WalkA11y(full)); got != was-1 {
		t.Errorf("announced controls: %d, a full model has %d (one fewer expected)", got, was)
	}
}

func TestForgeWithoutLocatableHasNoFolderInAnyLayout(t *testing.T) {
	m := newFakeForge()
	f := laidOutForge(chooserOnly{m, m}, 300, 1200)
	defer f.Close()
	for _, mode := range []ForgeMode{ForgeClient, ForgeFrame, ForgeServer} {
		m.mode.Set(mode)
		f.Draw(newP(makeSurface(300, 1200), 300), DefaultLight())
		if _, ok := f.Parts()["dir"]; ok {
			t.Errorf("mode %d shows a folder the model does not have", mode)
		}
		if _, ok := f.Parts()["ref"]; !ok {
			t.Errorf("mode %d lost the branch", mode)
		}
	}
	m.mode.Set(ForgeClient)
	if _, ok := f.Parts()["forge"]; !ok {
		t.Error("the chooser is present")
	}
}

func TestForgeBareModelHasNeitherAndNothingIsReachable(t *testing.T) {
	m := newFakeForge()
	f := laidOutForge(noOptional{m}, 300, 1200)
	defer f.Close()
	if f.chooser != nil || f.locatable != nil {
		t.Fatal("capabilities invented")
	}
	for _, mode := range []ForgeMode{ForgeClient, ForgeFrame, ForgeServer} {
		m.mode.Set(mode)
		f.Draw(newP(makeSurface(300, 1200), 300), DefaultLight())
		parts := f.Parts()
		for _, k := range []string{"forge", "dir"} {
			if _, ok := parts[k]; ok {
				t.Errorf("mode %d has %q", mode, k)
			}
		}
	}
	// not in the tree: neither is laid out, so nothing to hit, focus or announce
	m.mode.Set(ForgeClient)
	full := laidOutForge(m, 300, 1200)
	defer full.Close()
	if got, was := len(WalkA11y(f)), len(WalkA11y(full)); got != was-2 {
		t.Errorf("announced controls: %d, a full model has %d (two fewer expected)", got, was)
	}
	if r := f.client.dir.Bounds(); r.W != 0 || r.H != 0 {
		t.Errorf("the absent folder has a rectangle %v", r)
	}
}
