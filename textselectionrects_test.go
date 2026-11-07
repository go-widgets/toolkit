// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// fillRecorder records every FillRect, so a test can compare what Draw paints
// against what SelectedRects reports. pulltorefresh_test.go's recPainter
// declares a fills field but its FillRect body is empty, so it cannot serve.
type fillRecorder struct {
	fills []painter.Rect
}

func (r *fillRecorder) FillRect(rc painter.Rect, _ RGBA)             { r.fills = append(r.fills, rc) }
func (r *fillRecorder) StrokeRect(painter.Rect, RGBA, int)           {}
func (r *fillRecorder) FillRoundRect(painter.Rect, int, RGBA)        {}
func (r *fillRecorder) StrokeRoundRect(painter.Rect, int, RGBA, int) {}
func (r *fillRecorder) PutPixel(int, int, RGBA)                      {}
func (r *fillRecorder) Text(int, int, string, RGBA)                  {}
func (r *fillRecorder) Size() (int, int)                             { return 1000, 1000 }

// sameRects reports whether got holds exactly want, in order.
func sameRects(got, want []Rect) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestSelectedRectsIsEmptyForNoSelection(t *testing.T) {
	// No runs at all.
	if got := (&TextSelection{}).SelectedRects(); got != nil {
		t.Fatalf("no runs: want nil, got %v", got)
	}
	// Runs, but nothing selected.
	s := makeSel()
	if got := s.SelectedRects(); got != nil {
		t.Fatalf("untouched: want nil, got %v", got)
	}
	// A press with no drag is a collapsed selection, which lights nothing.
	s.Begin(0, 0)
	if got := s.SelectedRects(); got != nil {
		t.Fatalf("collapsed: want nil, got %v", got)
	}
}

func TestSelectedRectsSpansThePressedCharacters(t *testing.T) {
	// "Hello world" at 6px/char: press inside char 2, drag to char 5.
	s := makeSel()
	s.Begin(12, 3)
	s.Drag(30, 3)
	s.End()
	if txt := s.SelectedText(); txt != "llo" {
		t.Fatalf("selected text %q, want %q", txt, "llo")
	}
	want := []Rect{{X: 12, Y: 0, W: 18, H: 10}}
	if got := s.SelectedRects(); !sameRects(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestSelectedRectsIsOneRectPerLine(t *testing.T) {
	// From char 6 of "Hello world" to char 3 of "second line": the first rect
	// runs from the press to the end of its line, the second from the start of
	// the next line to the release.
	s := makeSel()
	s.Begin(36, 3)
	s.Drag(18, 13)
	s.End()
	want := []Rect{
		{X: 36, Y: 0, W: 30, H: 10}, // "world"
		{X: 0, Y: 10, W: 18, H: 10}, // "sec"
	}
	got := s.SelectedRects()
	if !sameRects(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if txt := s.SelectedText(); txt != "world\nsec" {
		t.Fatalf("selected text %q, want %q", txt, "world\nsec")
	}
}

func TestSelectedRectsLightsAnUnmeasurableInteriorLine(t *testing.T) {
	// A run the host gave no Font for cannot be measured inside, so every
	// character offset lands at its left edge and the per-character rect
	// collapses. A run wholly inside the selection is then painted across the
	// bounds the host did give, so a drag through it stays continuous; an edge
	// run, which holds only part of its text, contributes nothing rather than a
	// rect that would be wrong at both ends. SetRuns drops empty-text runs, so a
	// missing font is the only way a covered run measures zero.
	s := &TextSelection{}
	s.SetRuns([]TextRun{
		{Text: "first", Bounds: Rect{X: 0, Y: 0, W: 30, H: 10}},
		{Text: "middle", Bounds: Rect{X: 0, Y: 10, W: 40, H: 10}},
		{Text: "third", Bounds: Rect{X: 0, Y: 20, W: 30, H: 10}},
	})
	s.Begin(12, 3)
	s.Drag(18, 23)
	s.End()
	want := []Rect{{X: 0, Y: 10, W: 40, H: 10}}
	if got := s.SelectedRects(); !sameRects(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	// And Draw paints that one rect too, not the three runs it walked.
	rec := &fillRecorder{}
	s.Draw(rec, RGBA{A: 255})
	if len(rec.fills) != 1 || rec.fills[0] != (painter.Rect{X: 0, Y: 10, W: 40, H: 10}) {
		t.Fatalf("Draw filled %+v", rec.fills)
	}
}

func TestSelectedRectsSkipsAnEmptyEdgeRun(t *testing.T) {
	// The selection ends at the very start of the last run, which therefore
	// holds none of its characters and must contribute no rect — unlike an
	// interior run, it is not wholly covered.
	s := makeSel()
	s.Begin(36, 3)
	s.Drag(0, 13)
	s.End()
	want := []Rect{{X: 36, Y: 0, W: 30, H: 10}}
	if got := s.SelectedRects(); !sameRects(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDrawPaintsExactlySelectedRects(t *testing.T) {
	// The reason SelectedRects exists: a host that acts on a selection — the
	// workbench writing a PDF highlight annotation over the words, say — must
	// mark what the reader sees lit. Draw is a loop over SelectedRects, and
	// this is the control that keeps the two from drifting apart.
	cases := []struct {
		name           string
		bx, by, dx, dy int
	}{
		{"one line", 12, 3, 30, 3},
		{"two lines", 36, 3, 18, 13},
		{"backwards", 18, 13, 36, 3},
		{"whole document", 0, 3, 66, 13},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := makeSel()
			s.Begin(c.bx, c.by)
			s.Drag(c.dx, c.dy)
			s.End()
			rects := s.SelectedRects()
			if len(rects) == 0 {
				t.Fatal("selection reports no rects; the case proves nothing")
			}
			rec := &fillRecorder{}
			s.Draw(rec, RGBA{R: 1, G: 2, B: 3, A: 4})
			if len(rec.fills) != len(rects) {
				t.Fatalf("Draw filled %d rects, SelectedRects reports %d",
					len(rec.fills), len(rects))
			}
			for i, r := range rects {
				want := painter.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}
				if rec.fills[i] != want {
					t.Fatalf("fill %d is %+v, SelectedRects says %+v",
						i, rec.fills[i], want)
				}
			}
		})
	}
	// An empty selection paints nothing.
	rec := &fillRecorder{}
	makeSel().Draw(rec, RGBA{A: 255})
	if len(rec.fills) != 0 {
		t.Fatalf("empty selection painted %d rects", len(rec.fills))
	}
}
