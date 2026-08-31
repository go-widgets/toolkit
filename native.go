package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Native is a rectangle the toolkit LAYS OUT but does not paint: a host that can
// embed real platform controls places one of its own where a Native sits, so a
// password box is the operating system's secure field and a slider is the
// system slider — behaviour a drawn imitation cannot carry (secure text entry,
// the exact focus ring, native accessibility). The toolkit still does the
// layout; the host does the control.
//
// # Why this is not the old Foreign
//
// An earlier Foreign region carried only a rect, a key and an opaque payload,
// and it was retired for want of a consumer that a rect could serve. A real
// control is not a picture: the person edits it, so its value must flow BACK to
// the model, and it survives across frames, so the host must find the same
// control again rather than rebuild it. Native answers both — a [mvvm.Observable]
// per value for two-way binding, and a stable [Native.Key] the host diffs on —
// which is what makes it worth its place where Foreign was not.
//
// # It degrades
//
// Where no host claims it — Linux and the browser today, or before the first
// frame — a Native paints its [Native.Fallback] (a portable drawn widget) so the
// tree stays usable everywhere. Give the fallback the SAME observables and the
// two renderings stay in step. With no fallback it paints nothing, which is the
// right answer for a region something else is about to fill.
type Native struct {
	Base

	// Kind is which control this is. It is fixed at construction.
	Kind NativeKind

	// Key is the caller's stable identity for this control. A host keys its
	// live native object on it, so the control is found again — and its focus
	// and selection kept — across the frames it is laid out in. Two Natives that
	// are "the same control" over time must share a Key; two different controls
	// must not.
	Key string

	// Items are the entries of a [NativePopUp]. Ignored for other kinds.
	Items []string

	// Min and Max bound a [NativeSlider]. Ignored for other kinds.
	Min, Max float64

	text   *mvvm.Observable[string]
	on     *mvvm.Observable[bool]
	number *mvvm.Observable[float64]

	onActivate func()
	claimed    *mvvm.Observable[bool]

	// Fallback renders in place while no host has claimed this region.
	Fallback Widget
}

// NativeKind is which platform control a [Native] stands for.
type NativeKind int

const (
	// NativeButton is a momentary push button; its title is [Native.Text] and
	// its click is the activation handler.
	NativeButton NativeKind = iota
	// NativeLabel is static, non-editable text ([Native.Text]).
	NativeLabel
	// NativeEntry is an editable single-line text field: [Native.Text] two-way,
	// activation on commit (Return).
	NativeEntry
	// NativeSecureEntry is an editable field whose glyphs are bullets and whose
	// contents the platform fills without the process seeing the keystrokes —
	// the control a drawn toolkit must not imitate for a password.
	NativeSecureEntry
	// NativeCheckbox is a labelled on/off control ([Native.On] two-way,
	// [Native.Text] label).
	NativeCheckbox
	// NativeRadio is one of a group of mutually exclusive controls; a host
	// groups the Natives that share a container.
	NativeRadio
	// NativeSwitch is a sliding on/off control ([Native.On] two-way).
	NativeSwitch
	// NativeSlider is a continuous control over [Native.Min],[Native.Max]
	// ([Native.Number] two-way).
	NativeSlider
	// NativePopUp is a drop-down of [Native.Items]; the selected title is
	// [Native.Text].
	NativePopUp
)

// NewNativeButton makes a push button. onClick runs when it is activated.
func NewNativeButton(title string, onClick func()) *Native {
	return &Native{Kind: NativeButton, text: mvvm.NewObservable(title), onActivate: onClick}
}

// NewNativeLabel makes a static text label.
func NewNativeLabel(text string) *Native {
	return &Native{Kind: NativeLabel, text: mvvm.NewObservable(text)}
}

// NewNativeEntry makes an editable text field.
func NewNativeEntry(text string) *Native {
	return &Native{Kind: NativeEntry, text: mvvm.NewObservable(text)}
}

// NewNativeSecureEntry makes a secure (bulleted) text field.
func NewNativeSecureEntry(text string) *Native {
	return &Native{Kind: NativeSecureEntry, text: mvvm.NewObservable(text)}
}

// NewNativeCheckbox makes a labelled checkbox.
func NewNativeCheckbox(title string, on bool) *Native {
	return &Native{Kind: NativeCheckbox, text: mvvm.NewObservable(title), on: mvvm.NewObservable(on)}
}

// NewNativeRadio makes a radio button. Natives that share a container form a
// group.
func NewNativeRadio(title string, on bool) *Native {
	return &Native{Kind: NativeRadio, text: mvvm.NewObservable(title), on: mvvm.NewObservable(on)}
}

// NewNativeSwitch makes an on/off switch.
func NewNativeSwitch(on bool) *Native {
	return &Native{Kind: NativeSwitch, on: mvvm.NewObservable(on)}
}

// NewNativeSlider makes a slider over [min,max] positioned at value.
func NewNativeSlider(min, max, value float64) *Native {
	return &Native{Kind: NativeSlider, Min: min, Max: max, number: mvvm.NewObservable(value)}
}

// NewNativePopUp makes a drop-down of items with the given selection.
func NewNativePopUp(items []string, selected string) *Native {
	return &Native{Kind: NativePopUp, Items: items, text: mvvm.NewObservable(selected)}
}

// Text is the two-way string value of a text control, and the title of a
// button, label, checkbox or radio. It is created on first use.
func (n *Native) Text() *mvvm.Observable[string] {
	if n.text == nil {
		n.text = mvvm.NewObservable("")
	}
	return n.text
}

// On is the two-way on/off state of a checkbox, radio or switch.
func (n *Native) On() *mvvm.Observable[bool] {
	if n.on == nil {
		n.on = mvvm.NewObservable(false)
	}
	return n.on
}

// Number is the two-way value of a slider.
func (n *Native) Number() *mvvm.Observable[float64] {
	if n.number == nil {
		n.number = mvvm.NewObservable[float64](0)
	}
	return n.number
}

// SetOnActivate sets the handler run when the control is activated (a button
// clicked, a text field committed with Return, a selection made). A host calls
// [Native.Activate] to fire it.
func (n *Native) SetOnActivate(fn func()) { n.onActivate = fn }

// Activate runs the activation handler if one is set. A host calls it when its
// live control fires its primary action.
func (n *Native) Activate() {
	if n.onActivate != nil {
		n.onActivate()
	}
}

// Claimed reports whether a host has taken over rendering this region. It is a
// cross-boundary observable: the host sets it true when it places its control
// and false when it takes it away, and the toolkit reads it to know whether to
// paint the fallback. Created on first use.
func (n *Native) Claimed() *mvvm.Observable[bool] {
	if n.claimed == nil {
		n.claimed = mvvm.NewObservable(false)
	}
	return n.claimed
}

// Draw paints the fallback while unclaimed; once a host has claimed the region,
// its own control is above the canvas and the toolkit paints nothing.
func (n *Native) Draw(p painter.Painter, theme *Theme) {
	if n.Claimed().Get() || n.Fallback == nil {
		return
	}
	n.Fallback.SetBounds(n.Bounds())
	n.Fallback.Draw(p, theme)
}

// OnEvent forwards to the fallback while unclaimed; a claimed region's events
// belong to the host's control.
func (n *Native) OnEvent(ev Event) {
	if n.Claimed().Get() || n.Fallback == nil {
		return
	}
	n.Fallback.OnEvent(ev)
}

// Children exposes the fallback while unclaimed, so a11y and layout descend into
// it. Once claimed, there are none: the host's control carries its own
// accessibility, and exposing the fallback too would double it.
func (n *Native) Children() []Widget {
	if n.Claimed().Get() {
		return nil
	}
	return nonNil(n.Fallback)
}

// A11y reports [RolePresentation]: this region's accessibility is the fallback's
// (walked as a child) while unclaimed, and the host control's own once claimed —
// never the Native's.
func (n *Native) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// NativePlacement is one Native and where it ended up, in surface coordinates.
// The host reads Control for the kind, configuration and value observables it
// binds to, and Rect/Clip/Visible for where to put the live control.
type NativePlacement struct {
	Control *Native
	Rect    Rect // where the control wants to be
	Clip    Rect // the part of Rect an enclosing viewport still shows
	Visible bool // false when Clip is empty
}

// WalkNative returns every [Native] in the tree rooted at w, each with its
// placement and the clip an enclosing viewport imposes, in visual order. It is
// the walk a host runs each frame to reconcile its live controls with the
// layout.
func WalkNative(w Widget) []NativePlacement {
	var out []NativePlacement
	var walk func(x Widget, dx, dy int, clip Rect, clipped bool)
	walk = func(x Widget, dx, dy int, clip Rect, clipped bool) {
		if x == nil {
			return
		}
		if nv, ok := x.(*Native); ok {
			r := nv.Bounds()
			r.X, r.Y = r.X+dx, r.Y+dy
			c := r
			if clipped {
				c = intersectRect(r, clip)
			}
			out = append(out, NativePlacement{
				Control: nv,
				Rect:    r,
				Clip:    c,
				Visible: c.W > 0 && c.H > 0,
			})
		}
		if o, ok := x.(childOffsetter); ok {
			vp := x.Bounds()
			if cc, ok := x.(childClipper); ok {
				vp = cc.ChildClip()
			}
			vp.X, vp.Y = vp.X+dx, vp.Y+dy
			if clipped {
				clip = intersectRect(clip, vp)
			} else {
				clip, clipped = vp, true
			}
			ox, oy := o.ChildOffset()
			dx, dy = dx+ox, dy+oy
		}
		if c, ok := x.(childContainer); ok {
			for _, child := range c.Children() {
				walk(child, dx, dy, clip, clipped)
			}
		}
	}
	walk(w, 0, 0, Rect{}, false)
	return out
}

// childClipper is a widget that shows its children in a rectangle smaller than
// its own bounds — a viewport that reserves room for scrollbars.
type childClipper interface{ ChildClip() Rect }
