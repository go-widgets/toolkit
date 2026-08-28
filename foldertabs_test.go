// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- API surface ---------------------------------------------------------

func TestFolderTabsConstants(t *testing.T) {
	if FolderTabsH != 24 || FolderTabsRadius != 6 || FolderTabsPadX != 12 ||
		FolderTabsGap != 3 || FolderTabsInset != 3 || FolderTabsAccentH != 2 {
		t.Fatalf("constants drifted: H=%d Rad=%d PadX=%d Gap=%d Inset=%d Accent=%d",
			FolderTabsH, FolderTabsRadius, FolderTabsPadX, FolderTabsGap, FolderTabsInset, FolderTabsAccentH)
	}
}

func TestNewFolderTabsClamps(t *testing.T) {
	// Empty labels: selected forced to 0.
	if ft := NewFolderTabs(nil, 7); ft.Selected().Get() != 0 {
		t.Fatalf("empty labels selected = %d, want 0", ft.Selected().Get())
	}
	// Negative clamped to 0.
	if ft := NewFolderTabs([]string{"A", "B"}, -5); ft.Selected().Get() != 0 {
		t.Fatalf("negative selected = %d, want 0", ft.Selected().Get())
	}
	// Overshoot clamped to len-1.
	if ft := NewFolderTabs([]string{"A", "B", "C"}, 99); ft.Selected().Get() != 2 {
		t.Fatalf("overshoot selected = %d, want 2", ft.Selected().Get())
	}
	// In-range preserved.
	if ft := NewFolderTabs([]string{"A", "B", "C"}, 1); ft.Selected().Get() != 1 {
		t.Fatalf("in-range selected = %d, want 1", ft.Selected().Get())
	}
}

// TestFolderTabsSelectedAccessorLazyInit covers the accessor on a bare
// zero-value widget: Selected() must lazy-init the Observable to 0 (no nil
// panic), and a host that binds it via Subscribe must observe a keyboard move.
func TestFolderTabsSelectedAccessorLazyInit(t *testing.T) {
	ft := &FolderTabs{Labels: []string{"A", "B"}}
	if got := ft.Selected().Get(); got != 0 { // lazy-inits on first read
		t.Fatalf("bare accessor Get() = %d, want 0", got)
	}
	seen := -1
	ft.Selected().Subscribe(func(i int) { seen = i })
	ft.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"}) // 0 -> 1
	if ft.Selected().Get() != 1 || seen != 1 {
		t.Fatalf("host bind: Selected=%d seen=%d, want 1/1", ft.Selected().Get(), seen)
	}
}

func TestFolderTabsHeight(t *testing.T) {
	if got, want := FolderTabsHeight(), TouchTarget(scaled(FolderTabsH)); got != want {
		t.Fatalf("FolderTabsHeight = %d, want %d", got, want)
	}
}

// --- Geometry ------------------------------------------------------------

func TestFolderTabsTabRectOutOfRange(t *testing.T) {
	ft := NewFolderTabs([]string{"A", "B"}, 0)
	ft.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 30})
	if r := ft.TabRect(-1); r != (Rect{}) {
		t.Fatalf("TabRect(-1) = %+v, want zero", r)
	}
	if r := ft.TabRect(2); r != (Rect{}) {
		t.Fatalf("TabRect(len) = %+v, want zero", r)
	}
}

func TestFolderTabsTabRectLayout(t *testing.T) {
	ft := NewFolderTabs([]string{"AAAA", "BB", "CCCCCC"}, 0)
	ft.SetBounds(Rect{X: 10, Y: 5, W: 400, H: 30})
	padX := scaled(FolderTabsPadX)
	prev := ft.TabRect(0)
	if prev.X != 10+scaled(FolderTabsInset) {
		t.Fatalf("first tab X = %d, want %d", prev.X, 10+scaled(FolderTabsInset))
	}
	if want := ft.textWidth("AAAA") + 2*padX; prev.W != want {
		t.Fatalf("tab0 W = %d, want %d", prev.W, want)
	}
	for i := 1; i < 3; i++ {
		r := ft.TabRect(i)
		if r.X <= prev.X {
			t.Fatalf("tab %d X = %d not right of previous %d", i, r.X, prev.X)
		}
		if r.Y != 5+scaled(FolderTabsInset) {
			t.Fatalf("tab %d Y = %d, want %d", i, r.Y, 5+scaled(FolderTabsInset))
		}
		prev = r
	}
}

// TestFolderTabsClosableWidth: a Closable strip reserves the × box on the right,
// so each tab is FolderTabsCloseW wider than the same tab on a plain strip.
func TestFolderTabsClosableWidth(t *testing.T) {
	plain := NewFolderTabs([]string{"AAAA", "BB"}, 0)
	plain.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 30})
	clos := NewFolderTabs([]string{"AAAA", "BB"}, 0)
	clos.Closable = true
	clos.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 30})
	for i := 0; i < 2; i++ {
		if want := plain.TabRect(i).W + scaled(FolderTabsCloseW); clos.TabRect(i).W != want {
			t.Fatalf("closable tab %d W = %d, want %d", i, clos.TabRect(i).W, want)
		}
	}
	// A non-Closable strip has an empty close box, so its Contains is always false.
	if plain.closeRect(0) != (Rect{}) {
		t.Fatalf("non-closable closeRect = %+v, want zero", plain.closeRect(0))
	}
	// The close box sits inside the tab, tucked against its right edge.
	tr, cr := clos.TabRect(0), clos.closeRect(0)
	if cr.X < tr.X || cr.X+cr.W != tr.X+tr.W {
		t.Fatalf("close box %+v not flush with tab right edge %+v", cr, tr)
	}
}

// TestFolderTabsCloseVsSelect: on a Closable strip a click on the × fires OnClose
// (not select); a click on the label body still selects.
func TestFolderTabsCloseVsSelect(t *testing.T) {
	ft := NewFolderTabs([]string{"one", "two", "three"}, 0)
	ft.Closable = true
	ft.SetBounds(Rect{X: 10, Y: 5, W: 400, H: 30})
	var closed []int
	ft.OnClose = func(i int) { closed = append(closed, i) }

	// Click the × of tab 1: closes it, does not select it.
	cr := ft.closeRect(1)
	ft.OnEvent(Event{Kind: EventClick, X: cr.X + cr.W/2 - ft.Bounds().X, Y: cr.Y + cr.H/2 - ft.Bounds().Y})
	if ft.Selected().Get() != 0 {
		t.Fatalf("clicking × changed the selection to %d, want 0", ft.Selected().Get())
	}
	if len(closed) != 1 || closed[0] != 1 {
		t.Fatalf("OnClose fired %v, want [1]", closed)
	}

	// Click the label body of tab 2 (left of its × box): selects it, no close.
	tr := ft.TabRect(2)
	lx := tr.X + scaled(FolderTabsPadX) - ft.Bounds().X // in the label area, left of ×
	ft.OnEvent(Event{Kind: EventClick, X: lx, Y: tr.Y + tr.H/2 - ft.Bounds().Y})
	if ft.Selected().Get() != 2 {
		t.Fatalf("clicking tab2 body selected %d, want 2", ft.Selected().Get())
	}
	if len(closed) != 1 {
		t.Fatalf("body click also closed: %v", closed)
	}
}

// TestFolderTabsCloseNilCallback: a × click with no OnClose set is a safe no-op
// (and still does not select).
func TestFolderTabsCloseNilCallback(t *testing.T) {
	ft := NewFolderTabs([]string{"a", "b"}, 1)
	ft.Closable = true // no OnClose
	ft.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 30})
	cr := ft.closeRect(0)
	ft.OnEvent(Event{Kind: EventClick, X: cr.X + cr.W/2, Y: cr.Y + cr.H/2})
	if ft.Selected().Get() != 1 {
		t.Fatalf("× click with nil OnClose changed selection to %d, want 1", ft.Selected().Get())
	}
}

// TestFolderTabsTabRectClampsTinyHeight covers the h<1 floor: a strip shorter
// than the top inset must still yield a 1px-high tab, never a negative one.
func TestFolderTabsTabRectClampsTinyHeight(t *testing.T) {
	ft := NewFolderTabs([]string{"A"}, 0)
	ft.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 1}) // H < inset
	if r := ft.TabRect(0); r.H != 1 {
		t.Fatalf("tiny-strip tab H = %d, want 1", r.H)
	}
}

// --- Draw ----------------------------------------------------------------

func TestFolderTabsDrawZeroArea(t *testing.T) {
	buf := makeSurface(50, 30)
	ft := NewFolderTabs([]string{"A"}, 0)
	ft.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 30}) // zero width short-circuits
	ft.Draw(newP(buf, 50), DefaultLight())
	if got := pixelAt(buf, 50, 5, 5); got != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) {
		t.Fatalf("zero-area Draw painted %+v, want the untouched sentinel", got)
	}
}

func TestFolderTabsDrawEmpty(t *testing.T) {
	const w, h = 120, 30
	theme := DefaultLight()
	buf := makeSurface(w, h)
	ft := NewFolderTabs(nil, 0)
	ft.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	ft.Draw(newP(buf, w), theme)
	// The strip background is painted...
	if !scanHasColor(buf, w, 0, 0, w-1, 2, theme.SurfaceAlt) {
		t.Fatal("empty strip: SurfaceAlt background not painted")
	}
	// ...and its bottom border, but no tab body.
	if !scanHasColor(buf, w, 0, h-1, w-1, h-1, theme.Border) {
		t.Fatal("empty strip: bottom border not painted")
	}
	if scanHasColor(buf, w, 0, 0, w-1, h-1, theme.Accent) {
		t.Fatal("empty strip: an accent bar was drawn with no tabs")
	}
}

// TestFolderTabsDrawPixels is the load-bearing look assertion: the ACTIVE tab
// paints a Surface body under a rounded top with an Accent bar along its top
// edge, while an INACTIVE tab paints a dimmer face and its top corner is cut
// away (rounded) to the strip colour.
// TestFolderTabsClosableDraw exercises the Closable draw path: a × is painted in
// each tab's close box (the box shows ink over the tab face), covering drawTab's
// closable branch.
func TestFolderTabsClosableDraw(t *testing.T) {
	const w, h = 300, 30
	theme := DefaultLight()
	buf := makeSurface(w, h)
	ft := NewFolderTabs([]string{"main.tex", "notes.md"}, 0)
	ft.Closable = true
	ft.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	ft.Draw(newP(buf, w), theme)

	// The × box of tab 0 must carry ink that differs from the plain tab face
	// (the glyph painted over the active Surface fill).
	cr := ft.closeRect(0)
	var inked bool
	for y := cr.Y; y < cr.Y+cr.H; y++ {
		for x := cr.X; x < cr.X+cr.W; x++ {
			if pixelAt(buf, w, x, y) != theme.Surface {
				inked = true
			}
		}
	}
	if !inked {
		t.Fatal("closable tab drew no × glyph in its close box")
	}
}

func TestFolderTabsDrawPixels(t *testing.T) {
	const w, h = 300, 30
	theme := DefaultLight()
	buf := makeSurface(w, h)
	ft := NewFolderTabs([]string{"Rendered", "Log", "Extra"}, 1) // "Log" active
	ft.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	ft.Draw(newP(buf, w), theme)

	act := ft.TabRect(1)   // active
	inact := ft.TabRect(0) // inactive
	cx := act.X + act.W/2

	// Active tab: Accent bar along the very top edge.
	if got := pixelAt(buf, w, cx, act.Y); got != theme.Accent {
		t.Fatalf("active top pixel = %+v, want Accent %+v", got, theme.Accent)
	}
	// Active tab: Surface body fill below the accent bar.
	if got := pixelAt(buf, w, cx, act.Y+act.H/2); got != theme.Surface {
		t.Fatalf("active body pixel = %+v, want Surface %+v", got, theme.Surface)
	}
	// Active tab: the TOP-LEFT corner is rounded (cut away), so a pixel at the
	// left edge two rows below the accent bar is NOT the Surface fill — it shows
	// the strip beneath. This is the rounded-top geometry.
	if got := pixelAt(buf, w, act.X, act.Y+2); got == theme.Surface {
		t.Fatalf("active top-left corner = Surface, want the rounded cut (strip colour)")
	}

	// Inactive tab: a dimmer face than the active Surface fill.
	dimFace := blendRGBA(theme.SurfaceAlt, theme.Background, 0.5)
	if dimFace == theme.Surface {
		t.Fatal("test precondition: inactive dimFace must differ from active Surface")
	}
	if !scanHasColor(buf, w, inact.X+inact.W/2-2, inact.Y+inact.H/2,
		inact.X+inact.W/2+2, inact.Y+inact.H/2, dimFace) {
		t.Fatal("inactive tab: dimmed face not painted")
	}
	// Inactive tab is dimmer: its fill is lower luminance than the active fill,
	// and its label ink is faded toward the strip (higher luminance than the
	// crisp OnSurface used on the active tab).
	if lum(dimFace) >= lum(theme.Surface) {
		t.Fatalf("inactive face lum %d >= active face lum %d, want dimmer", lum(dimFace), lum(theme.Surface))
	}
	dimInk := blendRGBA(theme.OnSurface, theme.SurfaceAlt, 0.5)
	if lum(dimInk) <= lum(theme.OnSurface) {
		t.Fatalf("inactive ink not faded: lum(dimInk)=%d lum(OnSurface)=%d", lum(dimInk), lum(theme.OnSurface))
	}
	// Inactive tab: TOP-LEFT corner cut away to the strip colour (rounded top).
	if got := pixelAt(buf, w, inact.X, inact.Y); got != theme.SurfaceAlt {
		t.Fatalf("inactive top-left corner = %+v, want the strip colour %+v (rounded cut)", got, theme.SurfaceAlt)
	}
}

// TestFolderTabsDrawOutOfRangeActive covers the guard that a directly-Set
// out-of-range selection draws no active tab (and does not panic): every tab
// paints inactive.
func TestFolderTabsDrawOutOfRangeActive(t *testing.T) {
	const w, h = 200, 30
	theme := DefaultLight()
	buf := makeSurface(w, h)
	ft := NewFolderTabs([]string{"A", "B"}, 0)
	ft.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	ft.Selected().Set(99) // host pushed an out-of-range index onto the Observable
	ft.Draw(newP(buf, w), theme)
	if scanHasColor(buf, w, 0, 0, w-1, h-1, theme.Accent) {
		t.Fatal("out-of-range active: an accent bar was drawn")
	}
}

// lum is a cheap luminance proxy (channel sum) for "dimmer than" assertions.
func lum(c RGBA) int { return int(c.R) + int(c.G) + int(c.B) }

// --- Events: click -------------------------------------------------------

func TestFolderTabsClickSelects(t *testing.T) {
	ft := NewFolderTabs([]string{"A", "B", "C"}, 0)
	ft.SetBounds(Rect{X: 10, Y: 5, W: 300, H: 30}) // non-zero origin exercises coord conversion
	var fired []int
	ft.OnSelect = func(i int) { fired = append(fired, i) }

	// Click tab 2 (event coords are widget-local: surface point minus bounds origin).
	r2 := ft.TabRect(2)
	lx := r2.X + r2.W/2 - ft.Bounds().X
	ly := r2.Y + r2.H/2 - ft.Bounds().Y
	ft.OnEvent(Event{Kind: EventClick, X: lx, Y: ly})
	if ft.Selected().Get() != 2 {
		t.Fatalf("after click tab2, Selected = %d, want 2", ft.Selected().Get())
	}
	if len(fired) != 1 || fired[0] != 2 {
		t.Fatalf("OnSelect fired %v, want [2]", fired)
	}

	// Click on empty strip area (right of the last tab) selects nothing.
	ft.OnEvent(Event{Kind: EventClick, X: 295, Y: 10})
	if ft.Selected().Get() != 2 || len(fired) != 1 {
		t.Fatalf("click-off changed state: Selected=%d fired=%v", ft.Selected().Get(), fired)
	}
}

// TestFolderTabsClickSameTabIsNoOp covers setActive's unchanged-guard: clicking
// the already-active tab neither re-fires OnSelect nor re-notifies.
func TestFolderTabsClickSameTabIsNoOp(t *testing.T) {
	ft := NewFolderTabs([]string{"A", "B"}, 1)
	ft.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 30})
	fired := 0
	ft.OnSelect = func(int) { fired++ }
	r1 := ft.TabRect(1)
	ft.OnEvent(Event{Kind: EventClick, X: r1.X + r1.W/2, Y: r1.Y + r1.H/2})
	if fired != 0 || ft.Selected().Get() != 1 {
		t.Fatalf("clicking the active tab fired=%d Selected=%d, want 0/1", fired, ft.Selected().Get())
	}
}

// --- Events: keyboard ----------------------------------------------------

func TestFolderTabsKeyboardWraps(t *testing.T) {
	ft := NewFolderTabs([]string{"A", "B", "C"}, 0)
	cases := []struct {
		code string
		want int
	}{
		{"ArrowRight", 1},
		{"ArrowDown", 2},
		{"ArrowRight", 0}, // wrap forward past the end
		{"ArrowLeft", 2},  // wrap back past the start
		{"ArrowUp", 1},
		{"Home", 1}, // unknown code: no-op (inner switch default)
	}
	for _, c := range cases {
		ft.OnEvent(Event{Kind: EventKeyDown, Code: c.code})
		if got := ft.Selected().Get(); got != c.want {
			t.Fatalf("%s -> Selected %d, want %d", c.code, got, c.want)
		}
	}
	// A non-click/non-key event kind is ignored (outer switch fall-through).
	ft.OnEvent(Event{Kind: EventMouseDrag, X: 5, Y: 5})
	if ft.Selected().Get() != 1 {
		t.Fatalf("mouse-drag changed selection to %d", ft.Selected().Get())
	}
}

func TestFolderTabsDisabledIgnoresInput(t *testing.T) {
	ft := NewFolderTabs([]string{"A", "B"}, 0)
	ft.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 30})
	ft.Disabled().Set(true)
	r1 := ft.TabRect(1)
	ft.OnEvent(Event{Kind: EventClick, X: r1.X + r1.W/2, Y: r1.Y + r1.H/2})
	ft.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if ft.Selected().Get() != 0 {
		t.Fatalf("disabled strip changed selection to %d", ft.Selected().Get())
	}
}

// --- setActive / step guards --------------------------------------------

func TestFolderTabsSetActiveOutOfRangeGuard(t *testing.T) {
	ft := NewFolderTabs([]string{"A", "B"}, 0)
	ft.setActive(-1)
	ft.setActive(99)
	if ft.Selected().Get() != 0 {
		t.Fatalf("out-of-range setActive changed selection to %d", ft.Selected().Get())
	}
}

func TestFolderTabsStepEmptyIsNoOp(t *testing.T) {
	ft := NewFolderTabs(nil, 0)
	ft.step(1) // n == 0
	if ft.Selected().Get() != 0 {
		t.Fatalf("step on empty labels = %d, want 0", ft.Selected().Get())
	}
}

// TestFolderTabsStepResetsOutOfRange covers the step reset: an out-of-range
// current is normalised to 0 before the move.
func TestFolderTabsStepResetsOutOfRange(t *testing.T) {
	ft := NewFolderTabs([]string{"A", "B", "C"}, 0)
	ft.Selected().Set(-5) // out of range, pushed straight onto the Observable
	ft.step(1)            // cur reset to 0, then +1
	if got := ft.Selected().Get(); got != 1 {
		t.Fatalf("step from out-of-range = %d, want 1", got)
	}
}

// --- Accessibility -------------------------------------------------------

func TestFolderTabsA11y(t *testing.T) {
	ft := NewFolderTabs([]string{"Rendered", "Log"}, 1)
	if got := ft.A11y(); got.Role != RoleTablist || got.Name != "Log" {
		t.Fatalf("A11y = %+v, want tablist named Log", got)
	}
	// Out-of-range active: no name.
	ft.Selected().Set(99)
	if got := ft.A11y(); got.Role != RoleTablist || got.Name != "" {
		t.Fatalf("out-of-range A11y = %+v, want tablist with empty name", got)
	}
	// Empty labels: no name.
	if got := NewFolderTabs(nil, 0).A11y(); got.Name != "" {
		t.Fatalf("empty A11y Name = %q, want empty", got.Name)
	}
}

func TestFolderTabsChildrenAreTabs(t *testing.T) {
	ft := NewFolderTabs([]string{"A", "B", "C"}, 1)
	ft.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 30})
	kids := ft.Children()
	if len(kids) != 3 {
		t.Fatalf("Children len = %d, want 3", len(kids))
	}
	for i, k := range kids {
		info := k.(Accessible).A11y()
		if info.Role != RoleTab {
			t.Fatalf("child %d role = %q, want tab", i, info.Role)
		}
		wantVal := ""
		if i == 1 {
			wantVal = "selected"
		}
		if info.Value != wantVal {
			t.Fatalf("child %d Value = %q, want %q", i, info.Value, wantVal)
		}
		if info.Name == "" {
			t.Fatalf("child %d has empty name", i)
		}
		// The synthetic node carries the tab's surface rectangle.
		if k.Bounds() != ft.TabRect(i) {
			t.Fatalf("child %d bounds = %+v, want %+v", i, k.Bounds(), ft.TabRect(i))
		}
	}
}

// --- fillTopRoundRect primitive -----------------------------------------

func TestFillTopRoundRect(t *testing.T) {
	const w, h = 40, 40
	red := RGBA{R: 0xFF, A: 0xFF}

	// Zero area: nothing painted.
	buf := makeSurface(w, h)
	fillTopRoundRect(newP(buf, w), 5, 5, 0, 10, 6, red) // w<=0
	if scanHasColor(buf, w, 0, 0, w-1, h-1, red) {
		t.Fatal("zero-width fillTopRoundRect painted")
	}

	// radius < 1 degrades to a plain fillRect: the corner IS painted.
	buf = makeSurface(w, h)
	fillTopRoundRect(newP(buf, w), 5, 5, 20, 20, 0, red)
	if pixelAt(buf, w, 5, 5) != red {
		t.Fatal("radius<1: top-left corner should be a square fill")
	}

	// Rounded top (h > radius): the TOP corners are cut, the BOTTOM corners are
	// square.
	buf = makeSurface(w, h)
	fillTopRoundRect(newP(buf, w), 5, 5, 20, 20, 6, red)
	if pixelAt(buf, w, 5, 5) == red {
		t.Fatal("rounded top: top-left corner should be cut away")
	}
	if pixelAt(buf, w, 5, 5+20-1) != red {
		t.Fatal("rounded top: bottom-left corner should be squared (painted)")
	}
	if pixelAt(buf, w, 15, 15) != red {
		t.Fatal("rounded top: body interior should be filled")
	}

	// Degenerate short body (h <= radius): the bottom over-paint is skipped; it
	// must still paint and not panic.
	buf = makeSurface(w, h)
	fillTopRoundRect(newP(buf, w), 5, 5, 30, 4, 6, red)
	if !scanHasColor(buf, w, 5, 5, 34, 8, red) {
		t.Fatal("short body: nothing painted")
	}
}
