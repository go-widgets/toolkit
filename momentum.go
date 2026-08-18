// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "math"

// Momentum is the pure-logic inertial ("fling") scroll engine that gives a
// touch surface its signature feel: let go of a flick and the content keeps
// gliding, decelerating to a smooth stop; push past an edge and it stretches
// against a rubber band, then springs back to rest exactly on the boundary.
//
// It is ONE axis. A two-dimensional scroller composes two Momentum values, one
// for X and one for Y — exactly the way [ScrollView] keeps OffsetX and OffsetY
// as independent clamps — so the same engine drives a vertical list, a
// horizontal carousel, or both at once without either axis knowing about the
// other. It holds no widget reference, no geometry beyond the [min, max] offset
// window it is told to clamp against, and draws nothing: a consumer feeds it a
// release velocity (or raw drag samples via [VelocityTracker]), reads back an
// offset each frame, and paints wherever that offset says.
//
// # Determinism
//
// Like the rest of the toolkit ([GestureRecognizer], [Animator]) the engine
// owns no goroutine, no timer and never reads a clock: it is advanced by an
// explicit [Momentum.Tick] whose dt (elapsed seconds) is supplied by the
// caller. Given the same configuration, bounds, release velocity and dt
// sequence it produces the same offsets on every platform and in every test —
// no time.Now, no wall-clock, so it is as reproducible under a unit test as it
// is under a 60 Hz present loop, and it is safe in the wasm and headless
// contexts where a real clock may be unavailable.
//
// # Physics model — exponential deceleration + damped-spring overscroll
//
// The fling uses EXPONENTIAL velocity decay: velocity retains a fixed FRACTION
// of itself per second, v(t) = v0 · Friction^t. This is the model behind the
// familiar iOS/UIScrollView "flywheel" glide (Android's OverScroller reaches
// for the same asymptotic-friction curve), and it is chosen over a constant
// deceleration (v decreasing by a fixed amount per second) for two concrete
// reasons:
//
//   - Feel. Constant deceleration stops abruptly — the last instant of motion
//     is as fast as any other and then it simply ceases. Exponential decay eases
//     out, spending most of its travel slowing gently, which is what reads as
//     "momentum" rather than "a puck sliding to a halt".
//   - Frame-rate independence. Because retention COMPOUNDS, the total decay over
//     an interval is Friction^dt regardless of how that interval is chopped into
//     ticks: one 1/30 s tick and two 1/60 s ticks leave the velocity in the same
//     place (Friction^(a+b) = Friction^a · Friction^b). A constant-deceleration
//     model has to special-case the tick in which velocity crosses zero or it
//     drifts with the frame rate. So a fling looks identical whether the host is
//     rendering at 30, 60 or 120 Hz.
//
// A tiny residual velocity would let the exponential creep forever, so once
// |velocity| falls below StopVelocity the fling settles to rest — the clean
// stop. Overscroll (when [Momentum.Bounce] is enabled) is a critically-tuned
// damped spring: past a bound the offset is pulled back by a spring force
// (−Stiffness·overshoot) opposed by damping (−Damping·velocity), so it
// decelerates, reverses and returns, and the instant it reaches the bound it
// snaps exactly onto it and rests — no residual jitter, no drift past home into
// the content. With Bounce disabled a fling that reaches a bound clamps exactly
// to it and stops, so a plain (non-touch) scroller keeps hard edges.
type Momentum struct {
	// Friction is the fraction of velocity retained per second during a fling
	// (0 < Friction < 1). Smaller = more drag = a shorter glide; a value near 1
	// coasts a long way. It is a per-second figure, applied as Friction^dt each
	// tick, so it is independent of the frame rate.
	Friction float64
	// StopVelocity is the speed (in px/s, always compared as a magnitude) at or
	// below which a coasting fling is considered stopped and settles to rest.
	// It must be > 0 so the exponential tail terminates.
	StopVelocity float64

	// Bounce enables rubber-band overscroll. When true, a drag past a bound is
	// resisted (see DragBy) and a fling that reaches a bound stretches past it
	// and springs back. When false, both the drag and the fling clamp hard at
	// the bound — the behaviour a mouse/desktop scroller wants.
	Bounce bool
	// Stiffness is the spring constant (per second squared) pulling an
	// overscrolled offset back toward its bound. Higher = a snappier return.
	Stiffness float64
	// Damping is the spring's velocity damping (per second). It bleeds energy
	// out of the return so the offset settles onto the bound instead of
	// oscillating around it.
	Damping float64
	// MaxOverscroll caps how far (px) past a bound the offset may travel, for
	// both a resisted drag and a spring. Zero means "no travel past the bound"
	// even with Bounce on. The rubber-band's resisted stretch asymptotes to this.
	MaxOverscroll float64
	// SnapDistance is the overshoot magnitude (px) within which the spring, once
	// it is also below StopVelocity, snaps exactly to the bound and rests. It
	// terminates the spring's asymptotic tail the way StopVelocity terminates
	// the fling's.
	SnapDistance float64

	// min, max are the inclusive offset bounds the engine clamps against —
	// typically 0 and (contentSize − viewportSize). A consumer sets them from
	// its own geometry via SetBounds; the engine never derives them.
	min, max float64

	// offset is the current scroll offset; velocity the current signed speed
	// (px/s). phase is the state-machine position.
	offset   float64
	velocity float64
	phase    momentumPhase

	// dragging / dragRaw track an in-progress finger drag: dragRaw is the
	// UNCLAMPED offset the finger has accumulated, from which the resisted,
	// displayed offset is derived so releasing hands back a faithful overshoot.
	dragging bool
	dragRaw  float64
}

// momentumPhase is the engine's state-machine position.
type momentumPhase int

const (
	// momentumRest: no motion; Tick is a no-op and Settling reports false.
	momentumRest momentumPhase = iota
	// momentumFling: coasting within bounds under exponential deceleration.
	momentumFling
	// momentumSpring: overscrolled past a bound, returning under a damped spring.
	momentumSpring
)

// Default momentum tuning. These are deliberately gentle, frame-rate-neutral
// values; a touch profile (density-aware) may scale them. They are exported
// only through NewMomentum so a caller gets a working engine without knowing
// the physics.
const (
	momentumFriction      = 0.06 // ~6% of velocity survives each second: a firm, natural glide
	momentumStopVelocity  = 8.0  // px/s; below this the glide has visually stopped
	momentumStiffness     = 200.0
	momentumDamping       = 28.0
	momentumMaxOverscroll = 120.0 // px of rubber-band stretch
	momentumSnapDistance  = 0.5   // px; sub-pixel overshoot is "home"
)

// NewMomentum returns a ready-to-use single-axis engine with the default
// deceleration and (enabled) rubber-band spring tuning, bounds [0, 0] (a caller
// sets real bounds with SetBounds) and offset 0, at rest. Callers wanting a
// hard-edged desktop scroller set Bounce = false; callers wanting different
// physics may override any field or build a Momentum{} literal directly — the
// Tick logic depends only on the field values, not on how the struct was made.
func NewMomentum() *Momentum {
	return &Momentum{
		Friction:      momentumFriction,
		StopVelocity:  momentumStopVelocity,
		Bounce:        true,
		Stiffness:     momentumStiffness,
		Damping:       momentumDamping,
		MaxOverscroll: momentumMaxOverscroll,
		SnapDistance:  momentumSnapDistance,
	}
}

// SetBounds sets the inclusive offset window [min, max] the engine clamps
// against. If the two are given out of order they are swapped, so a caller may
// pass them in either order. Bounds may be updated at any time (e.g. when the
// content is re-measured); the current offset is NOT forcibly re-clamped here,
// so a live spring keeps its overshoot — call SetOffset to re-clamp explicitly.
func (m *Momentum) SetBounds(min, max float64) {
	if min > max {
		min, max = max, min
	}
	m.min, m.max = min, max
}

// Bounds returns the current clamp window.
func (m *Momentum) Bounds() (min, max float64) { return m.min, m.max }

// Offset returns the current scroll offset (may be fractional; may be slightly
// outside the bounds mid-overscroll).
func (m *Momentum) Offset() float64 { return m.offset }

// OffsetInt returns the current offset rounded to the nearest integer pixel,
// the form a pixel-blitting consumer paints with.
func (m *Momentum) OffsetInt() int { return int(math.Round(m.offset)) }

// Velocity returns the current signed velocity in px/s (0 at rest).
func (m *Momentum) Velocity() float64 { return m.velocity }

// Settling reports whether the engine still owes motion — a coasting fling or a
// returning spring. A host schedules another frame while this is true and may
// stop once it goes false, mirroring [Animator].Animating.
func (m *Momentum) Settling() bool { return m.phase != momentumRest }

// SetOffset jumps the offset to o, clamped to [min, max], and halts all motion
// (velocity 0, at rest, any drag cancelled). It is the way to seed or reset the
// scroll position — after a keyboard jump, a programmatic scroll-to, or a
// content re-measure that must re-clamp.
func (m *Momentum) SetOffset(o float64) {
	if o < m.min {
		o = m.min
	}
	if o > m.max {
		o = m.max
	}
	m.offset = o
	m.velocity = 0
	m.phase = momentumRest
	m.dragging = false
}

// Stop halts any in-flight motion at the CURRENT offset without re-clamping it
// (a finger coming down on a coasting list stops it dead, wherever it is). Use
// SetOffset to also clamp.
func (m *Momentum) Stop() {
	m.velocity = 0
	m.phase = momentumRest
	m.dragging = false
}

// BeginDrag starts a finger drag from the current offset. It stops any coast in
// progress and seeds the unclamped drag accumulator, so the subsequent DragBy
// deltas track from where the content actually is. A caller wires this to its
// touch-down / press.
func (m *Momentum) BeginDrag() {
	m.velocity = 0
	m.phase = momentumRest
	m.dragging = true
	m.dragRaw = m.offset
}

// DragBy moves the content by delta (in offset units — positive scrolls toward
// max) for one drag sample and returns the new displayed offset. Inside the
// bounds the content tracks the finger one-for-one. Past a bound, with Bounce
// on, the movement is rubber-banded: the displayed overshoot is a diminishing
// function of the raw finger overshoot, asymptoting to MaxOverscroll, so the
// content stretches less and less the harder it is pulled — the classic elastic
// edge. With Bounce off it clamps hard at the bound. DragBy without a preceding
// BeginDrag begins one implicitly, so a caller may just start dragging.
func (m *Momentum) DragBy(delta float64) float64 {
	if !m.dragging {
		m.BeginDrag()
	}
	m.dragRaw += delta
	m.offset = m.resist(m.dragRaw)
	return m.offset
}

// resist maps an unclamped raw offset to the displayed offset, applying the
// rubber-band curve past either bound (when Bounce is on) or a hard clamp (when
// it is off). The elastic curve is stretch = over / (1 + over/MaxOverscroll):
// at over = MaxOverscroll it is already halved, and it never reaches
// MaxOverscroll however far the finger travels, which is exactly the "wall you
// can lean on but not walk through" feel.
func (m *Momentum) resist(raw float64) float64 {
	switch {
	case raw < m.min:
		over := m.min - raw
		return m.min - m.rubber(over)
	case raw > m.max:
		over := raw - m.max
		return m.max + m.rubber(over)
	default:
		return raw
	}
}

// rubber applies the diminishing-returns elastic curve to a positive raw
// overshoot, or a hard 0 when Bounce is off (no stretch past the bound).
func (m *Momentum) rubber(over float64) float64 {
	if !m.Bounce || m.MaxOverscroll <= 0 {
		return 0
	}
	return over / (1 + over/m.MaxOverscroll)
}

// EndDrag releases a finger drag with the given release velocity (px/s, signed
// like the offset). It is BeginDrag's partner: it clears the drag state and,
// via Fling, hands the released velocity to the coast/spring. A release with a
// tiny velocity while in bounds simply comes to rest; a release while
// overscrolled always springs home regardless of velocity.
func (m *Momentum) EndDrag(velocity float64) {
	m.dragging = false
	m.Fling(velocity)
}

// Fling launches a coast from the current offset at the given release velocity
// (px/s, signed). If the offset is currently past a bound (a release mid-
// rubber-band) it enters the spring instead, so it always returns home. If it
// is within bounds it begins an exponential-deceleration fling — unless the
// speed is already at/below StopVelocity, in which case it just rests. A caller
// that tracks its own release velocity (see VelocityTracker) can call Fling
// directly without the Begin/EndDrag pair.
func (m *Momentum) Fling(velocity float64) {
	m.dragging = false
	m.velocity = velocity
	switch {
	case m.offset < m.min || m.offset > m.max:
		// Released while stretched past an edge: spring home whatever the flick.
		m.phase = momentumSpring
	case math.Abs(velocity) <= m.StopVelocity:
		m.velocity = 0
		m.phase = momentumRest
	default:
		m.phase = momentumFling
	}
}

// Tick advances the engine by dt seconds and reports whether it is still
// settling (a host uses the return exactly like Settling to decide whether to
// schedule another frame). A non-positive dt, or a call at rest, is a no-op.
//
// The step is semi-implicit: the velocity is updated first, then the offset is
// integrated with the NEW velocity. Doing it in this order keeps the fling
// unconditionally stable (it can never gain energy) and makes each tick an
// exact, closed-form function of the previous one.
func (m *Momentum) Tick(dt float64) bool {
	if dt <= 0 {
		return m.phase != momentumRest
	}
	switch m.phase {
	case momentumFling:
		return m.tickFling(dt)
	case momentumSpring:
		return m.tickSpring(dt)
	default:
		return false
	}
}

// tickFling advances a coasting fling: decay the velocity by Friction^dt, stop
// if it has dropped to StopVelocity, otherwise integrate. A step that would
// carry the offset past a bound either clamps hard (Bounce off) or hands the
// offset — now just past the bound — to the spring (Bounce on).
func (m *Momentum) tickFling(dt float64) bool {
	m.velocity *= math.Pow(m.Friction, dt)
	if math.Abs(m.velocity) <= m.StopVelocity {
		m.velocity = 0
		m.phase = momentumRest
		return false
	}
	next := m.offset + m.velocity*dt
	switch {
	case next < m.min:
		if !m.Bounce {
			m.offset = m.min
			m.velocity = 0
			m.phase = momentumRest
			return false
		}
		m.offset = m.clampOverscroll(next)
		m.phase = momentumSpring
		return true
	case next > m.max:
		if !m.Bounce {
			m.offset = m.max
			m.velocity = 0
			m.phase = momentumRest
			return false
		}
		m.offset = m.clampOverscroll(next)
		m.phase = momentumSpring
		return true
	default:
		m.offset = next
		return true
	}
}

// tickSpring advances a damped-spring return from an overscrolled offset toward
// its nearest bound. The moment the offset reaches (or crosses back to) the
// bound it snaps exactly onto it and rests — the spring never carries motion
// past home into the content, so the return is clean and lands on an exact
// value. A tiny overshoot moving slowly also snaps, terminating the asymptote.
func (m *Momentum) tickSpring(dt float64) bool {
	bound, below := m.nearestBound()
	x := m.offset - bound // signed overshoot: negative below min, positive above max

	// Damped-spring acceleration, semi-implicit (update v, then x).
	a := -m.Stiffness*x - m.Damping*m.velocity
	m.velocity += a * dt
	m.offset += m.velocity * dt

	// Reached / crossed the bound: home. Snap exactly and rest.
	if below && m.offset >= bound || !below && m.offset <= bound {
		m.offset = bound
		m.velocity = 0
		m.phase = momentumRest
		return false
	}
	// Sub-threshold and slow: terminate the tail on the bound.
	if math.Abs(m.offset-bound) <= m.SnapDistance && math.Abs(m.velocity) <= m.StopVelocity {
		m.offset = bound
		m.velocity = 0
		m.phase = momentumRest
		return false
	}
	// Cap how far the spring may stretch (a very fast fling into the edge).
	m.offset = m.clampOverscroll(m.offset)
	return true
}

// nearestBound reports which bound an overscrolled (or in-bounds) offset is
// closest to, and whether that bound is the lower one. Below-min overscroll
// returns (min, true); at/above it returns (max, false).
func (m *Momentum) nearestBound() (bound float64, below bool) {
	if m.offset < m.min {
		return m.min, true
	}
	return m.max, false
}

// clampOverscroll limits an offset to at most MaxOverscroll past whichever
// bound it exceeds, and zeroes the velocity when the cap is hit (the offset has
// run into the far wall of the rubber band and can travel no further out).
func (m *Momentum) clampOverscroll(o float64) float64 {
	if lo := m.min - m.MaxOverscroll; o < lo {
		m.velocity = 0
		return lo
	}
	if hi := m.max + m.MaxOverscroll; o > hi {
		m.velocity = 0
		return hi
	}
	return o
}

// VelocityTracker turns a stream of drag POSITION samples into a smoothed
// release velocity, so a consumer that only knows where the finger is each
// frame (not how fast it is moving) can still hand [Momentum.Fling] /
// [Momentum.EndDrag] a sensible flick speed.
//
// Like everything else here it is clock-free: each Sample carries its own dt.
// It averages the most recent samples (a short window) so a single jittery
// final frame — common right at lift-off — cannot dominate the launch velocity
// the way a raw last-delta would. The zero value is an empty tracker ready to
// use; Window defaults to 4 samples when left 0.
type VelocityTracker struct {
	// Window is how many recent per-sample velocities are averaged (<= 0 means
	// the default of 4). A larger window is smoother but lags a fast flick.
	Window int

	vels []float64
}

// Reset clears the tracked samples, so the next Sample starts a fresh drag. A
// consumer calls it on touch-down.
func (v *VelocityTracker) Reset() { v.vels = v.vels[:0] }

// Sample records that the finger moved by delta (offset units) over dt seconds
// and returns the current smoothed velocity. A non-positive dt is ignored (it
// carries no velocity information) and returns the unchanged estimate.
func (v *VelocityTracker) Sample(delta, dt float64) float64 {
	if dt <= 0 {
		return v.Velocity()
	}
	w := v.Window
	if w <= 0 {
		w = 4
	}
	v.vels = append(v.vels, delta/dt)
	if len(v.vels) > w {
		v.vels = v.vels[len(v.vels)-w:]
	}
	return v.Velocity()
}

// Velocity returns the mean of the windowed per-sample velocities, or 0 when no
// sample has been recorded yet. This is the value to hand Fling/EndDrag on
// release.
func (v *VelocityTracker) Velocity() float64 {
	if len(v.vels) == 0 {
		return 0
	}
	var sum float64
	for _, s := range v.vels {
		sum += s
	}
	return sum / float64(len(v.vels))
}
