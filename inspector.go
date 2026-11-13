// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"slices"
	"strconv"
	"strings"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// InspectorKind is the editor an [Inspector] row carries.
type InspectorKind int

// The row kinds.
const (
	// InspectorText is a one-line text field.
	InspectorText InspectorKind = iota
	// InspectorColor is a CSS colour: a swatch, a text field and, opened by the
	// swatch, a [ColorPicker].
	InspectorColor
	// InspectorFont is a font family, through a [FontPicker].
	InspectorFont
	// InspectorNumber is a number with a unit: a spinner and a unit drop-down.
	InspectorNumber
	// InspectorEnum is one of a fixed list, through a [DropDown].
	InspectorEnum
	// InspectorSection is a heading that groups the rows under it; it has no
	// value.
	InspectorSection
)

// InspectorRow describes one row of an [Inspector].
type InspectorRow struct {
	// ID names the row's value; it is the key of Value, Get, Set and OnChange.
	ID string
	// Label is the caption, already in the interface's language.
	Label string
	Kind  InspectorKind
	// Choices are the values of an Enum row, and the families of a Font row.
	Choices []string
	// Units are the units of a Number row ("px", "pt", "%"); the first is the
	// default. Empty means a bare number.
	Units []string
	// Min, Max and Step bound a Number row, in units of 10^-Decimals.
	Min, Max, Step int
	// Decimals is how many decimal digits a Number row shows (0, 1 or 2): the
	// spinner holds integers, so 1.5 with Decimals 1 is the integer 15.
	Decimals int
}

// Inspector is a property grid in the sense of a design tool's inspector: a
// column of labelled rows, each with the editor that fits its kind (colour,
// font, number with unit, enum, text), grouped under section headings. It
// differs from [PropertyGrid], whose Value column is plain text, in that the
// value is edited by a control that cannot produce an invalid one: a colour is
// picked, a unit is chosen from a list.
//
// Every value is a string, the form it takes in a style sheet or a settings
// file ("#1a2b3c", "16px", "Inter", "bold"), so a host maps rows to properties
// without a type per row. Value(id) is the row's value as an [mvvm.Observable]
// (bind it, or subscribe); OnChange is the hook for edits made by the user, so a
// host that writes the value back does not hear its own Set.
//
// A value the row's editor cannot show (a colour written var(--x), a length of
// 1.25em in a row of whole pixels) is KEPT as it is and shown as text: the
// editor never rewrites what it does not understand.
//
// Place an Inspector in a [ScrollView]; it implements [Measurer].
type Inspector struct {
	Base
	// OnChange fires when the user edits row id, with its new value. A host's
	// own Set does not fire it.
	OnChange func(id, value string)
	// LabelFraction is the share of the width the caption column takes, in
	// percent (40 when zero).
	LabelFraction int

	rows  []*inspectorRow
	byID  map[string]*inspectorRow
	quiet bool
}

type inspectorRow struct {
	spec InspectorRow
	val  *mvvm.Observable[string]
	// editors; only those of the row's kind are non-nil
	entry  *Entry
	picker *ColorPicker
	open   bool // a colour row's picker is showing
	font   *FontPicker
	num    *SpinButton
	unit   *DropDown
	enum   *DropDown
	// custom: the value is one the editor cannot show; the raw text is drawn.
	custom bool
	// y/h: the row's rectangle in the inspector (computed by layout)
	y, h int
	// swatch and field rectangles, in surface coordinates
	swatch Rect
}

// NewInspector builds an inspector over rows. Every row starts empty: Set the
// values.
func NewInspector(rows []InspectorRow) *Inspector {
	in := &Inspector{byID: map[string]*inspectorRow{}}
	for _, spec := range rows {
		r := &inspectorRow{spec: spec, val: mvvm.NewObservable("")}
		if spec.Kind != InspectorSection {
			if spec.ID == "" || in.byID[spec.ID] != nil {
				continue // a row without a unique id cannot be addressed
			}
			in.byID[spec.ID] = r
		}
		in.build(r)
		in.rows = append(in.rows, r)
	}
	return in
}

func (in *Inspector) build(r *inspectorRow) {
	s := r.spec
	id := s.ID
	// edited is what every editor calls after the user changed it.
	edited := func(v string) {
		if in.quiet {
			return
		}
		in.hush(func() { r.val.Set(v) })
		r.custom = false
		if in.OnChange != nil {
			in.OnChange(id, v)
		}
	}
	switch s.Kind {
	case InspectorText:
		r.entry = NewEntry("")
		r.entry.Text().Subscribe(func(t string) { edited(t) })
	case InspectorColor:
		r.entry = NewEntry("")
		r.picker = NewColorPicker(RGBA{A: 255})
		r.entry.Text().Subscribe(func(t string) {
			if in.quiet {
				return
			}
			if c, ok := ParseCSSColor(t); ok {
				in.hush(func() { r.picker.SetColor(c) })
				edited(strings.TrimSpace(t))
			}
		})
		r.picker.Color().Subscribe(func(c RGBA) {
			if in.quiet {
				return
			}
			hex := FormatCSSColor(c)
			in.hush(func() { r.entry.SetText(hex) })
			edited(hex)
		})
	case InspectorFont:
		r.font = NewFontPicker(s.Choices)
		r.font.Spec().Subscribe(func(f FontSpec) { edited(f.Family) })
	case InspectorNumber:
		lo, hi, step := s.Min, s.Max, s.Step
		if hi <= lo {
			lo, hi = 0, 10000
		}
		r.num = NewSpinButton(lo, hi, max(lo, 0), max(step, 1))
		units := s.Units
		if len(units) == 0 {
			units = []string{""}
		}
		r.unit = NewDropDown(units, 0)
		enc := func() { edited(in.formatNumber(r)) }
		r.num.Value().Subscribe(func(int) { enc() })
		r.unit.Selected().Subscribe(func(int) { enc() })
	case InspectorEnum:
		r.enum = NewDropDown(slices.Clone(s.Choices), 0)
		r.enum.Selected().Subscribe(func(int) { edited(r.enum.Current()) })
	}
	// a host Set (or an edit) moves the editors to show the value
	r.val.Subscribe(func(v string) {
		if in.quiet {
			return
		}
		in.show(r, v)
	})
}

// formatNumber is the value of a number row from its spinner and unit.
func (in *Inspector) formatNumber(r *inspectorRow) string {
	n, dec := r.num.Value().Get(), r.spec.Decimals
	neg := n < 0
	if neg {
		n = -n
	}
	digits := strconv.Itoa(n)
	if dec > 0 {
		for len(digits) <= dec {
			digits = "0" + digits
		}
		frac := strings.TrimRight(digits[len(digits)-dec:], "0")
		digits = digits[:len(digits)-dec]
		if frac != "" {
			digits += "." + frac
		}
	}
	if neg {
		digits = "-" + digits
	}
	return digits + r.unit.Current()
}

// parseNumber reads "16px" / "1.5" into the spinner's integer and the unit,
// reporting whether the row's editor can show it exactly.
func parseNumber(r *inspectorRow, v string) (n int, unit string, ok bool) {
	v = strings.TrimSpace(v)
	end := len(v)
	for end > 0 && !strings.ContainsRune("0123456789.", rune(v[end-1])) {
		end--
	}
	numText, unit := v[:end], v[end:]
	if numText == "" {
		return 0, "", false
	}
	f, err := strconv.ParseFloat(numText, 64)
	if err != nil {
		return 0, "", false
	}
	scale := 1
	for i := 0; i < r.spec.Decimals; i++ {
		scale *= 10
	}
	n = int(f*float64(scale) + 0.5*sign(f))
	if float64(n)/float64(scale) != f { // a digit the row cannot hold
		return 0, "", false
	}
	if !slices.Contains(r.unit.Options, unit) {
		return 0, "", false
	}
	return n, unit, true
}

func sign(f float64) float64 {
	if f < 0 {
		return -1
	}
	return 1
}

// show moves a row's editors to display v.
func (in *Inspector) show(r *inspectorRow, v string) {
	defer func(prev bool) { in.quiet = prev }(in.quiet)
	in.quiet = true
	r.custom = false
	switch r.spec.Kind {
	case InspectorText:
		r.entry.SetText(v)
	case InspectorColor:
		r.entry.SetText(v)
		if c, ok := ParseCSSColor(v); ok {
			r.picker.SetColor(c)
		} else {
			r.custom = v != ""
		}
	case InspectorFont:
		if v != "" {
			r.font.Spec().Set(FontSpec{Family: v})
		}
	case InspectorNumber:
		if n, unit, ok := parseNumber(r, v); ok {
			r.num.SetValue(n)
			r.unit.Select(slices.Index(r.unit.Options, unit))
			r.custom = r.num.Value().Get() != n // clamped: not the value given
		} else {
			r.custom = v != ""
		}
	case InspectorEnum:
		i := slices.Index(r.enum.Options, v)
		if i < 0 && v != "" {
			r.enum.Options = append(r.enum.Options, v)
			i = len(r.enum.Options) - 1
		}
		if i >= 0 {
			r.enum.Select(i)
		}
	}
}

// Value is row id's value as a shared [mvvm.Observable], or nil for an unknown
// id.
func (in *Inspector) Value(id string) *mvvm.Observable[string] {
	if r := in.byID[id]; r != nil {
		return r.val
	}
	return nil
}

// Get is row id's current value ("" for an unknown id).
func (in *Inspector) Get(id string) string {
	if r := in.byID[id]; r != nil {
		return r.val.Get()
	}
	return ""
}

// Set puts v in row id. It does not fire OnChange. A value the row cannot show
// is kept and drawn as text (see Custom).
func (in *Inspector) Set(id, v string) {
	r := in.byID[id]
	if r == nil {
		return
	}
	// the editors are moved first, so a failed parse can mark the row custom,
	// then the value is recorded verbatim
	in.show(r, v)
	in.hush(func() { r.val.Set(v) })
}

// hush runs f with the editors' echo muted: the editors are being moved to
// follow a value, and that is not an edit.
func (in *Inspector) hush(f func()) {
	prev := in.quiet
	in.quiet = true
	f()
	in.quiet = prev
}

// Custom reports whether row id holds a value its editor cannot show exactly,
// so that it is displayed as text and kept until the user edits the row.
func (in *Inspector) Custom(id string) bool {
	r := in.byID[id]
	return r != nil && r.custom
}

// Rows is the number of rows, headings included.
func (in *Inspector) Rows() int { return len(in.rows) }

// EditorRect is where row id's editor is, in surface coordinates (a colour row's
// swatch included), for a test harness that clicks it; ok is false for an
// unknown id. It is as laid out the last time the inspector was placed or drawn.
func (in *Inspector) EditorRect(id string) (Rect, bool) {
	r := in.byID[id]
	if r == nil {
		return Rect{}, false
	}
	switch r.spec.Kind {
	case InspectorColor:
		sw, e := r.swatch, r.entry.Bounds()
		return Rect{X: sw.X, Y: sw.Y, W: e.X + e.W - sw.X, H: sw.H}, true
	case InspectorFont:
		return r.font.Bounds(), true
	case InspectorNumber:
		n, u := r.num.Bounds(), r.unit.Bounds()
		return Rect{X: n.X, Y: n.Y, W: u.X + u.W - n.X, H: n.H}, true
	case InspectorEnum:
		return r.enum.Bounds(), true
	}
	return r.entry.Bounds(), true
}

// ToggleColor opens or closes the picker of colour row id.
func (in *Inspector) ToggleColor(id string) {
	if r := in.byID[id]; r != nil && r.picker != nil && !in.Disabled().Get() {
		r.open = !r.open
	}
}

// Inspector layout, in logical pixels.
const (
	inspectorPad     = 4
	inspectorSwatchW = 22
	inspectorUnitW   = 52
)

func (in *Inspector) rowH() int { return in.glyphHeight() + scaled(10) }

func (in *Inspector) labelW(w int) int {
	f := in.LabelFraction
	if f <= 0 || f >= 90 {
		f = 40
	}
	return w * f / 100
}

// layout computes every row's rectangle and places its editors. y0 is the
// surface y of the first row.
func (in *Inspector) layout() int {
	b := in.Bounds()
	rh, pad := in.rowH(), scaled(inspectorPad)
	y := b.Y + pad
	lw := in.labelW(b.W)
	ex := b.X + lw + pad
	ew := max(b.X+b.W-pad-ex, 0)
	for _, r := range in.rows {
		r.y, r.h = y-b.Y, rh
		er := Rect{X: ex, Y: y, W: ew, H: rh}
		switch r.spec.Kind {
		case InspectorSection:
			r.h = rh + pad
		case InspectorText:
			r.entry.SetBounds(er)
		case InspectorColor:
			sw := scaled(inspectorSwatchW)
			r.swatch = Rect{X: er.X, Y: er.Y, W: sw, H: rh}
			r.entry.SetBounds(Rect{X: er.X + sw + pad, Y: er.Y, W: max(er.W-sw-pad, 0), H: rh})
			if r.open {
				pw, ph := ColorPickerNaturalSize()
				r.picker.SetBounds(Rect{X: er.X, Y: y + rh + pad, W: pw, H: ph})
				r.h = rh + pad + ph + pad
			}
		case InspectorFont:
			r.font.SetBounds(er)
		case InspectorNumber:
			uw := 0
			if len(r.unit.Options) > 1 || r.unit.Current() != "" {
				uw = scaled(inspectorUnitW)
			}
			r.num.SetBounds(Rect{X: er.X, Y: y, W: max(er.W-uw-pad, 0), H: rh})
			r.unit.SetBounds(Rect{X: er.X + er.W - uw, Y: y, W: uw, H: rh})
		case InspectorEnum:
			r.enum.SetBounds(er)
		}
		y += r.h + pad
	}
	return y - b.Y
}

// Measure is the room the rows need at availW.
func (in *Inspector) Measure(availW, _ int) (int, int) {
	saved := in.Bounds()
	in.Base.SetBounds(Rect{X: saved.X, Y: saved.Y, W: availW, H: saved.H})
	h := in.layout()
	in.Base.SetBounds(saved)
	in.layout()
	return availW, h
}

// SetBounds places the inspector and its rows.
func (in *Inspector) SetBounds(r Rect) {
	in.Base.SetBounds(r)
	in.layout()
}

// editors are the controls of the visible rows, in order.
func (in *Inspector) editors() []Widget {
	var out []Widget
	for _, r := range in.rows {
		switch r.spec.Kind {
		case InspectorText:
			out = append(out, r.entry)
		case InspectorColor:
			out = append(out, r.entry)
		case InspectorFont:
			out = append(out, r.font)
		case InspectorNumber:
			out = append(out, r.num)
			if r.unit.Bounds().W > 0 {
				out = append(out, r.unit)
			}
		case InspectorEnum:
			out = append(out, r.enum)
		}
	}
	return out
}

// Children yields the editors, and the colour pickers that are open, so the
// walks that find an open drop-down list or read the screen reach them.
func (in *Inspector) Children() []Widget {
	out := in.editors()
	for _, r := range in.rows {
		if r.picker != nil && r.open {
			out = append(out, r.picker)
		}
	}
	return out
}

func (in *Inspector) focusableChildren() []Widget { return in.editors() }

// Draw paints the headings, captions and editors.
func (in *Inspector) Draw(p painter.Painter, theme *Theme) {
	in.layout()
	b := in.Bounds()
	pad := scaled(inspectorPad)
	lw := in.labelW(b.W)
	for _, r := range in.rows {
		y := b.Y + r.y
		ty := y + (in.rowH()-in.glyphHeight())/2
		if r.spec.Kind == InspectorSection {
			in.drawText(p, b.X+pad, ty, r.spec.Label, theme.OnBackground)
			fillRect(p, b.X+pad, y+in.rowH()+pad/2, b.W-2*pad, 1, theme.Border)
			continue
		}
		in.drawText(p, b.X+pad, ty, in.fit(r.spec.Label, lw-pad), theme.OnBackground)
		switch r.spec.Kind {
		case InspectorText:
			r.entry.Draw(p, theme)
		case InspectorColor:
			in.drawSwatch(p, theme, r)
			r.entry.Draw(p, theme)
			if r.open {
				r.picker.Draw(p, theme)
			}
		case InspectorFont:
			r.font.Draw(p, theme)
		case InspectorNumber:
			if r.custom {
				in.drawCustom(p, theme, r)
				break
			}
			r.num.Draw(p, theme)
			if r.unit.Bounds().W > 0 {
				r.unit.Draw(p, theme)
			}
		case InspectorEnum:
			r.enum.Draw(p, theme)
		}
	}
}

// drawSwatch paints the colour square; a colour the picker cannot parse shows
// the surface with a diagonal, so it is not mistaken for white.
func (in *Inspector) drawSwatch(p painter.Painter, theme *Theme, r *inspectorRow) {
	s := r.swatch
	if r.custom {
		fillRect(p, s.X, s.Y, s.W, s.H, theme.SurfaceAlt)
		in.drawText(p, s.X+(s.W-in.textWidth("?"))/2, s.Y+(s.H-in.glyphHeight())/2, "?", theme.OnSurface)
	} else {
		fillRect(p, s.X, s.Y, s.W, s.H, r.picker.Color().Get())
	}
	strokeRect(p, s.X, s.Y, s.W, s.H, theme.Border)
}

func (in *Inspector) drawCustom(p painter.Painter, theme *Theme, r *inspectorRow) {
	b := r.num.Bounds()
	w := r.unit.Bounds().X + r.unit.Bounds().W - b.X
	fillRect(p, b.X, b.Y, w, b.H, theme.SurfaceAlt)
	strokeRect(p, b.X, b.Y, w, b.H, theme.Border)
	in.drawText(p, b.X+scaled(inspectorPad), b.Y+(b.H-in.glyphHeight())/2, in.fit(r.val.Get(), w-2*scaled(inspectorPad)), theme.OnSurface)
}

// fit cuts s to w pixels, with an ellipsis.
func (in *Inspector) fit(s string, w int) string {
	if in.textWidth(s) <= w {
		return s
	}
	rs := []rune(s)
	for len(rs) > 0 && in.textWidth(string(rs)+"…") > w {
		rs = rs[:len(rs)-1]
	}
	return string(rs) + "…"
}

// OnEvent gives keys to the focused editor, a click on a swatch to the picker
// toggle, and any other click to the editor under it.
func (in *Inspector) OnEvent(ev Event) {
	if in.Disabled().Get() {
		return
	}
	if routeFocusKey(in, ev) {
		return
	}
	b := in.Bounds()
	sx, sy := ev.X+b.X, ev.Y+b.Y
	if ev.Kind == EventClick {
		focusClick(in, sx, sy)
		for _, r := range in.rows {
			if r.picker != nil && r.swatch.Contains(sx, sy) {
				r.open = !r.open
				in.layout()
				return
			}
		}
	}
	// an open drop-down list belongs to the popover host; here the click goes
	// to whichever control it lands on
	for _, w := range in.Children() {
		if cb := w.Bounds(); cb.W > 0 && cb.H > 0 && w.HitTest(sx, sy) {
			w.OnEvent(translateEvent(ev, b, cb))
			return
		}
	}
}

// A11y reports the inspector as a group.
func (in *Inspector) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Disabled: in.Disabled().Get()}
}
