package toolkit

import (
	"fmt"

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
	// Image is a picture the control shows, in PNG (or any format the host's
	// image loader reads). A toolbar is icons, and a control that can only
	// carry a title cannot make one.
	Image []byte
	// ImageOnly says the picture replaces the caption on screen. The caption
	// stays SET: it is what a screen reader announces, so an icon-only control
	// that dropped it would be one nobody using assistive technology could
	// name.
	ImageOnly bool
	// Menu is the context menu this control offers on a secondary click, or
	// nil for none. An item with no Pick is inert rather than absent: a menu
	// that hides what does not apply moves everything else while a person is
	// reading it, and an empty Label is a separator.
	Menu  []NativeMenuItem
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
	// NativeProgress is a determinate progress bar over [Native.Min],[Native.Max]
	// showing [Native.Number]. It is read-only (no handler).
	NativeProgress
	// NativeSpinner is an indeterminate activity indicator; [Native.On] runs or
	// stops the animation. Read-only.
	NativeSpinner
	// NativeStepper is a small increment/decrement control over
	// [Native.Min],[Native.Max] ([Native.Number] two-way via [Native.OnNumber]).
	NativeStepper
	// NativeSearch is a search field: an editable [Native.Text] two-way, activation
	// on commit (Return). Like [NativeEntry] but the platform's search styling.
	NativeSearch
	// NativeCombo is an EDITABLE drop-down: [Native.Items] to pick from AND a
	// free-text [Native.Text] two-way — a person may choose an item or type their
	// own. ([NativePopUp] is choose-only.)
	NativeCombo
	// NativeSegmented is a horizontal row of mutually exclusive segments
	// [Native.Items]; the selected segment's title is [Native.Text] two-way — the
	// native equivalent of a row of selection pills.
	NativeSegmented
	// NativeTextView is an editable MULTI-LINE text area ([Native.Text] two-way).
	NativeTextView
	// NativeLink is a hyperlink-styled button: [Native.Text] is the visible text and
	// its activation ([Native.OnActivate]) opens or follows it.
	NativeLink
	// NativeDate is a date picker; [Native.Text] is the date as an ISO-8601 string
	// (YYYY-MM-DD) two-way.
	NativeDate
	// NativeColor is a colour well; [Native.Text] is the colour as a #RRGGBB hex
	// string two-way.
	NativeColor
	// NativeList is a scrolling list of [Native.Items] a person picks a row
	// from: [Native.Number] is the chosen row as a zero-based index, -1 when
	// none is, two-way.
	//
	// It is the one kind here a drawn widget cannot stand in for cheaply. A
	// list is not a picture of rows: it scrolls with the platform's own
	// physics, it is walked by the keyboard the way every other list on the
	// machine is, and a screen reader reads it as a list rather than as a
	// rectangle. The number rides the same observable a slider uses, so a list
	// binds through a seam that already exists.
	NativeList
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

// NewNativeList makes a scrolling list of items with row selected, or -1 for
// none.
func NewNativeList(items []string, selected int) *Native {
	return &Native{Kind: NativeList, Items: items, number: mvvm.NewObservable(float64(selected))}
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

// NativeControl is the flat description of one native platform control a host
// should embed: what it is, where it is, its current value, and how to report
// the person's changes back. It is the single currency between the toolkit and a
// host backend — produced either by [WalkNative] from a tree of [Native] widgets
// or directly by a [Surface] through its NativeControls field — so the backend
// reconciles one shape regardless of how the app is built.
//
// The value fields carry meaning by kind: Text for an entry/secure/label/button/
// pop-up, On for a checkbox/radio/switch, Number for a slider. The On* callbacks
// run when the person changes the control; a host also calls OnClaim(true) once
// it has taken the region over, so a drawn fallback can stand down. Any callback
// may be nil.
// NativeMenuItem is one line of a control's context menu. An empty Label is a
// separator, and a nil Pick is a verb that does not apply right now.
type NativeMenuItem struct {
	Label string
	Pick  func()
}

type NativeControl struct {
	Kind NativeKind
	Key  string

	Rect    Rect // where the control wants to be, in surface coordinates
	Clip    Rect // the part of Rect an enclosing viewport still shows
	Visible bool // false when Clip is empty

	Text      string // entry/secure/label/button/pop-up value or title
	On        bool   // checkbox/radio/switch state
	Number    float64
	Min, Max  float64
	Items     []string
	Menu      []NativeMenuItem
	Image     []byte
	ImageOnly bool

	OnText     func(string)
	OnBool     func(bool)
	OnNumber   func(float64)
	OnActivate func()
	OnClaim    func(bool)
}

// WalkNative returns a [NativeControl] for every [Native] in the tree rooted at
// w, each with the clip an enclosing viewport imposes and its value and change
// callbacks wired to the widget's observables, in visual order. It is the
// producer a host uses for an app built as a widget tree; a [Surface]-based app
// supplies its own controls through [Surface.NativeControls] instead.
func WalkNative(w Widget) []NativeControl {
	var out []NativeControl
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
			out = append(out, nv.control(r, c))
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

// control adapts a Native widget to a [NativeControl] descriptor: its value reads
// the widget's observable, and each change callback writes back through the same
// observable (or fires the activation handler, or sets Claimed), so a host that
// speaks descriptors drives a widget-tree app with no knowledge of widgets.
func (n *Native) control(rect, clip Rect) NativeControl {
	key := n.Key
	if key == "" {
		// A widget-tree app need not name every control: its address is a stable
		// identity across frames in this retained-mode toolkit, where the same
		// *Native is laid out frame after frame. A Surface app, which rebuilds its
		// descriptors each frame, must set Key itself.
		key = fmt.Sprintf("native:%p", n)
	}
	c := NativeControl{
		Kind: n.Kind, Key: key, Menu: n.Menu,
		Image: n.Image, ImageOnly: n.ImageOnly,
		Rect: rect, Clip: clip, Visible: clip.W > 0 && clip.H > 0,
		Min: n.Min, Max: n.Max, Items: n.Items,
		OnText:     func(s string) { n.Text().Set(s) },
		OnBool:     func(b bool) { n.On().Set(b) },
		OnNumber:   func(v float64) { n.Number().Set(v) },
		OnActivate: n.Activate,
		OnClaim:    func(b bool) { n.Claimed().Set(b) },
	}
	if n.text != nil {
		c.Text = n.text.Get()
	}
	if n.on != nil {
		c.On = n.on.Get()
	}
	if n.number != nil {
		c.Number = n.number.Get()
	}
	return c
}

// childClipper is a widget that shows its children in a rectangle smaller than
// its own bounds — a viewport that reserves room for scrollbars.
type childClipper interface{ ChildClip() Rect }
