// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// TabBar is a mobile bottom-navigation bar: a strip of equal-width items,
// each an icon over an optional label, pinned to the bottom edge of a phone
// or tablet surface with exactly one item Selected. It is the touch-first
// counterpart of the desktop ViewSwitcher / SegmentedBar — same "pick one of
// N views" job, laid out for a fingertip instead of a mouse — and is a
// separate widget precisely so the desktop pickers keep their compact,
// horizontal-strip look untouched.
//
// Each item spans the FULL height of the bar and 1/N of its width, so the
// whole bar is one uninterrupted band of touch targets with no dead gaps
// between items. The bar's own height auto-sizes (when Bounds().H is zero) to
// [TabBarHeight] routed through [TouchTarget], guaranteeing a hit height at
// or above the 44-logical-pixel finger floor under [DensityTouch] while
// staying byte-identical to a fixed 56-px bar under the desktop default
// [DensityCompact]. Per-item hit rectangles are available via [TabBar.ItemHitRect],
// the worked [TouchTarget] example mirroring [Switch.HitRect]: a narrow item
// grows a finger-sized hit area around its unchanged pixels.
//
// The Selected item is highlighted two ways: its icon + label paint in
// Theme.Accent (unselected items paint in Theme.OnSurface), and a
// [TabBarIndicatorH]-thick accent indicator bar runs along the TOP edge of
// the selected item's column. A tap anywhere in an item's column selects it
// and fires OnSelect exactly once; optional horizontal swipe navigation
// (gated by SwipeNavigation) steps the selection one item per swipe.
//
// A TabBar with no Items paints only its background band + the accent-free
// top border; every input is a no-op. This lets a caller assemble the widget
// before it knows which destinations the app will surface.
type TabBar struct {
	Base
	focusState

	// Items are the navigation destinations, laid out left→right in slice
	// order. Each carries an Icon glyph, an optional Label, and an optional
	// Badge counter string.
	Items []TabItem
	// SwipeNavigation opts the bar into horizontal-swipe navigation: a left
	// swipe advances to the next item, a right swipe returns to the previous
	// one (both clamped at the ends). It is false by default, so a plain
	// TabBar ignores swipes and only a caller that wants the gesture pays for
	// it — the "optional" in the widget's contract.
	SwipeNavigation bool
	// Gestures is the recognizer that turns the touch stream into taps and
	// swipes. NewTabBar wires it; a caller may retune its thresholds
	// (TapSlop, SwipeMinDist, ...) or leave the defaults.
	Gestures *GestureRecognizer

	// selected is the highlighted item index, reactive via the Selected()
	// accessor. NewTabBar clamps it into range; OnEvent keeps it in range.
	// Subscribers replace the old OnSelect callback.
	selected *mvvm.Observable[int]
}

// Selected is the highlighted item index as a shared [mvvm.Observable]: a host
// binds it (or subscribes for the old OnSelect notification) instead of reading
// a field, and a tap / swipe / arrow key Sets it. Lazily created so a bare
// &TabBar{} works.
func (t *TabBar) Selected() *mvvm.Observable[int] {
	if t.selected == nil {
		t.selected = mvvm.NewObservable(0)
	}
	return t.selected
}

// TabItem is one destination in a TabBar: a short Icon glyph (rendered in the
// toolkit's bitmap font, e.g. a symbol or a 1–2 letter stand-in), an optional
// Label beneath it, and an optional Badge — a short counter/indicator string
// ("3", "9+") drawn as a pill hanging off the icon's top-right corner via the
// same body as the standalone Badge widget. An empty Badge draws nothing.
type TabItem struct {
	Icon  string
	Label string
	Badge string
}

// Layout constants, all in LOGICAL pixels (routed through scaled at use).
const (
	// TabBarHeight is the default bar height when Bounds().H is zero — the
	// Material bottom-navigation height, comfortably above the 44-px touch
	// floor at DensityCompact.
	TabBarHeight = 56
	// TabBarIndicatorH is the thickness of the accent indicator bar along the
	// selected item's top edge.
	TabBarIndicatorH = 3
	// TabBarIconLabelGap is the vertical gap between an item's icon and its
	// label when both are present.
	TabBarIconLabelGap = 2
)

// RoleTab is the WAI-ARIA role each TabBar item reports in the accessibility
// tree — a "tab" inside the bar's "tablist" (see RoleTablist). It sits beside
// the roles in a11y.go / a11y_more.go.
const RoleTab Role = "tab"

// NewTabBar builds a TabBar over items with the initial selection at selected.
// selected is clamped into [0, len(items)-1], or forced to 0 when items is
// empty, so the widget never holds an out-of-range selection. The gesture
// recognizer is wired with the package defaults; horizontal-swipe navigation
// stays off until the caller sets SwipeNavigation.
func NewTabBar(items []TabItem, selected int) *TabBar {
	n := len(items)
	switch {
	case n == 0:
		selected = 0
	case selected < 0:
		selected = 0
	case selected >= n:
		selected = n - 1
	}
	t := &TabBar{Items: items}
	t.selected = mvvm.NewObservable(selected)
	g := NewGestureRecognizer()
	g.OnTap = func(x, _ int) { t.selectAt(x) }
	g.OnSwipe = func(dir SwipeDir) {
		if !t.SwipeNavigation {
			return
		}
		switch dir {
		case SwipeLeft:
			t.step(+1)
		case SwipeRight:
			t.step(-1)
		}
	}
	t.Gestures = g
	return t
}

// itemWidth is the shared per-item column width: an even integer-division of
// the bar's width by the item count. Any leftover pixels on the right stay
// background, exactly as ViewSwitcher tolerates a non-integer strip. It is 0
// when there are no items (callers guard on that).
func (t *TabBar) itemWidth() int {
	n := len(t.Items)
	if n == 0 {
		return 0
	}
	return t.Bounds().W / n
}

// ItemRect returns the VISUAL (drawn) rectangle of item i in surface
// coordinates: its column of the bar, full bar height. An out-of-range i (or
// a zero-width bar) returns the zero Rect, so a caller can range over
// len(Items) and skip empties without a bounds check.
func (t *TabBar) ItemRect(i int) Rect {
	n := len(t.Items)
	if i < 0 || i >= n {
		return Rect{}
	}
	iw := t.itemWidth()
	if iw <= 0 {
		return Rect{}
	}
	r := t.Bounds()
	return Rect{X: r.X + i*iw, Y: r.Y, W: iw, H: r.H}
}

// ItemHitRect returns item i's interactive rectangle: its visual [TabBar.ItemRect]
// with each axis clamped UP to [MinHitTarget] and centred over the drawn
// column, the same construction [Switch.HitRect] uses. Under the default
// [DensityCompact] the minimum is 0, so the hit rect equals the visual rect
// byte-for-byte; under [DensityTouch] a narrow item grows a finger-sized hit
// area around its unchanged pixels. Only the hit region grows — Draw is
// untouched. An out-of-range i returns the zero Rect.
func (t *TabBar) ItemHitRect(i int) Rect {
	vr := t.ItemRect(i)
	if vr.W == 0 && vr.H == 0 {
		return Rect{}
	}
	w, h := TouchTarget(vr.W), TouchTarget(vr.H)
	return Rect{
		X: vr.X - (w-vr.W)/2,
		Y: vr.Y - (h-vr.H)/2,
		W: w,
		H: h,
	}
}

// autosize fills in a zero height with TabBarHeight routed through TouchTarget
// so the bar is always at least a finger tall under a touch density, and a
// zero width is left to the caller (a bottom bar spans its parent). Called at
// the top of Draw and by geometry-consuming callers indirectly via Bounds.
func (t *TabBar) autosize() {
	r := t.Bounds()
	if r.H == 0 {
		r.H = TouchTarget(scaled(TabBarHeight))
		t.SetBounds(r)
	}
}

// Draw paints the background band, a 1-pixel accent-free TOP border (so the
// bar reads as a discrete band beneath the scrolled content above it), each
// item's icon/label (+ optional badge), and the accent top indicator over the
// selected item. A zero height is auto-filled first.
func (t *TabBar) Draw(p painter.Painter, theme *Theme) {
	t.autosize()
	r := t.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	// Top separator so the bar sits visually below the app content.
	fillRect(p, r.X, r.Y, r.W, strokeWidth(), theme.Border)

	n := len(t.Items)
	if n > 0 {
		for i := range t.Items {
			t.drawItem(p, theme, i)
		}
	}
	t.drawFocusRing(p, theme, r)
}

// drawItem paints one item's indicator, icon, label and badge inside its
// column. The selected item paints in Theme.Accent with a top indicator bar;
// the rest paint in Theme.OnSurface. Disabled mutes every ink.
func (t *TabBar) drawItem(p painter.Painter, theme *Theme, i int) {
	vr := t.ItemRect(i)
	item := t.Items[i]
	selected := i == t.Selected().Get()

	ink := theme.OnSurface
	if selected {
		ink = theme.Accent
	}
	if t.Disabled().Get() {
		ink = mutedInk(theme)
	}

	// Selected indicator: an accent bar along the item's top edge.
	if selected {
		indicator := theme.Accent
		if t.Disabled().Get() {
			indicator = mutedInk(theme)
		}
		fillRect(p, vr.X, vr.Y, vr.W, scaled(TabBarIndicatorH), indicator)
	}

	iconW := t.textWidth(item.Icon)
	gh := t.glyphHeight()
	iconX := vr.X + (vr.W-iconW)/2
	var iconY int
	if item.Label != "" {
		gap := scaled(TabBarIconLabelGap)
		contentH := gh + gap + gh
		top := vr.Y + (vr.H-contentH)/2
		iconY = top
		labelW := t.textWidth(item.Label)
		labelX := vr.X + (vr.W-labelW)/2
		labelY := top + gh + gap
		t.drawText(p, iconX, iconY, item.Icon, ink)
		t.drawText(p, labelX, labelY, item.Label, ink)
	} else {
		iconY = vr.Y + (vr.H-gh)/2
		t.drawText(p, iconX, iconY, item.Icon, ink)
	}

	if item.Badge != "" {
		t.drawBadge(p, theme, item.Badge, iconX+iconW, iconY, vr)
	}
}

// drawBadge paints item's Badge pill centred on the icon's top-right corner
// (cornerX, cornerY), clamped to stay inside the item column vr. It reuses the
// standalone Badge widget's body so a counter here looks identical to one
// hanging off any other widget.
func (t *TabBar) drawBadge(p painter.Painter, theme *Theme, text string, cornerX, cornerY int, vr Rect) {
	bw, bh := t.badgeSize(text)
	bx := cornerX - bw/2
	by := cornerY - bh/2
	// Keep the pill inside the item column horizontally + off the top edge.
	if bx+bw > vr.X+vr.W {
		bx = vr.X + vr.W - bw
	}
	if bx < vr.X {
		bx = vr.X
	}
	if by < vr.Y {
		by = vr.Y
	}
	b := NewBadge(text)
	b.Font = t.Font
	b.SetBounds(Rect{X: bx, Y: by, W: bw, H: bh})
	b.Draw(p, theme)
}

// badgeSize computes the pill dimensions for text using the exact same formula
// Badge.Draw auto-sizes with (text width + BadgePadX either side; glyph height
// + BadgePadY either side), so a pre-sized badge here matches a self-sized
// standalone Badge to the pixel.
func (t *TabBar) badgeSize(text string) (w, h int) {
	w = t.textWidth(text) + 2*scaled(BadgePadX)
	h = t.glyphHeight() + 2*scaled(BadgePadY)
	return w, h
}

// OnEvent selects an item on a tap/click, steps the selection on an arrow key,
// and feeds the touch stream to the gesture recognizer (whose OnTap selects
// and OnSwipe navigates when SwipeNavigation is on). A Disabled bar ignores
// every kind.
func (t *TabBar) OnEvent(ev Event) {
	if t.Disabled().Get() {
		return
	}
	switch ev.Kind {
	case EventClick:
		t.selectAt(ev.X)
	case EventKeyDown:
		switch ev.Code {
		case "ArrowLeft", "ArrowUp":
			t.step(-1)
		case "ArrowRight", "ArrowDown":
			t.step(+1)
		}
	case EventTouchStart, EventTouchMove, EventTouchEnd:
		if t.Gestures != nil {
			t.Gestures.Feed(ev)
		}
	}
}

// selectAt maps a widget-local x to its item column and selects it. A click in
// the trailing remainder region (past the last full column), on an empty bar,
// or on a zero-width bar is a no-op.
func (t *TabBar) selectAt(localX int) {
	n := len(t.Items)
	if n == 0 {
		return
	}
	iw := t.itemWidth()
	if iw <= 0 {
		return
	}
	idx := localX / iw
	if localX < 0 || idx < 0 || idx >= n {
		return
	}
	t.setSelected(idx)
}

// step moves the selection by delta, clamped at both ends (a bottom bar does
// not wrap), and fires OnSelect only when the index actually changes. A no-op
// on an empty bar.
func (t *TabBar) step(delta int) {
	n := len(t.Items)
	if n == 0 {
		return
	}
	next := t.Selected().Get() + delta
	if next < 0 {
		next = 0
	} else if next >= n {
		next = n - 1
	}
	if next == t.Selected().Get() {
		return
	}
	t.setSelected(next)
}

// setSelected records idx as the selection and fires OnSelect (nil-safe) — the
// shared mutate+callback path for a tap, a swipe and an arrow key.
func (t *TabBar) setSelected(idx int) {
	t.Selected().Set(idx)
}

// A11y reports the TabBar as a tablist named by its selected item's label
// (falling back to that item's icon when it has no label). Each item is
// published as an individual RoleTab node via Children, so a screen reader
// announces the bar as a tablist containing N tabs with one selected.
func (t *TabBar) A11y() A11yInfo {
	name := ""
	if t.Selected().Get() >= 0 && t.Selected().Get() < len(t.Items) {
		it := t.Items[t.Selected().Get()]
		if it.Label != "" {
			name = it.Label
		} else {
			name = it.Icon
		}
	}
	return A11yInfo{Role: RoleTablist, Name: name}
}

// Children exposes each item as a synthetic, non-drawing tab node so the
// generic accessibility walk (WalkA11y) descends the bar into a tablist → tab
// structure, each tab carrying its own name, selected state and surface
// rectangle. The nodes are rebuilt on demand from the current geometry, so a
// resize or a selection change is reflected without any retained child state.
// These nodes are for the a11y/geometry walk only — the TabBar paints every
// item itself in Draw.
func (t *TabBar) Children() []Widget {
	out := make([]Widget, 0, len(t.Items))
	for i := range t.Items {
		node := &tabItemNode{item: t.Items[i], selected: i == t.Selected().Get()}
		node.SetBounds(t.ItemRect(i))
		out = append(out, node)
	}
	return out
}

// tabItemNode is a synthetic, non-interactive stand-in for one TabBar item,
// materialised only so the shared a11y walk can report each item as its own
// RoleTab node with the right name, selected state and bounds. It draws
// nothing (the TabBar paints the items) and handles no events.
type tabItemNode struct {
	Base
	item     TabItem
	selected bool
}

// A11y reports the node as a tab named by its label (or icon when the label is
// empty), with Value "selected" on the active item and "" otherwise — the
// standard tab selected-state string a host maps to aria-selected.
func (n *tabItemNode) A11y() A11yInfo {
	name := n.item.Label
	if name == "" {
		name = n.item.Icon
	}
	return A11yInfo{Role: RoleTab, Name: name, Value: stateValue(n.selected, "selected")}
}

// Compile-time checks: the TabBar and its synthetic item node both satisfy
// Accessible so the accessibility walk never silently skips them.
var (
	_ Accessible = (*TabBar)(nil)
	_ Accessible = (*tabItemNode)(nil)
)
