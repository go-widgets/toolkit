// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"

	"github.com/go-widgets/painter"
)

// ComboBox is an editable, type-to-filter dropdown: a single-line text field
// the user can type into, backed by a popover list of Options filtered to
// those containing the typed Text. It sits between Entry (a free-text field
// with no list) and DropDown (a closed list with no typing): the field accepts
// free text AND offers the matching options for one-click / Enter selection.
//
// Like DropDown and DatePicker, the popover appears just below the field. The
// widget renders that list itself when Open (so it works standalone), while a
// host that composites overlays on a separate surface can instead read Open +
// PopoverBounds and draw the list there.
type ComboBox struct {
	Base
	focusState
	Options []string
	// Text is the current field value — either free text the user typed or an
	// option they selected. Filtered() narrows Options against it.
	Text string
	// Placeholder is shown in the muted tone when Text is empty (a hint such as
	// "search…" or "pick a colour").
	Placeholder string
	// Open reports whether the filtered popover list is showing.
	Open bool
	// OnChange fires whenever Text changes (a keystroke edit or a selection).
	OnChange func(string)
	// OnSelect fires when an option is chosen (click or Enter).
	OnSelect func(string)

	// highlight is the ABSOLUTE index (into the full Filtered() list) the
	// keyboard highlight sits on. ArrowDown/ArrowUp move it and Enter selects
	// it. It is reset to 0 whenever the filter changes, so it always starts on
	// the first match. highlightRow clamps it against the current match count on
	// use.
	highlight int

	// popScroll is the index (into Filtered()) of the option painted at the top
	// of the open popover window — the persistent scroll offset that makes
	// matches beyond PopoverMaxRows reachable. The wheel (EventScroll) shifts
	// it, arrow-key navigation scrolls it to keep the highlight visible, and the
	// click hit-test maps through it. Reads clamp on the fly (clampedPopScroll),
	// so a value left stale after the filter shrank is harmless.
	popScroll int
}

// NewComboBox builds a ComboBox with the given options and an empty field.
func NewComboBox(options []string) *ComboBox {
	return &ComboBox{Options: options}
}

// Filtered returns the Options whose lowercased text contains the lowercased
// Text. When Text is empty every option matches, so the full list is returned.
func (c *ComboBox) Filtered() []string {
	if c.Text == "" {
		return c.Options
	}
	needle := strings.ToLower(c.Text)
	var out []string
	for _, o := range c.Options {
		if strings.Contains(strings.ToLower(o), needle) {
			out = append(out, o)
		}
	}
	return out
}

// visible is the window of filtered rows the popover actually shows:
// [popScroll, popScroll+PopoverMaxRows) clamped to the filtered list. Both
// PopoverBounds (for its height) and Draw (for the rows it paints) go through
// here, and the click hit-test indexes it directly, so the window lives in
// exactly one place. When the filter fits in PopoverMaxRows this is the whole
// list starting at 0, byte-identical to before scrolling existed.
func (c *ComboBox) visible() []string {
	f := c.Filtered()
	start := c.clampedPopScroll()
	end := start + PopoverMaxRows
	if end > len(f) {
		end = len(f)
	}
	return f[start:end]
}

// maxPopScroll is the highest popScroll that still fills the popover window:
// len(Filtered()) - PopoverMaxRows, floored at 0.
func (c *ComboBox) maxPopScroll() int {
	m := len(c.Filtered()) - PopoverMaxRows
	if m < 0 {
		m = 0
	}
	return m
}

// clampedPopScroll returns popScroll clamped to [0, maxPopScroll] without
// mutating the field, so a value left stale after the filter shrank never paints
// or hit-tests outside the valid window.
func (c *ComboBox) clampedPopScroll() int {
	s := c.popScroll
	if s < 0 {
		s = 0
	}
	if m := c.maxPopScroll(); s > m {
		s = m
	}
	return s
}

// scrollPopover shifts popScroll by delta rows, clamped and written back.
func (c *ComboBox) scrollPopover(delta int) {
	c.popScroll += delta
	c.popScroll = c.clampedPopScroll()
}

// scrollHighlightIntoView nudges popScroll so the highlighted match stays within
// the PopoverMaxRows-tall window, so arrow-key navigation follows the highlight
// past the fold.
func (c *ComboBox) scrollHighlightIntoView() {
	hi := c.highlightRow()
	if hi < c.popScroll {
		c.popScroll = hi
	} else if hi >= c.popScroll+PopoverMaxRows {
		c.popScroll = hi - PopoverMaxRows + 1
	}
	c.popScroll = c.clampedPopScroll()
}

// comboRowH is the LOGICAL height of one option row in the popover, matching the
// 18px row used by DropDown.PopoverBounds. Consumed through [ComboBox.rowH] so
// the picker cells scale with HiDPI and touch Density.
const comboRowH = 18

// rowH is the device-pixel height of one popover option row: comboRowH routed
// through scaled, so PopoverBounds, Draw and the click hit-test all agree on one
// scaled cell height. At compact/1x it is the literal comboRowH (byte-identical).
func (c *ComboBox) rowH() int { return scaled(comboRowH) }

// comboPadX is the left inset for the field text (and the placeholder/caret),
// scaled at use; comboChevronInset is the chevron's distance from the right
// edge. Both are LOGICAL bases, identity at compact/1x.
const (
	comboPadX         = 4
	comboChevronInset = 10
)

// HitRect is the ComboBox field's tap/toggle target: Bounds clamped up to the
// touch minimum on each axis and centred, byte-identical to Bounds at
// [DensityCompact].
func (c *ComboBox) HitRect() Rect { return hitRectFor(c.Bounds()) }

// PopoverBounds returns the Rect the filtered list occupies below the field:
// same X and W as the field, height proportional to the visible option count.
// Mirrors DropDown.PopoverBounds / DatePicker.PopoverBounds.
func (c *ComboBox) PopoverBounds() Rect {
	r := c.Bounds()
	return Rect{X: r.X, Y: r.Y + r.H, W: r.W, H: len(c.visible()) * c.rowH()}
}

// Draw paints the field (rounded border, Text or muted Placeholder, an
// end-of-text caret, and a right-side chevron) and, when Open, the filtered
// options as a plain list in PopoverBounds.
func (c *ComboBox) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	border := theme.Border
	if c.Open {
		border = theme.Accent
	}
	// A disabled combobox mutes its face, border, text/placeholder, caret and
	// chevron; only taken when Disabled so the enabled draw is byte-identical.
	face, ink, placeholderInk := theme.Surface, theme.OnSurface, theme.SurfaceAlt
	if c.Disabled {
		face, border, ink, placeholderInk = mutedFace(theme), mutedInk(theme), mutedInk(theme), mutedInk(theme)
	}
	fillRoundRect(p, r.X, r.Y, r.W, r.H, buttonRadius, face)
	strokeRoundRect(p, r.X, r.Y, r.W, r.H, buttonRadius, border)
	textY := r.Y + (r.H-c.glyphHeight())/2
	pad := scaled(comboPadX)
	if c.Text == "" && c.Placeholder != "" {
		c.drawText(p, r.X+pad, textY, c.Placeholder, placeholderInk)
	} else {
		c.drawText(p, r.X+pad, textY, c.Text, ink)
	}
	// Caret at the end of the typed text, measured through the effective font so
	// it lands correctly under a proportional / CJK face (mirrors Entry).
	cx := r.X + pad + c.textWidth(c.Text)
	fillRect(p, cx, textY-1, 1, c.glyphHeight()+2, ink)
	// ▼ chevron on the right edge, drawn exactly like DropDown's.
	cvx := r.X + r.W - scaled(comboChevronInset)
	cvy := r.Y + r.H/2
	for t := 0; t < 4; t++ {
		fillRect(p, cvx-t, cvy+2-t, 1+2*t, 1, ink)
	}
	if c.Open {
		pb := c.PopoverBounds()
		fillRect(p, pb.X, pb.Y, pb.W, pb.H, theme.Surface)
		hi := c.highlightRow()
		start := c.clampedPopScroll()
		rowH := c.rowH()
		for i, opt := range c.visible() {
			rowY := pb.Y + i*rowH
			if start+i == hi {
				// Highlighted row gets a SurfaceAlt band; the border is stroked
				// afterwards so the popover outline stays crisp over it.
				fillRect(p, pb.X, rowY, pb.W, rowH, theme.SurfaceAlt)
			}
			oy := rowY + (rowH-c.glyphHeight())/2
			c.drawText(p, pb.X+scaled(comboPadX), oy, opt, theme.OnSurface)
		}
		strokeRect(p, pb.X, pb.Y, pb.W, pb.H, theme.Border)
	}
	c.drawFocusRing(p, theme, r)
}

// OnEvent drives the type-to-filter behaviour: printable characters and
// Backspace edit Text (firing OnChange) and open the popover; a click on the
// field toggles Open; a click on a listed option selects it; Enter selects the
// first filtered option.
func (c *ComboBox) OnEvent(ev Event) {
	if c.Disabled {
		return
	}
	r := c.Bounds()
	switch ev.Kind {
	case EventClick:
		if c.Open {
			pb := c.PopoverBounds()
			lx := ev.X - (pb.X - r.X)
			ly := ev.Y - (pb.Y - r.Y)
			if lx >= 0 && lx < pb.W && ly >= 0 && ly < pb.H {
				// ly < pb.H == len(visible)*rowH guarantees the row index
				// is in range, so no extra bounds check is needed.
				c.selectOption(c.visible()[ly/c.rowH()])
				return
			}
		}
		// A click anywhere else (the field itself) toggles the popover.
		c.Open = !c.Open
	case EventKeyDown:
		switch ev.Code {
		case "Backspace":
			runes := []rune(c.Text)
			if len(runes) > 0 {
				c.Text = string(runes[:len(runes)-1])
				c.Open = true
				c.highlight = 0
				c.popScroll = 0
				if c.OnChange != nil {
					c.OnChange(c.Text)
				}
			}
		case "ArrowDown":
			// Move the highlight down through the FULL filtered list (opening the
			// popover if it was closed), clamped to the last match, and scroll
			// the window to keep it visible past the fold.
			c.Open = true
			if n := len(c.Filtered()); c.highlight < n-1 {
				c.highlight++
			}
			c.scrollHighlightIntoView()
		case "ArrowUp":
			if c.highlight > 0 {
				c.highlight--
			}
			c.scrollHighlightIntoView()
		case "Enter":
			// Commit the highlighted match (which defaults to the first, so a
			// plain type-then-Enter still picks the top option).
			if f := c.Filtered(); len(f) > 0 {
				c.selectOption(f[c.highlightRow()])
			}
		case "Escape":
			c.Open = false
		}
	case EventScroll:
		// Wheel over the open popover shifts its scroll window so matches beyond
		// PopoverMaxRows are reachable; ignored while closed.
		if c.Open {
			c.scrollPopover(ev.Delta)
		}
	case EventChar:
		if ev.Code == "" {
			return
		}
		c.Text += ev.Code
		c.Open = true
		c.highlight = 0
		c.popScroll = 0
		if c.OnChange != nil {
			c.OnChange(c.Text)
		}
	}
}

// highlightRow returns the (absolute) highlight index clamped to the current
// filtered-match count, so a filter that shrank the list can't leave it out of
// range.
func (c *ComboBox) highlightRow() int {
	n := len(c.Filtered())
	if n == 0 {
		return 0
	}
	if c.highlight >= n {
		return n - 1
	}
	if c.highlight < 0 {
		return 0
	}
	return c.highlight
}

// selectOption commits s as the field value, closes the popover, and fires
// OnSelect then OnChange (both guarded for nil).
func (c *ComboBox) selectOption(s string) {
	c.Text = s
	c.Open = false
	if c.OnSelect != nil {
		c.OnSelect(s)
	}
	if c.OnChange != nil {
		c.OnChange(s)
	}
}
