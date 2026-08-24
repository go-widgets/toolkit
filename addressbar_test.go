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

// abCountColor counts pixels in buf exactly equal to want. The test bitmap font
// paints solid ink pixels (no anti-aliasing), so an exact match locates glyphs
// drawn in a given ink — used to tell the muted placeholder tone (SurfaceAlt)
// apart from the value tone (OnSurface).
func abCountColor(buf []byte, w, h int, want RGBA) int {
	n := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if abPx(buf, w, x, y) == want {
				n++
			}
		}
	}
	return n
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

func TestAddressBarPlaceholder(t *testing.T) {
	const w, h = 200, 24
	th := DefaultLight()
	// SurfaceAlt is the muted placeholder ink; OnSurface is the value/caret ink.
	// The AddressBar paints SurfaceAlt nowhere else, so its presence == a
	// placeholder was drawn.
	muted, solid := th.SurfaceAlt, th.OnSurface

	// Empty value + unfocused + Placeholder set: the prompt paints in the muted
	// ink, and it is never counted as the value.
	a := &AddressBar{Radius: 4, TextPad: 4, Placeholder: "search or enter address"}
	buf := abRender(a, w, h, th)
	if abCountColor(buf, w, h, muted) == 0 {
		t.Fatal("empty unfocused field did not paint the placeholder in the muted ink")
	}
	if a.Value() != "" || a.A11y().Value != "" {
		t.Fatalf("placeholder leaked into the value: value=%q a11y=%q", a.Value(), a.A11y().Value)
	}

	// A real value hides the placeholder: text paints in the solid ink, no muted
	// glyphs (exercises the value-present branch).
	a.URL().Set("http://example.com")
	buf = abRender(a, w, h, th)
	if abCountColor(buf, w, h, muted) != 0 {
		t.Fatal("placeholder still painted while a value is present")
	}
	if abCountColor(buf, w, h, solid) == 0 {
		t.Fatal("value did not paint in the solid ink")
	}

	// Focus (editing) with an empty buffer hides the placeholder too — the field
	// is being edited, so only the caret shows (exercises the !focused branch).
	a.URL().Set("")
	a.Focused().Set(true)
	a.Editing().Set("")
	buf = abRender(a, w, h, th)
	if abCountColor(buf, w, h, muted) != 0 {
		t.Fatal("placeholder painted while the field was being edited")
	}
	if abCountColor(buf, w, h, solid) == 0 {
		t.Fatal("focused empty field should still paint its caret in the solid ink")
	}

	// No Placeholder configured: empty unfocused field paints no prompt at all
	// (exercises the empty-placeholder branch).
	blank := &AddressBar{Radius: 4, TextPad: 4}
	if abCountColor(abRender(blank, w, h, th), w, h, muted) != 0 {
		t.Fatal("a field with no Placeholder should paint no muted prompt")
	}
}

func TestAddressBarEscapeCancels(t *testing.T) {
	committed := ""
	a := &AddressBar{Radius: 4, TextPad: 4}
	a.Commit = mvvm.NewCommand(func() { committed = a.Editing().Get() }, nil)
	a.URL().Set("http://committed")

	// Focus seeds the buffer from URL; the user then edits it.
	abClick(a, 5, 10)
	a.OnEvent(Event{Kind: EventChar, Code: "X"})
	if a.Editing().Get() != "http://committedX" {
		t.Fatalf("pre-Escape buffer = %q", a.Editing().Get())
	}
	a.Copied().Set(true) // Escape also clears any copy highlight.
	// Escape cancels: the buffer reverts to the committed URL, focus/copy drop,
	// and Commit does NOT run.
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Escape"})
	if a.Editing().Get() != "http://committed" {
		t.Fatalf("Escape did not revert the buffer: %q", a.Editing().Get())
	}
	if a.Focused().Get() || a.Copied().Get() {
		t.Fatalf("Escape should defocus and clear copy: focused=%v copied=%v", a.Focused().Get(), a.Copied().Get())
	}
	if committed != "" {
		t.Fatalf("Escape must not commit, but Commit ran with %q", committed)
	}

	// Escape with no committed value reverts to the empty URL (no-committed-value
	// branch) and still does not commit.
	a.URL().Set("")
	a.Focused().Set(true)
	a.Editing().Set("typed")
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Escape"})
	if a.Editing().Get() != "" || a.Focused().Get() || committed != "" {
		t.Fatalf("Escape with empty URL: buffer=%q focused=%v committed=%q", a.Editing().Get(), a.Focused().Get(), committed)
	}

	// Enter still commits, unchanged by the new Escape path.
	a.URL().Set("http://committed")
	abClick(a, 5, 10)
	a.OnEvent(Event{Kind: EventChar, Code: "!"})
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if committed != "http://committed!" || a.Focused().Get() {
		t.Fatalf("Enter should still commit: committed=%q focused=%v", committed, a.Focused().Get())
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
