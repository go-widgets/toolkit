// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	stdcolor "image/color"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/go-gfx/gfx/raster"
)

// paletteTestRegistry is a small registry with a bare group, two named packs and
// one sprite icon — enough to exercise grouping, sorting and both icon kinds.
func paletteTestRegistry() *IsoIconRegistry {
	r := NewIsoIconRegistry()
	r.Register("alpha", IsoPrimitiveIcon{Build: isoBoxShapes})
	r.Register("beta", IsoPrimitiveIcon{Build: isoServerShapes})
	r.RegisterPack(IsoIconPack{Name: "net", Icons: map[string]IsoIcon{
		"router": IsoPrimitiveIcon{Build: isoRouterShapes},
		"switch": IsoPrimitiveIcon{Build: isoSwitchShapes},
	}})
	sp := raster.New(4, 4)
	for i := range sp.Pix {
		sp.Pix[i] = 255 // fully opaque white sprite
	}
	r.RegisterPack(IsoIconPack{Name: "art", Icons: map[string]IsoIcon{"star": IsoSpriteIcon{Img: sp}}})
	return r
}

// drawIsoPalette renders the palette onto a throwaway pixel surface sized to its
// bounds, under theme.
func drawIsoPalette(p *IsoIconPalette, theme *Theme) {
	b := p.Bounds()
	w, h := b.X+b.W+8, b.Y+b.H+8
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	buf := make([]byte, w*h*4)
	p.Draw(painterPixel(buf, w, h), theme)
}

// iconPointLocal is the widget-local pixel at the centre of icon id's row.
func iconPointLocal(p *IsoIconPalette, id string) (int, int, bool) {
	rows, _ := p.layout()
	for _, r := range rows {
		if !r.group && r.entry.ID == id {
			return scaled(isoPalPad) + 1, p.headerH() + r.y - p.scrollY + r.h/2, true
		}
	}
	return 0, 0, false
}

// groupPointLocal is the widget-local pixel at the centre of a heading row.
func groupPointLocal(p *IsoIconPalette, name string) (int, int, bool) {
	rows, _ := p.layout()
	for _, r := range rows {
		if r.group && r.name == name {
			return scaled(isoPalPad) + 1, p.headerH() + r.y - p.scrollY + r.h/2, true
		}
	}
	return 0, 0, false
}

// --- model: grouping matches the registry exactly ------------------------

func TestIsoPaletteGroupsMatchRegistry(t *testing.T) {
	reg := paletteTestRegistry()
	p := NewIsoIconPalette(reg)

	groups := p.Groups()
	gotNames := make([]string, len(groups))
	for i, g := range groups {
		gotNames[i] = g.Name
	}
	// Bare group first (DefaultGroupName), then packs alphabetically.
	wantNames := []string{"General", "art", "net"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("group order = %v, want %v", gotNames, wantNames)
	}
	// Within-group keys are sorted.
	if got := keysOf(groups[0]); !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Fatalf("General keys = %v", got)
	}
	if got := keysOf(groups[1]); !reflect.DeepEqual(got, []string{"star"}) {
		t.Fatalf("art keys = %v", got)
	}
	if got := keysOf(groups[2]); !reflect.DeepEqual(got, []string{"router", "switch"}) {
		t.Fatalf("net keys = %v", got)
	}

	// The palette lists EXACTLY the registry's ids — no more, no fewer.
	var gotIDs []string
	for _, e := range p.Entries() {
		gotIDs = append(gotIDs, e.ID)
	}
	wantIDs := reg.IDs()
	sort.Strings(gotIDs)
	sort.Strings(wantIDs)
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("entries %v != registry ids %v", gotIDs, wantIDs)
	}
	// The net/router entry keeps its pack + key split.
	for _, e := range p.Entries() {
		if e.ID == "net/router" && (e.Pack != "net" || e.Key != "router") {
			t.Fatalf("net/router split wrong: pack=%q key=%q", e.Pack, e.Key)
		}
	}
}

func keysOf(g IsoPaletteGroup) []string {
	out := make([]string, len(g.Entries))
	for i, e := range g.Entries {
		out[i] = e.Key
	}
	return out
}

func TestIsoPaletteDefaultGroupNameFallback(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	p.DefaultGroupName = "Basics"
	if got := p.Groups()[0].Name; got != "Basics" {
		t.Fatalf("bare group heading = %q, want Basics", got)
	}
	p.DefaultGroupName = ""
	if got := p.Groups()[0].Name; got != "General" {
		t.Fatalf("blank DefaultGroupName should fall back to General, got %q", got)
	}
}

func TestIsoPaletteNilRegistryUsesDefault(t *testing.T) {
	p := NewIsoIconPalette(nil)
	if p.Registry() != IsoDefaultIcons() {
		t.Fatal("nil registry should use the package default")
	}
	if len(p.Entries()) != len(IsoDefaultIcons().IDs()) {
		t.Fatal("default palette should list every default icon")
	}
}

func TestSplitIconID(t *testing.T) {
	if pk, k := splitIconID("net/router"); pk != "net" || k != "router" {
		t.Fatalf("split net/router = %q,%q", pk, k)
	}
	if pk, k := splitIconID("alpha"); pk != "" || k != "alpha" {
		t.Fatalf("split alpha = %q,%q", pk, k)
	}
}

// --- observables + accessors ---------------------------------------------

func TestIsoPaletteSelectionAndPick(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	var picked []string
	p.OnPickIcon = func(id string) { picked = append(picked, id) }
	inval := 0
	p.OnInvalidate = func() { inval++ }

	if p.DragData() != "" {
		t.Fatal("no selection -> empty DragData")
	}
	p.SelectIcon("beta")
	if p.SelectedIcon().Get() != "beta" {
		t.Fatal("SelectIcon did not set the observable")
	}
	if p.DragData() != EncodeIsoIconPayload("beta") {
		t.Fatalf("DragData = %q", p.DragData())
	}
	p.SelectIcon("") // clear
	if !reflect.DeepEqual(picked, []string{"beta", ""}) {
		t.Fatalf("OnPickIcon fired %v, want [beta \"\"]", picked)
	}
	if inval < 2 {
		t.Fatalf("selection changes should invalidate, got %d", inval)
	}
}

func TestIsoPalettePickWithoutCallback(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry()) // OnPickIcon nil
	p.SelectIcon("alpha")                         // must not panic on nil callback
	if p.SelectedIcon().Get() != "alpha" {
		t.Fatal("selection should hold even without a pick callback")
	}
}

func TestIsoPaletteCollapseObservable(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	inval := 0
	p.OnInvalidate = func() { inval++ }
	if p.Collapsed().Get() {
		t.Fatal("starts expanded")
	}
	p.Toggle()
	if !p.Collapsed().Get() {
		t.Fatal("Toggle did not collapse")
	}
	p.SetCollapsed(false)
	if p.Collapsed().Get() {
		t.Fatal("SetCollapsed(false) did not expand")
	}
	if inval < 2 {
		t.Fatalf("collapse changes should invalidate, got %d", inval)
	}
}

func TestIsoPaletteSetBoundsMirrorsOrigin(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	p.SetBounds(Rect{X: 10, Y: 20, W: 100, H: 200})
	if got := p.Origin().Get(); got != (IsoPalettePos{X: 10, Y: 20}) {
		t.Fatalf("Origin after SetBounds = %+v, want {10,20}", got)
	}
}

func TestIsoPaletteOriginSetMovesPanel(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	p.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})
	// A host restoring a saved position Sets the origin directly; the panel moves.
	p.Origin().Set(IsoPalettePos{X: 33, Y: 44})
	if b := p.Bounds(); b.X != 33 || b.Y != 44 || b.W != 100 || b.H != 200 {
		t.Fatalf("panel did not follow Origin: %+v", b)
	}
}

// --- geometry / scroll ---------------------------------------------------

func TestIsoPaletteScrollClamp(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	// Short body so the content overflows and the scrollbar is live.
	p.SetBounds(Rect{X: 0, Y: 0, W: scaled(isoPalWidth), H: p.headerH() + scaled(isoPalGroupH)*2})
	if _, live := p.vscrollGeom(); !live {
		t.Fatal("expected a live scrollbar")
	}
	p.scrollTo(-5) // clamps to 0 (was already 0 -> no change)
	if p.scrollY != 0 {
		t.Fatalf("scrollTo(-5) => %d, want 0", p.scrollY)
	}
	p.scrollTo(1 << 20) // clamps to maxOff
	_, contentH := p.layout()
	maxOff := contentH - p.bodyRect().H
	if p.scrollY != maxOff {
		t.Fatalf("scrollTo(huge) => %d, want maxOff %d", p.scrollY, maxOff)
	}
	p.scrollBy(-1 << 20) // back to 0
	if p.scrollY != 0 {
		t.Fatalf("scrollBy(-huge) => %d, want 0", p.scrollY)
	}
}

func TestIsoPaletteScrollToNoOverflow(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	// Tall body: content fits, maxOff < 0 path.
	p.SetBounds(Rect{X: 0, Y: 0, W: scaled(isoPalWidth), H: 2000})
	if _, live := p.vscrollGeom(); live {
		t.Fatal("did not expect a scrollbar when content fits")
	}
	p.scrollTo(50) // maxOff clamps to 0
	if p.scrollY != 0 {
		t.Fatalf("scrollTo on non-overflowing body => %d, want 0", p.scrollY)
	}
}

func TestIsoPaletteThumbMinHeight(t *testing.T) {
	r := NewIsoIconRegistry()
	for i := 0; i < 80; i++ {
		r.Register("n"+strconv.Itoa(i), IsoPrimitiveIcon{Build: isoBoxShapes})
	}
	p := NewIsoIconPalette(r)
	p.SetBounds(Rect{X: 0, Y: 0, W: scaled(isoPalWidth), H: p.headerH() + scaled(60)})
	g, live := p.vscrollGeom()
	if !live {
		t.Fatal("expected live scrollbar with 80 icons")
	}
	if g.thumbLen != scaled(8) {
		t.Fatalf("thumb height = %d, want clamped to %d", g.thumbLen, scaled(8))
	}
}

func TestIsoPaletteEntryAt(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	p.SetBounds(Rect{X: 0, Y: 0, W: scaled(isoPalWidth), H: 2000})

	x, y, ok := iconPointLocal(p, "beta")
	if !ok {
		t.Fatal("beta row not laid out")
	}
	if e, hit := p.entryAt(x, y); !hit || e.ID != "beta" {
		t.Fatalf("entryAt over beta = %+v,%v", e, hit)
	}
	// A heading row is not an icon.
	gx, gy, ok := groupPointLocal(p, "net")
	if !ok {
		t.Fatal("net heading not laid out")
	}
	if _, hit := p.entryAt(gx, gy); hit {
		t.Fatal("entryAt on a heading must miss")
	}
	// Outside the body / list.
	if _, hit := p.entryAt(-1, y); hit {
		t.Fatal("entryAt left of the list must miss")
	}
	if _, hit := p.entryAt(x, p.headerH()-1); hit {
		t.Fatal("entryAt above the body must miss")
	}
	if _, hit := p.entryAt(x, 100000); hit {
		t.Fatal("entryAt below the body must miss")
	}
}

func TestIsoPaletteEntryAtExcludesScrollbar(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	p.SetBounds(Rect{X: 0, Y: 0, W: scaled(isoPalWidth), H: p.headerH() + scaled(isoPalGroupH)*2})
	if _, live := p.vscrollGeom(); !live {
		t.Fatal("need a live scrollbar for this test")
	}
	// A point in the scrollbar column must not select the icon behind it.
	body := p.bodyRect()
	xbar := body.W - scrollbarTrack()/2
	if _, hit := p.entryAt(xbar, body.Y+1); hit {
		t.Fatal("entryAt in the scrollbar column must miss")
	}
}

// --- events --------------------------------------------------------------

func TestIsoPaletteClickSelectsIcon(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	p.SetBounds(Rect{X: 0, Y: 0, W: scaled(isoPalWidth), H: 2000})
	x, y, _ := iconPointLocal(p, "net/router")
	p.OnEvent(Event{Kind: EventClick, X: x, Y: y})
	if p.SelectedIcon().Get() != "net/router" {
		t.Fatalf("click selected %q, want net/router", p.SelectedIcon().Get())
	}
	// A click on a heading changes nothing.
	gx, gy, _ := groupPointLocal(p, "General")
	p.OnEvent(Event{Kind: EventClick, X: gx, Y: gy})
	if p.SelectedIcon().Get() != "net/router" {
		t.Fatal("heading click must not change selection")
	}
}

func TestIsoPaletteHeaderToggle(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	p.SetBounds(Rect{X: 0, Y: 0, W: scaled(isoPalWidth), H: 300})
	tr := p.toggleRect()
	p.OnEvent(Event{Kind: EventClick, X: tr.X + tr.W/2, Y: tr.Y + tr.H/2})
	if !p.Collapsed().Get() {
		t.Fatal("clicking the toggle should collapse")
	}
	p.OnEvent(Event{Kind: EventClick, X: tr.X + tr.W/2, Y: tr.Y + tr.H/2})
	if p.Collapsed().Get() {
		t.Fatal("clicking the toggle again should expand")
	}
}

func TestIsoPaletteHeaderDragMovesPanel(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	p.SetBounds(Rect{X: 100, Y: 100, W: scaled(isoPalWidth), H: 300})
	// Grab the header away from the toggle box, then drag.
	p.OnEvent(Event{Kind: EventClick, X: 5, Y: p.headerH() / 2})
	p.OnEvent(Event{Kind: EventMouseDrag, X: 35, Y: p.headerH()/2 + 15})
	if b := p.Bounds(); b.X != 130 || b.Y != 115 {
		t.Fatalf("header drag moved panel to (%d,%d), want (130,115)", b.X, b.Y)
	}
	if got := p.Origin().Get(); got != (IsoPalettePos{X: 130, Y: 115}) {
		t.Fatalf("Origin after drag = %+v", got)
	}
	// A drag that does not move the pointer leaves the panel put.
	before := p.Bounds()
	p.OnEvent(Event{Kind: EventMouseDrag, X: 5, Y: p.headerH() / 2})
	// (dx,dy computed from the grab point; equal point => no move)
	p.OnEvent(Event{Kind: EventMouseUp})
	if p.Bounds() != before {
		t.Fatal("zero-delta drag should not move the panel")
	}
}

func TestIsoPaletteScrollbarDragAndWheel(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	p.SetBounds(Rect{X: 0, Y: 0, W: scaled(isoPalWidth), H: p.headerH() + scaled(isoPalGroupH)*3})
	g, live := p.vscrollGeom()
	if !live {
		t.Fatal("need a live scrollbar")
	}
	// Wheel scrolls.
	p.OnEvent(Event{Kind: EventScroll, Delta: 1})
	if p.scrollY == 0 {
		t.Fatal("wheel did not scroll")
	}
	p.scrollTo(0)
	// Grab the thumb and drag it down.
	p.OnEvent(Event{Kind: EventClick, X: g.cross0 + g.crossW/2, Y: g.thumbStart + g.thumbLen/2})
	p.OnEvent(Event{Kind: EventMouseDrag, X: g.cross0 + g.crossW/2, Y: g.thumbStart + g.thumbLen/2 + scaled(20)})
	if p.scrollY == 0 {
		t.Fatal("thumb drag did not scroll")
	}
	p.OnEvent(Event{Kind: EventMouseUp})
	// A press on the track below the thumb pages down.
	p.scrollTo(0)
	g, _ = p.vscrollGeom()
	p.OnEvent(Event{Kind: EventClick, X: g.cross0 + g.crossW/2, Y: g.thumbStart + g.thumbLen + 2})
	if p.scrollY == 0 {
		t.Fatal("track paging did not scroll")
	}
}

func TestIsoPaletteWheelCollapsedIgnored(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	p.SetBounds(Rect{X: 0, Y: 0, W: scaled(isoPalWidth), H: p.headerH() + scaled(isoPalGroupH)*2})
	p.SetCollapsed(true)
	p.OnEvent(Event{Kind: EventScroll, Delta: 3})
	if p.scrollY != 0 {
		t.Fatal("a collapsed palette must not scroll")
	}
}

func TestIsoPaletteCollapsedBodyPressIgnored(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	p.SetBounds(Rect{X: 0, Y: 0, W: scaled(isoPalWidth), H: 300})
	p.SetCollapsed(true)
	if p.bodyRect() != (Rect{}) {
		t.Fatal("a collapsed palette has no body viewport")
	}
	// A press below the header while collapsed is a no-op (no selection).
	p.OnEvent(Event{Kind: EventClick, X: 5, Y: p.headerH() + 5})
	if p.SelectedIcon().Get() != "" {
		t.Fatal("collapsed body press must not select")
	}
}

func TestIsoPaletteDisabledIgnoresEvents(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	p.SetBounds(Rect{X: 0, Y: 0, W: scaled(isoPalWidth), H: 2000})
	p.Disabled().Set(true)
	x, y, _ := iconPointLocal(p, "alpha")
	p.OnEvent(Event{Kind: EventClick, X: x, Y: y})
	if p.SelectedIcon().Get() != "" {
		t.Fatal("a disabled palette must ignore events")
	}
}

// --- accessibility -------------------------------------------------------

func TestIsoPaletteA11y(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	p.SelectIcon("beta")
	info := p.A11y()
	if info.Role != RoleList || info.Name != "Icons" || info.Value != "beta" {
		t.Fatalf("A11y = %+v", info)
	}
	p.Title = ""
	if p.title() != "Icons" {
		t.Fatalf("blank Title should fall back, got %q", p.title())
	}
}

// --- rendering -----------------------------------------------------------

func TestIsoPaletteDrawExpanded(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	// Tall enough to show every row (incl. the sprite icon) with no scrollbar.
	p.SetBounds(Rect{X: 4, Y: 4, W: scaled(isoPalWidth), H: 340})
	p.SelectIcon("beta") // a selected row exercises the highlight fill
	for _, theme := range []*Theme{DefaultLight(), DefaultDark()} {
		drawIsoPalette(p, theme)
	}
}

func TestIsoPaletteDrawScrollableAndScrolled(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	p.SetBounds(Rect{X: 0, Y: 0, W: scaled(isoPalWidth), H: p.headerH() + scaled(isoPalGroupH)*3})
	// Scroll a fractional amount so the top row is partially clipped (the
	// partial-row skip path) while the scrollbar paints.
	p.scrollTo(scaled(10))
	drawIsoPalette(p, DefaultLight())
}

func TestIsoPaletteDrawCollapsed(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	p.SetBounds(Rect{X: 0, Y: 0, W: scaled(isoPalWidth), H: 300})
	p.SetCollapsed(true)
	drawIsoPalette(p, DefaultLight())
}

func TestIsoPaletteDrawDegenerate(t *testing.T) {
	p := NewIsoIconPalette(paletteTestRegistry())
	p.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 100})
	drawIsoPalette(p, DefaultLight()) // W<=0 early return, must not panic
}

// --- icon thumbnail ------------------------------------------------------

func TestRenderIsoIconThumbnailKinds(t *testing.T) {
	proj := isoThumbProjection(scaled(isoPalIconSize), scaled(isoPalThumbPad))
	base := stdcolor.RGBA{R: 40, G: 120, B: 200, A: 255}
	bg := RGBA{R: 250, G: 250, B: 250, A: 255}

	// Primitive icon -> shapes drawn over the bg.
	prim := renderIsoIconThumbnail(IsoPrimitiveIcon{Build: isoServerShapes}, proj, scaled(isoPalIconSize), bg, base)
	if prim.W != scaled(isoPalIconSize) || !hasNonBg(prim, bg) {
		t.Fatal("primitive thumbnail drew nothing")
	}
	// Sprite icon -> the sprite is blitted.
	sp := raster.New(3, 3)
	for i := range sp.Pix {
		sp.Pix[i] = 200
	}
	sp.Pix[3] = 255 // ensure an opaque pixel
	spr := renderIsoIconThumbnail(IsoSpriteIcon{Img: sp}, proj, scaled(isoPalIconSize), bg, base)
	if !hasNonBg(spr, bg) {
		t.Fatal("sprite thumbnail drew nothing")
	}
}

func hasNonBg(img *raster.Image, bg RGBA) bool {
	for i := 0; i+3 < len(img.Pix); i += 4 {
		if img.Pix[i] != bg.R || img.Pix[i+1] != bg.G || img.Pix[i+2] != bg.B {
			return true
		}
	}
	return false
}

func TestIsoThumbProjectionTinySize(t *testing.T) {
	// pad larger than half the size -> inner clamps to 1, must not divide by zero.
	proj := isoThumbProjection(2, 3)
	if proj == nil || proj.TileW <= 0 {
		t.Fatalf("degenerate thumb projection: %+v", proj)
	}
}
