// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// Children for every widget that holds other widgets.
//
// [CollectRuns] and [WalkA11y] descend a tree by asking each widget for its
// children through the childContainer interface. Only Container, HBox and VBox
// answered, so every generic walk stopped at the first ScrollView, Grid,
// Overlay, Popover, Paned, Notebook — anything else. The measured consequence
// was severe and silent: an application's entire scrollable content was
// INVISIBLE to a screen reader, because the accessibility bridges are built on
// exactly this walk, and a demo without a ScrollView looked perfectly complete.
//
// Each method returns the children in VISUAL order, since that is the order a
// reader announces them in and the order a text selection runs through. Nil
// slots are kept out rather than passed on: a container built incrementally has
// empty ones, and no walker should have to defend against them.
//
// TestEveryContainerExposesItsChildren parses this package and fails if a
// widget holds a Widget — directly or through a helper struct — and does not
// appear here, so the next container cannot quietly go missing.

func nonNil(ws ...Widget) []Widget {
	out := make([]Widget, 0, len(ws))
	for _, w := range ws {
		if w != nil {
			out = append(out, w)
		}
	}
	return out
}

// Children yields the section bodies, in section order.
func (a *Accordion) Children() []Widget {
	out := make([]Widget, 0, len(a.Sections))
	for _, s := range a.Sections {
		if s.Body != nil {
			out = append(out, s.Body)
		}
	}
	return out
}

// Children yields the row's leading then trailing widget.
func (a *ActionRow) Children() []Widget { return nonNil(a.Prefix, a.Suffix) }

// Children yields the five regions in reading order: the edges clockwise from
// the top, then the centre.
func (b *Border) Children() []Widget {
	return nonNil(b.North, b.East, b.South, b.West, b.Center)
}

// Children yields every slide, including those currently off-screen: a walker
// asking for structure wants the whole model, and a reader can say which one is
// showing from the widget's own state.
func (c *Carousel) Children() []Widget { return nonNil(c.Slides...) }

// Children yields the dialog's content.
func (d *Dialog) Children() []Widget { return nonNil(d.Content) }

// Children yields the docked bars and then the body.
func (d *Dock) Children() []Widget {
	out := make([]Widget, 0, len(d.docked)+1)
	for _, it := range d.docked {
		if it.w != nil {
			out = append(out, it.w)
		}
	}
	if d.body != nil {
		out = append(out, d.body)
	}
	return out
}

// Children yields the expander's content, whether or not it is expanded — see
// Carousel for why hidden content still belongs in the structure.
func (e *Expander) Children() []Widget { return nonNil(e.Content) }

// Children yields the field's control.
func (f *FormField) Children() []Widget { return nonNil(f.Child) }

// Children yields the framed widget.
func (f *Frame) Children() []Widget { return nonNil(f.child) }

// Children yields the cells in insertion order.
func (g *Grid) Children() []Widget {
	out := make([]Widget, 0, len(g.children))
	for _, c := range g.children {
		if c.w != nil {
			out = append(out, c.w)
		}
	}
	return out
}

// Children yields the leading widgets then the trailing ones, which is how the
// bar reads left to right.
func (h *HeaderBar) Children() []Widget {
	out := make([]Widget, 0, len(h.Start)+len(h.End))
	out = append(out, nonNil(h.Start...)...)
	out = append(out, nonNil(h.End...)...)
	return out
}

// Children exposes the material's child so generic walks (accessibility,
// material collection) descend into it.
func (m *Material) Children() []Widget { return nonNil(m.Child) }

// Children yields every tab's page.
func (n *Notebook) Children() []Widget {
	out := make([]Widget, 0, len(n.Tabs))
	for _, t := range n.Tabs {
		if t.Page != nil {
			out = append(out, t.Page)
		}
	}
	return out
}

// Children yields the content first and then the layers, bottom to top, which
// is the order they are painted in.
func (o *Overlay) Children() []Widget {
	out := make([]Widget, 0, len(o.Layers)+1)
	if o.Content != nil {
		out = append(out, o.Content)
	}
	return append(out, nonNil(o.Layers...)...)
}

// Children yields the two panes in order.
func (p *Paned) Children() []Widget { return nonNil(p.First, p.Second) }

// Children yields the popover's content.
func (p *Popover) Children() []Widget { return nonNil(p.Child) }

// Children yields the scrolled content.
func (s *ScrollView) Children() []Widget { return nonNil(s.Child) }

// ChildOffset reports how far the scrolled content is PAINTED from where its
// bounds say it is — see childOffsetter for why that difference exists at all.
//
// The rubber band counts. Draw shifts the content by the offset PLUS the
// overscroll, so during a bounce a reader given the offset alone would be told
// the content sits at the bound while it is visibly past it. Accessibility
// bridges read this between frames, and the whole reason this method exists is
// that they must not be lied to about where a control is.
func (s *ScrollView) ChildOffset() (int, int) {
	return -(s.OffsetX().Get() + s.overscrollX), -(s.OffsetY().Get() + s.overscrollY)
}

// Children yields the window's body.
func (w *Window) Children() []Widget { return nonNil(w.Body) }

// Children yields every step's body.
func (w *Wizard) Children() []Widget {
	out := make([]Widget, 0, len(w.Steps))
	for _, s := range w.Steps {
		if s.Body != nil {
			out = append(out, s.Body)
		}
	}
	return out
}
