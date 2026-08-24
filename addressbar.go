package toolkit

import (
	"strings"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// AddressBar is an editable URL / address field: a rounded [Backdrop] ground
// with an optional leading status-icon slot, an optional trailing bookmark-toggle
// slot, and head-clipped text that keeps a long URL's tail (its path) visible.
//
// Its mutable state is MVVM-only: the value, edit buffer, focus, bookmark and
// copy flags are unexported [mvvm.Observable]s exposed solely through accessors
// ([AddressBar.URL], [AddressBar.Editing], [AddressBar.Focused],
// [AddressBar.Bookmarked], [AddressBar.Copied]). There is no settable state
// field, so a host can only change the field through its Observables (bind them
// on a view model, or Set them) — never by imperative assignment. Enter runs the
// [AddressBar.Commit] command (a host binds it to normalise + navigate).
//
// It shows URL when unfocused and Editing while focused; a click focuses it
// (seeding Editing from URL), a click on the bookmark slot flips Bookmarked
// instead, and the container defocuses it with [AddressBar.Blur] when a click
// lands elsewhere. Reusable in any toolbar; the toolkit [Browser] composes one
// and shares its URL + Bookmarked Observables so neither side copies the other.
type AddressBar struct {
	Base

	// LeadingIcon / BookmarkIcon are optional host-drawn glyph slots at the left
	// and right (appearance config, not reactive state — painter funcs are not
	// Observable). BookmarkIcon takes the Bookmarked state so the host draws the
	// on/off variant. Same painter seam as the toolbar icons.
	LeadingIcon  func(p painter.Painter, r Rect, ink RGBA)
	BookmarkIcon func(p painter.Painter, r Rect, ink RGBA, on bool)

	// Commit is executed on Enter when the trimmed edit buffer is non-empty; a
	// host binds it to normalise + navigate to [AddressBar.Editing]. Nil → Enter
	// just defocuses. Escape cancels instead: it reverts the edit buffer to URL
	// and defocuses WITHOUT running Commit.
	Commit *mvvm.Command

	// Placeholder is prompt text shown in a muted ink when the field is empty and
	// not being edited (unfocused with no value). It is never part of the value:
	// [AddressBar.Value] and the a11y report ignore it, and Commit never sees it.
	// Empty (the default) shows nothing. Appearance config, not reactive state.
	Placeholder string

	// Radius is the corner radius and TextPad the left/right text inset, both in
	// device pixels so a HiDPI host scales them. Config, not state.
	Radius  int
	TextPad int

	url        *mvvm.Observable[string]
	editing    *mvvm.Observable[string]
	focused    *mvvm.Observable[bool]
	bookmarked *mvvm.Observable[bool]
	copied     *mvvm.Observable[bool]
}

// URL is the address shown when the field is not focused (a host binds it to the
// current page URL). Set it through the Observable; never assign a field.
func (a *AddressBar) URL() *mvvm.Observable[string] {
	if a.url == nil {
		a.url = mvvm.NewObservable("")
	}
	return a.url
}

// Editing is the in-progress edit buffer, shown while the field is focused.
func (a *AddressBar) Editing() *mvvm.Observable[string] {
	if a.editing == nil {
		a.editing = mvvm.NewObservable("")
	}
	return a.editing
}

// Focused reports (and drives) keyboard focus. A view model can observe it; the
// widget flips it on click / Blur.
func (a *AddressBar) Focused() *mvvm.Observable[bool] {
	if a.focused == nil {
		a.focused = mvvm.NewObservable(false)
	}
	return a.focused
}

// Bookmarked is the bookmark toggle state. A host binds it to its bookmark store;
// clicking the bookmark slot flips it. Subscribe to it instead of a callback.
func (a *AddressBar) Bookmarked() *mvvm.Observable[bool] {
	if a.bookmarked == nil {
		a.bookmarked = mvvm.NewObservable(false)
	}
	return a.bookmarked
}

// Copied reports whether a select-all copy highlight is currently shown.
func (a *AddressBar) Copied() *mvvm.Observable[bool] {
	if a.copied == nil {
		a.copied = mvvm.NewObservable(false)
	}
	return a.copied
}

// A11y reports the field as a textbox carrying its current value (the edit
// buffer while focused, else the URL).
func (a *AddressBar) A11y() A11yInfo { return A11yInfo{Role: RoleTextbox, Value: a.Value()} }

// Value returns the text the field shows: the edit buffer while focused, else URL.
func (a *AddressBar) Value() string {
	if a.Focused().Get() {
		return a.Editing().Get()
	}
	return a.URL().Get()
}

// Blur defocuses the field and clears the copy highlight. The container calls it
// when a click lands outside the field.
func (a *AddressBar) Blur() {
	a.Focused().Set(false)
	a.Copied().Set(false)
}

// dismissCopied clears only the copy highlight (any click in the container
// dismisses it), without touching focus.
func (a *AddressBar) dismissCopied() { a.Copied().Set(false) }

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
// (edit buffer when focused, else URL), a caret when focused, and a select-all
// highlight after a copy.
func (a *AddressBar) Draw(p painter.Painter, theme *Theme) {
	r := a.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	focused := a.Focused().Get()
	ring := theme.Border
	if focused {
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
		a.BookmarkIcon(p, star, theme.OnSurface, a.Bookmarked().Get())
	}
	text := a.URL().Get()
	if focused {
		text = a.Editing().Get()
	}
	// When the field is empty and not being edited, show the Placeholder in a
	// muted ink instead. It is a visual prompt only — never the value — so it is
	// painted here and nowhere else (Value / A11y ignore it).
	ink := theme.OnSurface
	if text == "" && !focused && a.Placeholder != "" {
		text = a.Placeholder
		ink = theme.SurfaceAlt
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
	if a.Copied().Get() && shown != "" {
		hl := blendRGBA(theme.Accent, theme.Surface, 0.62)
		fillRect(p, innerX, ty, a.textWidth(shown), a.glyphHeight(), hl)
	}
	a.drawText(p, innerX, ty, shown, ink)
	if focused {
		caretW := a.glyphHeight() / 12
		if caretW < 1 {
			caretW = 1
		}
		fillRect(p, innerX+a.textWidth(shown), ty, caretW, a.glyphHeight(), theme.OnSurface)
	}
}

// OnEvent routes a click (local coordinates, relative to the field's bounds
// origin), a typed rune or an edit key. A click on the bookmark slot toggles it;
// any other click focuses the field and seeds the edit buffer from URL. While
// focused, a rune appends, Backspace deletes, Enter commits and Escape cancels
// (reverts the edit buffer to URL and defocuses without committing).
func (a *AddressBar) OnEvent(ev Event) {
	switch ev.Kind {
	case EventClick:
		r := a.Bounds()
		ax, ay := r.X+ev.X, r.Y+ev.Y
		if _, _, star := a.zones(r); a.BookmarkIcon != nil && star.Contains(ax, ay) {
			a.Focused().Set(false)
			a.toggleBookmark()
			return
		}
		a.Focused().Set(true)
		a.Editing().Set(a.URL().Get())
		a.Copied().Set(false)
	case EventChar:
		if !a.Focused().Get() || ev.Code == "" {
			return
		}
		a.Copied().Set(false) // an edit dismisses the copied-highlight
		a.Editing().Set(a.Editing().Get() + ev.Code)
	case EventKeyDown:
		if !a.Focused().Get() {
			return
		}
		switch ev.Code {
		case "Backspace":
			runes := []rune(a.Editing().Get())
			if len(runes) == 0 {
				return
			}
			a.Copied().Set(false) // an edit dismisses the copied-highlight
			a.Editing().Set(string(runes[:len(runes)-1]))
		case "Enter":
			a.commit()
		case "Escape":
			a.cancel()
		}
	}
}

// commit runs the Commit command with the trimmed buffer (non-empty only), then
// defocuses. An empty buffer just defocuses. The host's command reads Editing.
func (a *AddressBar) commit() {
	raw := strings.TrimSpace(a.Editing().Get())
	a.Focused().Set(false)
	a.Copied().Set(false)
	if raw != "" && a.Commit != nil {
		a.Commit.Execute()
	}
}

// cancel abandons the in-progress edit: it reverts the edit buffer to the last
// committed URL, then defocuses and clears the copy highlight WITHOUT running
// Commit. Escape triggers it, so a stray edit never navigates.
func (a *AddressBar) cancel() {
	a.Editing().Set(a.URL().Get())
	a.Focused().Set(false)
	a.Copied().Set(false)
}

// CopySelectAll copies the field's value to the toolkit-wide clipboard and flags
// a select-all highlight, reporting the text and whether anything was copied. It
// is a no-op returning ("", false) when the field is not focused or is empty.
func (a *AddressBar) CopySelectAll() (string, bool) {
	if !a.Focused().Get() {
		return "", false
	}
	txt := a.Value()
	if txt == "" {
		return "", false
	}
	SetClipboardText(txt)
	a.Copied().Set(true)
	return txt, true
}

// toggleBookmark flips the Bookmarked Observable; a host subscribed to it reacts.
func (a *AddressBar) toggleBookmark() { a.Bookmarked().Set(!a.Bookmarked().Get()) }
