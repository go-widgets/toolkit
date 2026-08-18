package toolkit

import (
	"testing"

	"github.com/go-widgets/mvvm"
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

func TestAddressBarStateIsObservableOnly(t *testing.T) {
	a := &AddressBar{Radius: 4, TextPad: 4}
	a.URL().Set("http://x")
	if a.Focused().Get() || a.Copied().Get() || a.Bookmarked().Get() {
		t.Fatal("new field: focus/copy/bookmark should be false")
	}
	if a.Value() != "http://x" {
		t.Fatalf("unfocused Value = %q, want the URL", a.Value())
	}
	if info := a.A11y(); info.Role != RoleTextbox || info.Value != "http://x" {
		t.Fatalf("A11y = %+v, want a textbox carrying the value", info)
	}
	// A view model observing focus sees the click flip it (no field assignment
	// exists — the accessors are the only mutation path).
	var focusChanges int
	a.Focused().Subscribe(func(bool) { focusChanges++ })
	abClick(a, 5, 10) // plain click focuses, seeds Editing from URL
	if !a.Focused().Get() || a.Value() != "http://x" || focusChanges != 1 {
		t.Fatalf("focus click: focused=%v value=%q changes=%d", a.Focused().Get(), a.Value(), focusChanges)
	}
	a.Blur()
	if a.Focused().Get() || a.Copied().Get() {
		t.Fatal("Blur should clear focus and the copy highlight")
	}
}

func TestAddressBarEditing(t *testing.T) {
	var committed string
	a := &AddressBar{Radius: 4, TextPad: 4}
	a.URL().Set("seed")
	a.Commit = mvvm.NewCommand(func() { committed = a.Editing().Get() }, nil)
	// Unfocused char / key are ignored.
	a.OnEvent(Event{Kind: EventChar, Code: "z"})
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if a.Value() != "seed" {
		t.Fatalf("unfocused edit leaked: %q", a.Value())
	}
	abClick(a, 5, 10)                           // focus → Editing = "seed"
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
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})     // commit
	if committed != "seed" || a.Focused().Get() {
		t.Fatalf("commit: committed=%q focused=%v", committed, a.Focused().Get())
	}
	// Backspace down to empty then once more (the len==0 guard).
	abClick(a, 5, 10)
	a.Editing().Set("")
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if a.Value() != "" {
		t.Fatalf("empty backspace changed buffer to %q", a.Value())
	}
}

func TestAddressBarCommitVariants(t *testing.T) {
	a := &AddressBar{}
	called := false
	a.Commit = mvvm.NewCommand(func() { called = true }, nil)
	// Whitespace-only buffer: Commit not run, just defocus.
	a.Focused().Set(true)
	a.Editing().Set("   ")
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if called || a.Focused().Get() {
		t.Fatalf("empty commit ran=%v focused=%v", called, a.Focused().Get())
	}
	// Non-empty with a nil Commit: safe no-op, still defocuses.
	a.Commit = nil
	a.Focused().Set(true)
	a.Editing().Set("hello")
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if a.Focused().Get() {
		t.Fatal("nil-Commit Enter should still defocus")
	}
	// Non-empty with a bound Commit: runs.
	a.Commit = mvvm.NewCommand(func() { called = true }, nil)
	a.Focused().Set(true)
	a.Editing().Set("go")
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if !called {
		t.Fatal("bound Commit should run on a non-empty Enter")
	}
}

func TestAddressBarCopySelectAll(t *testing.T) {
	a := &AddressBar{Radius: 4, TextPad: 4}
	if _, ok := a.CopySelectAll(); ok {
		t.Fatal("copy while unfocused should be a no-op")
	}
	a.Focused().Set(true) // focused but empty value
	if _, ok := a.CopySelectAll(); ok {
		t.Fatal("copy of an empty value should be a no-op")
	}
	a.Editing().Set("https://copied")
	SetClipboardText("stale")
	txt, ok := a.CopySelectAll()
	if !ok || txt != "https://copied" || ClipboardText() != "https://copied" || !a.Copied().Get() {
		t.Fatalf("copy: ok=%v txt=%q clip=%q copied=%v", ok, txt, ClipboardText(), a.Copied().Get())
	}
}

func TestAddressBarBookmark(t *testing.T) {
	var toggled []bool
	a := &AddressBar{
		BookmarkIcon: func(p painter.Painter, r Rect, ink RGBA, on bool) {},
		Radius:       4, TextPad: 4,
	}
	// A host subscribes to the bookmark Observable instead of a callback.
	a.Bookmarked().Subscribe(func(on bool) { toggled = append(toggled, on) })
	a.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 30})
	_, _, star := a.zones(a.Bounds())
	abClick(a, star.X+star.W/2, star.Y+star.H/2) // star toggles, not focus
	if !a.Bookmarked().Get() || a.Focused().Get() || len(toggled) != 1 || !toggled[0] {
		t.Fatalf("star click: bookmarked=%v focused=%v toggled=%v", a.Bookmarked().Get(), a.Focused().Get(), toggled)
	}
	abClick(a, 10, 15) // text zone focuses instead
	if !a.Focused().Get() {
		t.Fatal("text-zone click should focus")
	}
	a.toggleBookmark() // back off
	if a.Bookmarked().Get() || len(toggled) != 2 || toggled[1] {
		t.Fatalf("second toggle: bookmarked=%v toggled=%v", a.Bookmarked().Get(), toggled)
	}
}

func TestAddressBarZones(t *testing.T) {
	a := &AddressBar{}
	full := Rect{X: 5, Y: 0, W: 100, H: 20}
	if l, txt, s := a.zones(full); l != (Rect{}) || s != (Rect{}) || txt != full {
		t.Fatalf("no-hooks zones: lead=%+v star=%+v text=%+v", l, s, txt)
	}
	a.LeadingIcon = func(p painter.Painter, r Rect, ink RGBA) {}
	a.BookmarkIcon = func(p painter.Painter, r Rect, ink RGBA, on bool) {}
	lead, txt, star := a.zones(full)
	if lead.W != full.H || star.X != full.X+full.W-full.H || txt.W != full.W-2*full.H {
		t.Fatalf("two-slot zones: lead=%+v text=%+v star=%+v", lead, txt, star)
	}
	if _, tz, _ := a.zones(Rect{X: 0, Y: 0, W: 10, H: 30}); tz.W != 0 {
		t.Fatalf("narrow text zone W = %d, want 0", tz.W)
	}
}

func TestAddressBarDraw(t *testing.T) {
	const w, h = 160, 24
	th := DefaultLight()
	// Zero-size bounds: Draw is a no-op (no panic).
	empty := &AddressBar{}
	empty.URL().Set("x")
	empty.SetBounds(Rect{})
	empty.Draw(painter.NewPixelPainter(make([]byte, 16), 2, 2), th)

	// Unfocused: the ground is painted.
	a := &AddressBar{Radius: 4, TextPad: 4}
	a.URL().Set("http://example.com/a/long/path")
	if abPx(abRender(a, w, h, th), w, 0, h/2).A == 0 {
		t.Fatal("unfocused field left its ground unpainted")
	}

	// Focused + icons + copy highlight + an overflowing URL (head-clip) + caret.
	leadN, starN := 0, 0
	a.LeadingIcon = func(p painter.Painter, r Rect, ink RGBA) { leadN++ }
	a.BookmarkIcon = func(p painter.Painter, r Rect, ink RGBA, on bool) { starN++ }
	a.Focused().Set(true)
	a.Editing().Set("http://example.com/an/even/longer/overflowing/path?with=query")
	a.Copied().Set(true)
	buf := abRender(a, w, h, th)
	if leadN == 0 || starN == 0 {
		t.Fatalf("focused draw did not invoke icon hooks: lead=%d star=%d", leadN, starN)
	}
	if abPx(buf, w, 0, h/2).A == 0 {
		t.Fatal("focused field left its ground unpainted")
	}
}
