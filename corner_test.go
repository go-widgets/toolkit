// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// anchorCorner places a w×h rect at each of the six corners of a host,
// inset by margin. This table exercises every switch branch (TopLeft is
// the default arm) with offset 0.
func TestAnchorCornerAllCorners(t *testing.T) {
	host := Rect{X: 0, Y: 0, W: 200, H: 100}
	const w, h, margin = 40, 20, 10
	cases := []struct {
		corner Corner
		wantX  int
		wantY  int
	}{
		{TopLeft, 10, 10},
		{TopRight, 150, 10},
		{BottomLeft, 10, 70},
		{BottomRight, 150, 70},
		{TopCenter, 80, 10},
		{BottomCenter, 80, 70},
	}
	for _, c := range cases {
		got := anchorCorner(host, w, h, c.corner, margin, 0)
		if got.X != c.wantX || got.Y != c.wantY || got.W != w || got.H != h {
			t.Errorf("corner %d = %+v, want X=%d Y=%d W=%d H=%d",
				c.corner, got, c.wantX, c.wantY, w, h)
		}
	}
}

// A non-zero offset stacks downward from a top corner and upward from a
// bottom corner.
func TestAnchorCornerStackOffset(t *testing.T) {
	host := Rect{X: 0, Y: 0, W: 200, H: 100}
	top := anchorCorner(host, 40, 20, TopLeft, 10, 15)
	if top.Y != 10+15 {
		t.Fatalf("top-corner stack Y = %d, want %d", top.Y, 25)
	}
	bot := anchorCorner(host, 40, 20, BottomLeft, 10, 15)
	if bot.Y != 100-10-20-15 {
		t.Fatalf("bottom-corner stack Y = %d, want %d", bot.Y, 55)
	}
}

// A non-origin host offsets the placement by the host's own X/Y.
func TestAnchorCornerNonOriginHost(t *testing.T) {
	host := Rect{X: 1000, Y: 500, W: 200, H: 100}
	got := anchorCorner(host, 40, 20, BottomRight, 10, 0)
	if got.X != 1000+200-10-40 || got.Y != 500+100-10-20 {
		t.Fatalf("non-origin BottomRight = %+v", got)
	}
}

// Toast.AnchorIn sizes to the text + stacks by index at a top corner.
func TestToastAnchorInStacks(t *testing.T) {
	host := Rect{X: 0, Y: 0, W: 300, H: 200}
	t0 := NewToast("first", ToastInfo)
	t0.AnchorIn(host, TopRight, 0)
	t1 := NewToast("second", ToastInfo)
	t1.AnchorIn(host, TopRight, 1)

	w := TextWidth("first") + 2*ToastPadX
	h := GlyphHeight() + 2*ToastPadY
	b0, b1 := t0.Bounds(), t1.Bounds()
	if b0.Y != ToastMargin {
		t.Fatalf("row 0 Y = %d, want %d", b0.Y, ToastMargin)
	}
	if b1.Y != ToastMargin+(h+ToastGap) {
		t.Fatalf("row 1 Y = %d, want %d", b1.Y, ToastMargin+(h+ToastGap))
	}
	// Right-docked: right edge sits margin px from the host's right edge.
	if b0.X+b0.W != host.W-ToastMargin {
		t.Fatalf("row 0 right edge = %d, want %d", b0.X+b0.W, host.W-ToastMargin)
	}
	if b0.W != w {
		t.Fatalf("row 0 W = %d, want %d", b0.W, w)
	}
}

// Notification.AnchorIn sizes to the text + docks at the given corner.
func TestNotificationAnchorIn(t *testing.T) {
	host := Rect{X: 0, Y: 0, W: 400, H: 300}
	n := NewNotification("hi there")
	n.AnchorIn(host, BottomCenter)
	b := n.Bounds()
	w := TextWidth("hi there") + 2*NotificationPadX
	h := GlyphHeight() + 2*NotificationPadY
	if b.W != w || b.H != h {
		t.Fatalf("size = %dx%d, want %dx%d", b.W, b.H, w, h)
	}
	if b.X != (host.W-w)/2 {
		t.Fatalf("centred X = %d, want %d", b.X, (host.W-w)/2)
	}
	if b.Y+b.H != host.H-NotificationMargin {
		t.Fatalf("bottom edge = %d, want %d", b.Y+b.H, host.H-NotificationMargin)
	}
}
