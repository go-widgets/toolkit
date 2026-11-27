package toolkit

import (
	"bytes"
	"testing"

	"github.com/go-widgets/painter"
)

// rawSweepAt is sweepAt without populate: the widget is built exactly as given,
// so a test can set Wrap and a text long enough to wrap.
func rawSweepAt(t *testing.T, newW func() Widget, s0, s1 float64) []byte {
	t.Helper()
	t.Cleanup(sweepReset)
	sweepReset()
	SetMetricScale(s0)
	if err := UseOpenTypeText(); err != nil {
		t.Fatal(err)
	}
	w := newW()
	sweepDraw(w)
	SetMetricScale(s1)
	buf, _, _ := sweepDraw(w)
	return buf
}

// rowHost draws one row through a ListBox item renderer, so LogoRows is swept
// without the list's own construction-time RowHeight in the way.
type rowHost struct {
	Base
	render func(p painter.Painter, theme *Theme, rc Rect, i int, item string, selected bool, ink RGBA)
}

func (h *rowHost) Draw(p painter.Painter, theme *Theme) {
	r := h.Bounds()
	h.render(p, theme, Rect{X: r.X, Y: r.Y, W: r.W, H: scaled(48)}, 0, "", false, theme.OnSurface)
}

const sweepLongText = "No server is selected, so there is nothing to connect to yet"

// TestWrappingAndLogoWidgetsFollowAScaleChange covers what the generic sweep
// cannot reach: populate never sets Label.Wrap, and LogoRows is a renderer, not
// a widget type. Each case is drawn fresh at 2x and moved 1x->2x and 2x->1x; the
// pixels must be identical to a fresh draw at the target scale.
func TestWrappingAndLogoWidgetsFollowAScaleChange(t *testing.T) {
	cases := map[string]func() Widget{
		"wrapping Label": func() Widget {
			l := &Label{Wrap: true, FontSize: 18}
			l.Text().Set(sweepLongText)
			return l
		},
		"wrapping Label, body face": func() Widget {
			l := &Label{Wrap: true}
			l.Text().Set(sweepLongText)
			return l
		},
		"Logo": func() Widget {
			return NewLogo("logo", func() string { return "" })
		},
		"LogoRows": func() Widget {
			return &rowHost{render: LogoRows(func(int) LogoRow {
				return LogoRow{Title: "No server selected", Subtitle: sweepLongText}
			})}
		},
	}
	for name, mk := range cases {
		fresh2 := rawSweepAt(t, mk, 2, 2)
		fresh1 := rawSweepAt(t, mk, 1, 1)
		_, _, a1, _ := ink(fresh1, int(sweepW), int(sweepH))
		_, _, b1, _ := ink(fresh2, 2*sweepW, 2*sweepH)
		if a1 < 0 || b1 <= a1 {
			t.Errorf("%s: ink reaches x=%d at 1x and x=%d at 2x: want something painted that grows", name, a1, b1)
		}
		if got := rawSweepAt(t, mk, 1, 2); !bytes.Equal(got, fresh2) {
			t.Errorf("%s: moved 1x->2x differs from a fresh 2x draw", name)
		}
		if got := rawSweepAt(t, mk, 2, 1); !bytes.Equal(got, fresh1) {
			t.Errorf("%s: moved 2x->1x differs from a fresh 1x draw", name)
		}
	}
}

// A wrapped label measures its height from the same logical FontSize x
// MetricScale as it paints, so the lines HeightFor counts are the lines drawn.
func TestWrappedLabelHeightScalesWithTheFace(t *testing.T) {
	t.Cleanup(sweepReset)
	sweepReset()
	if err := UseOpenTypeText(); err != nil {
		t.Fatal(err)
	}
	l := &Label{Wrap: true, FontSize: 18}
	l.Text().Set(sweepLongText)
	SetMetricScale(1)
	h1 := l.HeightFor(200)
	SetMetricScale(2)
	h2 := l.HeightFor(400)
	if h1 <= 0 || h2 < h1*3/2 {
		t.Fatalf("HeightFor 200px at 1x = %d, 400px at 2x = %d: want the 2x height about double", h1, h2)
	}
}
