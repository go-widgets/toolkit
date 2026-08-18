// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "strconv"

// This file completes the accessibility coverage: after it, EVERY widget in the
// toolkit answers A11y(), so CollectA11y over a composed tree can never hit a
// widget that simply says nothing.
//
// Twenty-one widgets had no A11y() at all. That is worse than an imperfect role,
// because CollectA11y silently skips a widget that does not implement
// Accessible: a consumer walking the tree cannot tell "this has no semantics"
// from "this was never described", and the gap is invisible until someone reads
// the code.
//
// Layout and decoration report RolePresentation. That is not a placeholder — it
// is the correct answer, the same one ARIA's role="presentation" gives: the
// widget exists to arrange or decorate, carries no information of its own, and a
// reader should look through it to the content inside.

// --- Layout and decoration: present, but not content -------------------

// A11y reports the HBox as presentational: it arranges its children and carries
// no meaning of its own.
func (b *HBox) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// A11y reports the VBox as presentational (see HBox).
func (b *VBox) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// A11y reports the Grid as presentational. A data table is RoleGrid; this is a
// layout grid, which is a different thing wearing a similar name.
func (g *Grid) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// A11y reports the Container as presentational.
func (c *Container) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// A11y reports the Stack as presentational: only the visible child is content.
func (s *Stack) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// A11y reports the Border layout as presentational.
func (b *Border) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// A11y reports the Backdrop as presentational: it dims what is behind a modal
// and holds nothing to read.
func (b *Backdrop) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// A11y reports the Wallpaper as presentational — decoration by definition.
func (w *Wallpaper) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// A11y reports the Scrollbar as presentational. Scroll position is a property of
// the region being scrolled, not a control to announce on its own.
func (s *Scrollbar) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// --- Content ------------------------------------------------------------

// A11y reports the LoadMask as a status region: it exists precisely to say that
// work is in progress, which is something a reader must be able to announce. The
// value distinguishes a mask that is actually up from one merely composed into
// the tree — an inactive mask draws nothing and blocks nothing.
func (m *LoadMask) A11y() A11yInfo {
	v := ""
	if m.Active {
		v = "busy"
	}
	return A11yInfo{Role: RoleStatus, Name: m.Message, Value: v}
}

// A11y reports the Browser as a document named by the page it is showing.
func (b *Browser) A11y() A11yInfo { return A11yInfo{Role: RoleDocument, Name: b.CurrentURL()} }

// A11y reports the Dock as a toolbar carrying its docked entries.
func (d *Dock) A11y() A11yInfo {
	return A11yInfo{Role: RoleToolbar, Value: strconv.Itoa(len(d.docked)) + " items"}
}

// A11y reports the Gantt chart as an img carrying its task count, matching the
// other charts.
func (g *Gantt) A11y() A11yInfo {
	return A11yInfo{Role: RoleImg, Value: strconv.Itoa(len(g.Tasks)) + " tasks"}
}

// A11y reports the PagingToolbar as a toolbar whose value is the position it
// controls — the one thing a reader needs from it.
func (t *PagingToolbar) A11y() A11yInfo {
	return A11yInfo{
		Role:  RoleToolbar,
		Value: "page " + strconv.Itoa(t.Page().Get()) + " of " + strconv.Itoa(t.PageCount),
	}
}

// A11y reports the StatusIcon as a status region. Its badge, when present,
// carries the count that makes the icon worth announcing at all.
func (i *StatusIcon) A11y() A11yInfo {
	v := ""
	if i.Badge != nil {
		v = i.Badge.Text
	}
	return A11yInfo{Role: RoleStatus, Value: v}
}

// A11y reports the StatusArea as a toolbar of status icons.
func (a *StatusArea) A11y() A11yInfo {
	return A11yInfo{Role: RoleToolbar, Value: strconv.Itoa(len(a.Icons)) + " icons"}
}

// A11y reports the AgendaSidebar as navigation named by its title.
func (s *AgendaSidebar) A11y() A11yInfo {
	return A11yInfo{Role: RoleNavigation, Name: s.Title,
		Value: strconv.Itoa(len(s.Calendars)) + " calendars"}
}

// A11y reports the Window as a dialog named by its title.
func (w *Window) A11y() A11yInfo { return A11yInfo{Role: RoleDialog, Name: w.Title} }

// A11y reports the WindowDecoration as a banner named by its title: it is the
// titlebar region, not the window itself.
func (d *WindowDecoration) A11y() A11yInfo { return A11yInfo{Role: RoleBanner, Name: d.Title} }

// A11y reports the ButtonGroup as a group carrying how many buttons it holds.
func (g *ButtonGroup) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Value: strconv.Itoa(len(g.Buttons)) + " buttons"}
}

// A11y reports the Thumbnail as an img named by its Alt text, falling back to
// the caption it already shows.
func (t *Thumbnail) A11y() A11yInfo {
	name := t.Alt
	if name == "" {
		name = t.Label
	}
	return A11yInfo{Role: RoleImg, Name: name}
}
