// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"

	"github.com/go-icons/iconoir"
	"github.com/go-widgets/painter"
)

// RichEditorToolbar is an accessible widget.
var _ Accessible = (*RichEditorToolbar)(nil)

// RichEditorToolbar is a horizontal strip of icon buttons that drive a bound
// [RichEditor]'s formatting verbs and reflect the formatting in force at its
// caret / selection. It groups the buttons into three logical clusters — inline
// styles, block kinds and lists — with a thin divider between groups:
//
//   - Inline: Bold, Italic, Strikethrough, Code.
//   - Block:  Paragraph, H1, H2, H3, Quote, Code block.
//   - Lists:  Bullet list, Numbered list.
//
// Clicking a button invokes the matching verb on the editor. The toolbar
// subscribes to the editor's Caret / Selection / Doc observables and lights the
// button whose formatting is active (a pressed pill drawn via each button's
// sticky Selected() state): Bold lights when the caret/selection is strong, the
// current block's Paragraph/Heading/Quote/Code button lights, and the list
// button lights while the caret is inside a list of that kind.
//
// All reactive state is MVVM: the pressed/active state lives on each child
// [Button]'s Selected() Observable (which the toolbar drives from the editor
// model), and the editor's own state lives on its Observables. IconSize and
// Spacing are set-once layout config, read once when the buttons are built by
// NewRichEditorToolbar; change them before construction, not after.
type RichEditorToolbar struct {
	Base

	// IconSize is the square edge, in LOGICAL pixels, of each icon button
	// (set-once layout config); it is routed through the HiDPI metric scale (see
	// [SetMetricScale]) at build, so the strip stays crisp at any DPI like every
	// sibling widget. Zero is replaced by [RichEditorToolbarIconSize] at build.
	IconSize int
	// Spacing is the gap, in LOGICAL pixels, between adjacent buttons and around
	// the group dividers (set-once layout config); it is metric-scaled at build.
	// Negative is clamped to 0.
	Spacing int

	ed      *RichEditor
	box     *HBox
	buttons []*Button
	actives []func(*RichEditor) bool
	subs    []func()
}

// Toolbar sizing defaults, in logical pixels.
const (
	// RichEditorToolbarIconSize is the default square edge of each icon button.
	RichEditorToolbarIconSize = 28
	// RichEditorToolbarSpacing is the default inter-button gap.
	RichEditorToolbarSpacing = 2
	// RichEditorToolbarSepW is the width of a group divider cell.
	RichEditorToolbarSepW = 9
)

// reToolbarSpec describes one icon button: the iconoir icon name to draw (empty
// = none, use the text glyph), a short text glyph shown when the icon is absent
// (or as a heading's caption), the verb the click invokes and the predicate that
// decides when the button is lit.
type reToolbarSpec struct {
	icon   string
	glyph  string
	verb   func(*RichEditor)
	active func(*RichEditor) bool
}

// reToolbarGroups is the button layout: three groups (inline, block, lists) with
// a divider drawn between consecutive groups. The iconoir names are verified to
// exist at build time; an absent one falls back to the text glyph.
var reToolbarGroups = [][]reToolbarSpec{
	{
		{icon: "bold", glyph: "B", verb: (*RichEditor).ToggleStrong,
			active: func(e *RichEditor) bool { return e.ActiveInlineStyles().Strong }},
		{icon: "italic", glyph: "I", verb: (*RichEditor).ToggleEmph,
			active: func(e *RichEditor) bool { return e.ActiveInlineStyles().Emph }},
		{icon: "strikethrough", glyph: "S", verb: (*RichEditor).ToggleStrikethrough,
			active: func(e *RichEditor) bool { return e.ActiveInlineStyles().Strikethrough }},
		{icon: "code", glyph: "<>", verb: (*RichEditor).ToggleCode,
			active: func(e *RichEditor) bool { return e.ActiveInlineStyles().Code }},
	},
	{
		{icon: "text", glyph: "P", verb: setBlock(BlockParagraph),
			active: blockIs(BlockParagraph)},
		{icon: "", glyph: "H1", verb: setBlock(BlockH1), active: blockIs(BlockH1)},
		{icon: "", glyph: "H2", verb: setBlock(BlockH2), active: blockIs(BlockH2)},
		{icon: "", glyph: "H3", verb: setBlock(BlockH3), active: blockIs(BlockH3)},
		{icon: "quote", glyph: "\"", verb: setBlock(BlockQuoteKind),
			active: blockIs(BlockQuoteKind)},
		{icon: "code-brackets", glyph: "{}", verb: setBlock(BlockCodeKind),
			active: blockIs(BlockCodeKind)},
	},
	{
		{icon: "list", glyph: "UL", verb: func(e *RichEditor) { e.ToggleList(false) },
			active: listIs(false)},
		{icon: "numbered-list-left", glyph: "OL", verb: func(e *RichEditor) { e.ToggleList(true) },
			active: listIs(true)},
	},
}

// setBlock returns a verb that converts the caret block to kind.
func setBlock(kind BlockKind) func(*RichEditor) {
	return func(e *RichEditor) { e.SetBlockType(kind) }
}

// blockIs returns a predicate true when the caret block's kind equals kind.
func blockIs(kind BlockKind) func(*RichEditor) bool {
	return func(e *RichEditor) bool { return e.CurrentBlockKind() == kind }
}

// listIs returns a predicate true when the caret is inside a list whose
// ordered-ness equals ordered.
func listIs(ordered bool) func(*RichEditor) bool {
	return func(e *RichEditor) bool {
		ord, isList := e.CurrentListOrdered()
		return isList && ord == ordered
	}
}

// NewRichEditorToolbar builds a toolbar bound to ed. A nil ed yields an inert
// toolbar (the buttons render but do nothing and never light). IconSize and
// Spacing take their defaults; set them on a struct literal before calling this
// to override, or use the returned toolbar's zero-config defaults.
func NewRichEditorToolbar(ed *RichEditor) *RichEditorToolbar {
	t := &RichEditorToolbar{
		ed:       ed,
		IconSize: RichEditorToolbarIconSize,
		Spacing:  RichEditorToolbarSpacing,
	}
	t.build()
	return t
}

// iconSize / sepW resolve the effective sizes, applying the defaults + clamps
// and routing the base (logical-pixel) metric through the HiDPI [scaled] seam so
// the toolbar doubles on a 2x panel in lockstep with the RichEditor above it.
func (t *RichEditorToolbar) iconSize() int {
	if t.IconSize > 0 {
		return scaled(t.IconSize)
	}
	return scaled(RichEditorToolbarIconSize)
}

func (t *RichEditorToolbar) sepW() int { return scaled(RichEditorToolbarSepW) }

// spacing is the effective inter-child gap: the Spacing field with negatives
// clamped to 0, then metric-scaled.
func (t *RichEditorToolbar) spacing() int {
	if t.Spacing < 0 {
		return 0
	}
	return scaled(t.Spacing)
}

// iconInset is the margin between a button's edge and its icon glyph, so the
// icon reads as an icon rather than filling the whole cell.
func (t *RichEditorToolbar) iconInset() int { return t.iconSize() / 5 }

// build constructs the child buttons + dividers into an HBox and wires the
// editor subscriptions. Called once from the constructor.
func (t *RichEditorToolbar) build() {
	t.box = NewHBox()
	t.box.Spacing = t.spacing()
	for gi, group := range reToolbarGroups {
		if gi > 0 {
			t.box.AddFixed(&reToolbarSeparator{}, t.sepW())
		}
		for _, spec := range group {
			b := t.makeButton(spec)
			t.buttons = append(t.buttons, b)
			t.actives = append(t.actives, spec.active)
			t.box.AddFixed(b, t.iconSize())
		}
	}
	t.subscribe()
	t.refresh()
}

// makeButton builds one icon button from a spec: an iconoir glyph when the icon
// exists, else the text glyph as a label; the click invokes the verb on the
// bound editor and refreshes the lit state.
func (t *RichEditorToolbar) makeButton(spec reToolbarSpec) *Button {
	b := NewButton(spec.glyph, nil)
	// The active state is the sticky Accent pill; no transient press flash.
	b.PressFeedback = false
	if icon := reToolbarIcon(spec.icon, t.iconInset()); icon != nil {
		b.Icon = icon
	}
	verb := spec.verb
	b.OnClick = func() {
		if t.ed != nil {
			verb(t.ed)
		}
		t.refresh()
	}
	return b
}

// reToolbarIcon returns a Button.Icon painter that draws the named iconoir glyph
// inset by inset pixels, or nil when name is empty or the icon does not exist —
// the caller then falls back to the button's text label.
func reToolbarIcon(name string, inset int) func(p painter.Painter, r Rect, ink RGBA) {
	if name == "" {
		return nil
	}
	if !iconoir.Has(name) {
		return nil
	}
	return func(p painter.Painter, r Rect, ink RGBA) {
		ir := Rect{X: r.X + inset, Y: r.Y + inset, W: r.W - 2*inset, H: r.H - 2*inset}
		DrawIconoir(p, ir, name, ink)
	}
}

// subscribe wires refresh() to the editor's caret / selection / document
// observables so the lit buttons track the caret as the user moves it.
func (t *RichEditorToolbar) subscribe() {
	if t.ed == nil {
		return
	}
	t.subs = append(t.subs,
		t.ed.Caret().SubscribeChanged(t.refresh),
		t.ed.Selection().SubscribeChanged(t.refresh),
		t.ed.Doc().SubscribeChanged(t.refresh),
	)
}

// refresh recomputes every button's lit (Selected) state from the editor.
func (t *RichEditorToolbar) refresh() {
	if t.ed == nil {
		return
	}
	for i, b := range t.buttons {
		b.Selected().Set(t.actives[i](t.ed))
	}
}

// Editor returns the bound editor (nil for an inert toolbar).
func (t *RichEditorToolbar) Editor() *RichEditor { return t.ed }

// A11y reports the strip as a toolbar carrying its button count, so an assistive
// technology announces it as a formatting toolbar; its buttons are exposed as
// children (see [RichEditorToolbar.Children]).
func (t *RichEditorToolbar) A11y() A11yInfo {
	return A11yInfo{Role: RoleToolbar, Value: strconv.Itoa(len(t.buttons)) + " buttons"}
}

// Children yields the button strip's children (buttons + dividers) so a11y and
// focus tree walkers descend into the individual buttons.
func (t *RichEditorToolbar) Children() []Widget { return t.box.Children() }

// Dispose removes the editor subscriptions so a discarded toolbar stops being
// notified. Safe to call more than once.
func (t *RichEditorToolbar) Dispose() {
	for _, un := range t.subs {
		un()
	}
	t.subs = nil
}

// SetBounds positions the toolbar and lays its buttons out left to right.
func (t *RichEditorToolbar) SetBounds(r Rect) {
	t.Base.SetBounds(r)
	t.box.SetBounds(r)
}

// Measure reports the strip's natural size: the summed button + divider widths
// with the inter-child gaps, by the icon-button height. It lets a box layout
// size the toolbar to its content.
func (t *RichEditorToolbar) Measure(availW, availH int) (w, h int) {
	n := len(t.buttons)
	seps := len(reToolbarGroups) - 1
	children := n + seps
	w = n*t.iconSize() + seps*t.sepW()
	if children > 1 {
		w += (children - 1) * t.spacing()
	}
	return w, t.iconSize()
}

// Draw paints the toolbar surface then every button + divider.
func (t *RichEditorToolbar) Draw(p painter.Painter, theme *Theme) {
	r := t.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	t.box.Draw(p, theme)
}

// OnEvent forwards the event to the button strip (which routes clicks to the
// button under the pointer).
func (t *RichEditorToolbar) OnEvent(ev Event) { t.box.OnEvent(ev) }

// reToolbarSeparator is a 1-pixel vertical divider drawn between button groups.
type reToolbarSeparator struct{ Base }

// Draw paints a centred vertical divider inset from the top and bottom edges.
// The stroke width and inset are metric-scaled so the hairline doubles on a 2x
// panel instead of thinning to half its intended weight.
func (s *reToolbarSeparator) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	x := r.X + r.W/2
	inset := scaled(3)
	fillRect(p, x, r.Y+inset, scaled(1), r.H-2*inset, theme.Border)
}
