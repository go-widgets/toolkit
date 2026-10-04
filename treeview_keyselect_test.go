package toolkit

import (
	"fmt"
	"testing"
)

// keyTree is the shared fixture, laid out, with multi-selection on.
// Visible rows: 0 root, 1 a, 2 b, 3 b1, 4 c, 5 d (collapsed), 6 e.
func keyTree() *TreeView {
	root, _, _, _, _, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	tv.flatten()
	return tv
}

func treeKey(tv *TreeView, code string, mods ...string) {
	ev := Event{Kind: EventKeyDown, Code: code}
	for _, m := range mods {
		switch m {
		case "shift":
			ev.Shift = true
		case "ctrl":
			ev.Ctrl = true
		case "meta":
			ev.Meta = true
		}
	}
	tv.OnEvent(ev)
}

// chosen names the selected nodes in visible order, so an assertion reads as
// what a person would see.
func chosen(tv *TreeView) string {
	tv.flatten()
	var out []string
	for _, row := range tv.rows {
		if tv.IsSelected(row.node) {
			out = append(out, row.node.Label)
		}
	}
	return fmt.Sprint(out)
}

// TestShiftAndAMovementKeyExtendsATreeInsteadOfDestroyingTheSelection.
//
// ⛔ Measured against the previous code: NOTHING extended from the keyboard. A
// Shift + movement fell through to setCursorRow, which in MultiSelect mode
// calls SetSelection -- so the gesture did not merely fail to extend, it
// COLLAPSED a selection of four nodes down to one.
func TestShiftAndAMovementKeyExtendsATreeInsteadOfDestroyingTheSelection(t *testing.T) {
	tv := keyTree()
	clickLabel(tv, 1, false, false) // a
	if got := chosen(tv); got != "[a]" {
		t.Fatalf("after clicking a: %s", got)
	}
	for i, want := range []string{"[a b]", "[a b b1]", "[a b b1 c]"} {
		treeKey(tv, "ArrowDown", "shift")
		if got := chosen(tv); got != want {
			t.Fatalf("after %d Shift-ArrowDown: %s, want %s", i+1, got, want)
		}
		if a := tv.Selected().Get(); a == nil || a.Label != "a" {
			t.Fatalf("the anchor moved to %v", a)
		}
	}
	// And it SHRINKS, which the accumulating SelectRange could not.
	treeKey(tv, "ArrowUp", "shift")
	if got := chosen(tv); got != "[a b b1]" {
		t.Fatalf("Shift-ArrowUp: %s, want [a b b1]", got)
	}
	// Past the anchor the range flips sides.
	for i := 0; i < 3; i++ {
		treeKey(tv, "ArrowUp", "shift")
	}
	if got := chosen(tv); got != "[root a]" {
		t.Fatalf("crossing the anchor: %s, want [root a]", got)
	}
	if a := tv.Selected().Get(); a == nil || a.Label != "a" {
		t.Fatalf("the anchor is %v after crossing, want a", a)
	}
}

// TestShiftEndAndShiftHomeExtendATreeToo: every key rovingIndex understands,
// not only the arrows.
func TestShiftEndAndShiftHomeExtendATreeToo(t *testing.T) {
	for _, c := range []struct{ code, want string }{
		{"End", "[a b b1 c d e]"},
		{"Home", "[root a]"},
		{"PageDown", "[a b b1 c d e]"},
	} {
		tv := keyTree()
		clickLabel(tv, 1, false, false) // a
		treeKey(tv, c.code, "shift")
		if got := chosen(tv); got != c.want {
			t.Errorf("Shift-%s: %s, want %s", c.code, got, c.want)
		}
	}
	// A key it does not understand changes nothing.
	tv := keyTree()
	clickLabel(tv, 1, false, false)
	treeKey(tv, "F5", "shift")
	if got := chosen(tv); got != "[a]" {
		t.Fatalf("Shift-F5: %s, want [a] unchanged", got)
	}
}

// TestATreeExtensionWithNoAnchorTakesTheFirstVisibleNode.
func TestATreeExtensionWithNoAnchorTakesTheFirstVisibleNode(t *testing.T) {
	tv := keyTree()
	treeKey(tv, "ArrowDown", "shift")
	if got := chosen(tv); got != "[root a]" {
		t.Fatalf("%s, want [root a]", got)
	}
	if a := tv.Selected().Get(); a == nil || a.Label != "root" {
		t.Fatalf("the anchor is %v, want root", a)
	}
	// An empty tree has nothing to anchor to.
	empty := NewTreeView(nil)
	empty.MultiSelect = true
	empty.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	treeKey(empty, "ArrowDown", "shift")
	if empty.Selected().Get() != nil || len(empty.SelectedNodes()) != 0 {
		t.Fatalf("empty tree: anchor %v, selection %v",
			empty.Selected().Get(), empty.SelectedNodes())
	}
}

// TestTheMovingEndSurvivesACollapseThatHidesIt.
//
// ⛔ This is why the moving end is a NODE and not a row index: collapsing a
// subtree renumbers every row below it, so a kept index would come to name a
// different node and the extension would jump somewhere the user never went.
// When the node itself goes out of sight the extension restarts from the
// anchor, which is the only node still known to be visible.
func TestTheMovingEndSurvivesACollapseThatHidesIt(t *testing.T) {
	tv := keyTree()
	_, _, b, b1, _, _, _, _ := treeNodes(tv)
	clickLabel(tv, 1, false, false) // anchor a
	treeKey(tv, "ArrowDown", "shift")
	treeKey(tv, "ArrowDown", "shift") // end at b1
	if got := chosen(tv); got != "[a b b1]" {
		t.Fatalf("%s, want [a b b1]", got)
	}
	if tv.extendTo != b1 {
		t.Fatalf("the moving end is %v, want b1", tv.extendTo)
	}
	// Collapse b: b1 is no longer visible, so the end is gone.
	b.Expanded = false
	tv.flatten()
	treeKey(tv, "ArrowDown", "shift")
	// It restarted from the anchor (a, row 1) and stepped to b (row 2).
	if got := chosen(tv); got != "[a b]" {
		t.Fatalf("after the collapse: %s, want [a b]", got)
	}
}

// treeNodes recovers the fixture's nodes from a built view, so a test can name
// one without rebuilding the tree.
func treeNodes(tv *TreeView) (root, a, b, b1, c, d, d1, e *TreeNode) {
	root = tv.Root
	a = root.Children[0]
	b = root.Children[1]
	b1 = b.Children[0]
	c = root.Children[2]
	d = root.Children[3]
	d1 = d.Children[0]
	e = root.Children[4]
	return
}

// TestATreeExtensionScrollsToItsMovingEndInBothDirections.
//
// ⛔ The anchor is already on screen; the node being reached for is the one that
// may not be. A window three rows tall over seven rows is what makes the
// difference visible at all -- a tree that fits entirely in its window never
// scrolls, so a test laid out generously proves nothing about this.
func TestATreeExtensionScrollsToItsMovingEndInBothDirections(t *testing.T) {
	root, _, _, _, _, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 3 * 18}) // three rows of seven
	tv.flatten()
	wr := tv.windowRows()
	if wr <= 0 || wr >= len(tv.rows) {
		t.Fatalf("the window shows %d of %d rows; this test needs it smaller",
			wr, len(tv.rows))
	}

	tv.Selected().Set(tv.rows[0].node) // anchor at the top
	treeKey(tv, "End", "shift")
	if got, want := tv.ScrollRow().Get(), len(tv.rows)-wr; got != want {
		t.Fatalf("after Shift-End ScrollRow = %d, want %d so the last row shows",
			got, want)
	}
	treeKey(tv, "Home", "shift")
	if got := tv.ScrollRow().Get(); got != 0 {
		t.Fatalf("after Shift-Home ScrollRow = %d, want 0 so the first row shows", got)
	}
}

// TestATreeSelectAllTakesTheVISIBLENodes.
//
// ⛔ Not every node in the tree. A collapsed subtree is not something the person
// can see, and a verb acting on "everything selected" would act on nodes they
// were never shown -- which for a delete is the difference between what they
// meant and what they lose.
func TestATreeSelectAllTakesTheVISIBLENodes(t *testing.T) {
	all := "[root a b b1 c d e]"
	for _, mod := range []string{"ctrl", "meta"} {
		for _, code := range []string{"a", "A"} {
			tv := keyTree()
			clickLabel(tv, 2, false, false) // anchor b
			treeKey(tv, code, mod)
			if got := chosen(tv); got != all {
				t.Errorf("%s+%q: %s, want %s", mod, code, got, all)
			}
			if a := tv.Selected().Get(); a == nil || a.Label != "b" {
				t.Errorf("%s+%q moved the anchor to %v", mod, code, a)
			}
			// d1 is inside a collapsed subtree and must NOT be selected.
			_, _, _, _, _, _, d1, _ := treeNodes(tv)
			if tv.IsSelected(d1) {
				t.Errorf("%s+%q selected d1, which is hidden inside a collapsed d",
					mod, code)
			}
		}
	}
	// Plain "a" is a letter, not a command.
	tv := keyTree()
	clickLabel(tv, 1, false, false)
	treeKey(tv, "a")
	if got := chosen(tv); got != "[a]" {
		t.Fatalf("plain \"a\": %s, want [a] unchanged", got)
	}
	// Without MultiSelect the chord does nothing.
	single := keyTree()
	single.MultiSelect = false
	treeKey(single, "a", "ctrl")
	if n := len(single.SelectedNodes()); n != 0 {
		t.Fatalf("Ctrl+A on a single-selection tree selected %d nodes", n)
	}
}

// TestCommandSpaceTogglesATreeNodeWithoutActivatingIt.
func TestCommandSpaceTogglesATreeNodeWithoutActivatingIt(t *testing.T) {
	for _, code := range []string{" ", "Space"} {
		for _, mod := range []string{"ctrl", "meta"} {
			tv := keyTree()
			activated := ""
			tv.OnActivate = func(n *TreeNode) { activated = n.Label }
			clickLabel(tv, 1, false, false) // selection {a}
			treeKey(tv, "ArrowDown")        // plain move: cursor and selection on b
			tv.Selected().Set(tv.rows[4].node)
			// ⛔ Cleared HERE and not at the top: the click and the arrow above
			// both fire OnActivate, so a recorder armed before them witnesses
			// THEIR activation and says nothing about the chord under test.
			activated = ""
			treeKey(tv, code, mod)
			if got := chosen(tv); got != "[b c]" {
				t.Errorf("%s+%q: %s, want [b c]", mod, code, got)
			}
			if activated != "" {
				t.Errorf("%s+%q activated %q; it must only toggle", mod, code, activated)
			}
			treeKey(tv, code, mod) // again removes it
			if got := chosen(tv); got != "[b]" {
				t.Errorf("%s+%q twice: %s, want [b]", mod, code, got)
			}
		}
	}
	// Ctrl+Enter still activates, and a plain Space does too.
	tv := keyTree()
	activated := ""
	tv.OnActivate = func(n *TreeNode) { activated = n.Label }
	clickLabel(tv, 1, false, false)
	treeKey(tv, "Enter", "ctrl")
	if activated != "a" {
		t.Fatalf("Ctrl+Enter activated %q, want a", activated)
	}
	activated = ""
	treeKey(tv, " ")
	if activated != "a" {
		t.Fatalf("plain Space activated %q, want a", activated)
	}
}

// TestACommandClickTogglesATreeNode.
//
// ⛔ Measured against the previous code: a ⌘-click fell through to the default
// branch and acted as a PLAIN click, replacing the selection. On macOS, where
// Ctrl-click is the secondary click, that left no way at all to add a node with
// the mouse.
func TestACommandClickTogglesATreeNode(t *testing.T) {
	for _, meta := range []bool{false, true} {
		tv := keyTree()
		clickLabel(tv, 1, false, false) // {a}
		ev := Event{Kind: EventClick, X: 80, Y: 4 * 18}
		if meta {
			ev.Meta = true
		} else {
			ev.Ctrl = true
		}
		tv.OnEvent(ev)
		if got := chosen(tv); got != "[a c]" {
			t.Errorf("meta=%v click: %s, want [a c]", meta, got)
		}
	}
}

// TestAShiftClickHandsATreeTheMovingEnd: the two devices drive one selection.
func TestAShiftClickHandsATreeTheMovingEnd(t *testing.T) {
	tv := keyTree()
	clickLabel(tv, 1, false, false) // anchor a
	clickLabel(tv, 3, false, true)  // Shift-click b1
	if got := chosen(tv); got != "[a b b1]" {
		t.Fatalf("Shift-click: %s, want [a b b1]", got)
	}
	treeKey(tv, "ArrowDown", "shift")
	if got := chosen(tv); got != "[a b b1 c]" {
		t.Fatalf("Shift-arrow after Shift-click: %s, want [a b b1 c]", got)
	}
}

// TestAPlainTreeMoveForgetsTheExtension.
func TestAPlainTreeMoveForgetsTheExtension(t *testing.T) {
	tv := keyTree()
	clickLabel(tv, 1, false, false)
	treeKey(tv, "ArrowDown", "shift")
	treeKey(tv, "ArrowDown", "shift") // end at b1
	treeKey(tv, "ArrowDown")          // plain
	if got := chosen(tv); got != "[b]" {
		t.Fatalf("after a plain move: %s, want [b]", got)
	}
	treeKey(tv, "ArrowDown", "shift")
	if got := chosen(tv); got != "[b b1]" {
		t.Fatalf("the forgotten end leaked: %s, want [b b1]", got)
	}
	// A plain click forgets it too.
	clickLabel(tv, 4, false, false) // c
	treeKey(tv, "ArrowDown", "shift")
	if got := chosen(tv); got != "[c d]" {
		t.Fatalf("after a plain click: %s, want [c d]", got)
	}
	// And so does a toggle click.
	clickLabel(tv, 1, false, false) // a
	treeKey(tv, "ArrowDown", "shift")
	tv.OnEvent(Event{Kind: EventClick, X: 80, Y: 6 * 18, Meta: true}) // toggle e
	tv.Selected().Set(tv.rows[1].node)                                // anchor back on a
	treeKey(tv, "ArrowDown", "shift")
	if got := chosen(tv); got != "[a b]" {
		t.Fatalf("after a toggle click: %s, want [a b]", got)
	}
}

// TestADisabledTreeIgnoresTheMultiSelectKeys.
func TestADisabledTreeIgnoresTheMultiSelectKeys(t *testing.T) {
	tv := keyTree()
	clickLabel(tv, 1, false, false)
	tv.Disabled().Set(true)
	for _, k := range [][]string{{"ArrowDown", "shift"}, {"a", "ctrl"}, {" ", "meta"}} {
		treeKey(tv, k[0], k[1])
	}
	if got := chosen(tv); got != "[a]" {
		t.Fatalf("disabled tree: %s, want [a] untouched", got)
	}
}
