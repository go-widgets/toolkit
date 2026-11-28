package toolkit

import (
	"math/rand"
	"testing"
	"time"

	"github.com/go-widgets/painter"
)

// pixelSet is a painter that only counts how often each pixel is lit, whether
// by PutPixel or inside a FillRect, and how many calls it took.
type pixelSet struct {
	painter.Painter
	hits  map[[2]int]int
	calls int
}

func newPixelSet() *pixelSet { return &pixelSet{hits: map[[2]int]int{}} }

func (s *pixelSet) PutPixel(x, y int, _ painter.RGBA) {
	s.calls++
	s.hits[[2]int{x, y}]++
}

func (s *pixelSet) FillRect(r painter.Rect, _ painter.RGBA) {
	s.calls++
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			s.hits[[2]int{x, y}]++
		}
	}
}

// refLine is the original pixel-by-pixel Bresenham walk, kept as the oracle.
func refLine(p painter.Painter, x0, y0, x1, y1 int, c RGBA) {
	dx, dy := x1-x0, y1-y0
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy
	for {
		p.PutPixel(x0, y0, c)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

// boundedDrawLine runs drawLine under a deadline. A line has at most
// max(|dx|,|dy|)+1 steps, so a walk that has not returned after seconds has lost
// its direction (a wrong step sign walks away from the end point for ever): the
// test fails instead of hanging. The spinning goroutine is abandoned; the failed
// test ends the process soon after.
func boundedDrawLine(t *testing.T, p painter.Painter, x0, y0, x1, y1 int) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		drawLine(p, x0, y0, x1, y1, RGBA{R: 1, A: 255})
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("(%d,%d)-(%d,%d): drawLine did not return: the walk never reaches its end point", x0, y0, x1, y1)
	}
}

func samePixels(t *testing.T, x0, y0, x1, y1 int) (spans, ref *pixelSet) {
	t.Helper()
	spans, ref = newPixelSet(), newPixelSet()
	boundedDrawLine(t, spans, x0, y0, x1, y1)
	refLine(ref, x0, y0, x1, y1, RGBA{R: 1, A: 255})
	if len(spans.hits) != len(ref.hits) {
		t.Fatalf("(%d,%d)-(%d,%d): %d pixels, want %d", x0, y0, x1, y1, len(spans.hits), len(ref.hits))
	}
	for k, n := range ref.hits {
		if spans.hits[k] != n {
			t.Fatalf("(%d,%d)-(%d,%d): pixel %v lit %d times, want %d", x0, y0, x1, y1, k, spans.hits[k], n)
		}
	}
	return spans, ref
}

// Every line lights exactly the pixels the pixel walk lights, once each, in all
// eight octants, both directions, and the degenerate point.
func TestDrawLineLightsTheSamePixelsAsTheBresenhamWalk(t *testing.T) {
	for _, d := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {5, 0}, {0, 5}, {5, 5}, {5, 2}, {2, 5}, {9, 1}, {1, 9}, {13, 7}, {7, 13}} {
		for _, sx := range []int{1, -1} {
			for _, sy := range []int{1, -1} {
				samePixels(t, 10, 10, 10+sx*d[0], 10+sy*d[1])
				samePixels(t, 10+sx*d[0], 10+sy*d[1], 10, 10)
			}
		}
	}
	rng := rand.New(rand.NewSource(1))
	for range 3000 {
		samePixels(t, rng.Intn(80)-40, rng.Intn(80)-40, rng.Intn(80)-40, rng.Intn(80)-40)
	}
}

// An axis rule is one call, a shallow line one call per row, a steep line one
// call per column: the point of the change.
func TestDrawLineCoalescesRuns(t *testing.T) {
	for name, c := range map[string]struct{ x0, y0, x1, y1, calls int }{
		"horizontal":     {0, 3, 99, 3, 1},
		"horizontal rtl": {99, 3, 0, 3, 1},
		"vertical":       {4, 0, 4, 99, 1},
		"vertical up":    {4, 99, 4, 0, 1},
		"point":          {2, 2, 2, 2, 1},
		"shallow 4:1":    {0, 0, 40, 10, 11},
		"steep 1:4":      {0, 0, 10, 40, 11},
	} {
		s, _ := samePixels(t, c.x0, c.y0, c.x1, c.y1)
		if s.calls != c.calls {
			t.Errorf("%s: %d paint calls, want %d", name, s.calls, c.calls)
		}
	}
}

func BenchmarkDrawLineCalls(b *testing.B) {
	for range b.N {
		s := newPixelSet()
		drawLine(s, 0, 0, 200, 37, RGBA{A: 255})
	}
}
