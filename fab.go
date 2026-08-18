// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Fab is a floating action button: a circular, raised, Accent-filled action
// that floats over content at a screen corner. It is the primary/most-common
// action on a surface — compose (a mail app), add (a list), post (a feed) —
// rendered as an [Button.ButtonProminent]-coloured disc with an elevation
// shadow so it reads as lifted off the content below.
//
// Placement is corner + margin, not layout: a Fab does not sit inside an HBox
// like a Button — it floats, so a host anchors it with [Fab.AnchorIn] (which
// resolves [Fab.Corner] + [Fab.Margin] through the shared [anchorCorner]
// primitive that Toast and Notification also use) and paints it last / as the
// top [Overlay] layer. Because it floats above everything, [Fab.OnEvent] and
// [Fab.HitTest] work in SURFACE coordinates (like an Overlay layer), not the
// parent-local coordinates a flow-laid widget receives.
//
// Speed-dial (optional): give the Fab [Fab.Actions] and a tap (or long-press,
// or a secondary click) expands a vertical stack of mini action buttons with a
// staggered slide-out animation; picking one fires its callback and collapses
// the dial, and a tap on the scrim collapses it without selecting. Each mini
// button is sized to at least [MinHitTarget] via [TouchTarget] so a fingertip
// always lands one. With no Actions a tap simply fires [Fab.OnTap].
//
// Density: the disc diameter, the mini diameter and the elevation shadow all
// scale through [scaled], and both diameters are clamped up to the density's
// minimum hit target — so the same Fab is a compact 56-px disc under a mouse
// and a generous finger-sized one on a phone, with no per-app code.
//
// Animation is manual-clock, like the rest of the toolkit: the Fab owns no
// goroutine or timer. A host advances it via [Fab.Tick] once per frame (which
// also drives the long-press timer) and consults [Fab.Animating] to stop
// repainting when the dial has fully settled. The expand/collapse target state
// is an [mvvm.Observable] so a view model can bind to it.
type Fab struct {
	Base
	focusState

	// Icon is the short glyph string painted on the disc ("+", "✎", ...),
	// centred in the toolkit's bitmap font. It doubles as the accessible name
	// when Label is empty.
	Icon string
	// Label is the Fab's accessible name (what a screen reader announces). When
	// empty the Icon glyph is used, mirroring how IconButton names itself.
	Label string
	// OnTap is the primary action, fired by a tap/click/Enter/Space when the Fab
	// has no speed-dial Actions. With Actions present a tap toggles the dial
	// instead and OnTap is not used. Nil is a safe no-op.
	OnTap func()
	// Corner is the docking position within the host rect passed to AnchorIn.
	// NewFab defaults it to BottomRight (the Material FAB home); the four true
	// corners and the two *Center positions are all honoured.
	Corner Corner
	// Margin is the logical-pixel inset from the docked edges. Zero (or negative)
	// means the FabMargin default; any positive value overrides it and is routed
	// through scaled so it tracks density + HiDPI.
	Margin int
	// Diameter is the logical-pixel disc diameter. Zero (or negative) means the
	// FabDiameter default; the resolved value is scaled and then clamped up to
	// MinHitTarget so a small custom disc still gives a finger a real target.
	Diameter int
	// Actions is the optional speed-dial: a tap expands them as a stack of mini
	// buttons. Empty leaves the Fab a plain single-action button.
	Actions []*FabAction

	// expanded is the speed-dial's target open/closed state, exposed for MVVM
	// binding via Expanded(). Lazily allocated so a zero-value Fab still runs.
	expanded *mvvm.Observable[bool]
	// state + frame drive the staggered slide animation off the manual clock.
	state fabState
	frame int

	// scrim is the host rect handed to AnchorIn; when the dial is open it is the
	// tap-catching backdrop that collapses the dial on an outside tap.
	scrim Rect

	// gest turns a touch stream into a long-press (expand) and a tap (activate),
	// so a touch-only host gets both without an EventClick. Lazily allocated.
	gest *GestureRecognizer

	// minis are the mini-button widgets mirroring Actions, rebuilt when Actions
	// changes and repositioned every frame — they carry the per-mini bounds a
	// generic a11y walk (WalkA11y) reports when the dial is open.
	minis []*fabMini

	pressed bool
	hovered bool
}

// FabAction is one speed-dial entry: a glyph, an optional accessible label, and
// the callback fired when the mini button is chosen (which also collapses the
// dial).
type FabAction struct {
	// Icon is the mini button's glyph.
	Icon string
	// Label is the mini's accessible name; empty falls back to Icon.
	Label string
	// OnTap fires exactly once when the mini is selected. Nil is a safe no-op
	// (the dial still collapses).
	OnTap func()
}

// a11yName is the mini action's accessible name: its Label, or the Icon glyph
// when no Label was given.
func (a *FabAction) a11yName() string {
	if a.Label != "" {
		return a.Label
	}
	return a.Icon
}

// fabState is the speed-dial's animation phase.
type fabState int

const (
	// fabCollapsed is the resting state: no dial, no scrim, minis hidden.
	fabCollapsed fabState = iota
	// fabExpanding is the slide-out: minis travel from behind the disc to their
	// slots, staggered.
	fabExpanding
	// fabExpanded is fully open: every mini at its slot, scrim at full alpha.
	fabExpanded
	// fabCollapsing is the slide-back: minis retract to behind the disc.
	fabCollapsing
)

// Fab metric defaults, in LOGICAL pixels (routed through scaled). The disc and
// mini diameters match Material's 56/40-dp FAB pair; the margin is the 16-dp
// screen inset; the elevation is the shadow's downward offset; the gap is the
// spacing between stacked minis and the disc.
const (
	// FabDiameter is the default disc diameter.
	FabDiameter = 56
	// FabMiniDiameter is the default speed-dial mini-button diameter.
	FabMiniDiameter = 40
	// FabMargin is the default inset from the docked screen edges.
	FabMargin = 16
	// FabElevation is the elevation shadow's downward offset.
	FabElevation = 6
	// FabActionGap is the spacing between the disc and the first mini, and
	// between successive minis.
	FabActionGap = 12
)

// Speed-dial animation timing, in Tick frames.
const (
	// fabExpandFrames is how many frames one mini takes to fully deploy.
	fabExpandFrames = 8
	// fabStaggerFrames is the per-mini start delay that cascades the stack.
	fabStaggerFrames = 3
)

// fabShadowColor is the translucent black of the elevation shadow, src-over
// composited under the disc.
var fabShadowColor = RGBA{R: 0, G: 0, B: 0, A: 0x40}

// fabScrimMaxAlpha is the peak alpha of the dim backdrop behind an open dial.
const fabScrimMaxAlpha = 0x66

// NewFab builds a BottomRight-anchored Fab carrying the given glyph + primary
// tap handler. onTap may be nil. Add speed-dial entries with AddAction.
func NewFab(icon string, onTap func()) *Fab {
	return &Fab{Icon: icon, OnTap: onTap, Corner: BottomRight}
}

// AddAction appends a speed-dial entry and returns the Fab for chaining. Adding
// an action invalidates the cached mini widgets so the next layout rebuilds
// them.
func (f *Fab) AddAction(icon, label string, onTap func()) *Fab {
	f.Actions = append(f.Actions, &FabAction{Icon: icon, Label: label, OnTap: onTap})
	f.minis = nil
	return f
}

// ensureObs lazily allocates the expanded observable (defaulting closed) so a
// zero-value Fab gains a real observable the moment its state is read or set.
func (f *Fab) ensureObs() {
	if f.expanded == nil {
		f.expanded = mvvm.NewObservable(false)
	}
}

// ensureGest lazily allocates the gesture recognizer, wiring a long-press to
// open the dial and a tap to the same activation path an EventClick takes.
func (f *Fab) ensureGest() {
	if f.gest != nil {
		return
	}
	g := NewGestureRecognizer()
	g.OnLongPress = func(_, _ int) { f.Expand() }
	g.OnTap = func(x, y int) { f.handleClick(x, y) }
	f.gest = g
}

// Expanded returns the observable speed-dial state so a view model can bind to
// (or observe) whether the dial is open. Get() is true from the moment Expand
// begins until Collapse begins.
func (f *Fab) Expanded() *mvvm.Observable[bool] {
	f.ensureObs()
	return f.expanded
}

// IsExpanded reports whether the dial is open or opening — the state in which a
// tap selects a mini or collapses rather than re-activating the disc.
func (f *Fab) IsExpanded() bool {
	return f.state == fabExpanded || f.state == fabExpanding
}

// Expand opens the speed-dial (a no-op with no Actions, or when already open /
// opening), starting the staggered slide-out from frame zero.
func (f *Fab) Expand() {
	if len(f.Actions) == 0 {
		return
	}
	if f.state == fabExpanding || f.state == fabExpanded {
		return
	}
	f.ensureObs()
	f.expanded.Set(true)
	f.state = fabExpanding
	f.frame = 0
}

// Collapse closes the speed-dial (a no-op when already closed / closing),
// starting the retract from frame zero.
func (f *Fab) Collapse() {
	if f.state == fabCollapsed || f.state == fabCollapsing {
		return
	}
	f.ensureObs()
	f.expanded.Set(false)
	f.state = fabCollapsing
	f.frame = 0
}

// Toggle opens a closed dial and closes an open one.
func (f *Fab) Toggle() {
	if f.IsExpanded() {
		f.Collapse()
		return
	}
	f.Expand()
}

// totalFrames is the animation length: one mini's deploy time plus the stagger
// accumulated across the rest of the stack. With no Actions it is the bare
// deploy time (never used, since Expand needs Actions).
func (f *Fab) totalFrames() int {
	n := len(f.Actions)
	if n <= 1 {
		return fabExpandFrames
	}
	return fabExpandFrames + (n-1)*fabStaggerFrames
}

// Tick advances the animation and the long-press timer by one frame. dt is
// accepted for the [Animator] contract; the discrete slide is frame-driven (one
// step per call), matching the Tween / GestureRecognizer manual-clock model.
func (f *Fab) Tick(dt float64) {
	_ = dt
	f.ensureGest()
	f.gest.Tick()
	switch f.state {
	case fabExpanding:
		if f.frame < f.totalFrames() {
			f.frame++
		}
		if f.frame >= f.totalFrames() {
			f.state = fabExpanded
		}
	case fabCollapsing:
		if f.frame < f.totalFrames() {
			f.frame++
		}
		if f.frame >= f.totalFrames() {
			f.state = fabCollapsed
			f.frame = 0
		}
	}
}

// Animating reports whether the dial is mid-slide and still needs frames.
func (f *Fab) Animating() bool {
	return f.state == fabExpanding || f.state == fabCollapsing
}

// diameter is the disc's device-pixel diameter: the resolved logical diameter
// scaled for density + HiDPI, then clamped up to the minimum hit target.
func (f *Fab) diameter() int {
	dl := f.Diameter
	if dl <= 0 {
		dl = FabDiameter
	}
	return TouchTarget(scaled(dl))
}

// miniDiameter is a speed-dial mini button's device-pixel diameter, likewise
// clamped up to the minimum hit target so every mini is finger-sized.
func (f *Fab) miniDiameter() int {
	return TouchTarget(scaled(FabMiniDiameter))
}

// margin is the docked-edge inset in device pixels.
func (f *Fab) margin() int {
	m := f.Margin
	if m <= 0 {
		m = FabMargin
	}
	return scaled(m)
}

// AnchorIn records host as the scrim and places the disc at Corner, inset by
// Margin — the convenience over a host computing SetBounds by hand, mirroring
// Notification.AnchorIn / Toast.AnchorIn.
func (f *Fab) AnchorIn(host Rect) {
	f.scrim = host
	d := f.diameter()
	f.SetBounds(anchorCorner(host, d, d, f.Corner, f.margin(), 0))
}

// stacksDown reports whether the speed-dial grows downward (top corners) rather
// than upward (bottom corners): minis deploy away from the nearer screen edge.
func (f *Fab) stacksDown() bool {
	switch f.Corner {
	case TopLeft, TopRight, TopCenter:
		return true
	default:
		return false
	}
}

// miniProgress is mini i's slide progress in [0, 1] at the current state/frame:
// 0 hidden behind the disc, 1 fully at its slot. Expanding eases out with a
// per-mini stagger; collapsing eases in and retracts in reverse order so the
// stack folds neatly back.
func (f *Fab) miniProgress(i int) float64 {
	if f.state == fabExpanded {
		return 1
	}
	if f.state == fabCollapsed {
		return 0
	}
	if f.state == fabExpanding {
		local := f.frame - i*fabStaggerFrames
		return EaseOutCubic(clampUnit(float64(local) / float64(fabExpandFrames)))
	}
	// fabCollapsing: retract last-deployed first.
	n := len(f.Actions)
	local := f.frame - (n-1-i)*fabStaggerFrames
	return 1 - EaseInCubic(clampUnit(float64(local)/float64(fabExpandFrames)))
}

// overallProgress is the whole dial's 0..1 openness, used to fade the scrim.
func (f *Fab) overallProgress() float64 {
	if f.state == fabExpanded {
		return 1
	}
	if f.state == fabCollapsed {
		return 0
	}
	frac := clampUnit(float64(f.frame) / float64(f.totalFrames()))
	if f.state == fabExpanding {
		return frac
	}
	return 1 - frac
}

// miniRect is mini i's current device-pixel rectangle in SURFACE coordinates,
// interpolated from behind the disc (progress 0) to its slot (progress 1) along
// the stack axis, horizontally centred on the disc.
func (f *Fab) miniRect(i int) Rect {
	b := f.Bounds()
	d := b.W
	md := f.miniDiameter()
	gap := scaled(FabActionGap)
	step := md + gap
	x := b.X + (d-md)/2
	collapsedTop := b.Y + (d-md)/2
	var fullTop int
	if f.stacksDown() {
		fullTop = b.Y + d + gap + i*step
	} else {
		fullTop = b.Y - gap - md - i*step
	}
	top := collapsedTop + fabRound(float64(fullTop-collapsedTop)*f.miniProgress(i))
	return Rect{X: x, Y: top, W: md, H: md}
}

// fabRound rounds a float to the nearest int, away from zero on a .5 tie, for
// either sign — the plain int(v+0.5) truncation only rounds correctly for
// non-negative v, and upward stacks give negative travel.
func fabRound(v float64) int {
	if v < 0 {
		return int(v - 0.5)
	}
	return int(v + 0.5)
}

// syncMinis rebuilds the mini widgets when Actions changed and repositions every
// one to its current slide rectangle, so a generic walk reading their Bounds
// (WalkA11y) sees live positions.
func (f *Fab) syncMinis() {
	if len(f.minis) != len(f.Actions) {
		f.minis = make([]*fabMini, len(f.Actions))
		for i := range f.Actions {
			f.minis[i] = &fabMini{}
		}
	}
	for i, a := range f.Actions {
		m := f.minis[i]
		m.icon = a.Icon
		m.name = a.a11yName()
		m.onTap = a.OnTap
		m.SetBounds(f.miniRect(i))
	}
}

// Children yields the mini-button widgets while the dial is open (or animating
// open/closed), and nothing when collapsed — so a generic accessibility walk
// announces the speed-dial actions exactly when they are on screen. Positions
// are refreshed first so each child's Bounds is current.
func (f *Fab) Children() []Widget {
	if f.state == fabCollapsed {
		return nil
	}
	f.syncMinis()
	out := make([]Widget, len(f.minis))
	for i := range f.minis {
		out[i] = f.minis[i]
	}
	return out
}

// HitTest reports whether a SURFACE point is sensitive. Collapsed, only the disc
// is (taps elsewhere fall through to content). Open, the scrim captures the
// whole host so an outside tap collapses the dial; without a scrim (a Fab placed
// by SetBounds rather than AnchorIn) it falls back to the disc plus the mini
// rectangles.
func (f *Fab) HitTest(px, py int) bool {
	if f.state != fabCollapsed {
		if f.scrim.W > 0 && f.scrim.H > 0 {
			return f.scrim.Contains(px, py)
		}
		f.syncMinis()
		for i := range f.minis {
			if f.minis[i].Bounds().Contains(px, py) {
				return true
			}
		}
	}
	return f.Bounds().Contains(px, py)
}

// OnEvent drives the Fab from SURFACE-coordinate events (it is a floating /
// Overlay-layer widget). A click selects a mini or collapses when the dial is
// open, else activates the disc; a secondary click or a long-press opens the
// dial; Enter/Space activate, Escape collapses. A Disabled Fab ignores
// everything. Touch events feed the gesture recognizer so a touch-only host
// gets tap + long-press without an EventClick.
func (f *Fab) OnEvent(ev Event) {
	if f.Disabled {
		return
	}
	f.ensureGest()
	f.gest.Feed(ev)
	switch ev.Kind {
	case EventClick:
		f.handleClick(ev.X, ev.Y)
	case EventSecondaryClick:
		if f.Bounds().Contains(ev.X, ev.Y) {
			f.Expand()
		}
	case EventKeyDown:
		switch ev.Code {
		case "Enter", " ", "Space":
			f.activatePrimary()
		case "Escape", "Esc":
			f.Collapse()
		}
	case EventMouseMove:
		f.hovered = f.Bounds().Contains(ev.X, ev.Y)
	case EventMouseUp:
		f.pressed = false
	}
}

// handleClick resolves a surface-coordinate click: a mini hit selects it (firing
// its callback once) and collapses; any other click while open collapses; a
// click on the disc while closed activates it.
func (f *Fab) handleClick(x, y int) {
	if f.IsExpanded() {
		for i := range f.Actions {
			if f.miniRect(i).Contains(x, y) {
				f.Collapse()
				if f.Actions[i].OnTap != nil {
					f.Actions[i].OnTap()
				}
				return
			}
		}
		f.Collapse()
		return
	}
	if f.Bounds().Contains(x, y) {
		f.pressed = true
		f.activatePrimary()
	}
}

// activatePrimary is the disc's primary action: toggle the dial when it has
// Actions, else fire OnTap (nil-safe). Shared by click, tap and Enter/Space.
func (f *Fab) activatePrimary() {
	if len(f.Actions) > 0 {
		f.Toggle()
		return
	}
	if f.OnTap != nil {
		f.OnTap()
	}
}

// Draw paints (in back-to-front order) the dim scrim, the mini stack, and the
// raised disc with its elevation shadow, icon and focus ring. A Disabled Fab
// paints only a muted, shadow-less disc.
func (f *Fab) Draw(p painter.Painter, theme *Theme) {
	if f.Disabled {
		f.drawButton(p, theme)
		return
	}
	if f.state != fabCollapsed && f.scrim.W > 0 && f.scrim.H > 0 {
		a := uint8(float64(fabScrimMaxAlpha)*f.overallProgress() + 0.5)
		fillRect(p, f.scrim.X, f.scrim.Y, f.scrim.W, f.scrim.H, RGBA{A: a})
	}
	if f.state != fabCollapsed {
		f.syncMinis()
		for i := len(f.minis) - 1; i >= 0; i-- {
			f.drawMini(p, theme, i)
		}
	}
	f.drawButton(p, theme)
}

// drawMini paints one mini button — a Surface disc with a bordered edge and a
// centred glyph — faded in by its slide progress; a not-yet-emerged mini
// (progress 0) is skipped.
func (f *Fab) drawMini(p painter.Painter, theme *Theme, i int) {
	pr := f.miniProgress(i)
	if pr <= 0 {
		return
	}
	r := f.minis[i].Bounds()
	md := r.W
	radius := md / 2
	fillRoundRect(p, r.X, r.Y, md, md, radius, fabFade(theme.Surface, pr))
	strokeRoundRect(p, r.X, r.Y, md, md, radius, fabFade(theme.Border, pr))
	if ic := f.Actions[i].Icon; ic != "" {
		ink := fabFade(theme.OnSurface, pr)
		tw := f.textWidth(ic)
		tx := r.X + (md-tw)/2
		ty := r.Y + (md-f.glyphHeight())/2
		f.drawText(p, tx, ty, ic, ink)
	}
}

// drawButton paints the disc: an elevation shadow (offset down by the scaled
// FabElevation), an Accent face (darkened while pressed, muted while disabled),
// the centred Icon glyph and the round focus ring.
func (f *Fab) drawButton(p painter.Painter, theme *Theme) {
	b := f.Bounds()
	d := b.W
	if d <= 0 {
		return
	}
	radius := d / 2
	if !f.Disabled {
		el := scaled(FabElevation)
		fillRoundRect(p, b.X, b.Y+el, d, d, radius, fabShadowColor)
	}
	face := theme.Accent
	ink := accentFg(theme)
	if f.pressed {
		face = blendRGBA(theme.Accent, theme.Background, 0.25)
	}
	if f.Disabled {
		face, ink = mutedFace(theme), mutedInk(theme)
	}
	fillRoundRect(p, b.X, b.Y, d, d, radius, face)
	if f.Icon != "" {
		tw := f.textWidth(f.Icon)
		tx := b.X + (d-tw)/2
		ty := b.Y + (d-f.glyphHeight())/2
		f.drawText(p, tx, ty, f.Icon, ink)
	}
	f.FocusRingRadius = radius
	if f.FocusRingWidth < 1 {
		f.FocusRingWidth = strokeWidth()
	}
	f.drawFocusRing(p, theme, b)
}

// fabFade scales c's alpha by t in [0, 1], for the slide-in fade of the mini
// stack; t <= 0 yields a fully transparent colour, t >= 1 the colour unchanged.
func fabFade(c RGBA, t float64) RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return RGBA{R: c.R, G: c.G, B: c.B, A: uint8(float64(c.A)*t + 0.5)}
}

// A11y reports the Fab as a button named by its Label (or Icon), whose Value is
// "expanded" while the speed-dial is open so a reader hears the disc's state.
func (f *Fab) A11y() A11yInfo {
	v := ""
	if f.IsExpanded() {
		v = "expanded"
	}
	return A11yInfo{Role: RoleButton, Name: f.a11yName(), Value: v}
}

// a11yName is the disc's accessible name: its Label, or the Icon glyph when no
// Label was set.
func (f *Fab) a11yName() string {
	if f.Label != "" {
		return f.Label
	}
	return f.Icon
}

// fabMini is a speed-dial mini button: a leaf widget whose sole jobs are to
// carry a per-mini bounds for the accessibility walk and to report itself as a
// button. It is drawn by its owning Fab (via drawMini), so its own Draw is the
// Base no-op.
type fabMini struct {
	Base
	icon  string
	name  string
	onTap func()
}

// A11y reports the mini as a button named by its action's label/glyph.
func (m *fabMini) A11y() A11yInfo {
	return A11yInfo{Role: RoleButton, Name: m.name}
}

// Compile-time checks that Fab and its mini satisfy the toolkit contracts.
var (
	_ Accessible     = (*Fab)(nil)
	_ Animator       = (*Fab)(nil)
	_ Focusable      = (*Fab)(nil)
	_ childContainer = (*Fab)(nil)
	_ Widget         = (*fabMini)(nil)
	_ Accessible     = (*fabMini)(nil)
)
