// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// SwipeActions is the mobile "swipe a list row to reveal actions" wrapper. It
// holds one row of [SwipeActions.Content] plus a leading and a trailing set of
// [SwipeAction]s, and turns a horizontal drag into the familiar reveal: the
// content slides sideways to uncover the actions pinned to the row's edge, snaps
// open or shut at a threshold, and — dragged far enough — fires the set's
// primary (destructive) action outright.
//
// # What it is made of
//
//   - Content is the row body (a [Label], an [ActionRow], any [Widget]). It is
//     drawn shifted by the live reveal offset and clipped to the row, so it
//     appears to glide over the actions beneath it.
//   - Leading / Trailing are the two action sets. Swiping RIGHT (content moves
//     right, offset positive) reveals the leading set at the row's left edge;
//     swiping LEFT (content moves left, offset negative) reveals the trailing
//     set at the right edge — the iOS/Material convention.
//   - Each action is surfaced as a real child [Button] (see [SwipeActions.Children]),
//     so it lands in the accessibility tree with a button role and its label:
//     a screen reader can read and INVOKE it (via the button's own click path,
//     which routes to [SwipeActions.InvokeTrailing] / [SwipeActions.InvokeLeading])
//     without ever performing the drag gesture. That is the a11y-invoke path.
//
// # The reveal, the snap and the destructive full-swipe
//
// A drag tracks the finger one-for-one through the pure-logic [Momentum] engine
// (so a pull past the far edge rubber-bands and a release settles with the same
// deterministic spring the rest of the toolkit uses). On release the widget
// SNAPS, choosing a target from the reveal magnitude (optionally projected by
// the release velocity so a fast flick opens from further out):
//
//   - past the destructive threshold (a large fraction of the row width) AND
//     [SwipeActions.DestructiveFull] is on: the set's primary action fires once,
//     immediately, and the row settles shut — the "full swipe to delete" shortcut;
//   - else past the open threshold (half the set's revealed width): it settles
//     OPEN, resting exactly on the set's full width;
//   - else: it settles shut, resting exactly on 0.
//
// Every rest lands on an exact offset (0, +leadingWidth or -trailingWidth) — the
// [Momentum] spring snaps onto its bound rather than drifting — so the open/closed
// state machine is crisp and testable to the pixel.
//
// # Determinism and the frame loop
//
// Like [GestureRecognizer], [Momentum] and [Animator], SwipeActions owns no clock
// and no goroutine. A drag is driven by the input events a host already routes
// (touch or mouse); the settle is advanced by an explicit [SwipeActions.Tick]
// whose dt (seconds) the host supplies each frame, exactly like [Momentum.Tick].
// Given the same events and dt sequence it produces the same offsets everywhere.
type SwipeActions struct {
	Base

	// Content is the row body slid to reveal the actions. May be nil (an empty
	// row still reveals its actions).
	Content Widget

	// Leading is the action set revealed by a rightward swipe (left edge);
	// Trailing is the set revealed by a leftward swipe (right edge). Either may
	// be empty, in which case a swipe that way rubber-bands against a closed row.
	Leading  []SwipeAction
	Trailing []SwipeAction

	// ActionWidth is the base (logical-pixel) width of one action lane, routed
	// through [Scaled] and clamped up to the density's [MinHitTarget]. NewSwipeActions
	// sets it to a finger-friendly default.
	ActionWidth int

	// DestructiveFull enables the full-swipe shortcut: a drag past the destructive
	// threshold fires the swiped set's primary action directly and closes. When
	// false, a far drag simply settles open like any other.
	DestructiveFull bool

	// DestructiveNum / DestructiveDen express the destructive threshold as a
	// fraction of the row width (DestructiveNum/DestructiveDen). NewSwipeActions
	// sets 3/4: the finger must cross three-quarters of the row to trigger.
	DestructiveNum, DestructiveDen int

	// Projection is how many seconds of the release velocity are added to the
	// reveal offset before the snap decision, so a fast flick opens (or triggers)
	// from further back. 0 makes the snap depend on the resting offset alone.
	Projection float64

	// open is the observable open state, so an app view-model can bind to and
	// react to the row opening/closing through the go-widgets MVVM layer.
	open *mvvm.Observable[SwipeOpenState]

	// mo is the shared settle engine; vt smooths a touch drag's release velocity.
	mo *Momentum
	vt VelocityTracker

	// off is the live content offset in device pixels: >0 reveals leading, <0
	// reveals trailing, 0 is shut. settling gates the Tick-driven settle; target
	// is the exact offset the settle is heading for.
	off      float64
	settling bool
	target   float64

	// Live drag bookkeeping.
	dragging bool
	moved    bool // the drag passed the tap slop on the swipe axis
	vertical bool // the gesture is dominantly vertical — not ours
	oriented bool // the horizontal/vertical lock has been decided
	startX   int  // press position (widget-local), for tap vs drag + orientation
	startY   int
	lastX    int            // previous move position, for per-sample deltas
	preState SwipeOpenState // open state at press, restored on a vertical bail-out

	// leadBtns / trailBtns are the a11y + invoke vehicles, one per action, kept in
	// step with Leading / Trailing by syncButtons.
	leadBtns  []*Button
	trailBtns []*Button
}

// SwipeAction is one revealed action: a coloured lane carrying a label or icon
// that fires OnInvoke when tapped (or invoked through the accessibility tree).
type SwipeAction struct {
	// Label is the action's caption, drawn centred when Icon is nil and used as
	// the accessible name.
	Label string
	// Icon, when set, draws the action's glyph into lane rect r in colour ink —
	// the same callback shape as [Button.Icon]. Optional.
	Icon func(p painter.Painter, r Rect, ink RGBA)
	// Color is the lane's background fill. A zero (fully-transparent) value falls
	// back to a danger red for a Destructive action, else the theme Accent.
	Color RGBA
	// Ink is the label/icon colour. A zero (fully-transparent) value falls back
	// to white, which reads on both the accent and danger fills.
	Ink RGBA
	// Destructive marks the set's primary action — the one a full swipe fires and
	// the one whose colour fills the lane during the destructive-primed drag. When
	// no action in a set is flagged, the set's edge action (nearest the screen
	// edge) is treated as primary.
	Destructive bool
	// OnInvoke is the command body, run once when the action is tapped or invoked.
	// Nil is a no-op.
	OnInvoke func()
}

// SwipeOpenState is the SwipeActions open/closed state machine position.
type SwipeOpenState int

const (
	// SwipeClosed is the row at rest, content covering both action sets.
	SwipeClosed SwipeOpenState = iota
	// SwipeLeadingOpen is the leading set fully revealed at the left edge.
	SwipeLeadingOpen
	// SwipeTrailingOpen is the trailing set fully revealed at the right edge.
	SwipeTrailingOpen
)

// Default SwipeActions tuning.
const (
	// swipeActionWidth is the base logical-pixel width of one action lane.
	swipeActionWidth = 72
	// swipeProjection is how many seconds of release velocity fold into the snap
	// decision by default — a gentle flick assist.
	swipeProjection = 0.05
	// swipeNominalDt is the assumed per-move interval feeding the VelocityTracker
	// when a touch stream carries no timestamps: one 60 Hz frame.
	swipeNominalDt = 1.0 / 60.0
)

// swipeTapSlop is the largest movement (device pixels, on the swipe axis) still
// treated as a tap rather than a drag. It scales with the panel DPI.
func swipeTapSlop() int { return dpiScaled(8) }

// NewSwipeActions wraps content in a shut SwipeActions with the finger-friendly
// defaults (72-logical-pixel lanes, full-swipe-to-primary on at 3/4 of the row,
// a 0.05 s flick projection). Add actions by appending to Leading / Trailing.
func NewSwipeActions(content Widget) *SwipeActions {
	return &SwipeActions{
		Content:         content,
		ActionWidth:     swipeActionWidth,
		DestructiveFull: true,
		DestructiveNum:  3,
		DestructiveDen:  4,
		Projection:      swipeProjection,
		open:            mvvm.NewObservable(SwipeClosed),
		mo:              NewMomentum(),
	}
}

// Open exposes the observable open state so an app view-model can bind to it
// (subscribe for a repaint, or drive it) through the go-widgets MVVM layer. It is
// lazily allocated, so a zero-value SwipeActions literal still works.
func (sa *SwipeActions) Open() *mvvm.Observable[SwipeOpenState] {
	if sa.open == nil {
		sa.open = mvvm.NewObservable(SwipeClosed)
	}
	return sa.open
}

// State reports the current open/closed state.
func (sa *SwipeActions) State() SwipeOpenState { return sa.Open().Get() }

// IsOpen reports whether either action set is (heading to be) revealed.
func (sa *SwipeActions) IsOpen() bool { return sa.State() != SwipeClosed }

// Offset returns the live content offset in device pixels: >0 leading revealed,
// <0 trailing revealed, 0 shut. It is the value Draw paints the content shifted
// by, and the exact snap target once a settle finishes.
func (sa *SwipeActions) Offset() int { return int(math.Round(sa.off)) }

// setState updates the observable, allocating it on first use.
func (sa *SwipeActions) setState(s SwipeOpenState) { sa.Open().Set(s) }

// --- geometry --------------------------------------------------------------

// actionWidth is one lane's device-pixel width: the base logical width scaled
// for DPI + density, then clamped up to the touch [MinHitTarget], so a lane is
// always at least a finger wide.
func (sa *SwipeActions) actionWidth() int {
	w := scaled(sa.ActionWidth)
	if w < 0 {
		w = 0
	}
	return TouchTarget(w)
}

// trailingWidth / leadingWidth are the total device-pixel widths of the two
// revealed sets — the exact offset magnitudes the row rests at when open.
func (sa *SwipeActions) trailingWidth() int { return len(sa.Trailing) * sa.actionWidth() }
func (sa *SwipeActions) leadingWidth() int  { return len(sa.Leading) * sa.actionWidth() }

// destructiveThreshold is the reveal magnitude (device pixels) past which a
// full swipe fires the set's primary action: DestructiveNum/DestructiveDen of
// the row width. A non-positive denominator disables it (returns a magnitude no
// drag can reach).
func (sa *SwipeActions) destructiveThreshold() int {
	if sa.DestructiveDen <= 0 {
		return math.MaxInt
	}
	return sa.Bounds().W * sa.DestructiveNum / sa.DestructiveDen
}

// trailingLaneRect returns the device-pixel rectangle of trailing action i at
// the given content offset, anchored to the content's trailing edge so the
// innermost action sits against the content and the set slides in as a unit.
// The painter clips it to the row.
func (sa *SwipeActions) trailingLaneRect(i, off int) Rect {
	r := sa.Bounds()
	aw := sa.actionWidth()
	return Rect{X: r.X + r.W + off + i*aw, Y: r.Y, W: aw, H: r.H}
}

// leadingLaneRect is trailingLaneRect's mirror for the leading set, anchored to
// the content's leading edge.
func (sa *SwipeActions) leadingLaneRect(i, off int) Rect {
	r := sa.Bounds()
	aw := sa.actionWidth()
	return Rect{X: r.X + off - sa.leadingWidth() + i*aw, Y: r.Y, W: aw, H: r.H}
}

// primaryTrailing / primaryLeading return the index of a set's primary action —
// the flagged Destructive one, else the edge action (rightmost trailing /
// leftmost leading) — or -1 for an empty set.
func (sa *SwipeActions) primaryTrailing() int {
	for i, a := range sa.Trailing {
		if a.Destructive {
			return i
		}
	}
	if len(sa.Trailing) > 0 {
		return len(sa.Trailing) - 1
	}
	return -1
}

func (sa *SwipeActions) primaryLeading() int {
	for i, a := range sa.Leading {
		if a.Destructive {
			return i
		}
	}
	if len(sa.Leading) > 0 {
		return 0
	}
	return -1
}

// --- drawing ---------------------------------------------------------------

// Draw paints the revealed action lanes and then the content shifted by the live
// offset over them, all clipped to the row. When shut (offset 0) only the content
// is visible and the render is that of the bare content. While a destructive
// full-swipe is primed, the primary action's colour fills the whole revealed
// strip instead of the individual lanes, the way a "release to delete" row flashes.
func (sa *SwipeActions) Draw(p painter.Painter, theme *Theme) {
	sa.layout()
	r := sa.Bounds()
	off := int(math.Round(sa.off))

	clr, canClip := p.(painter.Clipper)
	if canClip {
		clr.PushClip(r)
	}

	switch {
	case off < 0:
		sa.drawSet(p, theme, sa.Trailing, off, true)
	case off > 0:
		sa.drawSet(p, theme, sa.Leading, off, false)
	}

	// Content over the actions, shifted by the offset then restored so its stored
	// bounds stay at the row (a11y reads a stable position, like ScrollView).
	if sa.Content != nil {
		home := sa.Content.Bounds()
		sa.Content.SetBounds(Rect{X: r.X + off, Y: r.Y, W: r.W, H: r.H})
		sa.Content.Draw(p, theme)
		sa.Content.SetBounds(home)
	}

	if canClip {
		clr.PopClip()
	}
}

// drawSet paints one revealed action set at content offset off. trailing selects
// which edge/primary conventions apply. A primed destructive drag paints the
// primary colour across the whole strip; otherwise each lane is painted at its
// anchored position with its own colour and glyph/label.
func (sa *SwipeActions) drawSet(p painter.Painter, theme *Theme, set []SwipeAction, off int, trailing bool) {
	if len(set) == 0 {
		return
	}
	r := sa.Bounds()
	mag := abs(off)

	if sa.DestructiveFull && mag >= sa.destructiveThreshold() {
		// A non-empty set always has a valid primary index. Paint the whole
		// revealed strip in the primary colour — the "release to delete" flash.
		var pi int
		var strip Rect
		if trailing {
			pi = sa.primaryTrailing()
			strip = Rect{X: r.X + r.W + off, Y: r.Y, W: mag, H: r.H}
		} else {
			pi = sa.primaryLeading()
			strip = Rect{X: r.X, Y: r.Y, W: mag, H: r.H}
		}
		sa.paintLane(p, theme, set[pi], strip)
		return
	}

	for i := range set {
		var lane Rect
		if trailing {
			lane = sa.trailingLaneRect(i, off)
		} else {
			lane = sa.leadingLaneRect(i, off)
		}
		sa.paintLane(p, theme, set[i], lane)
	}
}

// paintLane fills one lane rect with the action's (resolved) colour and centres
// its icon or label in the (resolved) ink.
func (sa *SwipeActions) paintLane(p painter.Painter, theme *Theme, a SwipeAction, lane Rect) {
	fillRect(p, lane.X, lane.Y, lane.W, lane.H, laneColor(theme, a))
	ink := laneInk(a)
	switch {
	case a.Icon != nil:
		a.Icon(p, lane, ink)
	case a.Label != "":
		tw := sa.textWidth(a.Label)
		tx := lane.X + (lane.W-tw)/2
		ty := lane.Y + (lane.H-sa.glyphHeight())/2
		sa.drawText(p, tx, ty, a.Label, ink)
	}
}

// laneColor resolves an action's fill: its own Color when opaque, else a danger
// red for a Destructive action, else the theme Accent.
func laneColor(theme *Theme, a SwipeAction) RGBA {
	if a.Color.A != 0 {
		return a.Color
	}
	if a.Destructive {
		return dangerInk
	}
	return theme.Accent
}

// laneInk resolves an action's ink: its own Ink when opaque, else white.
func laneInk(a SwipeAction) RGBA {
	if a.Ink.A != 0 {
		return a.Ink
	}
	return RGB(0xFF, 0xFF, 0xFF)
}

// --- layout / a11y vehicles ------------------------------------------------

// layout keeps the child action Buttons in step with the action sets and pins
// each to its fully-open ("home") lane rect, so the accessibility walk reports a
// real, stable button rectangle for every action regardless of the live offset —
// exactly the way Carousel keeps its off-screen slides in the structure.
func (sa *SwipeActions) layout() {
	if sa.Content != nil {
		sa.Content.SetBounds(sa.Bounds())
	}
	sa.leadBtns = sa.syncButtons(sa.leadBtns, sa.Leading, false)
	sa.trailBtns = sa.syncButtons(sa.trailBtns, sa.Trailing, true)
}

// syncButtons rebuilds btns to one Button per action (only when the count
// changed, preserving state otherwise), refreshes each button's label/icon, and
// sets its bounds to the action's home lane. onClick routes to the matching
// Invoke method so a pointer tap on a revealed lane and an accessibility-driven
// button click share one path.
func (sa *SwipeActions) syncButtons(btns []*Button, set []SwipeAction, trailing bool) []*Button {
	if len(btns) != len(set) {
		btns = make([]*Button, len(set))
		for i := range set {
			idx := i
			b := NewButton("", nil)
			b.Flat = true
			b.PressFeedback = false
			if trailing {
				b.OnClick = func() { sa.InvokeTrailing(idx) }
			} else {
				b.OnClick = func() { sa.InvokeLeading(idx) }
			}
			btns[i] = b
		}
	}
	full := len(set) * sa.actionWidth()
	for i := range set {
		b := btns[i]
		b.Label = set[i].Label
		b.Icon = set[i].Icon
		if trailing {
			b.SetBounds(sa.trailingLaneRect(i, -full))
		} else {
			b.SetBounds(sa.leadingLaneRect(i, full))
		}
	}
	return btns
}

// Children yields the content then every action button (leading, then trailing),
// so a generic accessibility walk reaches the row body AND each action — the
// screen reader can announce and invoke an action without the swipe gesture.
// Hidden actions are still exposed, like Carousel's off-screen slides.
func (sa *SwipeActions) Children() []Widget {
	sa.layout()
	out := make([]Widget, 0, 1+len(sa.leadBtns)+len(sa.trailBtns))
	if sa.Content != nil {
		out = append(out, sa.Content)
	}
	for _, b := range sa.leadBtns {
		out = append(out, b)
	}
	for _, b := range sa.trailBtns {
		out = append(out, b)
	}
	return out
}

// A11y reports the row as a group whose value names the reveal state, so a screen
// reader can tell an open row from a shut one; its child action buttons carry the
// individual invocable semantics.
func (sa *SwipeActions) A11y() A11yInfo {
	v := "closed"
	switch sa.State() {
	case SwipeLeadingOpen:
		v = "leading actions revealed"
	case SwipeTrailingOpen:
		v = "trailing actions revealed"
	}
	return A11yInfo{Role: RoleGroup, Value: v}
}

// --- programmatic open / close / invoke ------------------------------------

// OpenTrailing reveals the trailing set (settling to its full width), or does
// nothing when the set is empty.
func (sa *SwipeActions) OpenTrailing() {
	if len(sa.Trailing) == 0 {
		return
	}
	sa.setState(SwipeTrailingOpen)
	sa.settleTo(-float64(sa.trailingWidth()), 0)
}

// OpenLeading reveals the leading set (settling to its full width), or does
// nothing when the set is empty.
func (sa *SwipeActions) OpenLeading() {
	if len(sa.Leading) == 0 {
		return
	}
	sa.setState(SwipeLeadingOpen)
	sa.settleTo(float64(sa.leadingWidth()), 0)
}

// Close settles the row shut.
func (sa *SwipeActions) Close() {
	sa.setState(SwipeClosed)
	sa.settleTo(0, 0)
}

// InvokeTrailing fires trailing action i's command exactly once and closes the
// row. Out-of-range i is ignored. This is the path a revealed-lane tap and an
// accessibility-tree button click both take.
func (sa *SwipeActions) InvokeTrailing(i int) {
	if i < 0 || i >= len(sa.Trailing) {
		return
	}
	sa.fire(sa.Trailing[i])
	sa.Close()
}

// InvokeLeading fires leading action i's command exactly once and closes the row.
// Out-of-range i is ignored.
func (sa *SwipeActions) InvokeLeading(i int) {
	if i < 0 || i >= len(sa.Leading) {
		return
	}
	sa.fire(sa.Leading[i])
	sa.Close()
}

// fire runs an action's command (nil-safe).
func (sa *SwipeActions) fire(a SwipeAction) {
	if a.OnInvoke != nil {
		a.OnInvoke()
	}
}

// --- settle engine ---------------------------------------------------------

// settleTo seeds the shared Momentum with the current offset and springs it to
// target with the given release velocity, reusing the toolkit's one deterministic
// spring. The offset is seeded through a wide bounds window (so SetOffset does
// not clamp it) and the bounds are then collapsed onto target, which makes the
// engine treat the current offset as an overscroll and return home — snapping
// exactly onto target and resting there. MaxOverscroll is widened past the travel
// so the spring never clamps mid-flight.
func (sa *SwipeActions) settleTo(target, velocity float64) {
	m := sa.mo
	span := math.Abs(sa.off-target) + float64(sa.Bounds().W) + 64
	m.Bounce = true
	m.MaxOverscroll = span
	m.SetBounds(-1e18, 1e18)
	m.SetOffset(sa.off)
	m.SetBounds(target, target)
	m.Fling(velocity)
	sa.target = target
	sa.settling = true
}

// Settling reports whether a settle is still in flight — a host schedules
// another frame (and calls Tick) while this is true, mirroring [Momentum.Settling].
func (sa *SwipeActions) Settling() bool { return sa.settling }

// Tick advances an in-flight settle by dt seconds, updating the live offset, and
// returns whether it is still settling. It is a no-op (returns false) when no
// settle is running. On the final tick the offset snaps to the exact target.
func (sa *SwipeActions) Tick(dt float64) bool {
	if !sa.settling {
		return false
	}
	still := sa.mo.Tick(dt)
	sa.off = sa.mo.Offset()
	if !still {
		sa.off = sa.target
		sa.settling = false
	}
	return still
}

// --- input -----------------------------------------------------------------

// OnEvent drives the reveal from the pointer stream a host already routes. A
// press (EventClick / EventTouchStart) begins a drag; moves (EventMouseDrag /
// EventTouchMove) slide the content and lock the gesture to the horizontal axis
// (a dominantly vertical drag is disowned so a list can scroll through it); a
// release (EventMouseUp / EventTouchEnd) either snaps (a real drag) or resolves a
// tap. Escape closes an open row. Events the shut row does not use are forwarded
// to the content.
func (sa *SwipeActions) OnEvent(ev Event) {
	if sa.Disabled {
		return
	}
	switch ev.Kind {
	case EventClick, EventTouchStart:
		sa.press(ev)
	case EventMouseDrag, EventTouchMove:
		sa.move(ev)
	case EventMouseUp, EventTouchEnd:
		sa.release(ev)
	case EventKeyDown:
		if ev.Code == "Escape" && sa.IsOpen() {
			sa.Close()
			return
		}
		sa.forwardToContent(ev)
	default:
		sa.forwardToContent(ev)
	}
}

// press starts a drag from the current offset. It stops any in-flight settle,
// seeds the Momentum drag from the live offset, and records the state to restore
// if the gesture turns out to be a vertical scroll.
func (sa *SwipeActions) press(ev Event) {
	sa.settling = false
	sa.dragging = true
	sa.moved = false
	sa.vertical = false
	sa.oriented = false
	sa.startX, sa.startY = ev.X, ev.Y
	sa.lastX = ev.X
	sa.preState = sa.State()
	sa.vt.Reset()
	m := sa.mo
	m.Bounce = true
	m.MaxOverscroll = momentumMaxOverscroll
	m.SetBounds(sa.dragMin(), sa.dragMax())
	m.SetOffset(sa.off)
	m.BeginDrag()
}

// dragMin / dragMax are the offset bounds during a drag: the content may travel a
// full row width toward a set that HAS actions (so a destructive full-swipe is
// reachable) and is pinned at 0 toward an empty set (a rubber-band wall).
func (sa *SwipeActions) dragMin() float64 {
	if len(sa.Trailing) == 0 {
		return 0
	}
	return -float64(sa.Bounds().W)
}

func (sa *SwipeActions) dragMax() float64 {
	if len(sa.Leading) == 0 {
		return 0
	}
	return float64(sa.Bounds().W)
}

// move slides the content by the per-sample delta once the gesture is locked to
// the horizontal axis. The first significant movement decides the lock: a
// dominantly vertical drag disowns the gesture (restores the pre-drag state) so
// the enclosing list can scroll.
func (sa *SwipeActions) move(ev Event) {
	if !sa.dragging {
		return
	}
	if !sa.oriented {
		adx, ady := abs(ev.X-sa.startX), abs(ev.Y-sa.startY)
		if adx < swipeTapSlop() && ady < swipeTapSlop() {
			return // not enough movement to decide the axis yet
		}
		sa.oriented = true
		if ady > adx {
			sa.vertical = true
			sa.dragging = false
			// Undo any tiny horizontal creep and restore the resting state.
			sa.off = sa.restingOffset(sa.preState)
			return
		}
	}
	// Once the axis is locked horizontal (a vertical lock sets dragging=false and
	// returns above, so control never reaches here vertically) the drag proceeds.
	dx := ev.X - sa.lastX
	sa.lastX = ev.X
	if abs(ev.X-sa.startX) > swipeTapSlop() {
		sa.moved = true
	}
	sa.off = sa.mo.DragBy(float64(dx))
	sa.vt.Sample(float64(dx), swipeNominalDt)
}

// restingOffset is the exact offset a given open state rests at.
func (sa *SwipeActions) restingOffset(s SwipeOpenState) float64 {
	switch s {
	case SwipeLeadingOpen:
		return float64(sa.leadingWidth())
	case SwipeTrailingOpen:
		return -float64(sa.trailingWidth())
	default:
		return 0
	}
}

// release ends a gesture: a real drag snaps via endDrag; a tap (no swipe-axis
// movement) resolves via handleTap; a disowned vertical gesture does nothing.
func (sa *SwipeActions) release(ev Event) {
	if sa.vertical {
		return
	}
	if !sa.dragging {
		return
	}
	sa.dragging = false
	if sa.moved {
		sa.endDrag(sa.vt.Velocity())
		return
	}
	sa.handleTap(sa.startX, sa.startY)
}

// endDrag chooses the snap target from the reveal magnitude, projected by the
// release velocity, on whichever side the drag revealed. A destructive-primed
// release fires the set's primary action once and closes; past half the set width
// it opens; otherwise it closes.
func (sa *SwipeActions) endDrag(velocity float64) {
	projected := sa.off + velocity*sa.Projection
	switch {
	case sa.off < 0 && len(sa.Trailing) > 0:
		mag := int(math.Round(-projected))
		if mag < 0 {
			mag = 0
		}
		tw := sa.trailingWidth()
		switch {
		case sa.DestructiveFull && sa.primaryTrailing() >= 0 && mag >= sa.destructiveThreshold():
			sa.fire(sa.Trailing[sa.primaryTrailing()])
			sa.setState(SwipeClosed)
			sa.settleTo(0, velocity)
		case mag >= tw/2:
			sa.setState(SwipeTrailingOpen)
			sa.settleTo(-float64(tw), velocity)
		default:
			sa.setState(SwipeClosed)
			sa.settleTo(0, velocity)
		}
	case sa.off > 0 && len(sa.Leading) > 0:
		mag := int(math.Round(projected))
		if mag < 0 {
			mag = 0
		}
		lw := sa.leadingWidth()
		switch {
		case sa.DestructiveFull && sa.primaryLeading() >= 0 && mag >= sa.destructiveThreshold():
			sa.fire(sa.Leading[sa.primaryLeading()])
			sa.setState(SwipeClosed)
			sa.settleTo(0, velocity)
		case mag >= lw/2:
			sa.setState(SwipeLeadingOpen)
			sa.settleTo(float64(lw), velocity)
		default:
			sa.setState(SwipeClosed)
			sa.settleTo(0, velocity)
		}
	default:
		sa.setState(SwipeClosed)
		sa.settleTo(0, velocity)
	}
}

// handleTap resolves a press-release with no swipe. On an open row a tap on a
// revealed lane invokes that action; a tap elsewhere closes. On a shut row the
// tap is forwarded to the content as a click.
func (sa *SwipeActions) handleTap(x, y int) {
	switch sa.State() {
	case SwipeTrailingOpen:
		if i := sa.laneAt(sa.Trailing, x, y, true); i >= 0 {
			sa.InvokeTrailing(i)
			return
		}
		sa.Close()
	case SwipeLeadingOpen:
		if i := sa.laneAt(sa.Leading, x, y, false); i >= 0 {
			sa.InvokeLeading(i)
			return
		}
		sa.Close()
	default:
		sa.forwardToContent(Event{Kind: EventClick, X: x, Y: y})
	}
}

// laneAt returns the index of the revealed action under widget-local (x, y) at
// the fully-open offset, or -1. Used to route a tap on an open row.
func (sa *SwipeActions) laneAt(set []SwipeAction, x, y int, trailing bool) int {
	full := len(set) * sa.actionWidth()
	for i := range set {
		var lane Rect
		if trailing {
			lane = sa.trailingLaneRect(i, -full)
		} else {
			lane = sa.leadingLaneRect(i, full)
		}
		if lane.Contains(x, y) {
			return i
		}
	}
	return -1
}

// forwardToContent hands an event to the content, translated by the live offset
// into the content's frame, so a shut row behaves as if the wrapper were not
// there. Nil-content-safe.
func (sa *SwipeActions) forwardToContent(ev Event) {
	if sa.Content == nil {
		return
	}
	ev.X -= int(math.Round(sa.off))
	sa.Content.OnEvent(ev)
}

// Compile-time checks: SwipeActions is a full widget, describes itself for
// accessibility, and exposes its children to a generic walk.
var (
	_ Widget                           = (*SwipeActions)(nil)
	_ Accessible                       = (*SwipeActions)(nil)
	_ interface{ Children() []Widget } = (*SwipeActions)(nil)
)
