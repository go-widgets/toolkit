package toolkit

import "testing"

func TestNativeTableRoundTripAndFallback(t *testing.T) {
	n := NewNativeTable([]string{"a", "b"}, [][]string{{"1", "2"}, {"3", "4"}}, 1)
	c := n.control(Rect{}, Rect{})
	if c.Kind != NativeTable || len(c.Columns) != 2 || len(c.Rows) != 2 || c.Number != 1 || c.Revision == 0 {
		t.Fatalf("descriptor = %+v", c)
	}
	c.OnNumber(0)
	if n.Number().Get() != 0 || n.Fallback.(*Table).Selected().Get() != 0 {
		t.Errorf("host selection did not reach observable and fallback")
	}
	n.Fallback.(*Table).Selected().Set(1)
	if n.Number().Get() != 1 {
		t.Errorf("fallback selection did not reach observable")
	}
	r := n.Revision()
	n.SetRows([][]string{{"x", "y"}})
	if n.Revision() != r+1 || len(n.Fallback.(*Table).Rows) != 1 {
		t.Errorf("SetRows: revision %d->%d, fallback rows %d", r, n.Revision(), len(n.Fallback.(*Table).Rows))
	}
}

func TestNativeTableIsBounded(t *testing.T) {
	cols := make([]string, MaxNativeColumns+5)
	rows := make([][]string, MaxNativeRows+5)
	c := NewNativeTable(cols, rows, -1).control(Rect{}, Rect{})
	if len(c.Columns) != MaxNativeColumns || len(c.Rows) != MaxNativeRows {
		t.Errorf("carried %d cols, %d rows", len(c.Columns), len(c.Rows))
	}
}

func TestNativeTreeSelectionPathBothWays(t *testing.T) {
	n := NewNativeTree([]NativeTreeNode{
		{Title: "a", Expanded: true, Children: []NativeTreeNode{{Title: "a0"}, {Title: "a1"}}},
		{Title: "b"},
	}, "0.1")
	tv := n.Fallback.(*TreeView)
	if got := tv.Selected().Get(); got == nil || got.Label != "a1" {
		t.Fatalf("initial selection = %v", got)
	}
	n.control(Rect{}, Rect{}).OnText("1")
	if tv.Selected().Get().Label != "b" {
		t.Errorf("host path did not select b")
	}
	tv.Selected().Set(tv.Root.Children[0].Children[0])
	if n.Text().Get() != "0.0" {
		t.Errorf("fallback selection path = %q", n.Text().Get())
	}
}

func TestNativeTreeIsBounded(t *testing.T) {
	deep := NativeTreeNode{Title: "leaf"}
	for i := 0; i < MaxNativeTreeDepth+10; i++ {
		deep = NativeTreeNode{Title: "n", Children: []NativeTreeNode{deep}}
	}
	got := clampedTree([]NativeTreeNode{deep})
	d := 0
	for ns := got; len(ns) > 0; ns = ns[0].Children {
		d++
	}
	if d != MaxNativeTreeDepth {
		t.Errorf("depth = %d, want %d", d, MaxNativeTreeDepth)
	}
	wide := make([]NativeTreeNode, MaxNativeTreeNodes+7)
	if len(clampedTree(wide)) != MaxNativeTreeNodes {
		t.Errorf("wide tree not cut to %d", MaxNativeTreeNodes)
	}
	small := []NativeTreeNode{{Title: "x"}}
	if &clampedTree(small)[0] != &small[0] {
		t.Errorf("a tree that fits was copied")
	}
}

func TestNativeTabsSelectionBothWays(t *testing.T) {
	n := NewNativeTabs([]string{"one", "two", "three"}, 2)
	tb := n.Fallback.(*TabBar)
	if tb.Selected().Get() != 2 {
		t.Fatalf("initial = %d", tb.Selected().Get())
	}
	n.control(Rect{}, Rect{}).OnNumber(0)
	if tb.Selected().Get() != 0 {
		t.Errorf("host tab did not reach the bar")
	}
	tb.Selected().Set(1)
	if n.Number().Get() != 1 {
		t.Errorf("bar tab did not reach observable")
	}
	many := make([]string, MaxNativeTabs+3)
	if len(NewNativeTabs(many, 0).control(Rect{}, Rect{}).Items) != MaxNativeTabs {
		t.Errorf("tabs not bounded")
	}
}

func TestParseNativeTreePath(t *testing.T) {
	if p, ok := ParseNativeTreePath("2.0.1"); !ok || len(p) != 3 || p[0] != 2 || p[2] != 1 {
		t.Errorf("got %v %v", p, ok)
	}
	for _, bad := range []string{"", "a", "1..2", "-1", "1."} {
		if _, ok := ParseNativeTreePath(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
}
