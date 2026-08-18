// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	stdcolor "image/color"
	"math"
	"sort"
	"strings"

	"github.com/go-gfx/gfx/geometry"
	"github.com/go-gfx/gfx/iso"
	"github.com/go-gfx/gfx/raster"
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// IsoIconDragPrefix namespaces an [IsoIconPalette] drag payload. A palette that
// starts a drag carries "iso-icon:<id>" (see [EncodeIsoIconPayload]) so an
// [IsoDiagram] can tell an icon drop from any other drag reaching the canvas.
const IsoIconDragPrefix = "iso-icon:"

// EncodeIsoIconPayload builds the drag payload for icon id — the string an
// [IsoIconPalette] returns from DragData and an [IsoDiagram] decodes on drop.
func EncodeIsoIconPayload(id string) string { return IsoIconDragPrefix + id }

// DecodeIsoIconPayload recovers the icon id from a drop payload, scanning the
// (possibly multi-item, newline-separated — see [SplitDropPayload]) payload for
// the first "iso-icon:<id>" item with a non-empty id. It returns "" and false
// when the payload carries no icon, so a diagram accepts and acts on exactly the
// drags a palette originates.
func DecodeIsoIconPayload(payload string) (id string, ok bool) {
	for _, item := range SplitDropPayload(payload) {
		if rest, found := strings.CutPrefix(item, IsoIconDragPrefix); found && rest != "" {
			return rest, true
		}
	}
	return "", false
}

// IsoPalettePos is an [IsoIconPalette]'s top-left position on its surface. It is
// a comparable value so it rides in an mvvm.Observable ([IsoIconPalette.Origin])
// the host can bind, observe and persist — the MVVM way the panel's position
// crosses the widget/host boundary while the panel repositions itself on a
// header drag.
type IsoPalettePos struct{ X, Y int }

// IsoPaletteEntry is one listed icon of an [IsoIconPalette]: its registry id, the
// pack it belongs to (the id's prefix before "/", or "" for a bare id), the
// within-pack key that labels it, and the resolved [IsoIcon] the thumbnail
// renders.
type IsoPaletteEntry struct {
	// ID is the registry id ("aws/ec2", or a bare "server").
	ID string
	// Pack is the id's namespace (the part before "/"), or "" for a bare id.
	Pack string
	// Key is the within-pack key (the part after "/", or the whole bare id) — the
	// entry's label.
	Key string
	// Icon is the resolved icon the entry's thumbnail draws.
	Icon IsoIcon
}

// IsoPaletteGroup is one pack's worth of [IsoPaletteEntry] under a heading, as an
// [IsoIconPalette] lays its list out. Name is the pack's display heading (a bare
// id's group takes the palette's DefaultGroupName).
type IsoPaletteGroup struct {
	Name    string
	Entries []IsoPaletteEntry
}

// IsoIconPalette metrics, in LOGICAL pixels (routed through [scaled] so the panel
// scales with the rest of the chrome under HiDPI / touch density).
const (
	isoPalWidth    = 184 // panel width
	isoPalHeaderH  = 30  // draggable header band height
	isoPalGroupH   = 20  // pack-heading row height
	isoPalRowH     = 44  // icon row height
	isoPalIconSize = 32  // icon thumbnail square side
	isoPalPad      = 8   // inner horizontal padding
	isoPalThumbPad = 4   // padding inside the icon thumbnail
	isoPalRadius   = 6   // panel corner radius
)

// IsoIconPalette is a draggable, collapsible panel that lists an
// [IsoIconRegistry]'s icons grouped by pack, each row a small isometric
// thumbnail (rendered through [IsoIcon.Render]) beside its key. It is the
// component drawer for an [IsoDiagram]: a user picks an icon here and drops it on
// the canvas (the palette is a [DragSource]; the diagram a [DropTarget]), or —
// where a back-end has no inter-widget drag — clicks an icon to arm it and taps
// the canvas to place it (bind [IsoIconPalette.SelectedIcon] to
// [IsoDiagram.PlacementIconObservable], or wire OnPickIcon).
//
// The panel repositions itself when its header is dragged and collapses to just
// the header on a click of the header's toggle; both the position
// ([IsoIconPalette.Origin]) and the collapsed flag ([IsoIconPalette.Collapsed])
// are mvvm.Observables so that cross-boundary state is observed, not polled.
// The icon list scrolls (reusing the toolkit's shared scrollbar machinery) when
// it overflows the body.
type IsoIconPalette struct {
	Base

	// Title is the header caption. A blank Title falls back to a generic default.
	Title string
	// DefaultGroupName is the heading for bare (un-namespaced) icon ids. A blank
	// value falls back to a generic default.
	DefaultGroupName string

	// OnPickIcon fires when the selected (armed) icon changes — with the new
	// registry id, or "" when the selection is cleared. Nil-guarded. It is the
	// click-to-place hook a host wires to [IsoDiagram.SetPlacementIcon] when it
	// does not route the drag-and-drop payload itself.
	OnPickIcon func(id string)
	// OnInvalidate, when set, is called whenever the panel's appearance changed
	// and it should be redrawn.
	OnInvalidate func()

	reg       *IsoIconRegistry
	selected  *mvvm.Observable[string]
	collapsed *mvvm.Observable[bool]
	origin    *mvvm.Observable[IsoPalettePos]

	scrollY int        // body scroll offset in device pixels
	sbDrag  scrollDrag // in-progress scrollbar thumb drag

	// header-drag (panel move) state
	moving         bool
	grabDX, grabDY int // the widget-local point grabbed in the header
}

// Compile-time proof of the drag-and-drop contract: a palette is a drag source,
// a diagram the matching drop target.
var (
	_ DragSource = (*IsoIconPalette)(nil)
	_ DropTarget = (*IsoDiagram)(nil)
	_ Accessible = (*IsoIconPalette)(nil)
)

// NewIsoIconPalette builds a palette over reg (nil uses the package-global
// [IsoDefaultIcons]). Nothing is selected, the panel is expanded, and its origin
// tracks whatever [IsoIconPalette.SetBounds] the host gives it.
func NewIsoIconPalette(reg *IsoIconRegistry) *IsoIconPalette {
	if reg == nil {
		reg = IsoDefaultIcons()
	}
	p := &IsoIconPalette{
		Title:            "Icons",
		DefaultGroupName: "General",
		reg:              reg,
		selected:         mvvm.NewObservable(""),
		collapsed:        mvvm.NewObservable(false),
		origin:           mvvm.NewObservable(IsoPalettePos{}),
	}
	// Selecting an icon arms placement and notifies the host; clearing it (to "")
	// notifies too, so a bound diagram disarms.
	p.selected.Subscribe(func(id string) {
		if p.OnPickIcon != nil {
			p.OnPickIcon(id)
		}
		p.invalidate()
	})
	p.collapsed.SubscribeChanged(func() { p.invalidate() })
	// A host that Sets the origin (restoring a saved position) moves the panel to
	// match; the panel's own header drag Sets the same observable, so position is
	// one source of truth whichever side moved it.
	p.origin.Subscribe(func(pos IsoPalettePos) {
		if r := p.Base.Bounds(); r.X != pos.X || r.Y != pos.Y {
			p.Base.SetBounds(Rect{X: pos.X, Y: pos.Y, W: r.W, H: r.H})
		}
		p.invalidate()
	})
	return p
}

// invalidate calls OnInvalidate when one is set.
func (p *IsoIconPalette) invalidate() {
	if p.OnInvalidate != nil {
		p.OnInvalidate()
	}
}

// SetBounds positions and sizes the panel and mirrors the position into the
// Origin observable (a no-op emit when unchanged), so binding Origin and calling
// SetBounds stay consistent.
func (p *IsoIconPalette) SetBounds(r Rect) {
	p.Base.SetBounds(r)
	p.origin.Set(IsoPalettePos{X: r.X, Y: r.Y})
}

// --- observable accessors ------------------------------------------------

// SelectedIcon exposes the armed icon id so a host binds it into an
// [IsoDiagram.PlacementIconObservable] (or its own view model) instead of
// polling. "" means nothing is selected.
func (p *IsoIconPalette) SelectedIcon() *mvvm.Observable[string] { return p.selected }

// SelectIcon selects (arms) the icon with id, or clears the selection when id is
// "". It is the programmatic counterpart of clicking a row.
func (p *IsoIconPalette) SelectIcon(id string) { p.selected.Set(id) }

// Collapsed exposes the collapsed flag so a host can observe or drive the
// fold/unfold. true hides the icon list, leaving only the header.
func (p *IsoIconPalette) Collapsed() *mvvm.Observable[bool] { return p.collapsed }

// SetCollapsed folds (true) or unfolds (false) the panel.
func (p *IsoIconPalette) SetCollapsed(v bool) { p.collapsed.Set(v) }

// Toggle flips the collapsed state.
func (p *IsoIconPalette) Toggle() { p.collapsed.Set(!p.collapsed.Get()) }

// Origin exposes the panel's top-left position so a host can observe (persist)
// or drive it; the panel Sets it when its header is dragged.
func (p *IsoIconPalette) Origin() *mvvm.Observable[IsoPalettePos] { return p.origin }

// Registry returns the icon registry the palette lists.
func (p *IsoIconPalette) Registry() *IsoIconRegistry { return p.reg }

// DragData makes the palette a [DragSource]: it returns the armed icon's drag
// payload, or "" when nothing is selected (so a drag that began on no icon
// carries nothing).
func (p *IsoIconPalette) DragData() string {
	if id := p.selected.Get(); id != "" {
		return EncodeIsoIconPayload(id)
	}
	return ""
}

// --- model: groups, entries, layout --------------------------------------

// splitIconID splits a registry id into its pack (before "/") and key (after
// "/"); a bare id has an empty pack and is its own key.
func splitIconID(id string) (pack, key string) {
	if i := strings.IndexByte(id, '/'); i >= 0 {
		return id[:i], id[i+1:]
	}
	return "", id
}

// groupName is g's display heading, falling back to DefaultGroupName (itself
// defaulting to a generic label) for the bare-id group.
func (p *IsoIconPalette) groupName(pack string) string {
	if pack != "" {
		return pack
	}
	if p.DefaultGroupName != "" {
		return p.DefaultGroupName
	}
	return "General"
}

// Groups returns the registry's icons grouped by pack, deterministically
// ordered: the bare-id group first, then packs alphabetically, and within each
// group the entries by key. It is what the panel draws and what a test asserts
// against — the palette lists exactly the registry's ids, by pack.
func (p *IsoIconPalette) Groups() []IsoPaletteGroup {
	byPack := map[string][]IsoPaletteEntry{}
	for _, id := range p.reg.IDs() {
		pack, key := splitIconID(id)
		icon, _ := p.reg.Resolve(id)
		byPack[pack] = append(byPack[pack], IsoPaletteEntry{ID: id, Pack: pack, Key: key, Icon: icon})
	}
	packs := make([]string, 0, len(byPack))
	for pk := range byPack {
		packs = append(packs, pk)
	}
	sort.Slice(packs, func(i, j int) bool {
		if (packs[i] == "") != (packs[j] == "") {
			return packs[i] == "" // bare group first
		}
		return packs[i] < packs[j]
	})
	out := make([]IsoPaletteGroup, 0, len(packs))
	for _, pk := range packs {
		es := byPack[pk]
		sort.Slice(es, func(i, j int) bool { return es[i].Key < es[j].Key })
		out = append(out, IsoPaletteGroup{Name: p.groupName(pk), Entries: es})
	}
	return out
}

// Entries returns every listed entry, flattened in the same order the groups lay
// out.
func (p *IsoIconPalette) Entries() []IsoPaletteEntry {
	var out []IsoPaletteEntry
	for _, g := range p.Groups() {
		out = append(out, g.Entries...)
	}
	return out
}

// isoPalRow is one laid-out row of the list at content-space top y and height h:
// a pack heading (group true, name set) or an icon (group false, entry set).
type isoPalRow struct {
	y, h  int
	group bool
	name  string
	entry IsoPaletteEntry
}

// layout places every heading and icon row in body content space and returns the
// rows and the total content height. Draw and hit-testing both derive from it so
// the painted rows and the click targets can never drift apart.
func (p *IsoIconPalette) layout() (rows []isoPalRow, contentH int) {
	gh, rh := scaled(isoPalGroupH), scaled(isoPalRowH)
	y := 0
	for _, g := range p.Groups() {
		rows = append(rows, isoPalRow{y: y, h: gh, group: true, name: g.Name})
		y += gh
		for _, e := range g.Entries {
			rows = append(rows, isoPalRow{y: y, h: rh, entry: e})
			y += rh
		}
	}
	return rows, y
}

// --- geometry ------------------------------------------------------------

// headerH is the header band height in device pixels.
func (p *IsoIconPalette) headerH() int { return scaled(isoPalHeaderH) }

// toggleRect is the header's collapse/expand hit box (a square at the header's
// right), in widget-local coordinates.
func (p *IsoIconPalette) toggleRect() Rect {
	h := p.headerH()
	return Rect{X: p.Base.Bounds().W - h, Y: 0, W: h, H: h}
}

// bodyRect is the icon-list viewport below the header, in widget-local
// coordinates. It is empty while the panel is collapsed.
func (p *IsoIconPalette) bodyRect() Rect {
	r := p.Base.Bounds()
	if p.collapsed.Get() {
		return Rect{}
	}
	h := p.headerH()
	return Rect{X: 0, Y: h, W: r.W, H: r.H - h}
}

// vscrollGeom returns the icon list's scrollbar geometry (widget-local) and
// whether it is live (the content overflows the body).
func (p *IsoIconPalette) vscrollGeom() (sbGeom, bool) {
	body := p.bodyRect()
	_, contentH := p.layout()
	if !(contentH > body.H && body.H > 0) {
		return sbGeom{}, false
	}
	thumbH := body.H * body.H / contentH
	if m := scaled(8); thumbH < m {
		thumbH = m
	}
	maxOff := contentH - body.H // > 0 here
	track := scrollbarTrack()
	return sbGeom{
		cross0:     body.W - track,
		crossW:     track,
		trackStart: body.Y,
		trackLen:   body.H,
		thumbStart: body.Y + p.scrollY*(body.H-thumbH)/maxOff,
		thumbLen:   thumbH,
		travelNum:  body.H - thumbH,
		travelDen:  maxOff,
		maxScroll:  maxOff,
	}, true
}

// scrollBy shifts the body scroll by dy device pixels, clamped to the content.
func (p *IsoIconPalette) scrollBy(dy int) { p.scrollTo(p.scrollY + dy) }

// scrollTo sets the body scroll to target device pixels, clamped to
// [0, contentH-bodyH].
func (p *IsoIconPalette) scrollTo(target int) {
	body := p.bodyRect()
	_, contentH := p.layout()
	maxOff := contentH - body.H
	if maxOff < 0 {
		maxOff = 0
	}
	if target < 0 {
		target = 0
	}
	if target > maxOff {
		target = maxOff
	}
	if target != p.scrollY {
		p.scrollY = target
		p.invalidate()
	}
}

// entryAt returns the icon entry at widget-local (x, y) within the body, and
// whether one is there (a heading row or empty space returns false). It excludes
// the scrollbar column so a click on the bar never selects the icon behind it.
func (p *IsoIconPalette) entryAt(x, y int) (IsoPaletteEntry, bool) {
	body := p.bodyRect()
	listW := body.W
	if _, live := p.vscrollGeom(); live {
		listW -= scrollbarTrack()
	}
	if x < 0 || x >= listW || y < body.Y || y >= body.Y+body.H {
		return IsoPaletteEntry{}, false
	}
	contentY := y - body.Y + p.scrollY
	rows, _ := p.layout()
	for _, r := range rows {
		if !r.group && contentY >= r.y && contentY < r.y+r.h {
			return r.entry, true
		}
	}
	return IsoPaletteEntry{}, false
}

// --- events --------------------------------------------------------------

// OnEvent drives the panel: a header press begins a move or toggles the fold, a
// body press selects an icon or grabs the scrollbar, drags advance the move or
// the thumb, the wheel scrolls the list. A disabled panel ignores everything.
func (p *IsoIconPalette) OnEvent(ev Event) {
	if p.Disabled {
		return
	}
	switch ev.Kind {
	case EventScroll:
		if !p.collapsed.Get() {
			p.scrollBy(ev.Delta * scaled(isoPalRowH))
		}
	case EventClick:
		p.onPress(ev)
	case EventMouseDrag:
		p.onDrag(ev)
	case EventMouseUp:
		p.onRelease(ev)
	}
}

// onPress dispatches a button press to the header (toggle / move) or the body
// (scrollbar / icon select).
func (p *IsoIconPalette) onPress(ev Event) {
	if ev.Y < p.headerH() {
		if p.toggleRect().Contains(ev.X, ev.Y) {
			p.Toggle()
			return
		}
		p.moving = true
		p.grabDX, p.grabDY = ev.X, ev.Y
		return
	}
	if p.collapsed.Get() {
		return
	}
	if g, live := p.vscrollGeom(); p.sbDrag.press(g, live, ev, p.bodyRect().H, func(d int) { p.scrollBy(d) }) {
		return
	}
	if e, ok := p.entryAt(ev.X, ev.Y); ok {
		p.selected.Set(e.ID)
	}
}

// onDrag advances a header move (repositioning the panel by the pointer's delta
// from the grabbed header point) or a scrollbar thumb drag.
func (p *IsoIconPalette) onDrag(ev Event) {
	if p.moving {
		dx, dy := ev.X-p.grabDX, ev.Y-p.grabDY
		if dx != 0 || dy != 0 {
			r := p.Base.Bounds()
			p.SetBounds(Rect{X: r.X + dx, Y: r.Y + dy, W: r.W, H: r.H})
		}
		return
	}
	g, live := p.vscrollGeom()
	p.sbDrag.drag(g, live, ev, func(target int) { p.scrollTo(target) })
}

// onRelease ends a header move or a scrollbar drag.
func (p *IsoIconPalette) onRelease(Event) {
	p.moving = false
	p.sbDrag.release()
}

// --- accessibility -------------------------------------------------------

// A11y reports the palette as a list, with the armed icon id as its value so a
// screen reader announces the current pick.
func (p *IsoIconPalette) A11y() A11yInfo {
	return A11yInfo{Role: RoleList, Name: p.title(), Value: p.selected.Get()}
}

// --- rendering -----------------------------------------------------------

// title is the header caption, falling back to a generic default when blank.
func (p *IsoIconPalette) title() string {
	if p.Title != "" {
		return p.Title
	}
	return "Icons"
}

// Draw paints the panel: a rounded surface with a header band (title + fold
// chevron) and, when expanded, the grouped icon list with its scrollbar. Each
// icon row renders its isometric thumbnail into a small buffer blitted at the
// row, with the selected row tinted.
func (p *IsoIconPalette) Draw(pt painter.Painter, theme *Theme) {
	b := p.Base.Bounds()
	if b.W <= 0 || b.H <= 0 {
		return
	}
	collapsed := p.collapsed.Get()
	headerH := p.headerH()
	panelH := b.H
	if collapsed {
		panelH = headerH
	}
	radius := scaled(isoPalRadius)
	fillRoundRect(pt, b.X, b.Y, b.W, panelH, radius, theme.Surface)
	strokeRoundRect(pt, b.X, b.Y, b.W, panelH, radius, theme.Border)

	// header band + caption + fold chevron
	fillRect(pt, b.X+strokeWidth(), b.Y+strokeWidth(), b.W-2*strokeWidth(), headerH-strokeWidth(), theme.SurfaceAlt)
	ty := b.Y + (headerH-p.glyphHeight())/2
	p.drawText(pt, b.X+scaled(isoPalPad), ty, p.title(), theme.OnSurface)
	p.drawChevron(pt, theme, b, headerH, collapsed)

	if collapsed {
		return
	}
	p.drawList(pt, theme, b, headerH)
}

// drawChevron paints the collapse/expand affordance in the header's toggle box:
// a down chevron when expanded (click to fold), a right chevron when collapsed.
func (p *IsoIconPalette) drawChevron(pt painter.Painter, theme *Theme, b Rect, headerH int, collapsed bool) {
	tr := p.toggleRect()
	cx := b.X + tr.X + tr.W/2
	cy := b.Y + tr.Y + tr.H/2
	s := scaled(4)
	ink := theme.OnSurface
	if collapsed {
		// right-pointing chevron ">"
		drawLine(pt, cx-s/2, cy-s, cx+s/2, cy, ink)
		drawLine(pt, cx-s/2, cy+s, cx+s/2, cy, ink)
	} else {
		// down-pointing chevron "v"
		drawLine(pt, cx-s, cy-s/2, cx, cy+s/2, ink)
		drawLine(pt, cx+s, cy-s/2, cx, cy+s/2, ink)
	}
}

// drawList paints the grouped icon rows within the body viewport (fully-visible
// rows only, so the list needs no painter clip) and the scrollbar.
func (p *IsoIconPalette) drawList(pt painter.Painter, theme *Theme, b Rect, headerH int) {
	body := p.bodyRect()
	bodyTop := b.Y + body.Y
	bodyBot := bodyTop + body.H
	listW := body.W
	g, live := p.vscrollGeom()
	if live {
		listW -= scrollbarTrack()
	}
	iconSz := scaled(isoPalIconSize)
	proj := isoThumbProjection(iconSz, scaled(isoPalThumbPad))
	rows, _ := p.layout()
	for _, r := range rows {
		top := bodyTop + r.y - p.scrollY
		if top < bodyTop || top+r.h > bodyBot {
			continue // partially or fully outside the viewport
		}
		if r.group {
			p.drawText(pt, b.X+scaled(isoPalPad), top+(r.h-p.glyphHeight())/2, r.name, dimInk(theme))
			continue
		}
		rowBg := theme.Surface
		if p.selected.Get() == r.entry.ID {
			rowBg = blendRGBA(theme.Accent, theme.Surface, 0.30)
			fillRect(pt, b.X+strokeWidth(), top, listW-strokeWidth(), r.h, rowBg)
		}
		ix := b.X + scaled(isoPalPad)
		iy := top + (r.h-iconSz)/2
		thumb := renderIsoIconThumbnail(r.entry.Icon, proj, iconSz, rowBg, stdColor(theme.Accent))
		blitImage(pt, Rect{X: ix, Y: iy, W: iconSz, H: iconSz}, Rect{X: ix, Y: iy, W: iconSz, H: iconSz}, thumb.Pix, thumb.W, thumb.H)
		lx := ix + iconSz + scaled(isoPalPad)
		p.drawText(pt, lx, top+(r.h-p.glyphHeight())/2, r.entry.Key, theme.OnSurface)
	}
	if live {
		paintScrollTrack(pt, theme, b.X+g.cross0, b.Y+g.trackStart, g.crossW, g.trackLen)
		paintScrollThumb(pt, theme, b.X+g.cross0, b.Y+g.thumbStart, g.crossW, g.thumbLen)
	}
}

// --- icon thumbnail ------------------------------------------------------

// isoThumbWorldBox is the world-space box every icon thumbnail is fit to: a unit
// footprint two units tall — the extent of the tallest built-in icon (a
// two-unit server tower). Fitting every thumbnail to the same box keeps the
// icons at a consistent relative scale, so a two-tall tower reads as taller than
// a one-tall box. A custom icon exceeding the box is clipped at the thumbnail
// edge, which is acceptable for a palette preview.
var isoThumbWorldBox = [8]iso.Vec3{
	iso.V(0, 0, 0), iso.V(1, 0, 0), iso.V(0, 1, 0), iso.V(1, 1, 0),
	iso.V(0, 0, 2), iso.V(1, 0, 2), iso.V(0, 1, 2), iso.V(1, 1, 2),
}

// projBBox is the screen bounding box of the world box projected through pr.
func projBBox(pr *iso.Projection) (minX, minY, maxX, maxY float64) {
	for i, v := range isoThumbWorldBox {
		s := pr.Project(v)
		if i == 0 {
			minX, maxX, minY, maxY = s.X, s.X, s.Y, s.Y
			continue
		}
		minX, maxX = math.Min(minX, s.X), math.Max(maxX, s.X)
		minY, maxY = math.Min(minY, s.Y), math.Max(maxY, s.Y)
	}
	return
}

// isoThumbProjection builds the 2:1 isometric projection that fits
// [isoThumbWorldBox] centred into a size×size thumbnail with pad device pixels of
// margin. The tile size is chosen so the box's larger screen dimension just fits;
// the origin then centres the projected box.
func isoThumbProjection(size, pad int) *iso.Projection {
	inner := float64(size - 2*pad)
	if inner < 1 {
		inner = 1
	}
	// Reference projection (unit tile) to measure the box's screen aspect.
	ref := iso.New(geometry.Pt(0, 0), 1, 0.5, 0.5)
	minX, minY, maxX, maxY := projBBox(ref)
	bw, bh := maxX-minX, maxY-minY
	scale := inner / bw
	if s := inner / bh; s < scale {
		scale = s
	}
	proj := iso.New(geometry.Pt(0, 0), scale, scale/2, scale/2)
	minX, minY, maxX, maxY = projBBox(proj)
	bw, bh = maxX-minX, maxY-minY
	proj.Origin = geometry.Pt(
		float64(pad)+(inner-bw)/2-minX,
		float64(pad)+(inner-bh)/2-minY,
	)
	return proj
}

// renderIsoIconThumbnail draws icon into a fresh size×size straight-alpha buffer
// filled with bg (so the thumbnail blends with its row): a primitive icon's
// shapes are depth-sorted through proj, a sprite icon's image is billboarded
// centred, and base tints the primitives the way a node's base colour would.
func renderIsoIconThumbnail(icon IsoIcon, proj *iso.Projection, size int, bg RGBA, base stdcolor.RGBA) *raster.Image {
	img := raster.New(size, size)
	fillRaster(img, bg)
	drawing := icon.Render(0, 0, base)
	if drawing.Sprite != nil {
		pad := size / 8
		blitSprite(img, Rect{X: pad, Y: pad, W: size - 2*pad, H: size - 2*pad}, drawing.Sprite)
	}
	if len(drawing.Shapes) > 0 {
		iso.NewScene(proj).Add(drawing.Shapes...).Render(img)
	}
	return img
}
