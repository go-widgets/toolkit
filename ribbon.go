// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/mvvm"

// Ribbon is an Office-style command ribbon: a strip of TABS, each holding GROUPS
// of labelled controls — large buttons (icon above the text), small buttons
// (icon beside the text), toggles, split buttons, drop-downs and a small
// gallery — that scales down as the window narrows instead of overflowing.
//
// It follows the ribbon guidelines of Windows 7 ("Windows 7 Ribbons", Microsoft
// Learn, uxguide/cmd-ribbons) for the parts that are about design, and the
// WAI-ARIA Authoring Practices for the parts that are about the keyboard:
//
//   - SCALING. "The ribbon will show the largest valid combination that fits the
//     current window size." Each group has up to four layouts — [RibbonFull]
//     (large and small controls as designed), [RibbonCompact] (every control
//     small), [RibbonIconOnly] (small controls without their text) and
//     [RibbonPopup] (one button that opens the group) — and the groups with the
//     lowest [RibbonGroup.Priority] shrink first, the rightmost first among
//     equals. "Don't scale groups containing a single button to a pop-up group
//     icon": such a group stops at icon-only.
//   - COLLAPSING. [Ribbon.Collapsed] hides the body and leaves the tabs; a tab
//     then shows its body over the content below until a command runs, Escape,
//     or a click elsewhere. Ctrl+F1 toggles it, as in Office. The state is an
//     Observable so a host can persist it across runs, as the guidelines ask.
//   - KEYBOARD. The tabs are a WAI-ARIA tablist (Left/Right/Home/End, automatic
//     activation); the body is a toolbar with arrow-key navigation (Left/Right
//     between controls, Up/Down inside a column, Home/End); Enter or Space
//     activates, Down opens a split button, drop-down or gallery, Escape goes
//     back one level. KEYTIPS: a press of Alt shows a letter over each tab, a
//     letter chooses the tab and shows one over each control, and the next
//     letters run it.
//   - TOOLTIPS give the command name with its shortcut, then a description,
//     after [Ribbon.TooltipDelayMs] of hovering.
//   - CONTRAST. Boundaries (the tab outline, the group dividers, the popup
//     edge) and the focus ring reach 3:1 against the surfaces they sit on, in
//     light and dark themes (WCAG 2.2 1.4.11), as [PopupBorder] does for popups.
//
// Everything a command does goes through an [Action], the toolkit's one source
// of truth for a command (label, icon, shortcut, enabled, visible, run): a
// control is a VIEW on an Action, so the same Action can sit in a menu, a
// palette and the ribbon, and disabling it greys every surface. The ribbon's own
// state (the active tab, whether it is collapsed) and a toggle's checked state,
// a drop-down's selection are [mvvm.Observable]s that a view model binds.
//
// A host sizes the ribbon with [Ribbon.Height] (it changes when the ribbon is
// collapsed) and sets its bounds. The menus, a collapsed ribbon's body, the
// tooltips and the keytips are drawn outside the ribbon's own rectangle, over
// everything else, so the ribbon is a [PopoverOwner]: under a [PopoverHost] (the
// root of an application) it needs nothing more; a host without one calls
// [Ribbon.DrawPopover] last and offers [Ribbon.PopoverClick] the clicks first.
// It is an [Animator] too: [TickTree] drives its tooltip delay.
type Ribbon struct {
	Base
	focusState

	// Tabs are the tabs in order. Set once (config); the active one is
	// [Ribbon.ActiveTab].
	Tabs []*RibbonTab
	// Keymap, when set, supplies the live accelerator shown for each Action
	// (otherwise the Action's own default Shortcut).
	Keymap *Keymap
	// TooltipDelayMs is how long the pointer rests on a control before its
	// tooltip shows, driven by [Ribbon.Tick]. Zero means 500.
	TooltipDelayMs int
	// OnLeave, when set, is called when the keyboard use of the ribbon is over:
	// a command ran from the keyboard, Escape was pressed on the tabs with
	// nothing open, or the keytips ended. A host gives the keyboard back to the
	// editor (or whatever held it) there.
	OnLeave func()

	byKey     bool
	active    *mvvm.Observable[int]
	collapsed *mvvm.Observable[bool]

	zone     ribbonZone
	focusCtl int // index into the visible cells, when zone is ribbonZoneBody
	focusTab int
	peek     bool // a collapsed ribbon is showing its active tab's body
	pop      *ribbonPopup
	hoverTab int
	hoverCtl int
	hoverMs  int
	chevHot  bool
	keytips  int // 0 off, 1 choosing a tab, 2 choosing a control
	tipBuf   string
}

// RibbonTab is one tab: a label and the groups it holds.
type RibbonTab struct {
	Label string
	// KeyTip is the keytip letter; empty picks the first free letter of Label.
	KeyTip string
	Groups []*RibbonGroup
}

// RibbonGroup is a labelled cluster of controls on a tab.
type RibbonGroup struct {
	Label string
	// Icon is drawn on the group's button when it has collapsed to a pop-up.
	Icon IconFunc
	// Priority orders the shrinking: the group with the lowest value gives up its
	// large form first. Equal priorities shrink right to left.
	Priority int
	Controls []*RibbonControl
}

// RibbonKind is what a control is.
type RibbonKind int

const (
	// RibbonButton runs its Action.
	RibbonButton RibbonKind = iota
	// RibbonToggle flips Checked, then runs its Action.
	RibbonToggle
	// RibbonSplit runs its Action from its main part and opens Items from its
	// arrow.
	RibbonSplit
	// RibbonDropdown chooses one of Options (the index is Selected).
	RibbonDropdown
	// RibbonGallery shows the first few of Items as a strip, with an arrow that
	// opens all of them; the chosen index is Selected.
	RibbonGallery
	// RibbonGrid opens a grid of GridCols by GridRows cells under it, as the Table
	// button of a word processor does: the pointer (or the arrow keys) lights the
	// cells from the corner up to the one it is on, a caption says "3 × 2", and
	// choosing calls OnGrid with that many columns and rows.
	RibbonGrid
)

// RibbonControl is one control of a group. Only the fields its Kind needs are
// read.
type RibbonControl struct {
	Kind RibbonKind
	// Action is the command (label, icon, shortcut, enabled, visible, run).
	Action *Action
	// Label names a drop-down or a gallery, which have no Action.
	Label string
	// Tip is the description shown under the name in the tooltip.
	Tip string
	// KeyTip is the keytip; empty picks the first free letter of the label.
	KeyTip string
	// Large asks for the large form (icon above the text) while the group is at
	// full size.
	Large bool
	// Checked is a toggle's state.
	Checked *mvvm.Observable[bool]
	// Items are a split button's menu, or a gallery's cells.
	Items []RibbonItem
	// Options are a drop-down's choices.
	Options []string
	// Selected is a drop-down's or a gallery's chosen index.
	Selected *mvvm.Observable[int]
	// GridCols and GridRows are the size of a grid's popup, in cells (8 by 6 when
	// zero, at most 16 by 16); OnGrid is called with the columns and rows chosen.
	GridCols, GridRows int
	OnGrid             func(cols, rows int)
}

// gridSize is the size of a grid control's popup: its GridCols by GridRows, 8 by 6
// by default, kept between 1 and 16 each.
func (c *RibbonControl) gridSize() (cols, rows int) {
	cols, rows = c.GridCols, c.GridRows
	if cols <= 0 {
		cols = 8
	}
	if rows <= 0 {
		rows = 6
	}
	return min(cols, 16), min(rows, 16)
}

// RibbonItem is a menu entry or a gallery cell. A menu entry runs Action; a
// gallery cell uses Label and Icon (or its Action's).
type RibbonItem struct {
	Action *Action
	Label  string
	Icon   IconFunc
}

// RibbonLevel is how much of a group is shown.
type RibbonLevel int

const (
	// RibbonFull is the group as designed: large and small controls.
	RibbonFull RibbonLevel = iota
	// RibbonCompact makes every control small.
	RibbonCompact
	// RibbonIconOnly drops the text of the small controls.
	RibbonIconOnly
	// RibbonPopup folds the group into one button that opens it.
	RibbonPopup
)

// ribbonZone says where the keyboard focus is inside the ribbon.
type ribbonZone int

const (
	ribbonZoneTabs ribbonZone = iota
	ribbonZoneBody
)

// ribbonForm is how one control is drawn.
type ribbonForm int

const (
	formLarge ribbonForm = iota
	formSmall
	formIcon
	formDropdown
	formGallery
	formPopup
)

// ribbonMetrics are the sizes everything is laid out from, all derived from the
// widget's font and the toolkit's metric scale so a HiDPI or touch density grows
// the ribbon with the text.
type ribbonMetrics struct {
	gh, padX, gap, iconS, iconL, rowH, tabH, bodyH, labelH, arrowW, sepW, groupPad, colGap int
}

// NewRibbon builds a ribbon over tabs, showing the first one, expanded.
func NewRibbon(tabs ...*RibbonTab) *Ribbon {
	return &Ribbon{Tabs: tabs, hoverTab: -1, hoverCtl: -1}
}

// ActiveTab is the index of the shown tab as a shared [mvvm.Observable]: a view
// model binds it, and a click, an arrow key or a keytip Sets it. It is clamped
// to the tabs when read.
func (r *Ribbon) ActiveTab() *mvvm.Observable[int] {
	if r.active == nil {
		r.active = mvvm.NewObservable(0)
	}
	return r.active
}

// Collapsed is whether the ribbon is reduced to its tab strip, as a shared
// [mvvm.Observable]. A host persists it so a ribbon the user minimised comes
// back minimised.
func (r *Ribbon) Collapsed() *mvvm.Observable[bool] {
	if r.collapsed == nil {
		r.collapsed = mvvm.NewObservable(false)
	}
	return r.collapsed
}

// activeIndex is ActiveTab clamped into the tabs (0 with no tabs).
func (r *Ribbon) activeIndex() int {
	i := r.ActiveTab().Get()
	if i >= len(r.Tabs) {
		i = len(r.Tabs) - 1
	}
	if i < 0 {
		i = 0
	}
	return i
}

// activeTab is the shown tab, nil with no tabs.
func (r *Ribbon) activeTab() *RibbonTab {
	if len(r.Tabs) == 0 {
		return nil
	}
	return r.Tabs[r.activeIndex()]
}

func (r *Ribbon) metrics() ribbonMetrics {
	gh := r.glyphHeight()
	m := ribbonMetrics{gh: gh, padX: scaled(6), gap: scaled(4), iconS: scaled(16), iconL: scaled(32),
		arrowW: scaled(9), sepW: scaled(1), groupPad: scaled(6), colGap: scaled(2)}
	m.rowH = max(m.iconS, gh) + 2*scaled(3)
	m.tabH = gh + 2*scaled(6)
	m.labelH = gh + scaled(6)
	// A large control stacks its icon, its label (two lines at most) and a gap
	// between each; the body is as tall as that or three small rows.
	m.bodyH = max(3*m.rowH, m.iconL+2*gh+4*scaled(3))
	return m
}

// Height is the height the ribbon needs: the tab strip alone when collapsed,
// else the tabs, the body and the group labels. A host reads it on every layout
// (and when [Ribbon.Collapsed] changes) and gives the ribbon that much room.
func (r *Ribbon) Height() int {
	m := r.metrics()
	if r.Collapsed().Get() {
		return m.tabH + 1
	}
	return m.tabH + 1 + m.bodyH + m.labelH + 1
}

// visible reports whether a control is shown at all (its Action is visible).
func (c *RibbonControl) visible() bool { return c.Action == nil || c.Action.IsVisible() }

// enabled reports whether a control can be used (its Action is enabled).
func (c *RibbonControl) enabled() bool { return c.Action == nil || c.Action.IsEnabled() }

// label is the control's text: its Action's label, else its own.
func (c *RibbonControl) label() string {
	if c.Action != nil {
		return c.Action.Label
	}
	return c.Label
}

// icon is the control's glyph painter, nil when it has none.
func (c *RibbonControl) icon() IconFunc {
	if c.Action != nil && c.Action.Icon != nil {
		return IconFunc(c.Action.Icon)
	}
	return nil
}

// hasArrow is whether the control opens something from an arrow part.
func (c *RibbonControl) hasArrow() bool {
	return c.Kind == RibbonSplit || c.Kind == RibbonDropdown || c.Kind == RibbonGallery || c.Kind == RibbonGrid
}

// visibleControls are the group's shown controls, in order.
func (g *RibbonGroup) visibleControls() []*RibbonControl {
	var out []*RibbonControl
	for _, c := range g.Controls {
		if c.visible() {
			out = append(out, c)
		}
	}
	return out
}

// formAt is the form a control takes at a group level.
func (c *RibbonControl) formAt(l RibbonLevel) ribbonForm {
	switch {
	case c.Kind == RibbonDropdown:
		return formDropdown
	case l == RibbonFull && c.Kind == RibbonGallery:
		return formGallery
	case l == RibbonFull && c.Large:
		return formLarge
	case l == RibbonIconOnly && c.icon() != nil:
		return formIcon
	}
	return formSmall
}

// largeLines splits a large control's label over two lines at the space nearest
// its middle, the way Office wraps "Paste Special"; one word stays on one line.
func largeLines(s string) []string {
	best, bestD := -1, len(s)
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			continue
		}
		if d := abs(2*i - len(s)); d < bestD {
			best, bestD = i, d
		}
	}
	if best < 0 {
		return []string{s}
	}
	return []string{s[:best], s[best+1:]}
}

// ribbonCell is one control as placed: where it is, how it is drawn and the
// parts a pointer can hit.
type ribbonCell struct {
	rect  Rect
	ctrl  *RibbonControl
	grp   *RibbonGroup // the group a popup button opens
	form  ribbonForm
	arrow Rect   // the part that opens a menu; zero when the control has none
	cells []Rect // a gallery's visible cells, then nothing
}

// ribbonGroupBox is one group as placed.
type ribbonGroupBox struct {
	rect  Rect
	group *RibbonGroup
	level RibbonLevel
}

// ribbonBody is a tab's body laid out at a width.
type ribbonBody struct {
	groups []ribbonGroupBox
	cells  []ribbonCell
}

// controlSize is the size a control takes in a form; large and gallery forms
// are as tall as the body, the small forms one row.
func (r *Ribbon) controlSize(c *RibbonControl, f ribbonForm, m ribbonMetrics) (w, h int) {
	label := c.label()
	switch f {
	case formLarge:
		tw := 0
		for _, ln := range largeLines(label) {
			tw = max(tw, r.textWidth(ln))
		}
		if c.hasArrow() {
			tw += m.arrowW + m.gap
		}
		return max(m.iconL, tw) + 2*m.padX, m.bodyH
	case formIcon:
		w = m.iconS + 2*m.padX
		if c.hasArrow() {
			w += m.arrowW
		}
		return w, m.rowH
	case formDropdown:
		tw := r.textWidth(label)
		for _, o := range c.Options {
			tw = max(tw, r.textWidth(o))
		}
		return min(tw+m.arrowW+3*m.padX, scaled(180)), m.rowH
	case formGallery:
		return r.galleryCells(c)*r.galleryCellW(m) + m.arrowW + 2*m.padX, m.bodyH
	}
	// formSmall
	w = 2*m.padX + m.iconS + m.gap + r.textWidth(label)
	if c.hasArrow() {
		w += m.gap + m.arrowW
	}
	return w, m.rowH
}

// galleryCells is how many cells an in-ribbon gallery shows: three at most (the
// guidelines ask for at least three when there is room).
func (r *Ribbon) galleryCells(c *RibbonControl) int { return min(len(c.Items), 3) }

func (r *Ribbon) galleryCellW(m ribbonMetrics) int { return m.iconL + m.padX }

// popupSize is the size of a folded group's button.
func (r *Ribbon) popupSize(g *RibbonGroup, m ribbonMetrics) (w, h int) {
	return max(m.iconL, r.textWidth(g.Label)+m.gap+m.arrowW) + 2*m.padX, m.bodyH
}

// columns packs a group's shown controls into columns for a level: a large
// control or a gallery takes a column of its own, small ones stack up to three.
func (r *Ribbon) columns(g *RibbonGroup, l RibbonLevel) [][]*RibbonControl {
	var cols [][]*RibbonControl
	var small []*RibbonControl
	flush := func() {
		if len(small) > 0 {
			cols = append(cols, small)
			small = nil
		}
	}
	for _, c := range g.visibleControls() {
		switch f := c.formAt(l); f {
		case formLarge, formGallery:
			flush()
			cols = append(cols, []*RibbonControl{c})
		default:
			small = append(small, c)
			if len(small) == 3 {
				flush()
			}
		}
	}
	flush()
	return cols
}

// columnWidth is the width of a column: its widest control.
func (r *Ribbon) columnWidth(col []*RibbonControl, l RibbonLevel, m ribbonMetrics) int {
	w := 0
	for _, c := range col {
		cw, _ := r.controlSize(c, c.formAt(l), m)
		w = max(w, cw)
	}
	return w
}

// groupWidth is the width of a group at a level, label included.
func (r *Ribbon) groupWidth(g *RibbonGroup, l RibbonLevel, m ribbonMetrics) int {
	if l == RibbonPopup {
		w, _ := r.popupSize(g, m)
		return w + 2*m.groupPad
	}
	cols := r.columns(g, l)
	w := 0
	for i, col := range cols {
		if i > 0 {
			w += m.colGap
		}
		w += r.columnWidth(col, l, m)
	}
	return max(w, r.textWidth(g.Label)) + 2*m.groupPad
}

// nextLevel is the next level below cur that makes the group narrower, and false
// when there is none. A group of a single control never folds into a pop-up.
func (r *Ribbon) nextLevel(g *RibbonGroup, cur RibbonLevel, m ribbonMetrics) (RibbonLevel, bool) {
	w := r.groupWidth(g, cur, m)
	for l := cur + 1; l <= RibbonPopup; l++ {
		if l == RibbonPopup && len(g.visibleControls()) < 2 {
			break
		}
		if r.groupWidth(g, l, m) < w {
			return l, true
		}
	}
	return cur, false
}

// chooseLevels picks each group's level so the tab fits avail: every group
// starts full, and while the total is too wide the group with the lowest
// priority (the rightmost among equals) that can still shrink steps down.
func (r *Ribbon) chooseLevels(t *RibbonTab, avail int, m ribbonMetrics) []RibbonLevel {
	levels := make([]RibbonLevel, len(t.Groups))
	total := func() int {
		w := 0
		for i, g := range t.Groups {
			w += r.groupWidth(g, levels[i], m) + m.sepW
		}
		return w
	}
	for total() > avail {
		pick := -1
		var to RibbonLevel
		for i, g := range t.Groups {
			next, ok := r.nextLevel(g, levels[i], m)
			if ok && (pick < 0 || g.Priority <= t.Groups[pick].Priority) {
				pick, to = i, next
			}
		}
		if pick < 0 {
			break
		}
		levels[pick] = to
	}
	return levels
}

// Levels is the level each group of the active tab is shown at for the current
// width — what a host (or a test) reads to see how far the ribbon has scaled.
func (r *Ribbon) Levels() []RibbonLevel {
	t := r.activeTab()
	if t == nil {
		return nil
	}
	return r.chooseLevels(t, r.Bounds().W, r.metrics())
}

// layoutBody places the groups and controls of tab t, at the given levels, with
// the body's top-left corner at (x, y).
func (r *Ribbon) layoutBody(t *RibbonTab, levels []RibbonLevel, x, y int, m ribbonMetrics) ribbonBody {
	var body ribbonBody
	for i, g := range t.Groups {
		gw := r.groupWidth(g, levels[i], m)
		body.groups = append(body.groups, ribbonGroupBox{rect: Rect{X: x, Y: y, W: gw, H: m.bodyH + m.labelH}, group: g, level: levels[i]})
		body.cells = append(body.cells, r.layoutGroup(g, levels[i], x+m.groupPad, y, m)...)
		x += gw + m.sepW
	}
	return body
}

// layoutGroup places one group's controls with its content's top-left at (x, y).
func (r *Ribbon) layoutGroup(g *RibbonGroup, l RibbonLevel, x, y int, m ribbonMetrics) []ribbonCell {
	if l == RibbonPopup {
		w, h := r.popupSize(g, m)
		rc := Rect{X: x, Y: y, W: w, H: h}
		return []ribbonCell{{rect: rc, grp: g, form: formPopup}}
	}
	var cells []ribbonCell
	for i, col := range r.columns(g, l) {
		if i > 0 {
			x += m.colGap
		}
		cw := r.columnWidth(col, l, m)
		// A column of small controls is centred in the body's height.
		cy := y
		if f := col[0].formAt(l); f != formLarge && f != formGallery {
			cy += (m.bodyH - len(col)*m.rowH) / 2
		}
		for _, c := range col {
			f := c.formAt(l)
			_, ch := r.controlSize(c, f, m)
			cell := ribbonCell{rect: Rect{X: x, Y: cy, W: cw, H: ch}, ctrl: c, form: f}
			r.placeParts(&cell, m)
			cells = append(cells, cell)
			cy += ch
		}
		x += cw
	}
	return cells
}

// placeParts fills in the arrow part and the gallery cells of a placed control.
func (r *Ribbon) placeParts(c *ribbonCell, m ribbonMetrics) {
	rc := c.rect
	switch {
	case c.form == formGallery:
		n := r.galleryCells(c.ctrl)
		cw := r.galleryCellW(m)
		for i := 0; i < n; i++ {
			c.cells = append(c.cells, Rect{X: rc.X + m.padX + i*cw, Y: rc.Y, W: cw, H: rc.H})
		}
		c.arrow = Rect{X: rc.X + rc.W - m.arrowW - m.padX, Y: rc.Y, W: m.arrowW + m.padX, H: rc.H}
	case c.form == formLarge && c.ctrl.hasArrow():
		// The label half of a large split opens the menu, the icon half runs.
		c.arrow = Rect{X: rc.X, Y: rc.Y + m.padX + m.iconL, W: rc.W, H: rc.H - m.padX - m.iconL}
	case c.form == formDropdown:
		c.arrow = rc
	case c.ctrl.hasArrow() && (c.form == formSmall || c.form == formIcon):
		c.arrow = Rect{X: rc.X + rc.W - m.arrowW - m.padX, Y: rc.Y, W: m.arrowW + m.padX, H: rc.H}
	}
}

// tabRects places the tab labels along the strip, then the collapse chevron.
func (r *Ribbon) tabRects(m ribbonMetrics) (tabs []Rect, chev Rect) {
	b := r.Bounds()
	x := b.X + m.padX
	for _, t := range r.Tabs {
		w := r.textWidth(t.Label) + 4*m.padX
		tabs = append(tabs, Rect{X: x, Y: b.Y, W: w, H: m.tabH})
		x += w
	}
	chev = Rect{X: b.X + b.W - m.tabH - m.padX, Y: b.Y, W: m.tabH, H: m.tabH}
	return tabs, chev
}

// body lays out the active tab's body at the ribbon's width, below the tab
// strip (also when it is only peeking over the content of a collapsed one).
func (r *Ribbon) body() ribbonBody {
	t := r.activeTab()
	if t == nil {
		return ribbonBody{}
	}
	m := r.metrics()
	b := r.Bounds()
	return r.layoutBody(t, r.chooseLevels(t, b.W, m), b.X, b.Y+m.tabH+1, m)
}

// showsBody is whether the body is on screen: always when expanded, only while
// peeking when collapsed.
func (r *Ribbon) showsBody() bool { return !r.Collapsed().Get() || r.peek }

// bodyRect is the rectangle the body fills.
func (r *Ribbon) bodyRect() Rect {
	m := r.metrics()
	b := r.Bounds()
	return Rect{X: b.X, Y: b.Y + m.tabH + 1, W: b.W, H: m.bodyH + m.labelH + 1}
}

// TabRect is where tab i's label is, in surface coordinates; false when there is
// no such tab. A test harness that drives the page with synthetic events clicks
// there.
func (r *Ribbon) TabRect(i int) (Rect, bool) {
	tabs, _ := r.tabRects(r.metrics())
	if i < 0 || i >= len(tabs) {
		return Rect{}, false
	}
	return tabs[i], true
}

// ControlRect is where the control (or folded group) labelled label is, in surface
// coordinates, among those the ribbon shows now; false when it is not shown (on
// another tab, hidden, or folded into a group's pop-up).
func (r *Ribbon) ControlRect(label string) (Rect, bool) {
	for _, c := range r.cells() {
		l := ""
		if c.form == formPopup {
			l = c.grp.Label
		} else {
			l = c.ctrl.label()
		}
		if l == label {
			return c.rect, true
		}
	}
	return Rect{}, false
}
