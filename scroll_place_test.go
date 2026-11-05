// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// countingColumn is a column of markers that cannot measure itself, which is
// what a VBox of fixed rows is: it lays its children out relative to its own
// bounds and says nothing about its natural height.
type countingColumn struct {
	Base
	rows []*markerWidget
	rowH int
}

func newCountingColumn(n, rowH int) *countingColumn {
	c := &countingColumn{}
	for range n {
		c.rows = append(c.rows, &markerWidget{})
	}
	c.rowH = rowH
	return c
}

func (c *countingColumn) SetBounds(r Rect) {
	c.Base.SetBounds(r)
	for i, row := range c.rows {
		row.SetBounds(Rect{X: r.X, Y: r.Y + i*c.rowH, W: r.W, H: c.rowH})
	}
}

func (c *countingColumn) Children() []Widget {
	out := make([]Widget, len(c.rows))
	for i, row := range c.rows {
		out[i] = row
	}
	return out
}

func (c *countingColumn) Draw(p painter.Painter, th *Theme) {
	for _, row := range c.rows {
		row.Draw(p, th)
	}
}

// ⛔ A CHILD THAT CANNOT BE MEASURED IS STILL PLACED.
//
// SetBounds only ever SIZED a measurable child, and left an unmeasurable one
// untouched -- position included. Nothing else placed it until Draw did, so
// between a rebuild and the next frame the child and everything under it
// carried content-local rectangles while callers hit-tested them with surface
// points.
//
// That does not read as "nothing is there". A stale rectangle at the origin
// still CONTAINS plausible points, so the answer came back confident and
// wrong: go-pdfkit/app measured a press on its third minipage being reported
// as the fourth, and the test that should have caught it passed on exactly
// that wrong answer.
func TestAnUnmeasurableChildIsPlacedAtTheViewportBySetBounds(t *testing.T) {
	col := newCountingColumn(4, 100)
	col.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 400})
	sv := NewScrollView(col)
	sv.SetContentSize(100, 400)

	sv.SetBounds(Rect{X: 40, Y: 30, W: 120, H: 200})

	vp := sv.viewport()
	if got := col.Bounds(); got.X != vp.X || got.Y != vp.Y {
		t.Errorf("the column sits at %d,%d after SetBounds; the viewport is at %d,%d",
			got.X, got.Y, vp.X, vp.Y)
	}
	// The row is what a caller hit-tests, and it is the row that was wrong.
	if got := col.rows[2].Bounds(); got.Y != vp.Y+200 {
		t.Errorf("the third row sits at y=%d; the viewport starts at %d, so it belongs at %d",
			got.Y, vp.Y, vp.Y+200)
	}
	// Its own size is still the caller's: SetBounds sizes a measurable child
	// and only ever moves this one.
	if got := col.Bounds(); got.W != 100 || got.H != 400 {
		t.Errorf("the column was resized to %dx%d; a child that cannot measure keeps the size its caller gave it",
			got.W, got.H)
	}
}

// The control that matters: SetBounds and Draw now agree. Drawing used to be
// the only thing that placed the child, so a reader of the bounds got one
// answer before a frame and another after it.
func TestPlacingAChildAgreesWithDrawingIt(t *testing.T) {
	col := newCountingColumn(4, 100)
	col.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 400})
	sv := NewScrollView(col)
	sv.SetContentSize(100, 400)
	sv.SetBounds(Rect{X: 40, Y: 30, W: 120, H: 200})

	placed := col.rows[2].Bounds()

	buf := make([]byte, 400*400*4)
	sv.Draw(&painter.PixelPainter{Buf: buf, Width: 400, Height: 400}, DefaultDark())

	if drawn := col.rows[2].Bounds(); drawn != placed {
		t.Errorf("laying out put the third row at %+v and painting put it at %+v", placed, drawn)
	}
}
