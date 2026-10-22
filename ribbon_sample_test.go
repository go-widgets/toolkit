// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-icons/iconoir"
	"github.com/go-widgets/painter"
)

// ico is an IconFunc over the named Iconoir glyph, falling back to a plain
// square when the pack has no such name (so a renamed icon never panics a test).
func ico(name string) IconFunc {
	if !iconoir.Has(name) {
		return func(p painter.Painter, r Rect, ink RGBA) { strokeRect(p, r.X+2, r.Y+2, r.W-4, r.H-4, ink) }
	}
	return SVGIcon(iconoir.Icon(name))
}

// painterOver is a pixel painter over buf (w by h, RGBA).
func painterOver(buf []byte, w, h int) painter.Painter { return painter.NewPixelPainter(buf, w, h) }

// rtPixelAt reads the colour at (x, y) of an RGBA buffer w pixels wide.
func rtPixelAt(buf []byte, w, x, y int) RGBA {
	o := (y*w + x) * 4
	return RGBA{R: buf[o], G: buf[o+1], B: buf[o+2], A: buf[o+3]}
}

// nilPainter is a small pixel surface for tests that only need somewhere to draw.
func nilPainter() painter.Painter { return painter.NewPixelPainter(make([]byte, 4*64*64), 64, 64) }

// sampleRibbon is a ribbon shaped like a slide editor's: four tabs whose groups
// exercise every control kind, with `ran` recording the ids of the commands run.
type sampleRun struct{ ids []string }

func (s *sampleRun) act(id, label, icon string) *Action {
	a := NewAction(id, label, func() { s.ids = append(s.ids, id) })
	if icon != "" {
		a.Icon = ico(icon)
	}
	return a
}

func (s *sampleRun) ribbon() *Ribbon {
	newSlide := s.act("slide.new", "New slide", "page-plus")
	newSlide.Shortcut = MustParseChord("Ctrl+N")
	paste := s.act("paste", "Paste", "clipboard")
	layout := &RibbonControl{Kind: RibbonSplit, Action: newSlide, Large: true, Tip: "Add a slide after this one",
		Items: []RibbonItem{{Action: s.act("slide.title", "Title slide", "text")}, {Action: s.act("slide.code", "Code slide", "code")}}}
	home := &RibbonTab{Label: "Home", Groups: []*RibbonGroup{
		{Label: "Slides", Icon: ico("page-plus"), Priority: 3, Controls: []*RibbonControl{layout,
			{Kind: RibbonButton, Action: s.act("slide.dup", "Duplicate", "copy")},
			{Kind: RibbonButton, Action: s.act("slide.del", "Delete", "trash")}}},
		{Label: "Edit", Icon: ico("edit"), Priority: 2, Controls: []*RibbonControl{
			{Kind: RibbonButton, Action: paste, Large: true, Tip: "Paste the clipboard"},
			{Kind: RibbonButton, Action: s.act("undo", "Undo", "undo")},
			{Kind: RibbonButton, Action: s.act("redo", "Redo", "redo")},
			{Kind: RibbonToggle, Action: s.act("wrap", "Word wrap", "text"), Checked: nil}}},
		{Label: "Theme", Icon: ico("palette"), Priority: 1, Controls: []*RibbonControl{
			{Kind: RibbonDropdown, Label: "Style", Options: []string{"default", "gaia", "uncover"}},
			{Kind: RibbonGallery, Label: "Kinds", Items: []RibbonItem{
				{Label: "Title", Icon: ico("text")}, {Label: "Code", Icon: ico("code")}, {Label: "Image", Icon: ico("media-image")}, {Label: "Quote", Icon: ico("quote")}}}}},
	}}
	insert := &RibbonTab{Label: "Insert", Groups: []*RibbonGroup{
		{Label: "Media", Icon: ico("media-image"), Priority: 1, Controls: []*RibbonControl{
			{Kind: RibbonButton, Action: s.act("image", "Image", "media-image"), Large: true},
			{Kind: RibbonButton, Action: s.act("bg", "Background", "media-image-list")}}},
	}}
	view := &RibbonTab{Label: "View", Groups: []*RibbonGroup{
		{Label: "Show", Icon: ico("eye"), Priority: 1, Controls: []*RibbonControl{
			{Kind: RibbonToggle, Action: s.act("minimap", "Minimap", "view-grid")},
			{Kind: RibbonToggle, Action: s.act("thumbs", "Thumbnails", "view-grid")}}},
	}}
	return NewRibbon(home, insert, view)
}
