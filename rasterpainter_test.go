package toolkit

import (
	"bytes"
	"testing"

	"github.com/go-widgets/painter"
)

// otherRaster is a RasterPainter that is not a *painter.PixelPainter: what a
// GPU painter such as go-widgets/gpupainter looks like to the toolkit. It
// counts the Text calls it gets, because a raster painter must get the
// glyph bits, not a request to draw the string in its own font.
type otherRaster struct {
	*painter.PixelPainter
	texts int
}

func (o *otherRaster) Text(x, y int, s string, c painter.RGBA) { o.texts++ }

// The bitmap font and the synthetic italic draw the same pixels on any
// RasterPainter as on a PixelPainter, and never fall back to Text.
func TestARasterPainterGetsThePixelPaintersText(t *testing.T) {
	italic, err := NewSyntheticItalicFont(NewBitmapFont(2))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		f    Font
	}{
		{"bitmap ×1", NewBitmapFont(1)},
		{"bitmap ×3", NewBitmapFont(3)},
		{"synthetic italic", italic},
	} {
		const w, h = 160, 40
		ink := RGBA{R: 230, G: 200, B: 40, A: 255}
		want := make([]byte, 4*w*h)
		tc.f.Draw(painter.NewPixelPainter(want, w, h), 3, 4, "Ag àé 09", ink)
		got := make([]byte, 4*w*h)
		o := &otherRaster{PixelPainter: painter.NewPixelPainter(got, w, h)}
		tc.f.Draw(o, 3, 4, "Ag àé 09", ink)
		if o.texts != 0 {
			t.Errorf("%s: a RasterPainter was asked for Text %d times", tc.name, o.texts)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: the RasterPainter's pixels differ from the PixelPainter's", tc.name)
		}
		if bytes.Count(want, []byte{230}) == 0 {
			t.Fatalf("%s: nothing was drawn, the comparison proves nothing", tc.name)
		}
	}
}
