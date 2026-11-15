// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"reflect"
	"strconv"
	"strings"
)

// A11yNode is one accessible element together with WHERE it is.
//
// [CollectA11y] answers what a tree contains; this answers where each thing is,
// which is the other half every platform accessibility API asks for. A screen
// reader draws the focus ring, routes a touch or moves the pointer from this
// rectangle, so an element described without one can be read but not pointed
// at.
type A11yNode struct {
	A11yInfo

	// Rect is the element's placement in SURFACE coordinates.
	//
	// It is [Widget.Bounds] verbatim, because bounds in this toolkit are
	// already absolute rather than parent-relative — [translateEvent] converts
	// a parent-local event to child-local by `ev.X + parentRect.X -
	// childRect.X`, which only holds when both rectangles share the surface's
	// origin. Accumulating offsets during the walk, the obvious reading of
	// "placement within its parent surface", would double every position.
	Rect Rect

	// Key identifies the element across frames, so a host (the webcanvas
	// accessibility mirror) can diff two walks by it instead of rebuilding
	// nodes and losing a screen reader's position. It is the path of segments
	// from the root, "/"-separated: each segment is "@<id>" when the widget
	// implements [A11yIdentified], else "<Type>#<n>" with n the ordinal among
	// its siblings of that type. It depends only on tree structure and model
	// identity, never on bounds, scroll offset, focus or paint, and no two
	// nodes of one walk share a Key.
	Key string
}

// WalkA11y returns every meaningful element of the tree rooted at w, each with
// its bounds, in visual order.
//
// It descends through any widget that exposes its children via
// [childContainer] — the same convention [CollectRuns] uses — so a host does
// not have to keep a flat list of everything it composed, which is what
// [CollectA11y] requires and what a deeply nested layout makes impractical.
//
// Widgets reporting RolePresentation are skipped exactly as [CollectA11y]
// skips them: ARIA's role="presentation" means "look through me to the content
// inside", so a box, a scrim or a scrollbar contributes nothing to announce —
// but the walk still descends INTO it, because its children usually do.
//
// Nothing else is filtered here. Whether an unnamed or zero-area element is
// worth publishing is a decision for the platform bridge consuming this, which
// knows what its own screen reader does with one.
func WalkA11y(w Widget) []A11yNode {
	var out []A11yNode
	var walk func(x Widget, dx, dy int, key string, off bool)
	walk = func(x Widget, dx, dy int, key string, off bool) {
		if x == nil {
			return
		}
		if a, ok := x.(Accessible); ok {
			info := a.A11y()
			if off {
				info.Disabled = true
			}
			if m, ok := x.(a11yDisablesSubtree); ok && m.a11yDisablesSubtree() && info.Disabled {
				off = true
			}
			if info.Role != RolePresentation {
				r := x.Bounds()
				if ar, ok := x.(a11yRecter); ok {
					r = ar.A11yRect()
				}
				r.X, r.Y = r.X+dx, r.Y+dy
				out = append(out, A11yNode{A11yInfo: info, Rect: r, Key: key})
			}
		}
		if o, ok := x.(childOffsetter); ok {
			ox, oy := o.ChildOffset()
			dx, dy = dx+ox, dy+oy
		}
		if c, ok := x.(childContainer); ok {
			kids := c.Children()
			segs := a11ySegments(kids)
			for i, child := range kids {
				walk(child, dx, dy, key+"/"+segs[i], off)
			}
		}
	}
	walk(w, 0, 0, a11ySegment(w, 0), false)
	return out
}

// a11yDisablesSubtree marks a container whose Disabled flag also makes its
// children inert: it drops the events meant for them and focus traversal skips
// them. The walk then reports every descendant as Disabled while the container
// is, without touching the children's own flags -- so re-enabling the container
// restores each child to whatever it was, and a child that was disabled on its
// own stays so. ActionRow, SettingsGroup, SettingRow, ButtonGroup, FormField
// and Toolbar carry it, and so do Frame, Dialog, Window, Wizard, Inspector and
// Splitter (#580-#582 made them drop their children's input); TabBar and
// FolderTabs do the same through the tab nodes they build.
// TestA11yDisabledContainersAreMarked derives the set from the catalogue, so a
// container that holds children and carries neither this marker nor a reasoned
// exemption fails there.
type a11yDisablesSubtree interface{ a11yDisablesSubtree() bool }

func (*Frame) a11yDisablesSubtree() bool     { return true }
func (*Dialog) a11yDisablesSubtree() bool    { return true }
func (*Window) a11yDisablesSubtree() bool    { return true }
func (*Wizard) a11yDisablesSubtree() bool    { return true }
func (*Inspector) a11yDisablesSubtree() bool { return true }
func (*Splitter) a11yDisablesSubtree() bool  { return true }

// The rest of the containers whose OnEvent returns before forwarding while
// disabled (found by TestA11yDisabledContainersAreMarked).
func (*Border) a11yDisablesSubtree() bool            { return true }
func (*Dock) a11yDisablesSubtree() bool              { return true }
func (*ModalWindow) a11yDisablesSubtree() bool       { return true }
func (*Paned) a11yDisablesSubtree() bool             { return true }
func (*Surface) a11yDisablesSubtree() bool           { return true }
func (*ActionSheet) a11yDisablesSubtree() bool       { return true }
func (*ClosablePane) a11yDisablesSubtree() bool      { return true }
func (*DatabaseEditor) a11yDisablesSubtree() bool    { return true }
func (*FindReplace) a11yDisablesSubtree() bool       { return true }
func (*FontPicker) a11yDisablesSubtree() bool        { return true }
func (*ForgePane) a11yDisablesSubtree() bool         { return true }
func (*PullToRefresh) a11yDisablesSubtree() bool     { return true }
func (*RichEditorToolbar) a11yDisablesSubtree() bool { return true }
func (*SwipeActions) a11yDisablesSubtree() bool      { return true }
func (*Statusbar) a11yDisablesSubtree() bool         { return true }
func (*ListBox) a11yDisablesSubtree() bool           { return true }
func (*IsoDiagram) a11yDisablesSubtree() bool        { return true }
func (*Fab) a11yDisablesSubtree() bool               { return true }

func (*ActionRow) a11yDisablesSubtree() bool     { return true }
func (*SettingsGroup) a11yDisablesSubtree() bool { return true }
func (*SettingRow) a11yDisablesSubtree() bool    { return true }
func (*ButtonGroup) a11yDisablesSubtree() bool   { return true }
func (*FormField) a11yDisablesSubtree() bool     { return true }
func (*Toolbar) a11yDisablesSubtree() bool       { return true }
func (*AppDock) a11yDisablesSubtree() bool       { return true }

// a11yRecter is implemented by a widget whose accessible element should cover
// less than its bounds: [TextView] leaves its annotation column and its minimap
// out, so that the browser's text layer, which takes the pointer over its whole
// element, does not swallow a click meant for them.
type a11yRecter interface {
	A11yRect() Rect
}

// A11yIdentified is implemented by a widget that has an identity of its own,
// independent of where it sits among its siblings: a row's model key, an
// entity's ID. A container that creates or recycles children (a list that
// virtualizes its rows, a diagram's proxies) sets it from the MODEL key, never
// from the row slot, so [A11yNode.Key] survives scrolling, insertion and
// removal of other siblings. An empty string means "no identity".
type A11yIdentified interface {
	A11yID() string
}

// a11ySegments names every child of one parent, distinct from its siblings.
//
// A child with an explicit [A11yIdentified] ID is "@<id>"; the others are
// "<type>#<n>", n counting only the siblings of that type WITHOUT an explicit
// ID, so inserting or removing a sibling of another type (or one with an ID)
// never renumbers the rest. A second child claiming an ID already used gets
// "~2", "~3" ... appended in order, because two siblings must never share a
// Key (a mirror diffs by it). A nil child takes no ordinal.
func a11ySegments(kids []Widget) []string {
	segs := make([]string, len(kids))
	typeN := map[string]int{}
	idN := map[string]int{}
	for i, k := range kids {
		if k == nil {
			continue
		}
		if id := a11yID(k); id != "" {
			s := "@" + a11yEscape(id)
			idN[s]++
			if n := idN[s]; n > 1 {
				s += "~" + strconv.Itoa(n)
			}
			segs[i] = s
			continue
		}
		t := a11yTypeName(k)
		segs[i] = t + "#" + strconv.Itoa(typeN[t])
		typeN[t]++
	}
	return segs
}

// a11ySegment is the segment of a single widget with no siblings (the root),
// or at ordinal n among its own type.
func a11ySegment(w Widget, n int) string {
	if w == nil {
		return ""
	}
	if id := a11yID(w); id != "" {
		return "@" + a11yEscape(id)
	}
	return a11yTypeName(w) + "#" + strconv.Itoa(n)
}

func a11yID(w Widget) string {
	if i, ok := w.(A11yIdentified); ok {
		return i.A11yID()
	}
	return ""
}

// a11yTypeName is the widget's Go type name without package or pointer ("Button").
func a11yTypeName(w Widget) string {
	t := reflect.TypeOf(w)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}

// a11yEscape keeps "/" (the path separator) and "%" out of an explicit ID.
var a11yEscape = strings.NewReplacer("%", "%25", "/", "%2F").Replace

// childOffsetter is implemented by a widget that PAINTS its children at a
// translated origin — a scrolled viewport being the only case today.
//
// This toolkit has no viewport in the painting model: clipping exists
// (painter.Clipper) but coordinate translation does not, so ScrollView draws by
// moving its child's bounds and putting them back. Everything reading bounds
// outside Draw — this walk included — therefore sees the UNSCROLLED position.
// Measured before this existed: a button painted at y=50 under a 250px scroll
// was reported at y=300, which points a screen reader 250 pixels below the
// control it is describing.
//
// Declaring the offset is the smallest honest fix. The real one is a viewport
// that translates as well as clips, at which point this interface disappears
// and the bounds are simply correct.
type childOffsetter interface {
	ChildOffset() (dx, dy int)
}
