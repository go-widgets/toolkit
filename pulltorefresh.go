// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"

	"github.com/go-widgets/painter"
)

// PullToRefresh is a container that wraps scrollable content and adds the
// signature "pull down past the top to reload" touch gesture. When the wrapped
// content is already scrolled to its top and a finger drags DOWN past that
// bound, the content is pushed down against a rubber band (reusing the same
// [Momentum] overscroll model that drives touch flinging) and a refresh
// affordance — a chevron while pulling, a [Spinner] once refreshing — is
// revealed in the gap that opens at the top. Pull past a threshold and release
// and the widget fires [PullToRefresh.OnRefresh] and stays in a spinning
// refreshing state until the app declares the reload finished by calling
// [PullToRefresh.Done]. Release before the threshold and the content simply
// springs back home, no refresh.
//
// # State machine
//
// The interaction is a four-state machine — [PullIdle] → [PullPulling] →
// [PullArmed] → [PullRefreshing] → [PullIdle] — advanced entirely by explicit,
// clock-free calls so it is as reproducible under a unit test as under a 60 Hz
// present loop (exactly like [Momentum], [Spinner] and [GestureRecognizer]):
//
//   - [PullIdle]: at rest, nothing revealed. A touch-down at the top arms a
//     candidate pull without leaving idle.
//   - [PullPulling]: a finger is dragging the content down but has not yet
//     pulled far enough; the revealed pull distance is below the arm threshold.
//     Releasing here springs back to idle with no refresh.
//   - [PullArmed]: the pull has passed the threshold; releasing now WILL
//     refresh. The affordance flips (chevron turns up, tinted Accent) to signal
//     "release to refresh".
//   - [PullRefreshing]: [PullToRefresh.OnRefresh] has fired, the spinner spins,
//     and the content rests pulled-down by the indicator's rest height until
//     [PullToRefresh.Done] returns it home.
//
// # How it consumes momentum and density
//
// The rubber-band pull and every spring-back are one embedded single-axis
// [Momentum] engine whose offset IS the pull distance: dragging feeds it
// [Momentum.DragBy] (so the pull stretches with the same diminishing-returns
// elastic curve as an overscrolled list, asymptoting to a hard wall), and each
// release/Done sets the engine's bounds to the target hold distance and flings
// it there under the damped spring, which lands exactly on that distance and
// rests. No clock, no goroutine — [PullToRefresh.Tick] advances the spring and
// the spinner from a caller-supplied dt, making the widget an [Animator].
//
// Every metric is density-aware: the arm threshold, the refreshing rest height
// and the indicator box all pass through [scaled] (HiDPI × touch density) and
// the indicator/rest heights additionally through [TouchTarget], so under
// [DensityTouch] the affordance is a comfortable fingertip target and under the
// [DensityCompact] desktop default it is byte-for-byte its base size.
type PullToRefresh struct {
	Base

	// Child is the wrapped, scrollable content (typically a [ScrollView]). It
	// fills the container's bounds and is painted pushed DOWN by the current
	// pull distance, revealing the indicator in the gap above it.
	Child Widget

	// OnRefresh fires once, on release, when a pull that reached the arm
	// threshold is let go — the app's cue to start reloading. It also fires from
	// a programmatic [PullToRefresh.Refresh]. The widget then stays in
	// [PullRefreshing] until the app calls [PullToRefresh.Done]. Nil is safe.
	OnRefresh func()

	// AtTop reports whether the wrapped content is scrolled to its top, i.e.
	// whether a downward drag should pull-to-refresh rather than scroll the
	// child. When nil the container is treated as always at the top (a pure pull
	// surface). Wrapping a [ScrollView] sv, wire it to
	//	ptr.AtTop = func() bool { return sv.OffsetY <= 0 }
	AtTop func() bool

	// Threshold is the pull distance, in LOGICAL pixels, past the top that a
	// drag must reach to arm a refresh. Zero (or negative) selects the default
	// (pullThresholdLogical). It is scaled by [scaled] to device pixels, so it
	// grows with both HiDPI and touch density.
	Threshold int

	// Style selects the refreshing spinner's look; the zero value is the
	// default hand style (see [SpinnerStyle]).
	Style SpinnerStyle

	state   PullState
	spring  *Momentum // pull distance: rubber-band drag + spring-back
	spinner *Spinner  // refreshing indicator

	dragging bool // a touch contact is down
	startY   int  // touch-down Y (surface/widget-local; only the delta is used)
	lastY    int  // last processed move Y
	pulling  bool // this contact has begun consuming the drag as a pull
}

// PullState is the position of a [PullToRefresh] in its idle→pulling→armed→
// refreshing→idle state machine.
type PullState int

const (
	// PullIdle is the resting state: nothing revealed, no contact consuming a
	// pull. A spring-back to home also reports PullIdle while it animates.
	PullIdle PullState = iota
	// PullPulling is an in-progress pull that has not yet reached the arm
	// threshold; releasing springs back home without refreshing.
	PullPulling
	// PullArmed is a pull that has passed the threshold; releasing fires
	// OnRefresh.
	PullArmed
	// PullRefreshing is the post-release spinning state, held until Done.
	PullRefreshing
)

// PullToRefresh default metrics, in LOGICAL pixels (scaled to device pixels by
// [scaled]/[TouchTarget] at use). They are named constants rather than literals
// so the feel of the gesture reads in one place.
const (
	// pullThresholdLogical is the default arm threshold: how far past the top
	// the content must be pulled before a release refreshes.
	pullThresholdLogical = 64
	// pullRestLogical is the height the content rests pulled-down by while
	// refreshing — enough to seat the spinner. Passed through [TouchTarget].
	pullRestLogical = 48
	// pullIndicatorLogical is the side of the square indicator box (chevron or
	// spinner). Passed through [TouchTarget] so a fingertip-sized affordance is
	// revealed under [DensityTouch].
	pullIndicatorLogical = 24
	// pullMaxLogical is the rubber band's asymptote: the pull can stretch toward
	// this but never reach it, however hard the finger drags.
	pullMaxLogical = 160
)

// NewPullToRefresh wraps child in a PullToRefresh at rest ([PullIdle]) with the
// default arm threshold and a default-styled spinner. A caller wires OnRefresh
// (and, when wrapping a scroll view, AtTop) and drives the touch gesture with
// TouchDown/TouchMove/TouchUp plus Tick each frame.
func NewPullToRefresh(child Widget) *PullToRefresh {
	w := &PullToRefresh{
		Child:     child,
		Threshold: pullThresholdLogical,
		spring:    NewMomentum(),
		spinner:   NewSpinner(),
	}
	w.configure()
	return w
}

// configure refreshes the spring's rubber-band cap from the CURRENT density,
// so a mid-session [SetDensity]/[SetMetricScale] change is reflected on the next
// gesture. Called at construction and before each drag/spring transition.
func (w *PullToRefresh) configure() {
	w.spring.Bounce = true
	w.spring.MaxOverscroll = float64(w.effMax())
}

// effThreshold is the arm threshold in DEVICE pixels at the current density.
func (w *PullToRefresh) effThreshold() int {
	t := w.Threshold
	if t <= 0 {
		t = pullThresholdLogical
	}
	return scaled(t)
}

// effRest is the refreshing hold distance in DEVICE pixels, clamped up to the
// density's minimum hit target so the spinner always seats in a finger-sized gap.
func (w *PullToRefresh) effRest() int { return TouchTarget(scaled(pullRestLogical)) }

// effMax is the rubber-band asymptote in DEVICE pixels.
func (w *PullToRefresh) effMax() int { return scaled(pullMaxLogical) }

// indicatorSide is the side of the square indicator box in DEVICE pixels,
// clamped up to the density's minimum hit target.
func (w *PullToRefresh) indicatorSide() int { return TouchTarget(scaled(pullIndicatorLogical)) }

// State returns the current position in the state machine.
func (w *PullToRefresh) State() PullState { return w.state }

// Refreshing reports whether a refresh is in progress (state [PullRefreshing]).
func (w *PullToRefresh) Refreshing() bool { return w.state == PullRefreshing }

// Pull returns the current revealed pull distance in device pixels (>= 0). It
// is the spring engine's offset, floored at 0, and may be fractional mid-spring.
func (w *PullToRefresh) Pull() float64 {
	o := w.spring.Offset()
	if o < 0 {
		return 0
	}
	return o
}

// PullInt returns the current pull distance rounded to the nearest device pixel
// — the height the content is pushed down by, and the indicator gap.
func (w *PullToRefresh) PullInt() int { return int(math.Round(w.Pull())) }

// atTop reports whether a downward drag should pull rather than scroll: the
// AtTop predicate, or always-true when it is nil.
func (w *PullToRefresh) atTop() bool { return w.AtTop == nil || w.AtTop() }

// TouchDown begins a touch contact at ev. It stops any spring-back in progress
// (so a finger coming down on a settling pull catches it) and records the start
// position; the contact does not become a pull until TouchMove sees it drag
// downward while at the top. It never interrupts an in-flight refresh.
func (w *PullToRefresh) TouchDown(ev Event) {
	w.dragging = true
	w.pulling = false
	w.startY, w.lastY = ev.Y, ev.Y
	if w.state == PullIdle {
		// Catch any spring-back and reset the pull baseline to home, so the new
		// drag measures from a clean zero.
		w.configure()
		w.spring.SetBounds(0, 0)
		w.spring.SetOffset(0)
	}
}

// TouchMove feeds one drag sample. Until the contact has begun pulling it only
// starts one when the content is at its top and the finger has moved net-down
// from the start; once pulling, every sample stretches the rubber band and
// re-evaluates the arm threshold. Returns true when it consumed the sample as a
// pull (so a caller routing raw events knows not to also scroll the child).
func (w *PullToRefresh) TouchMove(ev Event) bool {
	if !w.dragging || w.state == PullRefreshing {
		w.lastY = ev.Y
		return false
	}
	if w.pulling {
		w.spring.DragBy(float64(ev.Y - w.lastY))
		w.lastY = ev.Y
		w.updateArm()
		return true
	}
	// Not yet pulling: begin only on a net-downward drag while at the top.
	if w.state == PullIdle && w.atTop() && ev.Y > w.startY {
		w.pulling = true
		w.state = PullPulling
		w.configure()
		w.spring.SetBounds(0, 0)
		w.spring.SetOffset(0)
		w.spring.BeginDrag()
		w.spring.DragBy(float64(ev.Y - w.startY))
		w.lastY = ev.Y
		w.updateArm()
		return true
	}
	w.lastY = ev.Y
	return false
}

// updateArm sets the state to PullArmed once the pull reaches the threshold, and
// back to PullPulling below it, so dragging back up before release disarms.
func (w *PullToRefresh) updateArm() {
	if w.Pull() >= float64(w.effThreshold()) {
		w.state = PullArmed
	} else {
		w.state = PullPulling
	}
}

// TouchUp releases the contact. An armed release enters the refreshing state
// (springing the content down to the rest height and firing OnRefresh); an
// un-armed pull springs back home to idle; anything else is a no-op.
func (w *PullToRefresh) TouchUp() {
	if !w.dragging {
		return
	}
	w.dragging = false
	wasPulling := w.pulling
	w.pulling = false
	switch {
	case w.state == PullArmed:
		w.startRefresh()
	case wasPulling:
		w.springHome()
	}
}

// startRefresh enters PullRefreshing: it activates the spinner, springs the pull
// toward the rest height, and fires OnRefresh (nil-safe). Used both by an armed
// release and by the programmatic Refresh.
func (w *PullToRefresh) startRefresh() {
	w.state = PullRefreshing
	w.spinner.Active().Set(true)
	w.configure()
	rest := float64(w.effRest())
	w.spring.SetBounds(rest, rest)
	w.spring.EndDrag(0) // springs the offset (wherever it is) onto rest
	if w.OnRefresh != nil {
		w.OnRefresh()
	}
}

// springHome springs the pull back to zero and returns the widget to idle.
func (w *PullToRefresh) springHome() {
	w.state = PullIdle
	w.spinner.Active().Set(false)
	w.configure()
	w.spring.SetBounds(0, 0)
	w.spring.EndDrag(0)
}

// Refresh triggers a refresh programmatically (e.g. from a toolbar button or a
// keyboard shortcut), exactly as an armed release would: it springs the content
// down to the rest height, spins the spinner, and fires OnRefresh. A no-op while
// already refreshing.
func (w *PullToRefresh) Refresh() {
	if w.state == PullRefreshing {
		return
	}
	w.dragging = false
	w.pulling = false
	w.startRefresh()
}

// Done ends the refreshing state: the spinner stops and the content springs
// back home to idle. It is the app's signal that the reload OnRefresh kicked off
// has finished. A no-op unless a refresh is in progress.
func (w *PullToRefresh) Done() {
	if w.state != PullRefreshing {
		return
	}
	w.springHome()
}

// Tick advances the spring-back animation and the spinner by dt seconds, making
// PullToRefresh an [Animator]. A non-positive dt is a no-op.
func (w *PullToRefresh) Tick(dt float64) {
	if dt <= 0 {
		return
	}
	w.spring.Tick(dt)
	if w.spinner.Active().Get() {
		w.spinner.Tick(dt)
	}
}

// Animating reports whether the widget still needs frames: while a spring-back
// is settling, or while it is refreshing (the spinner spins). At rest it is
// false, so a host stops repainting.
func (w *PullToRefresh) Animating() bool {
	return w.spring.Settling() || w.state == PullRefreshing
}

// indicatorRect is the square box the chevron/spinner is drawn in, centred
// horizontally and within the revealed gap (of height PullInt). It never rises
// above the container's top edge.
func (w *PullToRefresh) indicatorRect() Rect {
	r := w.Bounds()
	side := w.indicatorSide()
	gap := w.PullInt()
	y := r.Y + (gap-side)/2
	if y < r.Y {
		y = r.Y
	}
	return Rect{X: r.X + (r.W-side)/2, Y: y, W: side, H: side}
}

// Draw paints the child pushed down by the current pull, then the indicator in
// the gap that opens above it. The child is clipped to the container's bounds so
// content pushed off the bottom cannot overdraw a neighbour; back-ends without a
// clipper/translator fall back exactly as [ScrollView] does.
func (w *PullToRefresh) Draw(p painter.Painter, theme *Theme) {
	r := w.Bounds()
	pull := w.PullInt()
	if w.Child != nil {
		w.Child.SetBounds(r)
		clr, canClip := p.(painter.Clipper)
		if canClip {
			clr.PushClip(r)
		}
		if tr, canTr := p.(painter.Translator); canTr {
			tr.PushTranslate(0, pull)
			w.Child.Draw(p, theme)
			tr.PopTranslate()
		} else {
			w.Child.SetBounds(Rect{X: r.X, Y: r.Y + pull, W: r.W, H: r.H})
			w.Child.Draw(p, theme)
			w.Child.SetBounds(r)
		}
		if canClip {
			clr.PopClip()
		}
	}
	if pull > 0 || w.state == PullRefreshing {
		w.drawIndicator(p, theme)
	}
}

// drawIndicator paints the refreshing spinner or the pull/arm chevron in the
// indicator box.
func (w *PullToRefresh) drawIndicator(p painter.Painter, theme *Theme) {
	ir := w.indicatorRect()
	if w.state == PullRefreshing {
		w.spinner.Style = w.Style
		w.spinner.SetBounds(ir)
		w.spinner.Draw(p, theme)
		return
	}
	// Pulling: a down chevron in muted ink ("pull down"); armed: an up chevron in
	// Accent ("release to refresh").
	up := w.state == PullArmed
	ink := theme.OnSurface
	if up {
		ink = theme.Accent
	}
	drawPullChevron(p, ir, up, ink)
}

// drawPullChevron paints a solid up/down triangle centred in box r, built from
// stacked single-pixel rows (the technique Carousel/Expander use for their
// arrows) so it needs no curve rasteriser and stays strictly inside r.
func drawPullChevron(p painter.Painter, r Rect, up bool, ink RGBA) {
	side := r.W
	if r.H < side {
		side = r.H
	}
	if side < 3 {
		return
	}
	cx := r.X + r.W/2
	cy := r.Y + r.H/2
	half := side / 3 // side >= 3 here, so half >= 1
	for t := 0; t <= half; t++ {
		rowW := 2*(half-t) + 1
		x := cx - (half - t)
		var y int
		if up {
			// Tip at top: widest row at the bottom, narrowing upward.
			y = cy + half - t
		} else {
			// Tip at bottom: widest row at the top, narrowing downward.
			y = cy - half + t
		}
		fillRect(p, x, y, rowW, 1, ink)
	}
}

// Children yields the wrapped content so a generic walk (accessibility,
// animation, text selection) reaches it. See children.go for why every
// widget holding a Widget must expose it.
func (w *PullToRefresh) Children() []Widget { return nonNil(w.Child) }

// ChildOffset reports that the content is painted pushed DOWN by the current
// pull, so [WalkA11y] places the child's accessible rectangle where it is drawn
// (mirroring how [ScrollView] reports its scroll offset).
func (w *PullToRefresh) ChildOffset() (int, int) { return 0, w.PullInt() }

// OnEvent routes the touch stream through the pull gesture and forwards
// everything else (and any touch it does not consume) to the child, translated
// into the child's frame by the current pull so a press lands where the content
// is drawn. This lets a wrapped view keep its wheel/keyboard/mouse behaviour
// untouched while the container adds pull-to-refresh on top.
func (w *PullToRefresh) OnEvent(ev Event) {
	if w.Disabled {
		return
	}
	switch ev.Kind {
	case EventTouchStart:
		w.TouchDown(ev)
		return
	case EventTouchMove:
		if w.TouchMove(ev) {
			return // consumed as a pull
		}
	case EventTouchEnd:
		if w.pulling || w.state == PullArmed {
			w.TouchUp()
			return
		}
		w.TouchUp()
	}
	w.forwardToChild(ev)
}

// forwardToChild passes ev to the child translated by the current pull offset,
// so the child sees coordinates in its own (pushed-down) frame.
func (w *PullToRefresh) forwardToChild(ev Event) {
	if w.Child == nil {
		return
	}
	r := w.Bounds()
	w.Child.SetBounds(r)
	child := Rect{X: r.X, Y: r.Y + w.PullInt(), W: r.W, H: r.H}
	w.Child.OnEvent(translateEvent(ev, r, child))
	w.Child.SetBounds(r)
}

// A11y reports the container as a group, marked busy while it is refreshing so a
// screen reader announces the in-progress reload.
func (w *PullToRefresh) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Value: stateValue(w.Refreshing(), "busy")}
}

// Compile-time checks that PullToRefresh satisfies the container/animation/
// accessibility contracts a generic walk relies on.
var (
	_ Accessible     = (*PullToRefresh)(nil)
	_ Animator       = (*PullToRefresh)(nil)
	_ childContainer = (*PullToRefresh)(nil)
	_ childOffsetter = (*PullToRefresh)(nil)
)
