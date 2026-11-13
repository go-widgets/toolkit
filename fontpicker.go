// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"slices"
	"strconv"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// FontSpec is what a [FontPicker] edits: a family name, a CSS weight (100..900)
// and a size in pixels. Weight 0 and Size 0 mean "not chosen" and are what a
// picker without a weight or a size control reports.
type FontSpec struct {
	Family string
	Weight int
	Size   int
}

// DefaultFontWeights are the weights a picker offers unless told otherwise.
var DefaultFontWeights = []int{300, 400, 500, 600, 700, 800}

// FontPicker chooses a font by name: a drop-down of families, optionally a
// drop-down of weights and a pixel size, as one row. It is the picker for fonts
// that are NAMES (a CSS family, a font-file family of a document), where
// [FontChooser] lists bitmap fonts of the toolkit's own and draws each in its
// own face. The two answer different questions and do not overlap: FontChooser
// hands back a [Font] to draw with, FontPicker hands back a [FontSpec] to write
// down.
//
// A family the list does not hold is never dropped: Set (or AddFamily) puts it
// in the list, so loading a document that names a font the host did not
// foresee shows that font instead of silently replacing it.
type FontPicker struct {
	Base
	// Families is the set-once list of choices; add to it with AddFamily.
	Families []string
	// Weights are the weights offered (DefaultFontWeights when empty).
	Weights []int
	// ShowWeight and ShowSize switch the weight drop-down and the size spinner
	// on; a family-only picker leaves both off.
	ShowWeight, ShowSize bool
	// SizeMin and SizeMax bound the size spinner (6 and 400 when both zero).
	SizeMin, SizeMax int

	family *DropDown
	weight *DropDown
	size   *SpinButton
	spec   *mvvm.Observable[FontSpec]
	quiet  bool // a part is being moved to follow spec: do not echo it back
}

// NewFontPicker builds a family-only picker over families, showing the first.
func NewFontPicker(families []string) *FontPicker {
	f := &FontPicker{Families: slices.Clone(families), Weights: slices.Clone(DefaultFontWeights)}
	f.build(FontSpec{})
	return f
}

// NewFontPickerFull builds a picker with the weight drop-down and the size
// spinner as well, seeded from initial.
func NewFontPickerFull(families []string, initial FontSpec) *FontPicker {
	f := &FontPicker{Families: slices.Clone(families), Weights: slices.Clone(DefaultFontWeights),
		ShowWeight: true, ShowSize: true}
	f.build(initial)
	return f
}

func (f *FontPicker) build(initial FontSpec) {
	if initial.Family == "" && len(f.Families) > 0 {
		initial.Family = f.Families[0]
	}
	if initial.Family != "" && !slices.Contains(f.Families, initial.Family) {
		f.Families = append(f.Families, initial.Family)
	}
	f.family = NewDropDown(f.Families, max(slices.Index(f.Families, initial.Family), 0))
	wopts := make([]string, len(f.Weights))
	for i, w := range f.Weights {
		wopts[i] = strconv.Itoa(w)
	}
	if f.SizeMin == 0 && f.SizeMax == 0 {
		f.SizeMin, f.SizeMax = 6, 400
	}
	f.weight = NewDropDown(wopts, max(slices.Index(f.Weights, initial.Weight), 0))
	sz := initial.Size
	if sz == 0 {
		sz = 16
	}
	f.size = NewSpinButton(f.SizeMin, f.SizeMax, sz, 1)
	f.spec = mvvm.NewObservable(f.read())
	f.family.Selected().Subscribe(func(int) { f.fromParts() })
	f.weight.Selected().Subscribe(func(int) { f.fromParts() })
	f.size.Value().Subscribe(func(int) { f.fromParts() })
	f.spec.Subscribe(func(s FontSpec) { f.toParts(s) })
}

// read is the spec the parts currently show.
func (f *FontPicker) read() FontSpec {
	s := FontSpec{Family: f.family.Current()}
	if f.ShowWeight {
		if i := f.weight.Selected().Get(); i >= 0 && i < len(f.Weights) {
			s.Weight = f.Weights[i]
		}
	}
	if f.ShowSize {
		s.Size = f.size.Value().Get()
	}
	return s
}

func (f *FontPicker) fromParts() {
	if f.quiet {
		return
	}
	// the parts already show what they show: the spec's own echo must not move
	// them again
	f.quiet = true
	f.spec.Set(f.read())
	f.quiet = false
}

// toParts moves the parts to show s, adding an unknown family or weight rather
// than losing it.
func (f *FontPicker) toParts(s FontSpec) {
	if f.quiet {
		return
	}
	f.quiet = true
	defer func() { f.quiet = false }()
	if s.Family != "" {
		f.AddFamily(s.Family)
		f.family.Select(slices.Index(f.Families, s.Family))
	}
	if f.ShowWeight && s.Weight > 0 {
		i := slices.Index(f.Weights, s.Weight)
		if i < 0 {
			f.Weights = append(f.Weights, s.Weight)
			slices.Sort(f.Weights)
			opts := make([]string, len(f.Weights))
			for j, w := range f.Weights {
				opts[j] = strconv.Itoa(w)
			}
			f.weight.Options = opts
			i = slices.Index(f.Weights, s.Weight)
		}
		f.weight.Select(i)
	}
	if f.ShowSize && s.Size > 0 {
		f.size.SetValue(s.Size)
	}
}

// Spec is the chosen font as a shared [mvvm.Observable]: a host binds it two
// way. A user's choice Sets it; a host Set moves the parts.
func (f *FontPicker) Spec() *mvvm.Observable[FontSpec] { return f.spec }

// AddFamily appends a family to the list if it is not there (a font file the
// user just added, say). It does not change the choice.
func (f *FontPicker) AddFamily(name string) {
	if name == "" || slices.Contains(f.Families, name) {
		return
	}
	f.Families = append(f.Families, name)
	f.family.Options = f.Families
}

// parts are the visible controls, in order.
func (f *FontPicker) parts() []Widget {
	out := []Widget{f.family}
	if f.ShowWeight {
		out = append(out, f.weight)
	}
	if f.ShowSize {
		out = append(out, f.size)
	}
	return out
}

// Children yields the visible controls (so a generic walk, and the popover walk
// that finds the open family list, reach them).
func (f *FontPicker) Children() []Widget { return f.parts() }

func (f *FontPicker) focusableChildren() []Widget { return f.parts() }

// Font picker widths, in logical pixels.
const (
	fontPickerWeightW = 64
	fontPickerSizeW   = 72
	fontPickerGap     = 4
)

// SetBounds lays the controls out in one row: the family takes what the weight
// and size leave.
func (f *FontPicker) SetBounds(r Rect) {
	f.Base.SetBounds(r)
	x, right := r.X, r.X+r.W
	if f.ShowSize {
		w := scaled(fontPickerSizeW)
		f.size.SetBounds(Rect{X: right - w, Y: r.Y, W: w, H: r.H})
		right -= w + scaled(fontPickerGap)
	}
	if f.ShowWeight {
		w := scaled(fontPickerWeightW)
		f.weight.SetBounds(Rect{X: right - w, Y: r.Y, W: w, H: r.H})
		right -= w + scaled(fontPickerGap)
	}
	f.family.SetBounds(Rect{X: x, Y: r.Y, W: max(right-x, 0), H: r.H})
}

// Draw paints the controls.
func (f *FontPicker) Draw(p painter.Painter, theme *Theme) {
	f.SetBounds(f.Bounds())
	for _, w := range f.parts() {
		w.Draw(p, theme)
	}
}

// OnEvent gives keys to the focused control and a click to the control under it.
func (f *FontPicker) OnEvent(ev Event) {
	if f.Disabled().Get() {
		return
	}
	if routeFocusKey(f, ev) {
		return
	}
	pr := f.Bounds()
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	if ev.Kind == EventClick {
		focusClick(f, sx, sy)
	}
	for _, w := range f.parts() {
		if b := w.Bounds(); b.W > 0 && b.H > 0 && w.HitTest(sx, sy) {
			w.OnEvent(translateEvent(ev, pr, b))
			return
		}
	}
}

// A11y reports the picker as a group; its controls name themselves.
func (f *FontPicker) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Name: f.family.Current(), Disabled: f.Disabled().Get()}
}
