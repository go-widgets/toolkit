// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestNewGestureRecognizerDefaults(t *testing.T) {
	g := NewGestureRecognizer()
	if g.TapSlop <= 0 {
		t.Fatalf("TapSlop = %d, want > 0", g.TapSlop)
	}
	if g.LongPressTicks <= 0 {
		t.Fatalf("LongPressTicks = %d, want > 0", g.LongPressTicks)
	}
	if g.SwipeMinDist <= 0 {
		t.Fatalf("SwipeMinDist = %d, want > 0", g.SwipeMinDist)
	}
}

func TestGestureTap(t *testing.T) {
	g := NewGestureRecognizer()
	var tapX, tapY int
	tapped := 0
	g.OnTap = func(x, y int) { tapped++; tapX, tapY = x, y }

	g.Feed(Event{Kind: EventTouchStart, X: 10, Y: 10, Code: "p1"})
	g.Feed(Event{Kind: EventTouchMove, X: 12, Y: 8, Code: "p1"}) // within slop
	g.Feed(Event{Kind: EventTouchEnd, X: 13, Y: 9, Code: "p1"})

	if tapped != 1 {
		t.Fatalf("tapped = %d, want 1", tapped)
	}
	if tapX != 13 || tapY != 9 {
		t.Fatalf("tap pos = (%d,%d), want (13,9)", tapX, tapY)
	}
}

func TestGestureTapNilCallback(t *testing.T) {
	g := NewGestureRecognizer() // OnTap left nil
	g.Feed(Event{Kind: EventTouchStart, X: 0, Y: 0, Code: "p1"})
	g.Feed(Event{Kind: EventTouchEnd, X: 1, Y: 1, Code: "p1"}) // must not panic
}

func TestGestureLongPressThenSuppressesTap(t *testing.T) {
	g := NewGestureRecognizer()
	longPresses := 0
	var lpX, lpY int
	g.OnLongPress = func(x, y int) { longPresses++; lpX, lpY = x, y }
	tapped := 0
	g.OnTap = func(x, y int) { tapped++ }

	g.Feed(Event{Kind: EventTouchStart, X: 40, Y: 50, Code: "p1"})
	for i := 0; i < g.LongPressTicks+5; i++ { // overshoot: fires once, extra ticks no-op
		g.Tick()
	}
	if longPresses != 1 {
		t.Fatalf("longPresses = %d, want 1", longPresses)
	}
	if lpX != 40 || lpY != 50 {
		t.Fatalf("long-press pos = (%d,%d), want (40,50)", lpX, lpY)
	}

	g.Feed(Event{Kind: EventTouchEnd, X: 41, Y: 49, Code: "p1"}) // small move, would be a tap
	if tapped != 0 {
		t.Fatalf("tapped = %d, want 0 (suppressed by long press)", tapped)
	}
}

func TestGestureLongPressNilCallback(t *testing.T) {
	g := NewGestureRecognizer() // OnLongPress left nil
	g.Feed(Event{Kind: EventTouchStart, X: 0, Y: 0, Code: "p1"})
	for i := 0; i < g.LongPressTicks; i++ {
		g.Tick() // must not panic when threshold is crossed
	}
}

func TestGestureTickNoActiveTouch(t *testing.T) {
	g := NewGestureRecognizer()
	g.Tick() // no touch ever started; must be a no-op, not panic
	if g.holdTicks != 0 {
		t.Fatalf("holdTicks = %d, want 0", g.holdTicks)
	}
}

func TestGestureMovePastSlopStopsHoldTiming(t *testing.T) {
	g := NewGestureRecognizer()
	longPresses := 0
	g.OnLongPress = func(x, y int) { longPresses++ }

	g.Feed(Event{Kind: EventTouchStart, X: 0, Y: 0, Code: "p1"})
	g.Feed(Event{Kind: EventTouchMove, X: 0, Y: g.TapSlop + 20, Code: "p1"})
	for i := 0; i < g.LongPressTicks+5; i++ {
		g.Tick()
	}
	if longPresses != 0 {
		t.Fatalf("longPresses = %d, want 0 (moved past slop should suppress hold timing)", longPresses)
	}
	if g.holdTicks != 0 {
		t.Fatalf("holdTicks = %d, want 0 (never incremented once past slop)", g.holdTicks)
	}
}

func TestGestureSwipeDirections(t *testing.T) {
	cases := []struct {
		name   string
		dx, dy int
		want   SwipeDir
	}{
		{"right", 30, 0, SwipeRight},
		{"left", -30, 0, SwipeLeft},
		{"down", 0, 30, SwipeDown},
		{"up", 0, -30, SwipeUp},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := NewGestureRecognizer()
			var got SwipeDir
			fired := 0
			g.OnSwipe = func(dir SwipeDir) { fired++; got = dir }
			tapped := 0
			g.OnTap = func(x, y int) { tapped++ }

			g.Feed(Event{Kind: EventTouchStart, X: 100, Y: 100, Code: "p1"})
			g.Feed(Event{Kind: EventTouchEnd, X: 100 + c.dx, Y: 100 + c.dy, Code: "p1"})

			if fired != 1 {
				t.Fatalf("OnSwipe fired %d times, want 1", fired)
			}
			if got != c.want {
				t.Fatalf("swipe dir = %v, want %v", got, c.want)
			}
			if tapped != 0 {
				t.Fatalf("OnTap fired, want it suppressed by the swipe")
			}
		})
	}
}

func TestGestureSwipeNilCallback(t *testing.T) {
	g := NewGestureRecognizer() // OnSwipe left nil
	g.Feed(Event{Kind: EventTouchStart, X: 0, Y: 0, Code: "p1"})
	g.Feed(Event{Kind: EventTouchEnd, X: g.SwipeMinDist + 10, Y: 0, Code: "p1"}) // must not panic
}

func TestGestureMovementBetweenSlopAndSwipeIsNeither(t *testing.T) {
	g := NewGestureRecognizer()
	tapped, swiped := 0, 0
	g.OnTap = func(x, y int) { tapped++ }
	g.OnSwipe = func(dir SwipeDir) { swiped++ }

	mid := (g.TapSlop + g.SwipeMinDist) / 2
	if mid <= g.TapSlop || mid >= g.SwipeMinDist {
		t.Fatalf("test setup invalid: mid=%d must sit strictly between TapSlop=%d and SwipeMinDist=%d", mid, g.TapSlop, g.SwipeMinDist)
	}

	g.Feed(Event{Kind: EventTouchStart, X: 0, Y: 0, Code: "p1"})
	g.Feed(Event{Kind: EventTouchEnd, X: mid, Y: 0, Code: "p1"})

	if tapped != 0 || swiped != 0 {
		t.Fatalf("tapped=%d swiped=%d, want both 0", tapped, swiped)
	}
}

func TestGestureStrayEventsWithNoActiveTouch(t *testing.T) {
	g := NewGestureRecognizer()
	tapped, swiped, longPressed := 0, 0, 0
	g.OnTap = func(x, y int) { tapped++ }
	g.OnSwipe = func(dir SwipeDir) { swiped++ }
	g.OnLongPress = func(x, y int) { longPressed++ }

	g.Feed(Event{Kind: EventTouchMove, X: 5, Y: 5, Code: "ghost"}) // no Start yet
	g.Feed(Event{Kind: EventTouchEnd, X: 5, Y: 5, Code: "ghost"})

	if tapped != 0 || swiped != 0 || longPressed != 0 {
		t.Fatalf("stray events fired a callback: tapped=%d swiped=%d longPressed=%d", tapped, swiped, longPressed)
	}
	if g.active {
		t.Fatal("active = true after stray events, want false")
	}
}

func TestGestureMismatchedIDIgnored(t *testing.T) {
	g := NewGestureRecognizer()
	longPresses := 0
	g.OnLongPress = func(x, y int) { longPresses++ }

	g.Feed(Event{Kind: EventTouchStart, X: 0, Y: 0, Code: "a"})
	// A second, unrelated pointer's move must not perturb pointer "a"'s
	// tracked position: if it did, curX/curY would jump far enough past
	// TapSlop that the hold-timing loop below would never fire.
	g.Feed(Event{Kind: EventTouchMove, X: 500, Y: 500, Code: "b"})
	for i := 0; i < g.LongPressTicks; i++ {
		g.Tick()
	}
	if longPresses != 1 {
		t.Fatalf("longPresses = %d, want 1 (mismatched-id move should have been ignored)", longPresses)
	}

	// A mismatched End must not close out pointer "a"'s touch either.
	g.Feed(Event{Kind: EventTouchEnd, X: 999, Y: 999, Code: "b"})
	if !g.active {
		t.Fatal("active = false after mismatched End, want true (pointer a still active)")
	}

	tapped := 0
	g.OnTap = func(x, y int) { tapped++ }
	g.Feed(Event{Kind: EventTouchEnd, X: 1, Y: 1, Code: "a"})
	if g.active {
		t.Fatal("active = true after matching End, want false")
	}
	// Long press already fired for this touch, so the matching End's tiny
	// movement must NOT resolve to a tap.
	if tapped != 0 {
		t.Fatalf("tapped = %d, want 0 (long press already fired for this touch)", tapped)
	}
}

func TestGestureRestartOverwritesActiveTouch(t *testing.T) {
	g := NewGestureRecognizer()
	tapped := 0
	var tapX, tapY int
	g.OnTap = func(x, y int) { tapped++; tapX, tapY = x, y }

	g.Feed(Event{Kind: EventTouchStart, X: 0, Y: 0, Code: "a"})
	g.Feed(Event{Kind: EventTouchStart, X: 200, Y: 200, Code: "b"}) // new touch overwrites
	g.Feed(Event{Kind: EventTouchEnd, X: 201, Y: 199, Code: "b"})

	if tapped != 1 {
		t.Fatalf("tapped = %d, want 1", tapped)
	}
	if tapX != 201 || tapY != 199 {
		t.Fatalf("tap pos = (%d,%d), want (201,199)", tapX, tapY)
	}
}

func TestGestureNonTouchEventIgnored(t *testing.T) {
	g := NewGestureRecognizer()
	g.Feed(Event{Kind: EventClick, X: 5, Y: 5})
	if g.active {
		t.Fatal("active = true after a non-touch event, want false")
	}
	g.Tick() // must stay a no-op
	if g.holdTicks != 0 {
		t.Fatalf("holdTicks = %d, want 0", g.holdTicks)
	}
}

func TestGestureCustomThresholds(t *testing.T) {
	g := &GestureRecognizer{
		TapSlop:        2,
		LongPressTicks: 3,
		SwipeMinDist:   5,
	}
	longPresses := 0
	g.OnLongPress = func(x, y int) { longPresses++ }

	g.Feed(Event{Kind: EventTouchStart, X: 0, Y: 0, Code: "p1"})
	g.Tick()
	g.Tick()
	if longPresses != 0 {
		t.Fatalf("longPresses = %d after 2 ticks, want 0 (LongPressTicks=3)", longPresses)
	}
	g.Tick()
	if longPresses != 1 {
		t.Fatalf("longPresses = %d after 3 ticks, want 1 (LongPressTicks=3)", longPresses)
	}

	swiped := 0
	var dir SwipeDir
	g.OnSwipe = func(d SwipeDir) { swiped++; dir = d }
	g.Feed(Event{Kind: EventTouchStart, X: 0, Y: 0, Code: "p2"})
	g.Feed(Event{Kind: EventTouchEnd, X: 6, Y: 0, Code: "p2"}) // >= custom SwipeMinDist=5
	if swiped != 1 || dir != SwipeRight {
		t.Fatalf("swiped=%d dir=%v, want 1/SwipeRight", swiped, dir)
	}
}
