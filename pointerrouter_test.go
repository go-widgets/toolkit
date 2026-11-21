package toolkit

import (
	"testing"
	"time"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// rec records the events it is given.
type rec struct {
	Base
	got []Event
}

func (r *rec) Draw(p painter.Painter, th *Theme) {}
func (r *rec) OnEvent(e Event)                   { r.got = append(r.got, e) }

func newRec(b Rect) *rec { r := &rec{}; r.SetBounds(b); return r }

// offset builds a splitter whose bounds do not start at the origin, so a
// coordinate that was not translated shows.
func offset(t *testing.T) (*Splitter, *rec, *rec) {
	t.Helper()
	l, r := newRec(Rect{}), newRec(Rect{})
	s := NewSplitter(false,
		&SplitPane{Widget: l, Name: "l", Weight: mvvm.NewObservable(1), Min: 50},
		&SplitPane{Widget: r, Name: "r", Size: mvvm.NewObservable(200), Min: 50})
	s.Now = func() time.Time { return time.Unix(1000, 0) }
	s.SetBounds(Rect{X: 100, Y: 30, W: 800, H: 400})
	return s, l, r
}

func TestPointerRouterLocalCoordinatesAndGrab(t *testing.T) {
	s, _, _ := offset(t)
	btn := newRec(Rect{X: 10, Y: 500, W: 80, H: 20})
	empty := newRec(Rect{X: 0, Y: 0, W: 0, H: 0})
	pr := &PointerRouter{
		Edges:   func() []PointerClaimer { return []PointerClaimer{s} },
		Targets: func() []Widget { return []Widget{empty, btn} },
	}
	if w := pr.Press(15, 505, ""); w != Widget(btn) || pr.Grabbed() != Widget(btn) {
		t.Fatalf("press hit %v", w)
	}
	if e := btn.got[0]; e.Kind != EventClick || e.X != 5 || e.Y != 5 {
		t.Fatalf("click %+v, want local (5,5)", e)
	}
	if !pr.Drag(40, 510) || btn.got[1].Kind != EventMouseDrag || btn.got[1].X != 30 || btn.got[1].Y != 10 {
		t.Fatalf("drag %+v", btn.got)
	}
	// the grab holds when the pointer has left the widget
	if !pr.Release(900, 900) || btn.got[2].Kind != EventMouseUp || btn.got[2].X != 890 {
		t.Fatalf("release %+v", btn.got)
	}
	if pr.Drag(1, 1) || pr.Release(1, 1) || pr.Grabbed() != nil {
		t.Fatal("nothing is held after the release")
	}
	if pr.Press(3, 3, "") != nil || len(empty.got) != 0 {
		t.Fatal("an empty rectangle takes no press, and a press on nothing grabs nothing")
	}
	pr.Press(15, 505, "")
	pr.Press(3, 3, "")
	if pr.Grabbed() != nil {
		t.Fatal("a press on nothing drops the grab")
	}
	pr.Press(15, 505, "")
	pr.Cancel()
	if pr.Grabbed() != nil {
		t.Fatal("cancel drops the grab")
	}
}

func TestPointerRouterSplitterDragHoverCursor(t *testing.T) {
	s, l, _ := offset(t)
	other := newRec(Rect{X: 100, Y: 30, W: 800, H: 400})
	pr := &PointerRouter{
		Edges:   func() []PointerClaimer { return []PointerClaimer{s} },
		Targets: func() []Widget { return []Widget{other} },
	}
	hr, ok := s.HandleRect(0)
	if !ok {
		t.Fatal("no divider")
	}
	x, y := hr.X+hr.W/2, hr.Y+hr.H/2
	if pr.Cursor(x, y) != "col-resize" || pr.Cursor(x-100, y) != "" {
		t.Fatalf("cursor %q / %q", pr.Cursor(x, y), pr.Cursor(x-100, y))
	}
	pr.Hover(x, y)
	if s.Hovered() != 0 {
		t.Fatalf("hover %d", s.Hovered())
	}
	pr.Hover(x-100, y)
	if s.Hovered() != -1 {
		t.Fatalf("hover %d after leaving", s.Hovered())
	}
	before := s.PaneSize(1)
	if pr.Press(x, y, "") != Widget(s) || len(other.got) != 0 {
		t.Fatal("the divider is the edge, ahead of the target under it")
	}
	pr.Drag(x-40, y)
	pr.Release(x-40, y)
	if got := s.PaneSize(1); got != before+40 {
		t.Fatalf("right pane %d, want %d", got, before+40)
	}
	_ = l
	// a press away from a divider falls to the target
	if pr.Press(hr.X-200, y, "") != Widget(other) {
		t.Fatal("a press off the divider reaches the target")
	}
}

func TestPointerRouterScrollAndNilLists(t *testing.T) {
	w := newRec(Rect{X: 20, Y: 20, W: 50, H: 50})
	pr := &PointerRouter{Targets: func() []Widget { return []Widget{w} }}
	if !pr.Scroll(25, 30, 3) || w.got[0].Kind != EventScroll || w.got[0].X != 5 || w.got[0].Y != 10 || w.got[0].Delta != 3 {
		t.Fatalf("scroll %+v", w.got)
	}
	if pr.Scroll(5, 5, 1) {
		t.Fatal("a wheel turn over nothing is not taken")
	}
	pr.Hover(1, 1)
	if pr.Cursor(1, 1) != "" {
		t.Fatal("no edges, no cursor")
	}
	var none PointerRouter
	if none.Press(1, 1, "") != nil || none.Scroll(1, 1, 1) {
		t.Fatal("empty router")
	}
}
