// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// ClosablePane is a pane of a [Splitter] with a header: a title and a close
// button at its right end, over the Body.
//
//	pane := toolkit.NewClosablePane("Files", tree)
//	pane.CloseLabel, pane.LastLabel = "Close Files", "The last pane stays open"
//	split := toolkit.NewSplitter(false, &toolkit.SplitPane{Widget: pane, ...}, ...)
//	pane.Bind(split, 0)
//
// The button closes the pane through [Splitter.Close]; the pane comes back when
// something writes true into [Splitter.Open] (a View menu's toggle, a stored
// state). When the pane is the last one the splitter keeps, the button is
// disabled and its tooltip says why. The button is an icon from the toolkit's
// set ("xmark"), reachable with Tab, named for a screen reader by CloseLabel; a
// closed pane takes no focus, so a keyboard never lands in it.
//
// The words are the host's: the toolkit holds no language of its own, so a host
// passes CloseLabel and LastLabel in the person's.
type ClosablePane struct {
	Base
	// Title is the header's text.
	Title string
	// Body is what the pane shows under its header.
	Body Widget
	// CloseLabel is the button's accessible name and its tooltip ("Close Files").
	CloseLabel string
	// LastLabel is the tooltip while the button is disabled because this is the
	// last pane the splitter keeps open.
	LastLabel string
	// Button is the close button, exposed so that a host can restyle it.
	Button *IconButton

	s       *Splitter
	i       int
	hover   bool
	hoverMs int
}

// ClosablePaneHeaderH is the header's height and ClosablePaneTipDelayMs how long
// the pointer rests on the button before its tooltip shows, in logical pixels
// and milliseconds.
const (
	ClosablePaneHeaderH     = 26
	ClosablePaneTipDelayMs  = 450
	closablePaneButtonInset = 3
	closablePaneGlyphInset  = 4
)

// NewClosablePane builds a pane titled title around body. Bind it to the
// splitter it sits in once that exists.
func NewClosablePane(title string, body Widget) *ClosablePane {
	c := &ClosablePane{Title: title, Body: body}
	c.Button = NewIconButton("", nil)
	c.Button.Flat = true
	// the mark fills the button but for a small inset: [IconButton]'s own inset is
	// made for a 28 px tile and would leave a 6 px cross in this one
	c.Button.Glyph = func(p painter.Painter, _ Rect, ink RGBA) {
		b, in := c.Button.Bounds(), scaled(closablePaneGlyphInset)
		DrawIconoir(p, Rect{X: b.X + in, Y: b.Y + in, W: max(b.W-2*in, 1), H: max(b.H-2*in, 1)}, "xmark", ink)
	}
	return c
}

// Bind ties the pane to pane i of s: the button closes it, and follows the
// splitter's state (disabled while it is the last pane open).
func (c *ClosablePane) Bind(s *Splitter, i int) {
	c.s, c.i = s, i
	c.Button.OnClick = func() { c.s.Close(c.i) }
	for j := 0; j < s.Panes(); j++ {
		s.Open(j).Subscribe(func(bool) { c.Refresh() })
	}
	c.Refresh()
}

// Refresh brings the button's accessible name and enabled state up to date with
// the splitter. Bind does it, and every change of an open state; call it after
// changing CloseLabel or LastLabel (a language switch).
func (c *ClosablePane) Refresh() {
	c.Button.Icon = c.CloseLabel
	if c.s == nil {
		return
	}
	last := c.s.IsOpen(c.i) && !c.s.CanClose(c.i) // a closed pane is not "the last"
	c.Button.Disabled().Set(last)
	if last {
		c.Button.SetFocused(false)
	}
}

func (c *ClosablePane) headerH() int { return scaled(ClosablePaneHeaderH) }

// SetBounds places the header and the body; an empty rectangle (a closed pane)
// takes the button out of reach too.
func (c *ClosablePane) SetBounds(r Rect) {
	c.Base.SetBounds(r)
	if r.W <= 0 || r.H <= 0 {
		// a pane that goes off with the press still held on its button (the release
		// goes to whatever is under the pointer then) or the pointer on it would
		// come back drawn pressed
		c.resetButton()
		c.Button.SetBounds(Rect{})
		if c.Body != nil {
			c.Body.SetBounds(Rect{})
		}
		return
	}
	hh := min(c.headerH(), r.H)
	in := scaled(closablePaneButtonInset)
	bs := max(hh-2*in, 1)
	c.Button.SetBounds(Rect{X: r.X + r.W - bs - in, Y: r.Y + (hh-bs)/2, W: bs, H: bs})
	if c.Body != nil {
		c.Body.SetBounds(Rect{X: r.X, Y: r.Y + hh, W: r.W, H: r.H - hh})
	}
}

// resetButton releases the button and takes the pointer off it, as a release and
// a move away would: a pane that goes off with the pointer on it gets neither.
func (c *ClosablePane) resetButton() {
	// a disabled button ignores events: let it hear these two
	off := c.Button.Disabled().Get()
	c.Button.Disabled().Set(false)
	c.Button.OnEvent(Event{Kind: EventMouseUp})
	c.Button.OnEvent(Event{Kind: EventMouseMove, X: -1 << 20, Y: -1 << 20})
	c.Button.Disabled().Set(off)
	c.hover, c.hoverMs = false, 0
}

// Children are the button and the body, for the walks over the tree.
func (c *ClosablePane) Children() []Widget { return append([]Widget{c.Button}, nonNil(c.Body)...) }

func (c *ClosablePane) focusableChildren() []Widget { return c.Children() }

// A11y describes the pane as a group named by its title.
func (c *ClosablePane) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Name: c.Title, Disabled: c.Disabled().Get()}
}

// Draw paints the header (title, hairline, button), then the body, and nothing
// for a pane with no room.
func (c *ClosablePane) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	hh := min(c.headerH(), r.H)
	fillRect(p, r.X, r.Y, r.W, hh, theme.SurfaceAlt)
	fillRect(p, r.X, r.Y+hh-1, r.W, 1, theme.Border)
	pad := scaled(8)
	c.drawText(p, r.X+pad, r.Y+(hh-c.glyphHeight())/2, c.Title, theme.OnSurface)
	c.Button.Draw(p, theme)
	if c.Body != nil {
		c.Body.Draw(p, theme)
	}
}

// OnEvent routes keys to the focused part, a press on the button to the button,
// the rest to the body; a move also tracks the hover that times the tooltip.
func (c *ClosablePane) OnEvent(ev Event) {
	if c.Disabled().Get() {
		return
	}
	if routeFocusKey(c, ev) {
		return
	}
	pr := c.Bounds()
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	if ev.Kind == EventMouseMove {
		on := c.Button.HitTest(sx, sy)
		if on && !c.hover {
			c.hoverMs = 0
		}
		c.hover = on
		c.Button.OnEvent(translateEvent(ev, pr, c.Button.Bounds()))
		if c.Body != nil {
			c.Body.OnEvent(translateEvent(ev, pr, c.Body.Bounds()))
		}
		return
	}
	if ev.Kind == EventClick {
		focusClick(c, sx, sy)
	}
	if c.Button.HitTest(sx, sy) {
		c.Button.OnEvent(translateEvent(ev, pr, c.Button.Bounds()))
		return
	}
	if c.Body != nil && c.Body.HitTest(sx, sy) {
		c.Body.OnEvent(translateEvent(ev, pr, c.Body.Bounds()))
	}
}

// Tick advances the hover timer: the toolkit's [Animator] contract.
func (c *ClosablePane) Tick(dt float64) {
	if c.hover {
		c.hoverMs += int(dt * 1000)
	}
}

// Animating reports whether the tooltip is still waiting for its delay.
func (c *ClosablePane) Animating() bool { return c.hover && !c.tipShown() }

// tipShown is whether the tooltip shows: the pointer has rested on the button, or
// the button holds the keyboard (a keyboard person has no hover).
func (c *ClosablePane) tipShown() bool {
	return c.Tip() != "" && (c.hover && c.hoverMs >= ClosablePaneTipDelayMs || c.Button.Focused())
}

// Tip is the button's tooltip now: its reason while it is disabled, its name
// otherwise.
func (c *ClosablePane) Tip() string {
	if c.Button.Disabled().Get() {
		return c.LastLabel
	}
	return c.CloseLabel
}

func (c *ClosablePane) tipBox() Rect {
	b := c.Button.Bounds()
	w := c.textWidth(c.Tip()) + 2*scaled(TooltipPadX)
	h := c.glyphHeight() + 2*scaled(TooltipPadY)
	// right-aligned with the button: it sits at the pane's right edge
	return Rect{X: max(b.X+b.W-w, 0), Y: b.Y + b.H + scaled(2), W: w, H: h}
}

// PopoverOpen is whether the tooltip is showing: it is drawn over the panes that
// follow, which a pane cannot do from its own Draw.
func (c *ClosablePane) PopoverOpen() bool { return !c.Disabled().Get() && c.tipShown() }

// PopoverBounds is where the tooltip is.
func (c *ClosablePane) PopoverBounds() Rect { return c.tipBox() }

// DrawPopover paints the tooltip.
func (c *ClosablePane) DrawPopover(p painter.Painter, theme *Theme) {
	box := c.tipBox()
	fillRect(p, box.X, box.Y, box.W, box.H, theme.OnSurface)
	strokeRect(p, box.X, box.Y, box.W, box.H, theme.Border)
	c.drawText(p, box.X+scaled(TooltipPadX), box.Y+scaled(TooltipPadY), c.Tip(), theme.Background)
}

// PopoverClick lets every click through: a tooltip takes none.
func (c *ClosablePane) PopoverClick(x, y int) bool { return false }

var (
	_ PopoverOwner = (*ClosablePane)(nil)
	_ Animator     = (*ClosablePane)(nil)
)
