// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// ActionSheet is a modal panel that rises from the bottom edge over a dimming
// scrim — the "sheet" every mobile platform reaches for when a choice or a
// piece of transient content belongs at the thumb, not in the middle of the
// screen. It fills the whole surface (like [Dialog] and a modal [Overlay]) so
// it owns the scrim and the z-order, and it slides its panel up on open and
// down on dismiss.
//
// It serves the two shapes the pattern always comes in:
//
//   - ACTION mode (the iOS/Material "action sheet"): a vertical stack of
//     touch-sized action buttons plus an optional Cancel button. Every row is
//     sized through [TouchTarget] so it is at least the density's finger floor
//     (44 logical px under [DensityTouch]); a tap fires the action, and Cancel,
//     the scrim, or Esc dismisses. There is no drag-to-dismiss — an action
//     sheet is dismissed by choosing, cancelling or tapping away, exactly like
//     the platform sheets it mirrors.
//
//   - BOTTOM-SHEET mode: a panel hosting an arbitrary Content widget, with a
//     grab handle, DRAG-TO-DISMISS (drag the panel down past a threshold, or
//     fling it down, and it leaves) and optional DETENTS (half / full resting
//     heights). The interactive drag + release runs through the toolkit's
//     deterministic momentum model ([Momentum] + [VelocityTracker]); the
//     programmatic open / dismiss slides run through the animator's easing
//     ([Easing]). Both are clock-free and advanced by [ActionSheet.Tick].
//
// Modality and accessibility. While it is not [ActionSheetClosed] the sheet
// swallows every pointer event inside its bounds (a tap outside the panel is a
// scrim dismiss, not a click that leaks to the content beneath — see
// [ActionSheet.HitTest]); it reports itself to the a11y tree as a
// [RoleDialog] carrying its modal state (see [ActionSheet.A11y]); it exposes
// its action buttons / content to the tree walk (see [ActionSheet.Children]);
// and a Back / Esc key dismisses it. Focus is meant to live inside the sheet
// while it is up, the same modal-grab contract [Dialog] documents.
//
// The presented state is published as an [mvvm.Observable] so app state can
// drive the sheet (Set it true to open) and react to it (subscribe to learn
// when a user-driven dismiss finished), without the app polling the state
// machine.
type ActionSheet struct {
	Base

	// Title is an optional heading drawn in a strip at the top of the panel,
	// under the handle. Empty draws no strip.
	Title string

	// Actions is the vertical list of action buttons in ACTION mode. Each is
	// drawn and hit-tested at a row height clamped up to the density's finger
	// floor via [TouchTarget]. Populate it directly, or use [ActionSheet.AddAction]
	// for the common "run then dismiss" wiring.
	Actions []*Button

	// Cancel is the optional trailing dismiss button in ACTION mode, drawn below
	// the actions with a wider separating gap (the platform "Cancel" convention).
	// Its own OnClick runs first; wire it with [ActionSheet.SetCancel] to also
	// dismiss the sheet.
	Cancel *Button

	// Content is the hosted widget in BOTTOM-SHEET mode. When non-nil the sheet
	// is a bottom sheet (Draggable defaults on) and the Actions/Cancel list is
	// ignored; when nil the sheet is an action sheet.
	Content Widget

	// PreferredHeight is the bottom-sheet panel height in LOGICAL px (routed
	// through [scaled]). Zero means half the surface height. Ignored in ACTION
	// mode, whose height is computed from the rows.
	PreferredHeight int

	// Detents are the resting heights a bottom sheet snaps to on release, given
	// as VISIBLE FRACTIONS of the panel in (0, 1] — 1.0 fully shown, 0.5 half
	// shown. Empty means the single detent [DetentFull]. Kept ascending is
	// conventional but not required; the nearest one wins.
	Detents []float64

	// Draggable enables the interactive drag-to-dismiss / detent behaviour. The
	// constructors set it (on for a bottom sheet, off for an action sheet); a
	// caller may override it. A drag is only started from the handle strip at
	// the top of the panel, so it never steals a press aimed at the content.
	Draggable bool

	// ShowHandle draws the little grab bar at the top of the panel — the visual
	// affordance for the drag. The constructors set it with Draggable.
	ShowHandle bool

	// DismissFraction is the visible-fraction floor for a slow (non-fling)
	// release: let go with less than this fraction of the panel showing and the
	// sheet dismisses instead of settling to a detent. Zero means the default
	// [ActionSheetDismissFraction].
	DismissFraction float64

	// FlingVelocity is the release speed (device px/s, magnitude) at or above
	// which a flick decides the outcome regardless of position: a downward fling
	// dismisses, an upward fling settles to the fullest detent. Zero means the
	// default [ActionSheetFlingVelocity].
	FlingVelocity float64

	// SlideDuration is the programmatic open / dismiss slide length in SECONDS.
	// Zero means the default [ActionSheetSlideDuration]. The interactive settle
	// is governed by the momentum spring, not this.
	SlideDuration float64

	// FrameSeconds is the per-frame time (seconds) [ActionSheet.OnEvent] hands
	// the velocity tracker for a drag-move, since a raw pointer event carries no
	// timestamp. Zero means the default [ActionSheetFrameSeconds] (~60 Hz). Tests
	// and hosts that know their real frame period may drive the drag through the
	// explicit [ActionSheet.DragMove] instead.
	FrameSeconds float64

	// ScrimAlpha is the opacity (0..255) of the dimming scrim at full visibility;
	// it fades in proportion as the panel is dragged away. Zero means the default
	// [ActionSheetScrimAlpha].
	ScrimAlpha uint8

	// OnDismiss fires once, after a dismiss slide has fully completed and the
	// sheet has reached [ActionSheetClosed].
	OnDismiss func()

	state    ActionSheetState
	sheetHpx int     // computed panel height in device px (from SetBounds)
	hidden   float64 // px of the panel hidden below the bottom edge; 0 = fully shown

	slide sheetSlide // programmatic open/dismiss easing
	phys  *Momentum  // interactive drag + settle/dismiss spring
	vt    VelocityTracker

	dragging  bool
	dragLastY int

	presented *mvvm.Observable[bool]
}

// ActionSheetState is the sheet's open/dismiss state-machine position.
type ActionSheetState int

const (
	// ActionSheetClosed is the resting hidden state: the sheet draws nothing,
	// is event-transparent ([ActionSheet.HitTest] returns false) and ignores
	// input. The zero value, so a freshly built sheet is closed.
	ActionSheetClosed ActionSheetState = iota
	// ActionSheetOpening is the programmatic slide-in (animator easing).
	ActionSheetOpening
	// ActionSheetOpen is fully shown at a detent, at rest, interactive.
	ActionSheetOpen
	// ActionSheetDragging is a finger-driven drag in progress (no animation;
	// the panel tracks the finger with a rubber band at the top edge).
	ActionSheetDragging
	// ActionSheetSettling is the post-release momentum spring toward a detent.
	ActionSheetSettling
	// ActionSheetDismissing is the slide/spring to fully hidden; on completion
	// the sheet reaches ActionSheetClosed and fires OnDismiss.
	ActionSheetDismissing
)

// Detent visible-fraction constants for the common bottom-sheet stops.
const (
	// DetentFull shows the whole panel.
	DetentFull = 1.0
	// DetentHalf shows the bottom half of the panel.
	DetentHalf = 0.5
)

// ActionSheet metric + tuning defaults, all in LOGICAL px / seconds unless
// noted. Pixel metrics route through [scaled] so they follow HiDPI and density.
const (
	// ActionSheetPad is the inner margin around the panel body.
	ActionSheetPad = 10
	// ActionSheetGap is the vertical gap between adjacent action rows.
	ActionSheetGap = 8
	// ActionSheetCancelGap is the wider gap separating Cancel from the actions.
	ActionSheetCancelGap = 16
	// ActionSheetRowH is the base action-row height, before [TouchTarget] clamps
	// it up to the density's finger floor.
	ActionSheetRowH = 44
	// ActionSheetTitleH is the title strip height.
	ActionSheetTitleH = 28
	// ActionSheetHandleH is the grab-handle strip height (the drag zone).
	ActionSheetHandleH = 22
	// ActionSheetHandleBarW / ActionSheetHandleBarH size the little grab bar.
	ActionSheetHandleBarW = 40
	ActionSheetHandleBarH = 5
	// ActionSheetCornerR rounds the panel's top corners.
	ActionSheetCornerR = 14

	// ActionSheetScrimAlpha is the default scrim opacity at full visibility.
	ActionSheetScrimAlpha = 120
	// ActionSheetDismissFraction is the default slow-release dismiss floor.
	ActionSheetDismissFraction = 0.5
	// ActionSheetFlingVelocity is the default fling decision speed (px/s).
	ActionSheetFlingVelocity = 700.0
	// ActionSheetSlideDuration is the default open/dismiss slide length (s).
	ActionSheetSlideDuration = 0.24
	// ActionSheetFrameSeconds is the default per-frame time OnEvent assumes.
	ActionSheetFrameSeconds = 1.0 / 60.0
)

// NewActionSheet builds a closed ACTION sheet: a vertical list of the given
// action buttons under an optional title. Drag is off (an action sheet is
// dismissed by choosing, cancelling or tapping the scrim). Call
// [ActionSheet.Open] to present it.
func NewActionSheet(title string, actions ...*Button) *ActionSheet {
	return &ActionSheet{Title: title, Actions: actions}
}

// NewBottomSheet builds a closed BOTTOM sheet hosting content, with the grab
// handle and drag-to-dismiss enabled. Add detents via the Detents field (it
// defaults to a single full detent). Call [ActionSheet.Open] to present it.
func NewBottomSheet(content Widget) *ActionSheet {
	return &ActionSheet{Content: content, Draggable: true, ShowHandle: true}
}

// AddAction appends an action row labelled label whose tap runs fn and then
// dismisses the sheet — the usual action-sheet wiring — and returns the created
// Button so the caller can restyle it (e.g. Style = ButtonDanger). fn may be nil.
func (a *ActionSheet) AddAction(label string, fn func()) *Button {
	b := NewButton(label, nil)
	b.OnClick = func() {
		if fn != nil {
			fn()
		}
		a.Dismiss()
	}
	a.Actions = append(a.Actions, b)
	return b
}

// SetCancel installs the trailing Cancel button: its tap runs fn (if any) and
// dismisses. Returns the Button for restyling. fn may be nil.
func (a *ActionSheet) SetCancel(label string, fn func()) *Button {
	b := NewButton(label, nil)
	b.Style = ButtonSecondary
	b.OnClick = func() {
		if fn != nil {
			fn()
		}
		a.Dismiss()
	}
	a.Cancel = b
	return b
}

// State returns the sheet's current state-machine position.
func (a *ActionSheet) State() ActionSheetState { return a.state }

// Visible reports whether the sheet is anything other than fully closed.
func (a *ActionSheet) Visible() bool { return a.state != ActionSheetClosed }

// Presented is the [mvvm.Observable] carrying the sheet's presented (open)
// state: it becomes true when a present begins and false once a dismiss has
// fully completed. Lazily allocated, so a caller can subscribe or Set it.
// Setting it true opens the sheet; setting it false dismisses it.
func (a *ActionSheet) Presented() *mvvm.Observable[bool] {
	if a.presented == nil {
		a.presented = mvvm.NewObservable(false)
		a.presented.Subscribe(func(v bool) {
			if v {
				a.Open()
			} else {
				a.Dismiss()
			}
		})
	}
	return a.presented
}

// setPresented publishes the presented state. It compares first so an unchanged
// value is a no-op (matching Observable's own change semantics), and relies on
// the state guards in Open/Dismiss to make the subscription's re-entrant call a
// no-op — Observable has no mute, so the state machine is the guard.
func (a *ActionSheet) setPresented(v bool) {
	p := a.Presented()
	if p.Get() != v {
		p.Set(v)
	}
}

// SetBounds records the surface rectangle, recomputes the panel height for the
// current mode + density, and re-clamps the hidden offset into range.
func (a *ActionSheet) SetBounds(r Rect) {
	a.Base.SetBounds(r)
	a.sheetHpx = a.computeSheetH()
	if a.hidden > float64(a.sheetHpx) {
		a.hidden = float64(a.sheetHpx)
	}
	if a.hidden < 0 {
		a.hidden = 0
	}
}

// computeSheetH derives the panel height in device px: from the rows in ACTION
// mode, or from PreferredHeight (default half the surface) in BOTTOM-SHEET mode.
func (a *ActionSheet) computeSheetH() int {
	r := a.Bounds()
	pad := scaled(ActionSheetPad)
	top := pad + a.chromeH()
	if a.Content != nil {
		h := scaled(a.PreferredHeight)
		if a.PreferredHeight == 0 {
			h = r.H / 2
		}
		if h > r.H && r.H > 0 {
			h = r.H
		}
		if h < top+pad {
			h = top + pad
		}
		return h
	}
	row := a.actionRowH()
	body := 0
	if n := len(a.Actions); n > 0 {
		body += n*row + (n-1)*scaled(ActionSheetGap)
	}
	if a.Cancel != nil {
		body += scaled(ActionSheetCancelGap) + row
	}
	return top + body + pad
}

// chromeH is the height consumed above the body by the handle strip and title
// strip (each present only when enabled / set).
func (a *ActionSheet) chromeH() int {
	h := 0
	if a.ShowHandle {
		h += scaled(ActionSheetHandleH)
	}
	if a.Title != "" {
		h += scaled(ActionSheetTitleH)
	}
	return h
}

// actionRowH is the drawn + hit height of an action row: the base row height
// scaled for HiDPI/density, then clamped up to the density's finger floor via
// [TouchTarget]. It is therefore always at least [MinHitTarget], and never
// below the 44 logical-px floor under [DensityTouch].
func (a *ActionSheet) actionRowH() int { return TouchTarget(scaled(ActionSheetRowH)) }

// panelRect is the sheet panel's current surface rectangle, anchored to the
// bottom edge and shifted down by the hidden offset. Its bottom (and rounded
// bottom corners) sit below the surface when hidden > 0 — the painter clips
// them — leaving the rounded top edge on screen.
func (a *ActionSheet) panelRect() Rect {
	r := a.Bounds()
	hp := int(math.Round(a.hidden))
	return Rect{
		X: r.X,
		Y: r.Y + r.H - a.sheetHpx + hp,
		W: r.W,
		H: a.sheetHpx,
	}
}

// handleRect is the top strip of the panel that starts a drag (and shows the
// grab bar). Empty when the handle is off.
func (a *ActionSheet) handleRect() Rect {
	if !a.ShowHandle {
		return Rect{}
	}
	p := a.panelRect()
	return Rect{X: p.X, Y: p.Y, W: p.W, H: scaled(ActionSheetHandleH)}
}

// bodyTop is the panel-relative Y at which the body (actions/content) begins,
// below the handle + title chrome.
func (a *ActionSheet) bodyTop() int {
	return a.panelRect().Y + scaled(ActionSheetPad) + a.chromeH()
}

// actionRects returns the surface rectangles for the action rows in order, plus
// the Cancel rectangle (zero when there is no Cancel). Shared by Draw and
// OnEvent so what is drawn and what is hit are always the same geometry.
func (a *ActionSheet) actionRects() (rows []Rect, cancel Rect) {
	p := a.panelRect()
	pad := scaled(ActionSheetPad)
	row := a.actionRowH()
	x := p.X + pad
	w := p.W - 2*pad
	y := a.bodyTop()
	rows = make([]Rect, len(a.Actions))
	for i := range a.Actions {
		rows[i] = Rect{X: x, Y: y, W: w, H: row}
		y += row + scaled(ActionSheetGap)
	}
	if a.Cancel != nil {
		y += scaled(ActionSheetCancelGap) - scaled(ActionSheetGap)
		cancel = Rect{X: x, Y: y, W: w, H: row}
	}
	return rows, cancel
}

// contentRect is the surface rectangle the hosted Content fills in bottom-sheet
// mode, between the chrome and the bottom pad.
func (a *ActionSheet) contentRect() Rect {
	p := a.panelRect()
	pad := scaled(ActionSheetPad)
	top := a.bodyTop()
	return Rect{X: p.X + pad, Y: top, W: p.W - 2*pad, H: p.Y + p.H - pad - top}
}

// layout positions the child widgets at their current rectangles so both Draw
// and OnEvent read one consistent geometry.
func (a *ActionSheet) layout() {
	if a.sheetHpx <= 0 {
		return
	}
	if a.Content != nil {
		a.Content.SetBounds(a.contentRect())
		return
	}
	rows, cancel := a.actionRects()
	for i, b := range a.Actions {
		if b != nil {
			b.SetBounds(rows[i])
		}
	}
	if a.Cancel != nil {
		a.Cancel.SetBounds(cancel)
	}
}

// visibleFraction is the shown portion of the panel in [0, 1]: 1 fully shown,
// 0 fully hidden. Used to fade the scrim and to decide a slow-release dismiss.
func (a *ActionSheet) visibleFraction() float64 {
	if a.sheetHpx <= 0 {
		return 0
	}
	f := (float64(a.sheetHpx) - a.hidden) / float64(a.sheetHpx)
	return clampUnit(f)
}

// --- state-machine drivers ----------------------------------------------

// Open presents the sheet: it slides the panel up from fully hidden to its
// fullest detent via the animator easing. A no-op when already open or opening.
func (a *ActionSheet) Open() {
	if a.state == ActionSheetOpen || a.state == ActionSheetOpening {
		return
	}
	if a.state == ActionSheetClosed {
		a.hidden = float64(a.sheetHpx)
	}
	a.stopPhysics()
	// Set the state BEFORE publishing so the observable subscription's
	// re-entrant Open() call sees ActionSheetOpening and early-returns.
	a.state = ActionSheetOpening
	a.slide.start(a.hidden, a.openHidden(), a.slideDur(), EaseOutCubic)
	a.setPresented(true)
}

// Dismiss slides the panel down to fully hidden via the animator easing, then
// (on completion) closes the sheet and fires OnDismiss. A no-op when already
// closed or dismissing. This is the programmatic / Esc / scrim / Cancel path;
// an interactive drag-release dismiss instead uses the momentum spring.
func (a *ActionSheet) Dismiss() {
	if a.state == ActionSheetClosed || a.state == ActionSheetDismissing {
		return
	}
	a.dragging = false
	a.stopPhysics()
	a.slide.start(a.hidden, float64(a.sheetHpx), a.slideDur(), EaseInCubic)
	a.state = ActionSheetDismissing
}

// openHidden is the hidden offset of the fullest configured detent (the most
// visible one), i.e. where Open settles the panel.
func (a *ActionSheet) openHidden() float64 {
	max := 0.0
	for _, f := range a.detents() {
		if f > max {
			max = f
		}
	}
	if max <= 0 {
		max = DetentFull
	}
	return a.hiddenForFraction(max)
}

func (a *ActionSheet) detents() []float64 {
	if len(a.Detents) == 0 {
		return []float64{DetentFull}
	}
	return a.Detents
}

func (a *ActionSheet) hiddenForFraction(f float64) float64 {
	return float64(a.sheetHpx) * (1 - f)
}

// nearestDetentHidden returns the hidden offset of the detent closest to the
// current position — where a settle glide targets.
func (a *ActionSheet) nearestDetentHidden() float64 {
	best := a.openHidden()
	bestD := math.Abs(a.hidden - best)
	for _, f := range a.detents() {
		h := a.hiddenForFraction(f)
		if d := math.Abs(a.hidden - h); d < bestD {
			best, bestD = h, d
		}
	}
	return best
}

func (a *ActionSheet) slideDur() float64 {
	if a.SlideDuration > 0 {
		return a.SlideDuration
	}
	return ActionSheetSlideDuration
}

func (a *ActionSheet) dismissFraction() float64 {
	if a.DismissFraction > 0 {
		return a.DismissFraction
	}
	return ActionSheetDismissFraction
}

func (a *ActionSheet) flingVelocity() float64 {
	if a.FlingVelocity > 0 {
		return a.FlingVelocity
	}
	return ActionSheetFlingVelocity
}

func (a *ActionSheet) frameSeconds() float64 {
	if a.FrameSeconds > 0 {
		return a.FrameSeconds
	}
	return ActionSheetFrameSeconds
}

// engine lazily builds the momentum engine used for the drag and the release
// spring.
func (a *ActionSheet) engine() *Momentum {
	if a.phys == nil {
		a.phys = NewMomentum()
	}
	return a.phys
}

// stopPhysics halts any in-flight momentum glide so an animator slide can take
// over cleanly.
func (a *ActionSheet) stopPhysics() {
	if a.phys != nil {
		a.phys.Stop()
	}
}

// --- interactive drag (momentum model) ----------------------------------

// DragBegin starts an interactive drag from surface-Y y. It seeds the momentum
// engine at the current position with the panel travel [0, sheetH] as bounds
// (so a drag up past the full detent rubber-bands), resets the velocity tracker
// and enters the dragging state. A no-op when the sheet is not draggable or not
// present.
func (a *ActionSheet) DragBegin(y int) {
	if !a.Draggable || a.state == ActionSheetClosed {
		return
	}
	e := a.engine()
	e.SetBounds(0, float64(a.sheetHpx))
	e.SetOffset(a.hidden)
	e.BeginDrag()
	a.vt.Reset()
	a.dragging = true
	a.dragLastY = y
	a.slide.active = false
	a.state = ActionSheetDragging
}

// DragMove feeds one drag sample: the finger is now at surface-Y y, dt seconds
// after the previous sample. Moving down (increasing y) hides the panel; the
// panel tracks the finger, rubber-banding past the full detent. dt feeds the
// velocity tracker so the release knows the flick speed. A no-op unless a drag
// is active.
func (a *ActionSheet) DragMove(y int, dt float64) {
	if !a.dragging {
		return
	}
	dy := float64(y - a.dragLastY)
	a.dragLastY = y
	a.engine().DragBy(dy)
	a.vt.Sample(dy, dt)
	a.hidden = a.engine().Offset()
}

// DragRelease ends the drag and decides the outcome from the tracked release
// velocity and the current position, then hands the panel to the momentum
// spring to settle onto the chosen target:
//
//   - a downward fling (velocity >= FlingVelocity) dismisses;
//   - an upward fling (velocity <= -FlingVelocity) settles to the fullest detent;
//   - otherwise, a release with less than DismissFraction of the panel showing
//     dismisses, and any other release settles to the nearest detent.
//
// The spring is seeded with the release velocity, so a firmer flick reaches the
// target faster. A no-op unless a drag is active.
func (a *ActionSheet) DragRelease() {
	if !a.dragging {
		return
	}
	a.dragging = false
	v := a.vt.Velocity()
	fling := a.flingVelocity()

	var target float64
	dismiss := false
	switch {
	case v >= fling:
		target, dismiss = float64(a.sheetHpx), true
	case v <= -fling:
		target = a.openHidden()
	default:
		if a.visibleFraction() < a.dismissFraction() {
			target, dismiss = float64(a.sheetHpx), true
		} else {
			target = a.nearestDetentHidden()
		}
	}

	e := a.engine()
	e.SetBounds(target, target)
	e.Fling(v)
	a.hidden = e.Offset()
	if dismiss {
		a.state = ActionSheetDismissing
	} else {
		a.state = ActionSheetSettling
	}
	// A release that lands exactly on the target with no speed leaves the spring
	// already at rest; finish the transition now rather than waiting for a Tick
	// that would find nothing settling.
	if !e.Settling() {
		a.hidden = target
		a.finishMotion()
	}
}

// --- animation tick (Animator) ------------------------------------------

// Tick advances whichever motion is live — the programmatic animator slide or
// the interactive momentum spring — by dt seconds, updating the panel position
// and firing the terminal state transition (Open reached, or Closed + OnDismiss
// on a completed dismiss). A no-op when nothing is animating. Implements
// [Animator].
func (a *ActionSheet) Tick(dt float64) {
	switch {
	case a.slide.active:
		a.hidden = a.slide.advance(dt)
		if a.slide.done() {
			a.hidden = a.slide.to
			a.slide.active = false
			a.finishMotion()
		}
	case a.phys != nil && a.phys.Settling():
		a.phys.Tick(dt)
		a.hidden = a.phys.Offset()
		if !a.phys.Settling() {
			a.finishMotion()
		}
	}
}

// finishMotion applies the state transition a completed slide/spring implies.
func (a *ActionSheet) finishMotion() {
	switch a.state {
	case ActionSheetOpening, ActionSheetSettling:
		a.state = ActionSheetOpen
	case ActionSheetDismissing:
		a.state = ActionSheetClosed
		a.setPresented(false)
		if a.OnDismiss != nil {
			a.OnDismiss()
		}
	}
}

// Animating reports whether the sheet still needs frames: a live animator slide
// or a settling momentum spring. Implements [Animator], so [TickTree] /
// [TreeAnimating] drive it.
func (a *ActionSheet) Animating() bool {
	return a.slide.active || (a.phys != nil && a.phys.Settling())
}

// --- drawing -------------------------------------------------------------

// Draw paints the dimming scrim (its alpha fading with the panel's visibility),
// then the panel: a rounded-top surface with a border, an optional grab bar and
// title, and the action rows or hosted content. Nothing is drawn while closed.
func (a *ActionSheet) Draw(p painter.Painter, theme *Theme) {
	if a.state == ActionSheetClosed || a.sheetHpx <= 0 {
		return
	}
	a.layout()
	r := a.Bounds()

	// Scrim, faded by visibility so it lightens as the panel is dragged away.
	base := a.ScrimAlpha
	if base == 0 {
		base = ActionSheetScrimAlpha
	}
	alpha := uint8(math.Round(float64(base) * a.visibleFraction()))
	if alpha > 0 {
		fillRect(p, r.X, r.Y, r.W, r.H, painter.RGBA{A: alpha})
	}

	// Panel.
	panel := a.panelRect()
	cr := scaled(ActionSheetCornerR)
	fillRoundRect(p, panel.X, panel.Y, panel.W, panel.H, cr, theme.Background)
	strokeRoundRect(p, panel.X, panel.Y, panel.W, panel.H, cr, theme.Border)

	if a.ShowHandle {
		hb := a.handleRect()
		bw := scaled(ActionSheetHandleBarW)
		bh := scaled(ActionSheetHandleBarH)
		bx := hb.X + (hb.W-bw)/2
		by := hb.Y + (hb.H-bh)/2
		fillRoundRect(p, bx, by, bw, bh, bh/2, theme.Border)
	}
	if a.Title != "" {
		ty := panel.Y + scaled(ActionSheetPad)
		if a.ShowHandle {
			ty += scaled(ActionSheetHandleH)
		}
		tw := a.textWidth(a.Title)
		a.drawText(p, panel.X+(panel.W-tw)/2, ty, a.Title, theme.OnSurface)
	}

	if a.Content != nil {
		a.Content.Draw(p, theme)
		return
	}
	for _, b := range a.Actions {
		if b != nil {
			b.Draw(p, theme)
		}
	}
	if a.Cancel != nil {
		a.Cancel.Draw(p, theme)
	}
}

// --- events --------------------------------------------------------------

// HitTest makes the sheet a modal shield while it is visible: it catches every
// pointer event inside its bounds (so a tap outside the panel is a scrim
// dismiss, never a click leaking to the content beneath). While closed it is
// event-transparent, letting clicks pass through to whatever is behind it.
func (a *ActionSheet) HitTest(px, py int) bool {
	if a.state == ActionSheetClosed {
		return false
	}
	return a.Bounds().Contains(px, py)
}

// OnEvent routes input while the sheet is visible: Back/Esc dismisses; a press
// outside the panel dismisses (scrim tap); a press on the grab strip starts a
// drag (bottom-sheet mode); and presses on the body reach the action buttons /
// content. Drag moves feed the momentum tracker using FrameSeconds, since a
// pointer event carries no timestamp. Widget-local coordinates are converted to
// surface coordinates for hit-testing, and translated into each child's frame
// for forwarding.
func (a *ActionSheet) OnEvent(ev Event) {
	if a.state == ActionSheetClosed {
		return
	}
	a.layout()
	switch ev.Kind {
	case EventKeyDown:
		if isDismissKey(ev.Code) {
			a.Dismiss()
		}
	case EventClick, EventTouchStart:
		a.onPress(ev)
	case EventMouseDrag, EventTouchMove:
		if a.dragging {
			a.DragMove(a.surfaceY(ev), a.frameSeconds())
		} else {
			a.forwardToBody(ev)
		}
	case EventMouseUp, EventTouchEnd:
		if a.dragging {
			a.DragRelease()
		} else {
			a.forwardToBody(ev)
		}
	case EventMouseMove:
		a.forwardToBody(ev)
	}
}

// surfaceY converts a widget-local event Y into surface coordinates.
func (a *ActionSheet) surfaceY(ev Event) int { return ev.Y + a.Bounds().Y }

// surfacePoint converts a widget-local event position into surface coordinates.
func (a *ActionSheet) surfacePoint(ev Event) (int, int) {
	b := a.Bounds()
	return ev.X + b.X, ev.Y + b.Y
}

// onPress handles a primary press: scrim dismiss outside the panel, a drag start
// on the handle strip, or forwarding into the body.
func (a *ActionSheet) onPress(ev Event) {
	sx, sy := a.surfacePoint(ev)
	if !a.panelRect().Contains(sx, sy) {
		a.Dismiss()
		return
	}
	if a.Draggable && a.handleRect().Contains(sx, sy) {
		a.DragBegin(sy)
		return
	}
	a.forwardToBody(ev)
}

// forwardToBody forwards ev to whichever body child covers it, translated into
// that child's frame. In bottom-sheet mode that is the Content; in action mode
// it is the action row / Cancel button under the point.
func (a *ActionSheet) forwardToBody(ev Event) {
	sx, sy := a.surfacePoint(ev)
	if a.Content != nil {
		if a.contentRect().Contains(sx, sy) {
			a.Content.OnEvent(translateEvent(ev, a.Bounds(), a.Content.Bounds()))
		}
		return
	}
	rows, cancel := a.actionRects()
	for i, rr := range rows {
		if rr.Contains(sx, sy) && a.Actions[i] != nil {
			a.Actions[i].OnEvent(translateEvent(ev, a.Bounds(), a.Actions[i].Bounds()))
			return
		}
	}
	if a.Cancel != nil && cancel.Contains(sx, sy) {
		a.Cancel.OnEvent(translateEvent(ev, a.Bounds(), a.Cancel.Bounds()))
	}
}

// isDismissKey reports whether a key code is a "close this modal" key: Escape,
// or a platform Back key (Android hardware / browser back), so a back gesture
// dismisses the sheet exactly like tapping the scrim.
func isDismissKey(code string) bool {
	switch code {
	case "Escape", "Esc", "Back", "GoBack", "BrowserBack":
		return true
	}
	return false
}

// --- accessibility -------------------------------------------------------

// A11y reports the sheet as a dialog named by its Title, carrying its modal
// state so a screen reader announces a live sheet as a modal dialog and a closed
// one as nothing of note. The action buttons / content are exposed separately
// through [ActionSheet.Children] so the a11y tree walk descends into them.
func (a *ActionSheet) A11y() A11yInfo {
	return A11yInfo{Role: RoleDialog, Name: a.Title, Value: stateValue(a.Visible(), "modal")}
}

// Children yields the sheet's interactive contents in visual order — the hosted
// Content in bottom-sheet mode, or the action rows followed by Cancel in action
// mode — so [WalkA11y] and every other generic tree walk reaches them.
func (a *ActionSheet) Children() []Widget {
	if a.Content != nil {
		return nonNil(a.Content)
	}
	out := make([]Widget, 0, len(a.Actions)+1)
	for _, b := range a.Actions {
		if b != nil {
			out = append(out, b)
		}
	}
	if a.Cancel != nil {
		out = append(out, a.Cancel)
	}
	return out
}

// --- animator helper -----------------------------------------------------

// sheetSlide is a seconds-based scalar ease used for the programmatic open /
// dismiss slides: it advances an elapsed time by dt and reports the eased value
// between from and to. It is the animator counterpart of the interactive
// momentum spring — clock-free, deterministic, and reusing the package [Easing]
// curves — so given the same dt sequence it lands on exactly the same positions
// every time. (It is seconds-based, matching the [Animator] dt contract, rather
// than the frame-tick [Tween].)
type sheetSlide struct {
	from, to     float64
	dur, elapsed float64
	ease         Easing
	active       bool
}

// start arms the slide from `from` to `to` over dur seconds using ease (nil =
// Linear). A dur <= 0 makes it immediately done at `to` on the first advance.
func (s *sheetSlide) start(from, to, dur float64, ease Easing) {
	if ease == nil {
		ease = Linear
	}
	*s = sheetSlide{from: from, to: to, dur: dur, ease: ease, active: true}
}

// advance moves the slide forward by dt seconds (clamped at dur) and returns the
// current eased value. A non-positive dt does not advance but still reports the
// current value.
func (s *sheetSlide) advance(dt float64) float64 {
	if dt > 0 {
		s.elapsed += dt
		if s.elapsed > s.dur {
			s.elapsed = s.dur
		}
	}
	return s.value()
}

// value is the current eased position between from and to, without advancing.
func (s *sheetSlide) value() float64 {
	if s.dur <= 0 {
		return s.to
	}
	return s.from + (s.to-s.from)*s.ease(clampUnit(s.elapsed/s.dur))
}

// done reports whether the slide has run its full duration.
func (s *sheetSlide) done() bool { return s.elapsed >= s.dur }

// Compile-time checks: ActionSheet is an animatable, accessible container.
var (
	_ Animator   = (*ActionSheet)(nil)
	_ Accessible = (*ActionSheet)(nil)
	_ Widget     = (*ActionSheet)(nil)
)
