// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// FolderTabs is a compact, desktop folder-tab strip: a row of small tabs — each
// sized to its own label, NOT stretched to fill the width — with ROUNDED TOP
// corners sitting on a thin strip. The active tab is filled in Theme.Surface,
// carries a Theme.Accent bar along its top edge, and is extended down over the
// strip's bottom border so it reads as CONNECTED to the pane below it (the
// classic manila-folder look). Inactive tabs are dimmed and sit one pixel above
// that border so the border shows beneath them.
//
// It is a THIRD tab style, deliberately distinct from the two the toolkit
// already ships:
//
//   - TabBar is a MOBILE bottom-navigation bar — equal-width, full-height
//     finger targets with icon+label+badge items, laid out for a fingertip.
//   - ViewSwitcher is a GTK/libadwaita full-width SEGMENTED control — the strip
//     divided evenly into same-width segments, the active one a solid accent
//     block.
//
// FolderTabs is neither: its tabs are label-width (compact, left-packed, not
// full-bleed) and folder-shaped (rounded-top, connected to the pane), the
// desktop "notebook tab" idiom for switching a single pane's content — e.g. a
// Rendered│Log split under an editor. It is a separate widget precisely so the
// mobile bar and the segmented switcher keep their own looks untouched.
//
// Reactive selection lives on an unexported [mvvm.Observable] exposed through
// [FolderTabs.Selected]; a host binds it (Get / Set / Subscribe) exactly as it
// binds a ViewSwitcher's Current. The optional [FolderTabs.OnSelect] func hook
// fires alongside, for a caller that prefers a plain callback (as TabBar does).
//
// A FolderTabs with no Labels paints only the strip + its bottom border; every
// input is a no-op. This lets a caller assemble the widget before it knows which
// panes the app will surface.
type FolderTabs struct {
	Base
	focusState

	// Labels are the tab captions, laid out left→right in slice order. Set-once
	// layout config (the reactive part is the selected index, on the Observable),
	// so it stays a plain field under the MVVM gate.
	Labels []string
	// OnSelect fires with the newly-selected index whenever the selection changes
	// via a click or an arrow key — nil-safe, and fired only on an actual change.
	// It is a convenience seam beside the Selected Observable; a host may use
	// either or both.
	OnSelect func(index int)

	// Closable, when true, draws a small × affordance on the right of each tab and
	// makes a click on it fire OnClose(i) instead of selecting the tab — the editor
	// idiom of closeable document tabs. Off by default, so the Rendered│Log style
	// strips stay uncloseable.
	Closable bool
	// OnClose fires with a tab's index when its × is clicked (Closable only). The
	// host owns the tab list, so it decides what removing tab i means (drop the
	// label, pick a neighbour to select); FolderTabs itself does not mutate Labels.
	// nil-safe.
	OnClose func(index int)

	// selected is the reactive active-tab index. It is MVVM-only: the value lives
	// in an unexported Observable exposed via [FolderTabs.Selected], so there is
	// no settable Selected field to drift out of the reactive layer.
	selected *mvvm.Observable[int]
}

// Folder-tab layout metrics, all in LOGICAL pixels (routed through scaled at
// use). Exposed so a host allocating a strip can size it to match.
const (
	// FolderTabsH is the strip's default vertical extent — deliberately short so
	// the tabs read as slim notebook tabs, not buttons.
	FolderTabsH = 24
	// FolderTabsRadius is the corner radius applied to each tab's two TOP corners.
	FolderTabsRadius = 6
	// FolderTabsPadX is the horizontal padding either side of a tab's label.
	FolderTabsPadX = 12
	// FolderTabsGap is the horizontal gap between adjacent tabs.
	FolderTabsGap = 3
	// FolderTabsInset is the left margin before the first tab, and the small gap
	// above the tabs (off the strip's top edge).
	FolderTabsInset = 3
	// FolderTabsAccentH is the thickness of the active tab's top accent bar.
	FolderTabsAccentH = 2
	// FolderTabsCloseW is the width reserved on the right of a tab for its ×
	// close affordance when [FolderTabs.Closable] is set (label + gap + glyph box).
	FolderTabsCloseW = 16
)

// FolderTabsHeight is the recommended strip height in device pixels at the
// current [MetricScale] and touch [Density]: the scaled [FolderTabsH] clamped UP
// to the density minimum hit target via [TouchTarget]. A tab is a click target,
// so at [DensityTouch] the recommended height reaches the finger floor; under
// the default [DensityCompact] the clamp is a pass-through. A host allocating a
// strip sizes it to this.
func FolderTabsHeight() int { return TouchTarget(scaled(FolderTabsH)) }

// NewFolderTabs constructs a FolderTabs over labels with the initial active tab
// at selected. selected is clamped into [0, len(labels)-1], or forced to 0 when
// labels is empty, so the widget never holds an out-of-range selection.
func NewFolderTabs(labels []string, selected int) *FolderTabs {
	n := len(labels)
	switch {
	case n == 0:
		selected = 0
	case selected < 0:
		selected = 0
	case selected >= n:
		selected = n - 1
	}
	t := &FolderTabs{Labels: labels}
	t.selected = mvvm.NewObservable(selected)
	return t
}

// Selected is the active tab index as a shared [mvvm.Observable]: a host binds
// it (Set / Subscribe / two-way) — there is no settable Selected field. A click
// or an arrow key Sets it; subscribers are notified. It is lazily created so the
// zero-value FolderTabs is usable.
func (t *FolderTabs) Selected() *mvvm.Observable[int] {
	if t.selected == nil {
		t.selected = mvvm.NewObservable(0)
	}
	return t.selected
}

// height is the strip's nominal device height, used to vertically centre a
// label — deliberately shorter than a button so the tabs read as slim tabs.
func (t *FolderTabs) height() int { return scaled(FolderTabsH) }

// tabTop is the y at which a tab's rounded body begins — a small gap below the
// strip's top edge keeps the tabs off the very top.
func (t *FolderTabs) tabTop() int { return t.Bounds().Y + scaled(FolderTabsInset) }

// TabRect returns the device rectangle of tab i in SURFACE coordinates: sized to
// its label plus padding, laid left-to-right from a small left inset. The height
// runs from the tab top to the strip bottom (the active tab is extended over the
// border in Draw; an inactive one stops one pixel short of it). An out-of-range
// i (or an unmeasurable strip) returns the zero Rect, so a caller can range over
// len(Labels) without a bounds check.
func (t *FolderTabs) TabRect(i int) Rect {
	if i < 0 || i >= len(t.Labels) {
		return Rect{}
	}
	r := t.Bounds()
	gap := scaled(FolderTabsGap)
	x := r.X + scaled(FolderTabsInset)
	for k := 0; k < i; k++ {
		x += t.tabWidth(t.Labels[k]) + gap
	}
	top := t.tabTop()
	h := r.Y + r.H - top
	if h < 1 {
		h = 1
	}
	return Rect{X: x, Y: top, W: t.tabWidth(t.Labels[i]), H: h}
}

// tabWidth is a tab's device width: its label plus horizontal padding, and — when
// [FolderTabs.Closable] — the reserved close-affordance width on the right.
func (t *FolderTabs) tabWidth(label string) int {
	w := t.textWidth(label) + 2*scaled(FolderTabsPadX)
	if t.Closable {
		w += scaled(FolderTabsCloseW)
	}
	return w
}

// closeRect is the square hit box of tab i's × affordance, tucked against the
// tab's right edge and vertically centred. The zero Rect when the strip is not
// Closable or i is out of range, so a caller can test Contains unconditionally.
func (t *FolderTabs) closeRect(i int) Rect {
	if !t.Closable || i < 0 || i >= len(t.Labels) {
		return Rect{}
	}
	tr := t.TabRect(i)
	cw := scaled(FolderTabsCloseW)
	return Rect{X: tr.X + tr.W - cw, Y: tr.Y + (t.height()-cw)/2, W: cw, H: cw}
}

// tabAt returns the tab index under a SURFACE point, or -1 when none.
func (t *FolderTabs) tabAt(x, y int) int {
	for i := range t.Labels {
		if t.TabRect(i).Contains(x, y) {
			return i
		}
	}
	return -1
}

// Draw paints the strip, its bottom border, then each tab (the active one last
// so it covers the border and sits on top). A zero-area bounds short-circuits.
func (t *FolderTabs) Draw(p painter.Painter, theme *Theme) {
	r := t.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	borderH := atLeast1(scaled(1))
	// Strip bottom border (separates the strip from the pane below).
	fillRect(p, r.X, r.Y+r.H-borderH, r.W, borderH, theme.Border)

	if len(t.Labels) == 0 {
		t.drawFocusRing(p, theme, r)
		return
	}

	rad := scaled(FolderTabsRadius)
	dimFace := blendRGBA(theme.SurfaceAlt, theme.Background, 0.5)
	dimInk := blendRGBA(theme.OnSurface, theme.SurfaceAlt, 0.5)
	active := t.Selected().Get()

	for i := range t.Labels {
		if i == active {
			continue // drawn last, on top
		}
		t.drawTab(p, theme, i, dimFace, dimInk, false, rad)
	}
	if active >= 0 && active < len(t.Labels) {
		t.drawTab(p, theme, active, theme.Surface, theme.OnSurface, true, rad)
	}
	t.drawFocusRing(p, theme, r)
}

// drawTab paints one tab: a top-rounded body in face, its centred label in ink,
// and — for the active tab — an accent bar along the top and a body extended to
// cover the strip's bottom border so the tab connects to the pane below.
func (t *FolderTabs) drawTab(p painter.Painter, theme *Theme, i int, face, ink RGBA, active bool, rad int) {
	tr := t.TabRect(i)
	r := t.Bounds()
	if active {
		tr.H = r.Y + r.H - tr.Y // extend over the border to connect to the pane
	} else {
		tr.H = atLeast1(tr.H - atLeast1(scaled(1))) // leave the border visible below
	}
	fillTopRoundRect(p, tr.X, tr.Y, tr.W, tr.H, rad, face)
	if active {
		fillRect(p, tr.X, tr.Y, tr.W, atLeast1(scaled(FolderTabsAccentH)), theme.Accent)
	}
	label := t.Labels[i]
	tw := t.textWidth(label)
	// Centre the label in the tab, over the LABEL area (the reserved × box, when
	// Closable, is excluded so the caption stays centred in the room it actually has).
	labelW := tr.W
	if t.Closable {
		labelW -= scaled(FolderTabsCloseW)
	}
	tx := tr.X + (labelW-tw)/2
	ty := tr.Y + (t.height()-t.glyphHeight())/2
	t.drawText(p, tx, ty, label, ink)
	if t.Closable {
		cr := t.closeRect(i)
		xw := t.textWidth("×")
		t.drawText(p, cr.X+(cr.W-xw)/2, cr.Y+(cr.H-t.glyphHeight())/2, "×", ink)
	}
}

// OnEvent selects a tab on a click and steps the selection on an arrow key. The
// event's X/Y are widget-local, so a click is re-anchored to surface space
// before hit-testing. A click that falls on no tab, a click/key on an empty
// strip, and every other event kind are no-ops; a Disabled strip ignores
// everything.
func (t *FolderTabs) OnEvent(ev Event) {
	if t.Disabled().Get() {
		return
	}
	switch ev.Kind {
	case EventClick:
		sx, sy := t.Bounds().X+ev.X, t.Bounds().Y+ev.Y
		if i := t.tabAt(sx, sy); i >= 0 {
			// A click on the × closes the tab (Closable) rather than selecting it.
			if t.Closable && t.closeRect(i).Contains(sx, sy) {
				if t.OnClose != nil {
					t.OnClose(i)
				}
				return
			}
			t.setActive(i)
		}
	case EventKeyDown:
		switch ev.Code {
		case "ArrowLeft", "ArrowUp":
			t.step(-1)
		case "ArrowRight", "ArrowDown":
			t.step(+1)
		}
	}
}

// setActive selects tab i by Setting the Selected Observable and firing OnSelect
// — the shared mutate path for a click and a keyboard move. Both notifications
// fire only on an actual change (the Observable is a no-op on an unchanged Set,
// and OnSelect is guarded to match). Out-of-range i is ignored.
func (t *FolderTabs) setActive(i int) {
	if i < 0 || i >= len(t.Labels) {
		return
	}
	sel := t.Selected()
	if sel.Get() == i {
		return
	}
	sel.Set(i)
	if t.OnSelect != nil {
		t.OnSelect(i)
	}
}

// step moves the active tab delta positions, wrapping at both ends (the WAI-ARIA
// tablist convention, matching ViewSwitcher). A no-op when there are no labels.
func (t *FolderTabs) step(delta int) {
	n := len(t.Labels)
	if n == 0 {
		return
	}
	cur := t.Selected().Get()
	if cur < 0 || cur >= n {
		cur = 0
	}
	t.setActive(((cur+delta)%n + n) % n)
}

// A11y reports the strip as a tablist named by its active tab's label. Each tab
// is published as an individual RoleTab node via [FolderTabs.Children], so a
// screen reader announces a tablist containing N tabs with one selected.
func (t *FolderTabs) A11y() A11yInfo {
	name := ""
	if a := t.Selected().Get(); a >= 0 && a < len(t.Labels) {
		name = t.Labels[a]
	}
	return A11yInfo{Role: RoleTablist, Name: name}
}

// Children exposes each label as a synthetic, non-drawing RoleTab node so the
// generic accessibility walk (CollectA11y / WalkA11y) descends the strip into a
// tablist → tab structure, each tab carrying its own name, selected state and
// surface rectangle. The nodes are rebuilt on demand from the current geometry,
// so a resize or a selection change is reflected without any retained child
// state. These nodes are for the a11y/geometry walk only — FolderTabs paints
// every tab itself in Draw.
func (t *FolderTabs) Children() []Widget {
	active := t.Selected().Get()
	out := make([]Widget, 0, len(t.Labels))
	for i := range t.Labels {
		node := &folderTabNode{label: t.Labels[i], selected: i == active}
		node.SetBounds(t.TabRect(i))
		out = append(out, node)
	}
	return out
}

// folderTabNode is a synthetic, non-interactive stand-in for one FolderTabs tab,
// materialised only so the shared a11y walk can report each tab as its own
// RoleTab node with the right name, selected state and bounds. It draws nothing
// (FolderTabs paints the tabs) and handles no events.
type folderTabNode struct {
	Base
	label    string
	selected bool
}

// A11y reports the node as a tab named by its label, with Value "selected" on
// the active tab and "" otherwise — the standard tab selected-state string a
// host maps to aria-selected.
func (n *folderTabNode) A11y() A11yInfo {
	return A11yInfo{Role: RoleTab, Name: n.label, Value: stateValue(n.selected, "selected")}
}

// Compile-time checks: FolderTabs and its synthetic tab node both satisfy
// Accessible so the accessibility walk never silently skips them.
var (
	_ Accessible = (*FolderTabs)(nil)
	_ Accessible = (*folderTabNode)(nil)
)
