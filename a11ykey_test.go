// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"
	"testing"
)

// idBox is a container whose children and identities the test controls.
type idBox struct {
	Base
	kids []Widget
}

func (b *idBox) Children() []Widget { return b.kids }

// idLeaf is an accessible leaf with an optional explicit identity.
type idLeaf struct {
	Base
	id string
}

func (l *idLeaf) A11y() A11yInfo { return A11yInfo{Role: RoleText, Name: l.id} }
func (l *idLeaf) A11yID() string { return l.id }

func walkKeys(nodes []A11yNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Key
	}
	return out
}

func sameKeys(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestA11yKeyTable(t *testing.T) {
	tests := []struct {
		name string
		root func() Widget
		want []string
	}{
		{"root leaf", func() Widget { return NewButton("a", nil) }, []string{"Button#0"}},
		{"explicit root id", func() Widget { return &idLeaf{id: "x/y%"} }, []string{"@x%2Fy%25"}},
		{"siblings of one type count up", func() Widget {
			c := NewContainer(nil)
			c.AddWidget(NewButton("a", nil)).AddWidget(NewButton("b", nil))
			return c
		}, []string{"Container#0/Button#0", "Container#0/Button#1"}},
		{"types count apart", func() Widget {
			c := NewContainer(nil)
			c.AddWidget(NewButton("a", nil)).AddWidget(NewLabel("l")).AddWidget(NewButton("b", nil))
			return c
		}, []string{"Container#0/Button#0", "Container#0/Label#0", "Container#0/Button#1"}},
		{"nil child takes no ordinal", func() Widget {
			c := NewContainer(nil)
			c.AddWidget(nil).AddWidget(NewButton("a", nil))
			return c
		}, []string{"Container#0/Button#0"}},
		{"nested presentation is in the path", func() Widget {
			outer := NewContainer(nil)
			inner := NewContainer(nil)
			inner.AddWidget(NewLabel("l"))
			outer.AddWidget(inner)
			return outer
		}, []string{"Container#0/Container#0/Label#0"}},
		{"explicit ids do not take type ordinals", func() Widget {
			return &idBox{kids: []Widget{&idLeaf{id: "a"}, NewButton("b", nil), &idLeaf{id: "c"}, NewButton("d", nil)}}
		}, []string{"idBox#0/@a", "idBox#0/Button#0", "idBox#0/@c", "idBox#0/Button#1"}},
		{"duplicate explicit ids are suffixed", func() Widget {
			return &idBox{kids: []Widget{&idLeaf{id: "a"}, &idLeaf{id: "a"}, &idLeaf{id: "a"}}}
		}, []string{"idBox#0/@a", "idBox#0/@a~2", "idBox#0/@a~3"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := walkKeys(WalkA11y(tc.root())); !sameKeys(got, tc.want) {
				t.Fatalf("keys = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestA11yKeyRootNilSegment(t *testing.T) {
	if s := a11ySegment(nil, 0); s != "" {
		t.Fatalf("a11ySegment(nil) = %q, want empty", s)
	}
}

// Keys are unique within a walk, whatever the tree.
func TestA11yKeysDistinct(t *testing.T) {
	c := NewContainer(nil)
	for i := 0; i < 5; i++ {
		c.AddWidget(NewButton("same", nil))
		c.AddWidget(&idLeaf{id: "dup"})
	}
	seen := map[string]bool{}
	for _, n := range WalkA11y(c) {
		if seen[n.Key] {
			t.Fatalf("key %q repeated", n.Key)
		}
		seen[n.Key] = true
	}
	if len(seen) != 10 {
		t.Fatalf("%d keys, want 10", len(seen))
	}
}

// Inserting a sibling of a different type must not renumber the others.
func TestA11yKeyStableWhenOtherTypeInserted(t *testing.T) {
	a, b := NewButton("a", nil), NewButton("b", nil)
	before := NewContainer(nil)
	before.AddWidget(a).AddWidget(b)
	after := NewContainer(nil)
	after.AddWidget(NewLabel("new")).AddWidget(a).AddWidget(&idLeaf{id: "x"}).AddWidget(b)

	want := walkKeys(WalkA11y(before))
	var got []string
	for _, n := range WalkA11y(after) {
		if n.Role == RoleButton {
			got = append(got, n.Key)
		}
	}
	if !sameKeys(got, want) {
		t.Fatalf("button keys after insert = %q, want %q", got, want)
	}
}

// Repainting and focusing change no key.
func TestA11yKeyStableAcrossRepaintAndFocus(t *testing.T) {
	c := NewContainer(nil)
	tv := NewTextView("x")
	c.AddWidget(NewButton("a", nil)).AddWidget(tv)
	c.SetBounds(Rect{W: 100, H: 100})
	first := walkKeys(WalkA11y(c))
	tv.SetFocused(true)
	c.SetBounds(Rect{X: 5, Y: 7, W: 200, H: 50})
	tv.SetBounds(Rect{X: 9, Y: 9, W: 10, H: 10})
	if got := walkKeys(WalkA11y(c)); !sameKeys(got, first) {
		t.Fatalf("keys changed: %q -> %q", first, got)
	}
}

// Scrolling a ScrollView moves bounds only.
func TestA11yKeyStableAcrossScrollView(t *testing.T) {
	box := NewContainer(nil)
	for i := 0; i < 20; i++ {
		b := NewButton("b"+strconv.Itoa(i), nil)
		b.SetBounds(Rect{X: 0, Y: i * 30, W: 80, H: 24})
		box.AddWidget(b)
	}
	sv := NewScrollView(box)
	sv.SetBounds(Rect{W: 100, H: 100})
	box.SetBounds(Rect{W: 100, H: 600})
	before := WalkA11y(sv)
	sv.OffsetY().Set(120)
	after := WalkA11y(sv)
	if _, dy := sv.ChildOffset(); dy == 0 {
		t.Fatal("test did not scroll")
	}
	if !sameKeys(walkKeys(before), walkKeys(after)) {
		t.Fatalf("keys changed on scroll: %q -> %q", walkKeys(before), walkKeys(after))
	}
	if before[3].Rect == after[3].Rect {
		t.Fatal("rect did not move: the scroll was not observed")
	}
}

// A scrolled ListBox keeps every item's key, and an item keeps its key when a
// row is inserted before it.
func TestA11yKeyListBoxItems(t *testing.T) {
	mk := func(items ...string) *ListBox {
		l := NewSectionedListBox(ListSection{Title: "T", Items: items})
		l.SetBounds(Rect{W: 100, H: 60})
		return l
	}
	many := make([]string, 30)
	for i := range many {
		many[i] = "item" + strconv.Itoa(i)
	}
	l := mk(many...)
	before := WalkA11y(l)
	l.ScrollTo(20)
	after := WalkA11y(l)
	if !sameKeys(walkKeys(before), walkKeys(after)) {
		t.Fatalf("keys changed on scroll")
	}
	if before[5].Rect == after[5].Rect {
		t.Fatal("rect did not move: the scroll was not observed")
	}

	keyOf := func(nodes []A11yNode, name string) string {
		for _, n := range nodes {
			if n.Name == name {
				return n.Key
			}
		}
		t.Fatalf("no node named %q", name)
		return ""
	}
	inserted := WalkA11y(mk(append([]string{"fresh"}, many...)...))
	for _, name := range []string{"item0", "item7", "item29"} {
		if a, b := keyOf(before, name), keyOf(inserted, name); a != b {
			t.Errorf("%s key %q -> %q after an insert before it", name, a, b)
		}
	}
	if keyOf(before, "T") != "ListBox#0/@header:T" {
		t.Errorf("header key = %q", keyOf(before, "T"))
	}
}

// Duplicate row texts still get distinct keys.
func TestA11yKeyListBoxDuplicateRows(t *testing.T) {
	l := NewSectionedListBox(ListSection{Items: []string{"a", "a", "a"}})
	l.SetBounds(Rect{W: 100, H: 100})
	want := []string{"ListBox#0", "ListBox#0/@item:a", "ListBox#0/@item:a~2", "ListBox#0/@item:a~3"}
	if got := walkKeys(WalkA11y(l)); !sameKeys(got, want) {
		t.Fatalf("keys = %q, want %q", got, want)
	}
}

func TestA11yKeyIsoDiagramProxies(t *testing.T) {
	p := &isoProxy{id: "node:n1"}
	if p.A11yID() != "node:n1" {
		t.Fatalf("A11yID = %q", p.A11yID())
	}
}

func TestTextA11yFields(t *testing.T) {
	t.Run("entry", func(t *testing.T) {
		e := NewEntry("hi")
		e.AccessibleName = "Email"
		want := A11yInfo{Role: RoleTextbox, Name: "Email", Value: "hi"}
		if got := e.A11y(); got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
		e.SetFocused(true)
		e.Disabled().Set(true)
		want.Focused, want.Disabled = true, true
		if got := e.A11y(); got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})
	t.Run("textview", func(t *testing.T) {
		v := NewTextView("a\nb")
		v.AccessibleName = "Notes"
		want := A11yInfo{Role: RoleTextbox, Name: "Notes", Value: "a\nb", Multiline: true}
		if got := v.A11y(); got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
		v.HasFocus().Set(true)
		v.Disabled().Set(true)
		want.Focused, want.Disabled = true, true
		if got := v.A11y(); got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})
	t.Run("codeeditor", func(t *testing.T) {
		c := NewCodeEditor("x := 1")
		c.Language = "go"
		if got := c.A11y(); got.Name != "go" || !got.Multiline || got.Value != "x := 1" {
			t.Fatalf("language fallback: %+v", got)
		}
		c.AccessibleName = "Go cell 3"
		c.SetFocused(true)
		if got := c.A11y(); got.Name != "Go cell 3" || !got.Focused {
			t.Fatalf("accessible name: %+v", got)
		}
	})
}
