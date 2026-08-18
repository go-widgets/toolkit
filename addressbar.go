package toolkit

import (
	"strings"

	"github.com/go-widgets/painter"
)

// AddressBar is an editable URL / address field: a rounded [Backdrop] ground
// with an optional leading status-icon slot, an optional trailing bookmark-toggle
// slot, and head-clipped text that keeps a long URL's tail (its path) visible.
//
// It shows [AddressBar.Text] when unfocused and its own edit buffer while
// focused; a click focuses it (seeding the buffer from Text), typing edits the
// buffer, Enter commits (fires OnCommit with the trimmed buffer, then defocuses),
// and the container defocuses it with [AddressBar.Blur] when a click lands
// elsewhere. A click on the bookmark slot flips [AddressBar.Bookmarked] and fires
// OnBookmarkToggle instead of focusing. [AddressBar.CopySelectAll] copies the
// whole value to the toolkit clipboard and flags a select-all highlight, the same
// "no selection → copy the whole value" model as [Entry].
//
// It is a standalone widget usable in any toolbar; the toolkit [Browser] composes
// one for its address field.
type AddressBar struct {
	Base

	// Text is shown when the field is not focused — the host sets it to the
	// current URL. While focused the field shows its own edit buffer instead.
	Text string

	// LeadingIcon, when set, paints a status glyph at the LEFT of the field (e.g.
	// an SSL padlock whose look the host varies by certificate state); the text
	// indents to its right. Same painter seam as the toolbar icons. Nil → no
	// leading slot (text starts at the normal inset).
	LeadingIcon func(p painter.Painter, r Rect, ink RGBA)

	// BookmarkIcon, when set, paints a toggle glyph at the RIGHT of the field
	// (e.g. a star, filled when on). It takes the current Bookmarked state so the
	// host draws the on/off variant. Clicking the slot flips Bookmarked and fires
	// OnBookmarkToggle. Nil → no bookmark slot.
	BookmarkIcon     func(p painter.Painter, r Rect, ink RGBA, on bool)
	Bookmarked       bool
	OnBookmarkToggle func(on bool)

	// OnCommit fires on Enter with the trimmed edit buffer, but only when it is
	// non-empty (the host normalises + navigates). An empty buffer just defocuses.
	OnCommit func(text string)

	// OnChange fires whenever the field's observable state changes (focus, edit,
	// copy) so a host can redraw or a binder can push into an Observable. Nil is
	// safe.
	OnChange func()

	// Radius is the corner radius and TextPad the left/right text inset, both in
	// device pixels so a HiDPI host scales them. Zero Radius = square corners;
	// zero TextPad = flush text.
	Radius  int
	TextPad int

	focused bool
	buf     string
	// copied marks that the value was just copied (CopySelectAll), so the field
	// paints a select-all highlight. Cleared on any edit or focus change.
	copied bool
}

// A11y reports the field as a textbox carrying its current value (the edit
// buffer while focused, else Text).
func (a *AddressBar) A11y() A11yInfo { return A11yInfo{Role: RoleTextbox, Value: a.Value()} }

// Focused reports whether the field currently holds keyboard focus.
func (a *AddressBar) Focused() bool { return a.focused }

// Value returns the text the field shows: the edit buffer while focused, else Text.
func (a *AddressBar) Value() string {
	if a.focused {
		return a.buf
	}
	return a.Text
}

// Copied reports whether a select-all copy highlight is currently shown.
func (a *AddressBar) Copied() bool { return a.copied }

// Blur defocuses the field and clears the copy highlight. The container calls it
// when a click lands outside the field. It does not fire OnChange (the container
// owns its own redraw).
func (a *AddressBar) Blur() {
	a.focused = false
	a.copied = false
}

// dismissCopied clears only the copy highlight (any click in the container
// dismisses it), without touching focus.
func (a *AddressBar) dismissCopied() { a.copied = false }

func (a *AddressBar) fireChange() {
	if a.OnChange != nil {
		a.OnChange()
	}
}

// zones splits the field rect into an optional leading status-icon square (when
// LeadingIcon is set), an optional trailing bookmark square (when BookmarkIcon is
// set), and the text zone between them.
func (a *AddressBar) zones(r Rect) (lead, text, star Rect) {
	textX, textR := r.X, r.X+r.W
	if a.LeadingIcon != nil {
		lead = Rect{X: r.X, Y: r.Y, W: r.H, H: r.H}
		textX = r.X + r.H
	}
	if a.BookmarkIcon != nil {
		star = Rect{X: r.X + r.W - r.H, Y: r.Y, W: r.H, H: r.H}
		textR = r.X + r.W - r.H
	}
	if textR < textX {
		textR = textX
	}
	text = Rect{X: textX, Y: r.Y, W: textR - textX, H: r.H}
	return
}

// Draw paints the field: a rounded Backdrop ground (Surface fill + a border that
// turns Accent while focused), the optional icon slots, the head-clipped text
// (edit buffer when focused, else Text), a caret when focused, and a select-all
// highlight after a copy.
func (a *AddressBar) Draw(p painter.Painter, theme *Theme) {
	r := a.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	ring := theme.Border
	if a.focused {
		ring = theme.Accent
	}
	// The ground is a composed rounded Backdrop (fill + border), the same
	// canonical ground the rest of the toolkit's pills and fields use.
	ground := Backdrop{Fill: theme.Surface, Radius: a.Radius, Stroke: ring, StrokeWidth: strokeWidth()}
	ground.SetBounds(r)
	ground.Draw(p, theme)
	lead, tz, star := a.zones(r)
	if a.LeadingIcon != nil {
		a.LeadingIcon(p, lead, theme.OnSurface)
	}
	if a.BookmarkIcon != nil {
		a.BookmarkIcon(p, star, theme.OnSurface, a.Bookmarked)
	}
	text := a.Text
	if a.focused {
		text = a.buf
	}
	innerX := tz.X + a.TextPad
	avail := tz.W - 2*a.TextPad
	shown := text
	if a.textWidth(shown) > avail {
		shown = clipHeadToWidth(a.EffectiveFont(), shown, avail)
	}
	ty := r.Y + (r.H-a.glyphHeight())/2
	// After a copy, paint a select-all highlight behind the text so the user sees
	// exactly what went to the clipboard.
	if a.copied && shown != "" {
		hl := blendRGBA(theme.Accent, theme.Surface, 0.62)
		fillRect(p, innerX, ty, a.textWidth(shown), a.glyphHeight(), hl)
	}
	a.drawText(p, innerX, ty, shown, theme.OnSurface)
	if a.focused {
		caretW := a.glyphHeight() / 12
		if caretW < 1 {
			caretW = 1
		}
		fillRect(p, innerX+a.textWidth(shown), ty, caretW, a.glyphHeight(), theme.OnSurface)
	}
}

// OnEvent routes a click (local coordinates, relative to the field's bounds
// origin), a typed rune or an edit key. A click on the bookmark slot toggles it;
// any other click focuses the field and seeds the edit buffer from Text. While
// focused, a rune appends, Backspace deletes and Enter commits.
func (a *AddressBar) OnEvent(ev Event) {
	switch ev.Kind {
	case EventClick:
		r := a.Bounds()
		ax, ay := r.X+ev.X, r.Y+ev.Y
		if _, _, star := a.zones(r); a.BookmarkIcon != nil && star.Contains(ax, ay) {
			a.focused = false
			a.toggleBookmark()
			a.fireChange()
			return
		}
		a.focused = true
		a.buf = a.Text
		a.copied = false
		a.fireChange()
	case EventChar:
		if !a.focused || ev.Code == "" {
			return
		}
		a.copied = false // an edit dismisses the copied-highlight
		a.buf += ev.Code
		a.fireChange()
	case EventKeyDown:
		if !a.focused {
			return
		}
		switch ev.Code {
		case "Backspace":
			runes := []rune(a.buf)
			if len(runes) == 0 {
				return
			}
			a.copied = false // an edit dismisses the copied-highlight
			a.buf = string(runes[:len(runes)-1])
			a.fireChange()
		case "Enter":
			a.commit()
		}
	}
}

// commit fires OnCommit with the trimmed buffer (non-empty only), then defocuses.
// An empty buffer is a no-op beyond defocusing (it fires OnChange so the host
// redraws).
func (a *AddressBar) commit() {
	raw := strings.TrimSpace(a.buf)
	a.focused = false
	a.copied = false
	if raw == "" {
		a.fireChange()
		return
	}
	if a.OnCommit != nil {
		a.OnCommit(raw)
	} else {
		a.fireChange()
	}
}

// CopySelectAll copies the field's value to the toolkit-wide clipboard and flags
// a select-all highlight, reporting the text and whether anything was copied. It
// is a no-op returning ("", false) when the field is not focused or is empty — so
// a host can try it first and fall back to another copy action.
func (a *AddressBar) CopySelectAll() (string, bool) {
	if !a.focused {
		return "", false
	}
	txt := a.Value()
	if txt == "" {
		return "", false
	}
	SetClipboardText(txt)
	a.copied = true
	a.fireChange()
	return txt, true
}

// toggleBookmark flips Bookmarked + fires OnBookmarkToggle.
func (a *AddressBar) toggleBookmark() {
	a.Bookmarked = !a.Bookmarked
	if a.OnBookmarkToggle != nil {
		a.OnBookmarkToggle(a.Bookmarked)
	}
}
