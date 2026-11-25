package toolkit

import (
	"strconv"
	"strings"

	"github.com/go-widgets/mvvm"
)

// Bounds a [NativeControl] descriptor carries. A descriptor crosses into a host
// (cgo, a Win32 message loop, a GTK model) so what it holds is capped where it is
// produced, not trusted to every backend: a model that grew without limit would
// otherwise become the host's allocation.
const (
	// MaxNativeRows caps the rows of a [NativeTable].
	MaxNativeRows = 100000
	// MaxNativeColumns caps the columns of a [NativeTable].
	MaxNativeColumns = 64
	// MaxNativeTreeDepth caps the nesting of a [NativeTree]; deeper nodes are
	// dropped, which also bounds a host's recursion.
	MaxNativeTreeDepth = 32
	// MaxNativeTreeNodes caps the total nodes of a [NativeTree].
	MaxNativeTreeNodes = 100000
	// MaxNativeTabs caps the tabs of a [NativeTabs].
	MaxNativeTabs = 64
)

// NativeTreeNode is one entry of a [NativeTree]: a title, whether its children
// are shown when the tree is first built, and the children. Data, not a widget —
// it crosses to the host as plain values.
type NativeTreeNode struct {
	Title    string
	Expanded bool
	Children []NativeTreeNode
}

// NewNativeTable makes a table: columns with headers, rows of strings, and the
// chosen row as a zero-based index in [Native.Number] (-1 for none), two-way.
// Its fallback is the painted [Table].
//
// A host hosts the table in virtual (owner-data) mode: it asks for the cells it
// shows, so a table of thousands of rows is never rebuilt per frame. It re-reads
// the rows only when [Native.Revision] moves, so change the data through
// [Native.SetRows], which bumps it.
func NewNativeTable(columns []string, rows [][]string, selected int) *Native {
	n := &Native{Kind: NativeTable, Columns: columns, Rows: rows, number: mvvm.NewObservable(float64(selected))}
	cols := make([]TableColumn, len(columns))
	for i, c := range columns {
		cols[i] = TableColumn{Title: c}
	}
	t := NewTable(cols, rows)
	t.Selected().Set(selected)
	bridgeIntNumber(t.Selected(), n.number)
	n.Fallback = t
	n.revision = 1
	return n
}

// SetRows replaces the rows of a [NativeTable] and bumps the revision a host
// watches, so exactly one re-read follows.
func (n *Native) SetRows(rows [][]string) {
	n.Rows = rows
	n.revision++
	if t, ok := n.Fallback.(*Table); ok {
		t.Rows = rows
	}
}

// NewNativeTree makes a tree of nodes. The chosen node is its path in
// [Native.Text]: the child indices from the top level joined by dots ("2.0.1"),
// empty for none, two-way. Its fallback is the painted [TreeView].
func NewNativeTree(nodes []NativeTreeNode, selectedPath string) *Native {
	n := &Native{Kind: NativeTree, Tree: nodes, text: mvvm.NewObservable(selectedPath), revision: 1}
	root := &TreeNode{Expanded: true}
	paths := map[*TreeNode]string{}
	var build func(parent *TreeNode, src []NativeTreeNode, prefix string, depth int)
	build = func(parent *TreeNode, src []NativeTreeNode, prefix string, depth int) {
		if depth >= MaxNativeTreeDepth {
			return
		}
		for i, s := range src {
			p := strconv.Itoa(i)
			if prefix != "" {
				p = prefix + "." + p
			}
			c := &TreeNode{Label: s.Title, Expanded: s.Expanded, Data: p}
			paths[c] = p
			parent.Children = append(parent.Children, c)
			build(c, s.Children, p, depth+1)
		}
	}
	build(root, nodes, "", 0)
	tv := NewTreeView(root)
	tv.HideRoot = true
	byPath := map[string]*TreeNode{}
	for c, p := range paths {
		byPath[p] = c
	}
	if c := byPath[selectedPath]; c != nil {
		tv.Selected().Set(c)
	}
	tv.Selected().Subscribe(func(c *TreeNode) { n.text.Set(paths[c]) })
	n.text.Subscribe(func(p string) {
		if c := byPath[p]; c != tv.Selected().Get() {
			tv.Selected().Set(c)
		}
	})
	n.Fallback = tv
	return n
}

// NewNativeTabs makes a row of tabs with the given titles and the chosen tab in
// [Native.Number], two-way. It is the STRIP only: the toolkit lays the content
// out underneath, in the region below the Native, and shows the page the number
// names. Its fallback is the painted [TabBar].
func NewNativeTabs(titles []string, selected int) *Native {
	n := &Native{Kind: NativeTabs, Items: titles, number: mvvm.NewObservable(float64(selected)), revision: 1}
	items := make([]TabItem, len(titles))
	for i, t := range titles {
		items[i] = TabItem{Label: t}
	}
	tb := NewTabBar(items, selected)
	bridgeIntNumber(tb.Selected(), n.number)
	n.Fallback = tb
	return n
}

// Revision counts the changes made to a table's rows, a tree's nodes or a tab
// row's titles. A host compares it instead of the data, which is what lets a
// 10,000-row table be reconciled each frame at the price of one integer compare.
// A [Surface] that rebuilds descriptors itself must move it when its data moves.
func (n *Native) Revision() uint64 { return n.revision }

// bridgeIntNumber keeps an int selection and a float64 selection equal both
// ways. Observables do not notify on an unchanged value, so it settles.
func bridgeIntNumber(i *mvvm.Observable[int], f *mvvm.Observable[float64]) {
	i.Subscribe(func(v int) {
		if float64(v) != f.Get() {
			f.Set(float64(v))
		}
	})
	f.Subscribe(func(v float64) {
		if int(v) != i.Get() {
			i.Set(int(v))
		}
	})
}

// clampedTable returns the columns and rows a descriptor may carry: at most
// [MaxNativeColumns] and [MaxNativeRows]. It slices, never copies, so a 10,000-row
// table costs no allocation per descriptor.
func clampedTable(cols []string, rows [][]string) ([]string, [][]string) {
	if len(cols) > MaxNativeColumns {
		cols = cols[:MaxNativeColumns]
	}
	if len(rows) > MaxNativeRows {
		rows = rows[:MaxNativeRows]
	}
	return cols, rows
}

// clampedTree returns nodes cut to [MaxNativeTreeDepth] levels and
// [MaxNativeTreeNodes] nodes in all. It returns its argument untouched when it
// already fits, which is the common case and costs one walk, no allocation.
func clampedTree(nodes []NativeTreeNode) []NativeTreeNode {
	budget := MaxNativeTreeNodes
	var fits func(ns []NativeTreeNode, depth int) bool
	fits = func(ns []NativeTreeNode, depth int) bool {
		if depth >= MaxNativeTreeDepth {
			return len(ns) == 0
		}
		budget -= len(ns)
		if budget < 0 {
			return false
		}
		for i := range ns {
			if !fits(ns[i].Children, depth+1) {
				return false
			}
		}
		return true
	}
	if fits(nodes, 0) {
		return nodes
	}
	budget = MaxNativeTreeNodes
	var cut func(ns []NativeTreeNode, depth int) []NativeTreeNode
	cut = func(ns []NativeTreeNode, depth int) []NativeTreeNode {
		if depth >= MaxNativeTreeDepth || budget <= 0 {
			return nil
		}
		if len(ns) > budget {
			ns = ns[:budget]
		}
		budget -= len(ns)
		out := make([]NativeTreeNode, len(ns))
		for i, n := range ns {
			out[i] = NativeTreeNode{Title: n.Title, Expanded: n.Expanded, Children: cut(n.Children, depth+1)}
		}
		return out
	}
	return cut(nodes, 0)
}

// ParseNativeTreePath turns a tree path ("2.0.1") into its indices, or false for
// the empty path or a malformed one. Hosts use it to find the node to select.
func ParseNativeTreePath(p string) ([]int, bool) {
	parts := strings.Split(p, ".")
	if len(parts) > MaxNativeTreeDepth {
		return nil, false
	}
	out := make([]int, len(parts))
	for i, s := range parts {
		v, err := strconv.Atoi(s)
		if err != nil || v < 0 {
			return nil, false
		}
		out[i] = v
	}
	return out, true
}
