package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// abRender draws a into a fresh w×h pixel buffer and returns it (zero RGBA where
// untouched, so painted ground is distinguishable).
func abRender(a *AddressBar, w, h int, theme *Theme) []byte {
	buf := make([]byte, 4*w*h)
	p := painter.NewPixelPainter(buf, w, h)
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	a.Draw(p, theme)
	return buf
}

func abPx(buf []byte, w, x, y int) RGBA {
	i := 4 * (y*w + x)
	return RGBA{R: buf[i], G: buf[i+1], B: buf[i+2], A: buf[i+3]}
}

func abClick(a *AddressBar, x, y int) {
	a.OnEvent(Event{Kind: EventClick, X: x, Y: y})
}

func TestAddressBarValueAndFocus(t *testing.T) {
	a := &AddressBar{Text: "http://x", Radius: 4, TextPad: 4}
	if a.Focused() || a.Copied() {
		t.Fatal("new field should be unfocused and un-copied")
	}
	if a.Value() != "http://x" {
		t.Fatalf("unfocused Value = %q, want the Text", a.Value())
	}
	if info := a.A11y(); info.Role != RoleTextbox || info.Value != "http://x" {
		t.Fatalf("A11y = %+v, want a textbox carrying the value", info)
	}
	// A plain click (no bookmark slot) focuses and seeds the buffer from Text.
	abClick(a, 5, 10)
	if !a.Focused() || a.Value() != "http://x" {
		t.Fatalf("after focus click: focused=%v value=%q", a.Focused(), a.Value())
	}
	a.Blur()
	if a.Focused() || a.Copied() {
		t.Fatal("Blur should clear focus and the copy highlight")
	}
}

func TestAddressBarEditing(t *testing.T) {
	changes := 0
	var committed string
	a := &AddressBar{Text: "seed", Radius: 4, TextPad: 4,
		OnChange: func() { changes++ },
		OnCommit: func(s string) { committed = s },
	}
	// Unfocused char / key are ignored (no change).
	a.OnEvent(Event{Kind: EventChar, Code: "z"})
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if a.Value() != "seed" || changes != 0 {
		t.Fatalf("unfocused edit leaked: value=%q changes=%d", a.Value(), changes)
	}
	abClick(a, 5, 10)                           // focus → buf = "seed", fires change
	a.OnEvent(Event{Kind: EventChar, Code: ""}) // empty code ignored
	a.OnEvent(Event{Kind: EventChar, Code: "!"})
	if a.Value() != "seed!" {
		t.Fatalf("after typing: %q", a.Value())
	}
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if a.Value() != "seed" {
		t.Fatalf("after backspace: %q", a.Value())
	}
	a.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"}) // unrelated key: no-op
	// Enter commits the trimmed buffer.
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if committed != "seed" || a.Focused() {
		t.Fatalf("commit: committed=%q focused=%v", committed, a.Focused())
	}
	// Backspace down to empty then once more (the len==0 guard).
	abClick(a, 5, 10)
	a.buf = ""
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if a.Value() != "" {
		t.Fatalf("empty backspace changed buffer to %q", a.Value())
	}
}

func TestAddressBarCommitEmptyAndNilOnCommit(t *testing.T) {
	changes := 0
	a := &AddressBar{Text: "x", OnChange: func() { changes++ }}
	// Whitespace-only buffer: no OnCommit, just defocus + a change.
	a.focused, a.buf = true, "   "
	called := false
	a.OnCommit = func(string) { called = true }
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if called || a.Focused() {
		t.Fatalf("empty commit fired OnCommit=%v focused=%v", called, a.Focused())
	}
	// Non-empty buffer with a nil OnCommit: the else branch fires OnChange.
	a.OnCommit = nil
	a.focused, a.buf = true, "hello"
	before := changes
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if a.Focused() || changes == before {
		t.Fatalf("nil-OnCommit commit: focused=%v changes moved=%v", a.Focused(), changes != before)
	}
}

func TestAddressBarCopySelectAll(t *testing.T) {
	a := &AddressBar{Text: "", Radius: 4, TextPad: 4}
	if _, ok := a.CopySelectAll(); ok {
		t.Fatal("copy while unfocused should be a no-op")
	}
	a.focused = true // focused but empty value
	if _, ok := a.CopySelectAll(); ok {
		t.Fatal("copy of an empty value should be a no-op")
	}
	a.buf = "https://copied"
	SetClipboardText("stale")
	txt, ok := a.CopySelectAll()
	if !ok || txt != "https://copied" || ClipboardText() != "https://copied" || !a.Copied() {
		t.Fatalf("copy: ok=%v txt=%q clip=%q copied=%v", ok, txt, ClipboardText(), a.Copied())
	}
}

func TestAddressBarBookmark(t *testing.T) {
	var toggled []bool
	a := &AddressBar{
		BookmarkIcon:     func(p painter.Painter, r Rect, ink RGBA, on bool) {},
		OnBookmarkToggle: func(on bool) { toggled = append(toggled, on) },
		Radius:           4, TextPad: 4,
	}
	a.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 30})
	_, _, star := a.zones(a.Bounds())
	// A click on the star slot toggles the bookmark, not focus.
	abClick(a, star.X+star.W/2, star.Y+star.H/2)
	if !a.Bookmarked || a.Focused() || len(toggled) != 1 || !toggled[0] {
		t.Fatalf("star click: bookmarked=%v focused=%v toggled=%v", a.Bookmarked, a.Focused(), toggled)
	}
	// A click in the text zone focuses instead.
	abClick(a, 10, 15)
	if !a.Focused() {
		t.Fatal("text-zone click should focus")
	}
	// A nil OnBookmarkToggle is a safe no-op.
	a.OnBookmarkToggle = nil
	a.toggleBookmark()
	if a.Bookmarked {
		t.Fatal("second toggle should turn the bookmark back off")
	}
}

func TestAddressBarZones(t *testing.T) {
	// No hooks → the whole rect is the text zone.
	a := &AddressBar{}
	full := Rect{X: 5, Y: 0, W: 100, H: 20}
	if l, txt, s := a.zones(full); l != (Rect{}) || s != (Rect{}) || txt != full {
		t.Fatalf("no-hooks zones: lead=%+v star=%+v text=%+v", l, s, txt)
	}
	// Both hooks → square slots at each end, text between.
	a.LeadingIcon = func(p painter.Painter, r Rect, ink RGBA) {}
	a.BookmarkIcon = func(p painter.Painter, r Rect, ink RGBA, on bool) {}
	lead, txt, star := a.zones(full)
	if lead.W != full.H || star.X != full.X+full.W-full.H || txt.W != full.W-2*full.H {
		t.Fatalf("two-slot zones: lead=%+v text=%+v star=%+v", lead, txt, star)
	}
	// Too-narrow field: the text zone clamps to zero width.
	if _, tz, _ := a.zones(Rect{X: 0, Y: 0, W: 10, H: 30}); tz.W != 0 {
		t.Fatalf("narrow text zone W = %d, want 0", tz.W)
	}
}

func TestAddressBarDraw(t *testing.T) {
	const w, h = 160, 24
	th := DefaultLight()
	// Zero-size bounds: Draw is a no-op (buffer stays zeroed, no panic).
	empty := &AddressBar{Text: "x"}
	empty.SetBounds(Rect{})
	empty.Draw(painter.NewPixelPainter(make([]byte, 16), 2, 2), th)

	// Unfocused: ground is painted (border ring, non-zero alpha along the frame).
	a := &AddressBar{Text: "http://example.com/a/long/path", Radius: 4, TextPad: 4}
	buf := abRender(a, w, h, th)
	if abPx(buf, w, 0, h/2).A == 0 {
		t.Fatal("unfocused field left its ground unpainted")
	}

	// Focused with icons + a copy highlight + an overflowing URL (head-clip path)
	// + a caret. Just assert it paints and does not panic.
	leadN, starN := 0, 0
	a.LeadingIcon = func(p painter.Painter, r Rect, ink RGBA) { leadN++ }
	a.BookmarkIcon = func(p painter.Painter, r Rect, ink RGBA, on bool) { starN++ }
	a.focused = true
	a.buf = "http://example.com/an/even/longer/overflowing/path?with=query"
	a.copied = true
	buf = abRender(a, w, h, th)
	if leadN == 0 || starN == 0 {
		t.Fatalf("focused draw did not invoke icon hooks: lead=%d star=%d", leadN, starN)
	}
	if abPx(buf, w, 0, h/2).A == 0 {
		t.Fatal("focused field left its ground unpainted")
	}
}
