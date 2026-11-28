package toolkit

import (
	"bytes"
	"testing"

	"github.com/go-widgets/painter"
)

// counting is a back-end that carries painter.ImagePainter and only counts the
// calls it gets, as a recording back-end would write one element for each.
type counting struct {
	painter.Painter
	images, pixels, rects int
	imageW, imageH        int
}

func (c *counting) DrawImage(_ painter.Rect, _ []byte, w, h int) {
	c.images++
	c.imageW, c.imageH = w, h
}
func (c *counting) PutPixel(int, int, RGBA)            { c.pixels++ }
func (c *counting) FillRect(painter.Rect, RGBA)        { c.rects++ }
func (c *counting) StrokeRect(painter.Rect, RGBA, int) { c.rects++ }
func (c *counting) Text(int, int, string, RGBA)        {}

// The SV square is one image, not a PutPixel per pixel, and the hue strip and
// the alpha slider are a rect per row, column or checker run: a recording of the
// picker is a few hundred elements instead of about 20000.
func TestColorPickerPaintsFewCalls(t *testing.T) {
	cp := &ColorPicker{h: 200, s: 0.4, v: 0.7, alpha: 200}
	cp.SetBounds(Rect{X: 0, Y: 0, W: ColorPickerWidth, H: ColorPickerHeight})
	c := &counting{Painter: newP(makeSurface(ColorPickerWidth, ColorPickerHeight), ColorPickerWidth)}
	cp.Draw(c, DefaultLight())
	sv := cp.svRectLocal()
	if c.images != 1 || c.imageW != sv.W || c.imageH != sv.H {
		t.Errorf("%d images of %dx%d, want the SV square (%dx%d) as one", c.images, c.imageW, c.imageH, sv.W, sv.H)
	}
	if c.pixels > 100 {
		t.Errorf("%d PutPixel calls, want only the markers", c.pixels)
	}
	if c.rects > 800 {
		t.Errorf("%d rects, want a few hundred at most", c.rects)
	}
}

// perPixel turns every FillRect into one PutPixel per pixel: the picture the
// per-pixel loops this change replaced produced. putCalls counts the PutPixel
// calls that reach the back-end (a FillRect handed through is not one).
type perPixel struct {
	painter.Painter
	expand   bool
	putCalls int
}

func (q *perPixel) PutPixel(x, y int, c RGBA) {
	q.putCalls++
	q.Painter.PutPixel(x, y, c)
}

func (q *perPixel) FillRect(r painter.Rect, c RGBA) {
	if !q.expand {
		q.Painter.FillRect(r, c)
		return
	}
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			q.PutPixel(x, y, c)
		}
	}
}

// The hue strip and the alpha slider paint a row or a column of ONE colour per
// call now, not a pixel per call; the picture is the same, pixel for pixel,
// and only the SV square (a true 2D gradient) and the markers still go
// pixel by pixel.
func TestColorPickerRunsPaintTheSamePixels(t *testing.T) {
	for _, st := range []ColorPicker{
		{h: 180, s: 0.5, v: 0.5, alpha: 255},
		{h: 20, s: 1, v: 1, alpha: 0},
		{h: 359, s: 0, v: 0, alpha: 100},
	} {
		cp := st
		cp.SetBounds(Rect{X: 0, Y: 0, W: ColorPickerWidth, H: ColorPickerHeight})
		var pics [2][]byte
		var puts [2]int
		for i, expand := range []bool{true, false} {
			surf := makeSurface(ColorPickerWidth, ColorPickerHeight)
			q := &perPixel{Painter: newP(surf, ColorPickerWidth), expand: expand}
			cp.Draw(q, DefaultLight())
			pics[i], puts[i] = append([]byte(nil), surf...), q.putCalls
		}
		// The PixelPainter itself carries ImagePainter: the SV square is blitted.
		direct := makeSurface(ColorPickerWidth, ColorPickerHeight)
		cp.Draw(newP(direct, ColorPickerWidth), DefaultLight())
		if !bytes.Equal(pics[0], direct) {
			t.Errorf("h=%v: the image-blitted picture differs from the per-pixel one", st.h)
		}
		if !bytes.Equal(pics[0], pics[1]) {
			t.Errorf("h=%v: the run-painted picture differs from the per-pixel one", st.h)
		}
		sv := cp.svRectLocal()
		if limit := sv.W*sv.H + 400; puts[1] > limit {
			t.Errorf("h=%v: %d PutPixel calls, want at most %d (the SV square plus markers)", st.h, puts[1], limit)
		}
		if puts[1] >= puts[0] {
			t.Errorf("h=%v: %d PutPixel calls with runs, %d without", st.h, puts[1], puts[0])
		}
	}
}

// rectLog records every FillRect, in order, and forwards nothing else.
type rectLog struct {
	painter.Painter
	rects []painter.Rect
	cols  []RGBA
}

func (l *rectLog) FillRect(r painter.Rect, c RGBA) {
	l.rects = append(l.rects, r)
	l.cols = append(l.cols, c)
	l.Painter.FillRect(r, c)
}

// The checkerboard behind the alpha slider alternates light and dark by cell,
// on both axes: the run-painted picture must keep the parity, which a
// comparison between two renderings of the same code cannot see.
func TestColorPickerAlphaCheckerParity(t *testing.T) {
	cp := &ColorPicker{h: 200, s: 0.4, v: 0.7, alpha: 200}
	cp.SetBounds(Rect{X: 0, Y: 0, W: ColorPickerWidth, H: ColorPickerHeight})
	l := &rectLog{Painter: newP(makeSurface(ColorPickerWidth, ColorPickerHeight), ColorPickerWidth)}
	cp.Draw(l, DefaultLight())
	ar := cp.alphaRectLocal()
	cell := scaled(6)
	colorAt := func(x, y int) (RGBA, bool) {
		for i, r := range l.rects {
			if r.X == ar.X+x && r.Y == ar.Y+y && r.H == 1 && r.W <= cell {
				return l.cols[i], true
			}
		}
		return RGBA{}, false
	}
	light, dark := RGB(0xFF, 0xFF, 0xFF), RGB(0xC0, 0xC0, 0xC0)
	for _, tc := range []struct {
		x, y int
		want RGBA
	}{{0, 0, light}, {cell, 0, dark}, {0, cell, dark}, {cell, cell, light}} {
		got, ok := colorAt(tc.x, tc.y)
		if !ok {
			t.Fatalf("no checker run at (%d,%d)", tc.x, tc.y)
		}
		if got != tc.want {
			t.Errorf("checker cell at (%d,%d) = %v, want %v", tc.x, tc.y, got, tc.want)
		}
	}
}
