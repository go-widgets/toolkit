package toolkit

// PointerClaimer is a widget that takes a press only on part of its bounds: a
// Splitter on its dividers, whose bounds span the panes around them. An Edge of
// a PointerRouter must implement it.
type PointerClaimer interface {
	Widget
	// ClaimsPoint reports whether the surface point (x, y) is one the widget
	// takes for itself.
	ClaimsPoint(x, y int) bool
}

// ClaimsPoint reports whether the surface point (x, y) is on a divider: the part
// of the splitter's bounds it takes for itself, the panes keeping the rest.
func (s *Splitter) ClaimsPoint(x, y int) bool { return s.handleAt(x, y) >= 0 }

// Hovered is the divider under the pointer as of the last EventMouseMove it was
// given, or -1.
func (s *Splitter) Hovered() int { return s.hover }

// PointerRouter gives each mouse event to the widget under the pointer, in that
// widget's local coordinates, and keeps the widget a press grabbed until the
// release: a host never translates a coordinate, tests a rectangle or remembers
// whom a drag belongs to.
//
// Edges are the widgets that grab a press on a part of their bounds (a
// Splitter's dividers, a resizable panel's edge); they are asked first, and are
// the ones told of a move and asked for a cursor. Targets are the widgets that
// take a press, a wheel turn or a release on their whole bounds, topmost first. A
// widget laid out in an empty rectangle takes nothing. Either list is read at each
// event, so it follows the layout.
type PointerRouter struct {
	Edges   func() []PointerClaimer
	Targets func() []Widget

	grab  Widget
	grabR Rect
}

// local is e moved into the coordinates of a widget at r.
func localEvent(e Event, r Rect) Event {
	e.X -= r.X
	e.Y -= r.Y
	return e
}

// Press gives a press at the surface point (x, y) to the edge or target under it,
// which is then the grabbed widget, and returns it; with nothing under the point
// the grab is dropped and the result is nil. code is the Event.Code of the press.
func (r *PointerRouter) Press(x, y int, code string) Widget {
	r.grab = nil
	var hit Widget
	if r.Edges != nil {
		for _, e := range r.Edges() {
			if e.ClaimsPoint(x, y) {
				hit = e
				break
			}
		}
	}
	if hit == nil && r.Targets != nil {
		for _, w := range r.Targets() {
			if w.Bounds().Contains(x, y) {
				hit = w
				break
			}
		}
	}
	if hit == nil {
		return nil
	}
	b := hit.Bounds()
	r.grab, r.grabR = hit, b
	hit.OnEvent(localEvent(Event{Kind: EventClick, X: x, Y: y, Code: code}, b))
	return hit
}

// Grabbed is the widget a press holds until the release, or nil.
func (r *PointerRouter) Grabbed() Widget { return r.grab }

// Drag gives a motion to the grabbed widget; false when none is held.
func (r *PointerRouter) Drag(x, y int) bool {
	if r.grab == nil {
		return false
	}
	r.grab.OnEvent(localEvent(Event{Kind: EventMouseDrag, X: x, Y: y}, r.grabR))
	return true
}

// Release gives the release to the grabbed widget and lets it go; false when
// none is held.
func (r *PointerRouter) Release(x, y int) bool {
	if r.grab == nil {
		return false
	}
	g, b := r.grab, r.grabR
	r.grab = nil
	g.OnEvent(localEvent(Event{Kind: EventMouseUp, X: x, Y: y}, b))
	return true
}

// Cancel lets the grabbed widget go without telling it.
func (r *PointerRouter) Cancel() { r.grab = nil }

// Scroll gives a wheel turn (delta in rows, positive down) to the target under
// the pointer and reports whether one took it.
func (r *PointerRouter) Scroll(x, y, delta int) bool {
	if r.Targets == nil {
		return false
	}
	for _, w := range r.Targets() {
		if b := w.Bounds(); b.Contains(x, y) {
			w.OnEvent(localEvent(Event{Kind: EventScroll, X: x, Y: y, Delta: delta}, b))
			return true
		}
	}
	return false
}

// Hover tells every edge where the pointer is, so each can light the divider
// under it (and put it out when the pointer leaves).
func (r *PointerRouter) Hover(x, y int) {
	if r.Edges == nil {
		return
	}
	for _, e := range r.Edges() {
		e.OnEvent(localEvent(Event{Kind: EventMouseMove, X: x, Y: y}, e.Bounds()))
	}
}

// Cursor is the CSS cursor the pointer at the surface point (x, y) should show,
// asked of the edges in turn: "" (the host's default) when none wants one.
func (r *PointerRouter) Cursor(x, y int) string {
	if r.Edges == nil {
		return ""
	}
	for _, e := range r.Edges() {
		if c, ok := e.(interface{ Cursor(x, y int) string }); ok {
			b := e.Bounds()
			if s := c.Cursor(x-b.X, y-b.Y); s != "" {
				return s
			}
		}
	}
	return ""
}
