// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"strings"
	"testing"
)

// browserBounds is the standard placement used across the Browser tests: a
// 400×300 surface at the origin so a widget-local event coordinate equals its
// absolute coordinate.
var browserBounds = Rect{X: 0, Y: 0, W: 400, H: 300}

// newTestBrowser builds a Browser at browserBounds wired with an OnNavigate
// recorder and an OnChange counter, returning the widget plus pointers to the
// captured navigation list and the change counter.
func newTestBrowser() (*Browser, *[]string, *[]int, *int) {
	b := NewBrowser()
	b.SetBounds(browserBounds)
	var navTargets []string
	var navWidths []int
	changes := 0
	b.OnNavigate = func(target string, width int) {
		navTargets = append(navTargets, target)
		navWidths = append(navWidths, width)
	}
	b.OnChange = func() { changes++ }
	return b, &navTargets, &navWidths, &changes
}

func center(r Rect) (int, int) { return r.X + r.W/2, r.Y + r.H/2 }

// --- model: open / tab modes --------------------------------------------

func TestBrowserOpenMultiTab(t *testing.T) {
	b, navT, navW, changes := newTestBrowser()
	b.Open("http://a", "Alpha")
	if b.TabCount() != 1 || b.ActiveIndex() != 0 {
		t.Fatalf("after first Open: tabs=%d active=%d", b.TabCount(), b.ActiveIndex())
	}
	if b.CurrentURL() != "http://a" {
		t.Fatalf("CurrentURL = %q", b.CurrentURL())
	}
	if !b.Loading() {
		t.Fatal("tab should be loading after Open")
	}
	if b.ActiveTitle() != "Alpha" {
		t.Fatalf("ActiveTitle = %q", b.ActiveTitle())
	}
	if len(*navT) != 1 || (*navT)[0] != "http://a" || (*navW)[0] != 400 {
		t.Fatalf("nav = %v widths=%v, want [http://a]@400", *navT, *navW)
	}
	if *changes != 1 {
		t.Fatalf("changes = %d, want 1", *changes)
	}
	if b.showTabStrip() {
		t.Fatal("single tab must not show the strip")
	}
	b.Open("http://b", "Beta")
	if b.TabCount() != 2 || b.ActiveIndex() != 1 {
		t.Fatalf("after second Open: tabs=%d active=%d", b.TabCount(), b.ActiveIndex())
	}
	if !b.showTabStrip() {
		t.Fatal("two tabs in MultiTab must show the strip")
	}
	if b.Mode() != MultiTab {
		t.Fatalf("Mode = %v, want MultiTab", b.Mode())
	}
}

func TestBrowserOpenSingleTab(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	b.SetTabMode(SingleTab)
	b.Open("http://a", "A") // append branch (no tabs yet)
	b.Open("http://b", "B") // replace branch (one tab exists)
	if b.TabCount() != 1 {
		t.Fatalf("SingleTab kept %d tabs, want 1", b.TabCount())
	}
	if b.CurrentURL() != "http://b" || b.ActiveIndex() != 0 {
		t.Fatalf("SingleTab reuse broke: url=%q active=%d", b.CurrentURL(), b.ActiveIndex())
	}
	if b.showTabStrip() {
		t.Fatal("SingleTab must never show the strip")
	}
}

func TestBrowserOpenEvictsOldest(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	for i := 0; i < BrowserMaxTabs+3; i++ {
		b.Open("http://p", "")
	}
	if b.TabCount() != BrowserMaxTabs {
		t.Fatalf("TabCount = %d, want cap %d", b.TabCount(), BrowserMaxTabs)
	}
	if b.ActiveIndex() != BrowserMaxTabs-1 {
		t.Fatalf("active = %d, want last %d", b.ActiveIndex(), BrowserMaxTabs-1)
	}
}

// --- model: navigate / back / forward / reload --------------------------

func TestBrowserNavigateAndForwardTruncation(t *testing.T) {
	b, navT, _, _ := newTestBrowser()
	b.Open("http://a", "")
	b.Navigate("http://b")
	if b.CurrentURL() != "http://b" || !b.CanBack() || b.CanForward() {
		t.Fatalf("after Navigate b: url=%q back=%v fwd=%v", b.CurrentURL(), b.CanBack(), b.CanForward())
	}
	b.Back()
	if b.CurrentURL() != "http://a" || b.CanBack() || !b.CanForward() {
		t.Fatalf("after Back: url=%q back=%v fwd=%v", b.CurrentURL(), b.CanBack(), b.CanForward())
	}
	b.Navigate("http://c") // truncates the forward entry (b)
	if b.CurrentURL() != "http://c" || b.CanForward() {
		t.Fatalf("forward-truncation failed: url=%q fwd=%v", b.CurrentURL(), b.CanForward())
	}
	if !b.CanBack() {
		t.Fatal("history [a,c] at cursor 1 must allow Back")
	}
	last := (*navT)[len(*navT)-1]
	if last != "http://c" {
		t.Fatalf("last nav = %q, want http://c", last)
	}
}

func TestBrowserNavigateWithoutTabOpens(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	b.Navigate("http://x")
	if b.TabCount() != 1 || b.CurrentURL() != "http://x" {
		t.Fatalf("Navigate with no tab should Open: tabs=%d url=%q", b.TabCount(), b.CurrentURL())
	}
}

func TestBrowserBackForwardEdges(t *testing.T) {
	b, _, _, changes := newTestBrowser()
	// No active tab: every command is a no-op and fires no change.
	before := *changes
	b.Back()
	b.Forward()
	b.Reload()
	if *changes != before {
		t.Fatalf("no-tab commands fired %d changes", *changes-before)
	}
	b.Open("http://a", "")
	b.Navigate("http://b")
	b.Navigate("http://c") // cursor 2
	b.Back()               // cursor 1
	if b.CurrentURL() != "http://b" {
		t.Fatalf("Back landed on %q", b.CurrentURL())
	}
	if !b.CanBack() || !b.CanForward() {
		t.Fatal("middle of history must allow both directions")
	}
	b.Back() // cursor 0
	b.Back() // no-op at start
	if b.CurrentURL() != "http://a" {
		t.Fatalf("Back past start moved to %q", b.CurrentURL())
	}
	b.Forward() // cursor 1
	if b.CurrentURL() != "http://b" {
		t.Fatalf("Forward landed on %q", b.CurrentURL())
	}
	b.Forward() // cursor 2
	b.Forward() // no-op at end
	if b.CurrentURL() != "http://c" {
		t.Fatalf("Forward past end moved to %q", b.CurrentURL())
	}
}

func TestBrowserReload(t *testing.T) {
	b, navT, _, _ := newTestBrowser()
	b.Open("http://a", "")
	n := len(*navT)
	b.Reload()
	if len(*navT) != n+1 || (*navT)[n] != "http://a" {
		t.Fatalf("Reload nav = %v", *navT)
	}
}

// --- model: close tab ----------------------------------------------------

func TestBrowserCloseTabOutOfRange(t *testing.T) {
	b, _, _, changes := newTestBrowser()
	b.Open("http://a", "")
	before := *changes
	b.CloseTab(-1)
	b.CloseTab(5)
	if b.TabCount() != 1 || *changes != before {
		t.Fatalf("out-of-range CloseTab mutated state: tabs=%d changes=%d", b.TabCount(), *changes-before)
	}
}

func TestBrowserCloseTabActivatesNeighbour(t *testing.T) {
	build := func() *Browser {
		b := NewBrowser()
		b.SetBounds(browserBounds)
		b.Open("http://0", "")
		b.Open("http://1", "")
		b.Open("http://2", "")
		return b
	}
	// Close a tab before the active one: active index shifts down.
	b := build() // active = 2
	b.CloseTab(0)
	if b.TabCount() != 2 || b.ActiveIndex() != 1 || b.CurrentURL() != "http://2" {
		t.Fatalf("close-before-active: tabs=%d active=%d url=%q", b.TabCount(), b.ActiveIndex(), b.CurrentURL())
	}
	// Close the active tab when it is NOT last: the neighbour to the right slides in.
	b = build()
	b.active = 0
	b.CloseTab(0)
	if b.ActiveIndex() != 0 || b.CurrentURL() != "http://1" {
		t.Fatalf("close-active-not-last: active=%d url=%q", b.ActiveIndex(), b.CurrentURL())
	}
	// Close the active tab when it IS last: fall back to the left neighbour.
	b = build() // active = 2 (last)
	b.CloseTab(2)
	if b.ActiveIndex() != 1 || b.CurrentURL() != "http://1" {
		t.Fatalf("close-active-last: active=%d url=%q", b.ActiveIndex(), b.CurrentURL())
	}
	// Close a tab after the active one: active index unchanged.
	b = build()
	b.active = 0
	b.CloseTab(2)
	if b.ActiveIndex() != 0 || b.TabCount() != 2 {
		t.Fatalf("close-after-active: active=%d tabs=%d", b.ActiveIndex(), b.TabCount())
	}
	// Close down to empty.
	b = build()
	b.CloseTab(0)
	b.CloseTab(0)
	b.CloseTab(0)
	if b.TabCount() != 0 || b.ActiveIndex() != 0 {
		t.Fatalf("close-to-empty: tabs=%d active=%d", b.TabCount(), b.ActiveIndex())
	}
	if b.CurrentURL() != "" || b.ActiveTitle() != "" {
		t.Fatal("empty browser should report empty URL + title")
	}
}

// --- model: deliver / progress ------------------------------------------

func TestBrowserDeliver(t *testing.T) {
	b, _, _, changes := newTestBrowser()
	b.Open("http://a", "loading…")
	before := *changes
	links := []BrowserLink{{Rect: image.Rect(0, 0, 10, 10), Href: "http://a/x"}}
	px := make([]byte, 2*2*4)
	b.Deliver("http://a", px, 2, 2, 400, links, "Alpha")
	if b.Loading() {
		t.Fatal("Deliver should clear loading")
	}
	if b.ActiveTitle() != "Alpha" {
		t.Fatalf("title after Deliver = %q", b.ActiveTitle())
	}
	if *changes != before+1 {
		t.Fatalf("Deliver fired %d changes, want 1", *changes-before)
	}
	// Non-matching target is ignored.
	b.Open("http://c", "")
	before = *changes
	b.Deliver("http://WRONG", px, 2, 2, 400, nil, "nope")
	if !b.Loading() || *changes != before {
		t.Fatalf("stale Deliver leaked: loading=%v changes=%d", b.Loading(), *changes-before)
	}
}

func TestBrowserDeliverNoActiveTab(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	b.Deliver("http://x", nil, 0, 0, 0, nil, "") // must not panic
	if b.TabCount() != 0 {
		t.Fatal("Deliver with no tab must not create one")
	}
}

func TestBrowserSetProgress(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	// No tab: no-op.
	b.SetProgress(0.5)
	if b.Progress() != 0 {
		t.Fatalf("Progress with no tab = %v", b.Progress())
	}
	b.Open("http://a", "")
	b.SetProgress(0.5)
	if b.Progress() != 0.5 {
		t.Fatalf("Progress = %v, want 0.5", b.Progress())
	}
	b.SetProgress(-1)
	if b.Progress() != 0 {
		t.Fatalf("Progress clamp-low = %v", b.Progress())
	}
	b.SetProgress(2)
	if b.Progress() != 1 {
		t.Fatalf("Progress clamp-high = %v", b.Progress())
	}
}

func TestBrowserTick(t *testing.T) {
	b := NewBrowser()
	b.Tick(0.3)
	if b.Phase < 0.29 || b.Phase > 0.31 {
		t.Fatalf("Phase = %v, want ~0.3", b.Phase)
	}
	b.Tick(0.9) // wraps past 1
	if b.Phase < 0.19 || b.Phase > 0.21 {
		t.Fatalf("Phase after wrap = %v, want ~0.2", b.Phase)
	}
}

func TestBrowserTitleAccessors(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	if b.TabTitle(0) != "" {
		t.Fatal("out-of-range TabTitle should be empty")
	}
	b.Open("http://url", "") // empty title → falls back to URL
	if b.TabTitle(0) != "http://url" || b.ActiveTitle() != "http://url" {
		t.Fatalf("empty-title fallback: tab=%q active=%q", b.TabTitle(0), b.ActiveTitle())
	}
	b.Open("http://u2", "Named")
	if b.TabTitle(1) != "Named" {
		t.Fatalf("TabTitle(1) = %q, want Named", b.TabTitle(1))
	}
}

func TestBrowserOpenExternal(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	b.OpenExternal() // nil hook: no-op, no panic
	var got string
	b.OnOpenExternal = func(u string) { got = u }
	b.OpenExternal() // no tab → empty URL → no fire
	if got != "" {
		t.Fatalf("OpenExternal fired with no tab: %q", got)
	}
	b.Open("http://ext", "")
	b.OpenExternal()
	if got != "http://ext" {
		t.Fatalf("OpenExternal = %q, want http://ext", got)
	}
}

// --- pure helpers --------------------------------------------------------

func TestClipHeadToWidth(t *testing.T) {
	f := CurrentFont()
	// Wide budget: trims from the front and keeps the tail.
	got := clipHeadToWidth(f, "http://example.com/deep/path", 8*f.Advance())
	if !strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "path") {
		t.Fatalf("clipHeadToWidth = %q, want a head-ellipsis keeping the path", got)
	}
	// Budget too small for even one glyph + ellipsis: bare ellipsis.
	if got := clipHeadToWidth(f, "abc", 1); got != "…" {
		t.Fatalf("clipHeadToWidth tiny = %q, want bare ellipsis", got)
	}
}

func TestNormalizeURL(t *testing.T) {
	if got := normalizeURL("http://a"); got != "http://a" {
		t.Fatalf("scheme URL rewritten to %q", got)
	}
	if got := normalizeURL("example.com"); got != "https://example.com" {
		t.Fatalf("bare host = %q, want https:// prefixed", got)
	}
}

func TestBrowserToolbarLayoutClampsAddress(t *testing.T) {
	b := NewBrowser()
	b.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 60}) // too narrow for the buttons
	_, _, _, addr := b.toolbarLayout()
	if addr.W != 0 {
		t.Fatalf("address width = %d, want clamped to 0", addr.W)
	}
}

func TestBrowserContentRectClampsNegativeHeight(t *testing.T) {
	b := NewBrowser()
	b.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 10}) // shorter than the toolbar
	if h := b.contentRect().H; h != 0 {
		t.Fatalf("content height = %d, want clamped to 0", h)
	}
}

// --- drawing -------------------------------------------------------------

// scanFor reports whether colour c appears anywhere in the rect (x0..x1, y0..y1)
// of the surface.
func scanFor(buf []byte, w int, r Rect, c RGBA) bool {
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			if pixelAt(buf, w, x, y) == c {
				return true
			}
		}
	}
	return false
}

func TestBrowserDrawToolbarFaces(t *testing.T) {
	theme := DefaultLight()
	b, _, _, _ := newTestBrowser()
	// history [a,b,c] at cursor 1 → both Back and Forward enabled.
	b.Open("http://a", "")
	b.Navigate("http://b")
	b.Navigate("http://c")
	b.Back()
	buf := makeSurface(browserBounds.W, browserBounds.H)
	b.Draw(newP(buf, browserBounds.W), theme)

	back, fwd, reload, _ := b.toolbarLayout()
	// Enabled buttons paint their fill in Surface; a point at the left inset,
	// mid-height, is fill (left of the centred label).
	sample := func(r Rect) RGBA { return pixelAt(buf, browserBounds.W, r.X+3, r.Y+r.H/2) }
	if sample(back) != theme.Surface {
		t.Fatalf("enabled Back fill = %+v, want Surface", sample(back))
	}
	if sample(fwd) != theme.Surface {
		t.Fatalf("enabled Fwd fill = %+v, want Surface", sample(fwd))
	}
	if sample(reload) != theme.Surface {
		t.Fatalf("enabled Reload fill = %+v, want Surface", sample(reload))
	}
	// Now with a single-entry history, Back + Fwd are disabled (muted face).
	b2, _, _, _ := newTestBrowser()
	b2.Open("http://only", "")
	buf2 := makeSurface(browserBounds.W, browserBounds.H)
	b2.Draw(newP(buf2, browserBounds.W), theme)
	back2, fwd2, reload2, _ := b2.toolbarLayout()
	s2 := func(r Rect) RGBA { return pixelAt(buf2, browserBounds.W, r.X+3, r.Y+r.H/2) }
	if s2(back2) != mutedFace(theme) {
		t.Fatalf("disabled Back fill = %+v, want mutedFace", s2(back2))
	}
	if s2(fwd2) != mutedFace(theme) {
		t.Fatalf("disabled Fwd fill = %+v, want mutedFace", s2(fwd2))
	}
	if s2(reload2) != theme.Surface {
		t.Fatalf("Reload with a tab should be enabled, fill = %+v", s2(reload2))
	}
	// Content background is the theme Background.
	cr := b2.contentRect()
	if pixelAt(buf2, browserBounds.W, cr.X+cr.W-1, cr.Y+cr.H-1) != theme.Background {
		t.Fatal("content background should be theme Background")
	}
}

func TestBrowserDrawTabStrip(t *testing.T) {
	theme := DefaultLight()
	b, _, _, _ := newTestBrowser()
	b.Open("http://tab-one-with-a-very-long-title", "A very long tab title that will not fit")
	b.Open("http://b", "") // second tab (empty title → URL fallback), active
	buf := makeSurface(browserBounds.W, browserBounds.H)
	b.Draw(newP(buf, browserBounds.W), theme)
	rects := b.tabRects()
	// Active (index 1) pill fills Surface; inactive (index 0) fills SurfaceAlt.
	activeFill := pixelAt(buf, browserBounds.W, rects[1].X+3, rects[1].Y+rects[1].H/2)
	if activeFill != theme.Surface {
		t.Fatalf("active pill fill = %+v, want Surface", activeFill)
	}
	inactiveFill := pixelAt(buf, browserBounds.W, rects[0].X+3, rects[0].Y+rects[0].H/2)
	if inactiveFill != theme.SurfaceAlt {
		t.Fatalf("inactive pill fill = %+v, want SurfaceAlt", inactiveFill)
	}
	// The active pill carries an Accent ring somewhere on its border.
	if !scanFor(buf, browserBounds.W, rects[1], theme.Accent) {
		t.Fatal("active pill should have an Accent ring")
	}
}

func TestBrowserTabRectsEmpty(t *testing.T) {
	b := NewBrowser()
	b.SetBounds(browserBounds)
	if b.tabRects() != nil {
		t.Fatal("tabRects with no tabs should be nil")
	}
}

func TestBrowserDrawTabStripTinyWidthClampsPillWidth(t *testing.T) {
	theme := DefaultLight()
	b := NewBrowser()
	b.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 200})
	for i := 0; i < BrowserMaxTabs; i++ {
		b.Open("http://p", "")
	}
	buf := makeSurface(10, 200)
	b.Draw(newP(buf, 10), theme) // exercises the pill-width floor (w<1 → 1)
	for _, r := range b.tabRects() {
		if r.W < 1 {
			t.Fatalf("pill width %d not floored to >=1", r.W)
		}
	}
}

func TestBrowserDrawAddressFocusAndCaret(t *testing.T) {
	theme := DefaultLight()
	b, _, _, _ := newTestBrowser()
	b.Open("http://a", "")
	_, _, _, addr := b.toolbarLayout()

	// Unfocused: the address border is drawn in Border, not Accent.
	buf := makeSurface(browserBounds.W, browserBounds.H)
	b.Draw(newP(buf, browserBounds.W), theme)
	if scanFor(buf, browserBounds.W, Rect{X: addr.X, Y: addr.Y, W: addr.W, H: 1}, theme.Accent) {
		t.Fatal("unfocused address must not paint an Accent ring")
	}

	// Focus + type: an Accent ring appears and a caret (OnSurface) is painted.
	b.addrFocused = true
	b.addrBuf = "hi"
	buf2 := makeSurface(browserBounds.W, browserBounds.H)
	b.Draw(newP(buf2, browserBounds.W), theme)
	if !scanFor(buf2, browserBounds.W, addr, theme.Accent) {
		t.Fatal("focused address should paint an Accent ring")
	}
	caret := Rect{X: addr.X + BrowserBtnPad, Y: addr.Y, W: addr.W - BrowserBtnPad, H: addr.H}
	if !scanFor(buf2, browserBounds.W, caret, theme.OnSurface) {
		t.Fatal("focused address should paint a caret / text in OnSurface")
	}
}

func TestBrowserDrawAddressHeadClipsLongURL(t *testing.T) {
	theme := DefaultLight()
	b, _, _, _ := newTestBrowser()
	long := "http://example.com/" + strings.Repeat("segment/", 40)
	b.Open(long, "")
	buf := makeSurface(browserBounds.W, browserBounds.H)
	b.Draw(newP(buf, browserBounds.W), theme) // exercises the head-clip branch
}

func TestBrowserDrawProgressIndeterminateThenDeterminate(t *testing.T) {
	theme := DefaultLight()
	b, _, _, _ := newTestBrowser()
	b.Open("http://a", "")
	cr := b.contentRect()
	bar := Rect{X: cr.X, Y: cr.Y, W: cr.W, H: BrowserProgressH}

	// Indeterminate (SetProgress never called): a chunk is visible at Phase 0.5.
	b.Phase = 0.5
	buf := makeSurface(browserBounds.W, browserBounds.H)
	b.Draw(newP(buf, browserBounds.W), theme)
	if !scanFor(buf, browserBounds.W, bar, theme.Accent) {
		t.Fatal("indeterminate loading bar should paint an Accent chunk")
	}

	// Determinate: SetProgress → the fill appears at the left of the bar.
	b.SetProgress(0.8)
	buf2 := makeSurface(browserBounds.W, browserBounds.H)
	b.Draw(newP(buf2, browserBounds.W), theme)
	if pixelAt(buf2, browserBounds.W, bar.X+2, bar.Y+1) != theme.Accent {
		t.Fatalf("determinate bar left = %+v, want Accent", pixelAt(buf2, browserBounds.W, bar.X+2, bar.Y+1))
	}
}

func TestBrowserDrawPageBlitAndScroll(t *testing.T) {
	theme := DefaultLight()
	b, _, _, _ := newTestBrowser()
	b.Open("http://a", "")
	cr := b.contentRect()
	// A 2×2 render: top-left pixel a distinctive colour. Scaled to the content
	// width it becomes a large block, taller than the content (dispH=400 > H),
	// so the bottom-skip continue branch runs at scroll 0.
	tl := RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xFF}
	px := []byte{
		0x11, 0x22, 0x33, 0xFF, 0x44, 0x55, 0x66, 0xFF,
		0x77, 0x88, 0x99, 0xFF, 0xAA, 0xBB, 0xCC, 0xFF,
	}
	b.Deliver("http://a", px, 2, 2, cr.W, nil, "")
	buf := makeSurface(browserBounds.W, browserBounds.H)
	b.Draw(newP(buf, browserBounds.W), theme)
	if pixelAt(buf, browserBounds.W, cr.X, cr.Y) != tl {
		t.Fatalf("content top-left = %+v, want delivered %+v", pixelAt(buf, browserBounds.W, cr.X, cr.Y), tl)
	}
	// Scroll down and redraw: the top-skip continue branch runs.
	b.tabs[b.active].scroll = 50
	buf2 := makeSurface(browserBounds.W, browserBounds.H)
	b.Draw(newP(buf2, browserBounds.W), theme)
	if pixelAt(buf2, browserBounds.W, cr.X, cr.Y) == (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) {
		t.Fatal("scrolled page still expected to paint the content top row")
	}
}

func TestBrowserDrawPageGuards(t *testing.T) {
	theme := DefaultLight()
	// len(pixels) shorter than imgW*imgH*4 → drawPage bails out.
	b, _, _, _ := newTestBrowser()
	b.Open("http://a", "")
	b.Deliver("http://a", []byte{1, 2, 3, 4}, 2, 2, 400, nil, "") // only 4 of 16 bytes
	buf := makeSurface(browserBounds.W, browserBounds.H)
	b.Draw(newP(buf, browserBounds.W), theme) // must not panic

	// dispH < 1 (very wide, one-pixel-tall render into a narrow content area).
	b2 := NewBrowser()
	b2.SetBounds(Rect{X: 0, Y: 0, W: 8, H: 200})
	b2.Open("http://a", "")
	b2.Deliver("http://a", make([]byte, 10*1*4), 10, 1, 8, nil, "")
	buf2 := makeSurface(8, 200)
	b2.Draw(newP(buf2, 8), theme) // exercises the dispH<1 guard
}

func TestBrowserDrawNoContentArea(t *testing.T) {
	theme := DefaultLight()
	b := NewBrowser()
	b.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 10}) // no room for content
	b.Open("http://a", "")
	buf := makeSurface(400, 10)
	b.Draw(newP(buf, 400), theme) // drawContent early-returns
}

func TestBrowserDrawNoActiveTab(t *testing.T) {
	theme := DefaultLight()
	b := NewBrowser()
	b.SetBounds(browserBounds)
	buf := makeSurface(browserBounds.W, browserBounds.H)
	b.Draw(newP(buf, browserBounds.W), theme) // no tabs: content area drawn but empty
	cr := b.contentRect()
	if pixelAt(buf, browserBounds.W, cr.X+1, cr.Y+1) != theme.Background {
		t.Fatal("empty content should be Background")
	}
}

// --- input ---------------------------------------------------------------

func TestBrowserDisabledSwallowsEvents(t *testing.T) {
	b, _, _, changes := newTestBrowser()
	b.Open("http://a", "")
	b.Disabled = true
	before := *changes
	b.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	b.OnEvent(Event{Kind: EventScroll, Delta: 3})
	if *changes != before {
		t.Fatalf("disabled widget reacted to events (%d changes)", *changes-before)
	}
}

func TestBrowserClickToolbarButtons(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	b.Open("http://a", "")
	b.Navigate("http://b")
	b.Navigate("http://c") // cursor 2: Back enabled, Forward disabled

	back, fwd, reload, addr := b.toolbarLayout()

	// Forward is disabled here: clicking it is a no-op (guard branch).
	bx, by := center(fwd)
	b.OnEvent(Event{Kind: EventClick, X: bx, Y: by})
	if b.CurrentURL() != "http://c" {
		t.Fatalf("disabled Forward click navigated to %q", b.CurrentURL())
	}
	// Back is enabled.
	bx, by = center(back)
	b.OnEvent(Event{Kind: EventClick, X: bx, Y: by})
	if b.CurrentURL() != "http://b" {
		t.Fatalf("Back click landed on %q", b.CurrentURL())
	}
	// Now Forward is enabled.
	bx, by = center(fwd)
	b.OnEvent(Event{Kind: EventClick, X: bx, Y: by})
	if b.CurrentURL() != "http://c" {
		t.Fatalf("Forward click landed on %q", b.CurrentURL())
	}
	// Reload.
	rx, ry := center(reload)
	b.OnEvent(Event{Kind: EventClick, X: rx, Y: ry})
	if b.CurrentURL() != "http://c" {
		t.Fatalf("Reload click changed URL to %q", b.CurrentURL())
	}
	// Address focus.
	axc, ayc := center(addr)
	b.OnEvent(Event{Kind: EventClick, X: axc, Y: ayc})
	if !b.addrFocused || b.addrBuf != "http://c" {
		t.Fatalf("address click: focused=%v buf=%q", b.addrFocused, b.addrBuf)
	}
}

func TestBrowserClickReloadWithNoTab(t *testing.T) {
	b, navT, _, _ := newTestBrowser()
	_, _, reload, _ := b.toolbarLayout()
	rx, ry := center(reload)
	b.OnEvent(Event{Kind: EventClick, X: rx, Y: ry}) // no tab: Reload guard skipped
	if len(*navT) != 0 {
		t.Fatalf("Reload with no tab navigated: %v", *navT)
	}
}

func TestBrowserClickBackDisabledNoOp(t *testing.T) {
	b, navT, _, _ := newTestBrowser()
	b.Open("http://a", "") // single entry: Back disabled
	n := len(*navT)
	back, _, _, _ := b.toolbarLayout()
	bx, by := center(back)
	b.OnEvent(Event{Kind: EventClick, X: bx, Y: by})
	if len(*navT) != n {
		t.Fatal("disabled Back click should not navigate")
	}
}

func TestBrowserClickTabStrip(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	b.Open("http://0", "")
	b.Open("http://1", "") // active = 1, strip shown
	rects := b.tabRects()

	// Click pill 0's body → activate it.
	x, y := center(rects[0])
	b.OnEvent(Event{Kind: EventClick, X: x, Y: y})
	if b.ActiveIndex() != 0 {
		t.Fatalf("tab-body click set active=%d, want 0", b.ActiveIndex())
	}
	// Click pill 1's close box → close it.
	cxr := tabCloseRect(rects[1])
	cx, cy := center(cxr)
	b.OnEvent(Event{Kind: EventClick, X: cx, Y: cy})
	if b.TabCount() != 1 {
		t.Fatalf("close-box click left %d tabs, want 1", b.TabCount())
	}
}

func TestBrowserClickTabStripGap(t *testing.T) {
	b, _, _, changes := newTestBrowser()
	b.Open("http://0", "")
	b.Open("http://1", "")
	sr := b.tabStripRect()
	before := *changes
	// Far right of the strip: past both pills (integer division leftover) → the
	// no-pill-hit return inside the strip branch.
	b.OnEvent(Event{Kind: EventClick, X: sr.X + sr.W - 1, Y: sr.Y + sr.H/2})
	if *changes != before {
		t.Fatalf("strip-gap click fired %d changes", *changes-before)
	}
}

func TestBrowserClickContentLink(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	b.Open("http://a", "")
	cr := b.contentRect()
	// A 100×100 render scaled to fill the content width. A link over the top-left
	// 20×20 render region.
	b.Deliver("http://a", make([]byte, 100*100*4), 100, 100, cr.W,
		[]BrowserLink{{Rect: image.Rect(0, 0, 20, 20), Href: "http://a/link"}}, "")

	// Click well inside the link region.
	b.OnEvent(Event{Kind: EventClick, X: cr.X + 1, Y: cr.Y + 1})
	if b.CurrentURL() != "http://a/link" {
		t.Fatalf("link click navigated to %q, want http://a/link", b.CurrentURL())
	}
	// Click content where there is no link → no navigation.
	b.Deliver("http://a/link", make([]byte, 100*100*4), 100, 100, cr.W, nil, "")
	url := b.CurrentURL()
	b.OnEvent(Event{Kind: EventClick, X: cr.X + cr.W/2, Y: cr.Y + cr.H/2})
	if b.CurrentURL() != url {
		t.Fatalf("no-link content click navigated to %q", b.CurrentURL())
	}
}

func TestBrowserClickContentNoTab(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	b.addrFocused = true
	cr := b.contentRect()
	b.OnEvent(Event{Kind: EventClick, X: cr.X + 1, Y: cr.Y + 1}) // no tab: link hit skipped
	if b.addrFocused {
		t.Fatal("content click should defocus the address field")
	}
}

func TestBrowserClickContentLoadingTabNoImage(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	b.Open("http://a", "") // loading, imgW == 0
	cr := b.contentRect()
	before := b.CurrentURL()
	b.OnEvent(Event{Kind: EventClick, X: cr.X + 1, Y: cr.Y + 1}) // linkAt imgW<=0 branch
	if b.CurrentURL() != before {
		t.Fatal("click on a not-yet-rendered page should not navigate")
	}
}

func TestBrowserLinkAtDispHZeroGuard(t *testing.T) {
	b := NewBrowser()
	b.SetBounds(Rect{X: 0, Y: 0, W: 8, H: 200})
	b.Open("http://a", "")
	cr := b.contentRect()
	// Wide, one-pixel-tall render → dispH = imgH*cr.W/imgW = 0.
	b.Deliver("http://a", make([]byte, 10*1*4), 10, 1, cr.W,
		[]BrowserLink{{Rect: image.Rect(0, 0, 10, 1), Href: "x"}}, "")
	before := b.CurrentURL()
	b.OnEvent(Event{Kind: EventClick, X: cr.X + 1, Y: cr.Y + 1}) // linkAt dispH<=0 branch
	if b.CurrentURL() != before {
		t.Fatal("degenerate render should not resolve a link")
	}
}

func TestBrowserClickOutsideEverythingDefocuses(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	b.Open("http://a", "")
	b.addrFocused = true
	tr := b.toolbarRect()
	// The right pad of the toolbar row: not a button, not the address field, not
	// the content area → the final defocus fallthrough.
	b.OnEvent(Event{Kind: EventClick, X: tr.X + tr.W - 1, Y: tr.Y + tr.H/2})
	if b.addrFocused {
		t.Fatal("click outside all regions should defocus the address field")
	}
}

func TestBrowserAddressTyping(t *testing.T) {
	b, _, _, changes := newTestBrowser()
	b.Open("http://a", "")

	// Char while unfocused is ignored.
	before := *changes
	b.OnEvent(Event{Kind: EventChar, Code: "z"})
	if b.addrBuf != "" || *changes != before {
		t.Fatalf("unfocused char leaked: buf=%q", b.addrBuf)
	}
	// Focus and type.
	b.addrFocused = true
	b.addrBuf = ""
	b.OnEvent(Event{Kind: EventChar, Code: ""}) // empty code ignored
	b.OnEvent(Event{Kind: EventChar, Code: "e"})
	b.OnEvent(Event{Kind: EventChar, Code: "g"})
	if b.addrBuf != "eg" {
		t.Fatalf("addrBuf = %q, want eg", b.addrBuf)
	}
	// KeyDown while unfocused ignored.
	b.addrFocused = false
	b.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	b.addrFocused = true
	// Backspace on non-empty then empty.
	b.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if b.addrBuf != "e" {
		t.Fatalf("after Backspace buf=%q, want e", b.addrBuf)
	}
	b.addrBuf = ""
	b.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"}) // empty: no-op
	// An unrelated key while focused is a no-op.
	b.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
}

func TestBrowserAddressCommit(t *testing.T) {
	// Enter on empty buffer: defocus, no navigation.
	b, navT, _, _ := newTestBrowser()
	b.Open("http://a", "")
	n := len(*navT)
	b.addrFocused = true
	b.addrBuf = "   "
	b.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if b.addrFocused || len(*navT) != n {
		t.Fatalf("empty commit: focused=%v navs=%d", b.addrFocused, len(*navT)-n)
	}
	// Enter with a scheme: navigated verbatim.
	b.addrFocused = true
	b.addrBuf = "http://typed"
	b.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if b.CurrentURL() != "http://typed" {
		t.Fatalf("scheme commit URL = %q", b.CurrentURL())
	}
	// Enter with a bare host: https:// is added.
	b.addrFocused = true
	b.addrBuf = "bare.example"
	b.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if b.CurrentURL() != "https://bare.example" {
		t.Fatalf("bare commit URL = %q, want https:// prefixed", b.CurrentURL())
	}
}

func TestBrowserScroll(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	b.Open("http://a", "")
	cr := b.contentRect()

	// Loading tab (no image): maxScroll imgW<=0 branch pins to 0.
	b.OnEvent(Event{Kind: EventScroll, Delta: 5})
	if b.tabs[b.active].scroll != 0 {
		t.Fatalf("scroll on unrendered page = %d, want 0", b.tabs[b.active].scroll)
	}

	// Tall render: maxScroll > 0.
	b.Deliver("http://a", make([]byte, 2*2*4), 2, 2, cr.W, nil, "")
	max := b.maxScroll(b.tabs[b.active], cr)
	if max <= 0 {
		t.Fatalf("expected a positive maxScroll, got %d", max)
	}
	// Normal scroll within range.
	b.OnEvent(Event{Kind: EventScroll, Delta: 1})
	if got := b.tabs[b.active].scroll; got != BrowserScrollStep {
		t.Fatalf("scroll = %d, want %d", got, BrowserScrollStep)
	}
	// Over-scroll clamps to max.
	b.OnEvent(Event{Kind: EventScroll, Delta: 1000})
	if got := b.tabs[b.active].scroll; got != max {
		t.Fatalf("over-scroll = %d, want clamp %d", got, max)
	}
	// Negative clamps to 0.
	b.OnEvent(Event{Kind: EventScroll, Delta: -1000})
	if got := b.tabs[b.active].scroll; got != 0 {
		t.Fatalf("negative scroll = %d, want 0", got)
	}
}

func TestBrowserScrollNoTab(t *testing.T) {
	b, _, _, changes := newTestBrowser()
	before := *changes
	b.OnEvent(Event{Kind: EventScroll, Delta: 3}) // no tab: early return
	if *changes != before {
		t.Fatalf("scroll with no tab fired %d changes", *changes-before)
	}
}

func TestBrowserMaxScrollShortImage(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	b.Open("http://a", "")
	cr := b.contentRect()
	// Wide, short render: dispH < content height → maxScroll m<0 clamps to 0.
	b.Deliver("http://a", make([]byte, 400*1*4), 400, 1, cr.W, nil, "")
	if m := b.maxScroll(b.tabs[b.active], cr); m != 0 {
		t.Fatalf("maxScroll of a short page = %d, want 0", m)
	}
}
