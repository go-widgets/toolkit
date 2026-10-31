// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// FileEntry is one item a [FileTree] shows: a slash-separated path relative to
// the tree's root ("themes/dark.css"), whether it is a folder, and a file's size
// in bytes. A folder is also implied by any path under it, so an empty folder is
// the only one that needs an entry of its own.
type FileEntry struct {
	Path string
	Dir  bool
	Size int64
}

// FileAction is what a [FileRequest] asks the host to do.
type FileAction int

const (
	// FileOpen opens the file at Path (a click, Enter).
	FileOpen FileAction = iota
	// FileCreate creates an empty file at Path.
	FileCreate
	// FileCreateDir creates a folder at Path.
	FileCreateDir
	// FileRename renames Path to To (both full paths; To may be in another
	// folder when the new name holds a slash).
	FileRename
	// FileDelete deletes Path, and a folder's contents with it. It is only
	// asked once the person has confirmed.
	FileDelete
	// FileMove moves Path into another folder: To is the new full path.
	FileMove
	// FileImport takes files dropped from outside into the folder To ("" is
	// the root). Names are the drop payload's items; the host holds the bytes.
	FileImport
)

// FileRequest is a change the FileTree asks its host for. The tree never
// changes its own entries: the host applies the request to its store and sets
// [FileTree.Entries], so the store stays the one source of truth.
type FileRequest struct {
	Action   FileAction
	Path, To string
	Names    []string
}

// FileTree is a file explorer over a flat list of paths: VS Code's Explorer
// as a widget. Folders come first, then files, each by name; folders expand and
// collapse, and keep that state across updates of the list.
//
// Interaction follows the WAI-ARIA APG treeview pattern (arrows, Home/End,
// Enter, type-ahead, * to expand the siblings) on top of [TreeView], plus the
// Explorer's verbs: F2 renames in place, Delete asks for confirmation, a
// secondary click (Context) opens a menu with New File, New Folder, Rename,
// Delete and the host's own items, a row dragged onto a folder moves there,
// and files dropped from outside (EventDragStart/Move/Drop) are offered to the
// folder under the pointer, which is outlined meanwhile.
//
// Every change is a [FileRequest] handed to OnRequest; names are validated
// first with the Explorer's messages. State crossing the boundary is MVVM:
// [FileTree.Entries] in, [FileTree.Selected] both ways.
type FileTree struct {
	Base
	focusState

	// Label names the tree for assistive technology ("Project files").
	Label string
	// ShowSizes prints each file's size at the right of its row.
	ShowSizes bool
	// Icon, when set, returns the glyph drawer for an entry (an icon pack's
	// SVG through [SVGIcon]); nil draws Iconoir's folder and page.
	Icon func(name string, dir, open bool) func(painter.Painter, Rect, RGBA)
	// OnRequest receives every change the person asks for.
	OnRequest func(FileRequest)
	// MenuExtra adds the host's items to the context menu of an entry ("" and
	// dir for the root).
	MenuExtra func(p string, dir bool) []MenuItem

	entries  *mvvm.Observable[[]FileEntry]
	selected *mvvm.Observable[string]

	tree      *TreeView
	root      *TreeNode
	nodes     map[string]*TreeNode
	collapsed map[string]bool
	stale     bool
	syncing   bool

	edit    *fileEdit
	confirm string // the path whose deletion awaits confirmation
	menu    *ContextMenu

	press        *TreeNode // the row pressed, for a drag
	pressX       int
	pressY       int
	dragging     bool
	pressing     bool      // a press is being handed to the tree
	toggle       *TreeNode // the folder a press opens or closes on release
	dropping     bool      // files from outside are over the tree
	dropDir      string
	typed        string
	typedAt      time.Time
	footerButton [2]Rect // Delete, Cancel (surface coordinates)
}

// fileNode is what a FileTree keeps in each TreeNode's Data.
type fileNode struct {
	path    string
	dir     bool
	size    int64
	pending bool // the row of a name being typed for a new entry
}

// fileEdit is a name being typed in place: a rename, or a new file or folder.
type fileEdit struct {
	action FileAction // FileRename, FileCreate or FileCreateDir
	path   string     // the renamed path, or the folder the new entry goes in
	entry  *Entry
	node   *TreeNode
	msg    string // the validation message, "" when the name is good
	block  bool   // msg is an error (not a warning): Enter is refused
}

// fileTreeNow is the clock type-ahead reads; tests replace it.
var fileTreeNow = time.Now

// fileTypeAheadGap is how long a pause ends a type-ahead string.
const fileTypeAheadGap = time.Second

// NewFileTree builds an explorer over entries.
func NewFileTree(entries []FileEntry) *FileTree {
	f := &FileTree{collapsed: map[string]bool{}}
	f.entries = mvvm.NewObservableEq(slices.Clone(entries), slices.Equal[[]FileEntry])
	f.selected = mvvm.NewObservable("")
	f.tree = NewTreeView(nil)
	f.tree.HideRoot = true
	f.tree.RowRenderer = f.drawRow
	f.tree.OnActivate = f.activate
	f.menu = NewContextMenu(NewMenu(nil))
	f.entries.Subscribe(func([]FileEntry) { f.stale = true })
	f.tree.Selected().Subscribe(func(n *TreeNode) {
		if !f.syncing && n != nil {
			f.selected.Set(n.Data.(*fileNode).path)
		}
	})
	f.selected.Subscribe(func(p string) {
		f.syncing = true
		f.tree.Selected().Set(f.nodeOf(p))
		f.syncing = false
		f.tree.scrollToSelected()
	})
	f.rebuild()
	return f
}

// Entries is the list the tree shows, as a shared [mvvm.Observable]: the host
// sets it after applying a request, or whenever its store changes.
func (f *FileTree) Entries() *mvvm.Observable[[]FileEntry] { return f.entries }

// Selected is the selected path ("" for none) as a shared [mvvm.Observable]:
// a click or the keyboard sets it, and a host that sets it moves the selection
// (expanding the folders above it).
func (f *FileTree) Selected() *mvvm.Observable[string] { return f.selected }

// Reveal expands the folders above p and selects it.
func (f *FileTree) Reveal(p string) {
	f.ensure()
	for d := path.Dir(p); d != "." && d != "/"; d = path.Dir(d) {
		delete(f.collapsed, d)
		if n := f.nodes[d]; n != nil {
			n.Expanded = true
		}
	}
	f.selected.Set(p)
	f.tree.scrollToSelected()
}

// ensure rebuilds the nodes when the entries changed.
func (f *FileTree) ensure() {
	if f.stale {
		f.rebuild()
	}
}

// rebuild turns the entries into nodes, keeping which folders are collapsed.
func (f *FileTree) rebuild() {
	f.stale = false
	for p, n := range f.nodes {
		if n.Data.(*fileNode).dir {
			if n.Expanded {
				delete(f.collapsed, p)
			} else {
				f.collapsed[p] = true
			}
		}
	}
	f.root = &TreeNode{Expanded: true, Data: &fileNode{dir: true}}
	f.nodes = map[string]*TreeNode{"": f.root}
	for _, e := range f.entries.Get() {
		p := cleanFilePath(e.Path)
		if p == "" {
			continue
		}
		n := f.folder(path.Dir(p))
		if e.Dir {
			f.folder(p)
			continue
		}
		if f.nodes[p] != nil {
			continue
		}
		leaf := &TreeNode{Label: path.Base(p), Data: &fileNode{path: p, size: e.Size}}
		f.nodes[p] = leaf
		n.Children = append(n.Children, leaf)
	}
	sortFileNodes(f.root)
	f.tree.Root = f.root
	if f.edit != nil {
		if f.edit.action != FileRename {
			f.placePending()
		} else if f.edit.node = f.nodes[f.edit.path]; f.edit.node == nil {
			f.edit = nil // renamed or deleted meanwhile
		}
	}
	f.syncing = true
	f.tree.Selected().Set(f.nodeOf(f.selected.Get()))
	f.syncing = false
	if f.tree.Selected().Get() == nil {
		f.selected.Set("")
	}
}

// nodeOf is the visible node of path p: nil for the root and for no entry.
func (f *FileTree) nodeOf(p string) *TreeNode {
	if p == "" {
		return nil
	}
	return f.nodes[p]
}

// folder returns the node of folder p ("." is the root), making it and the
// folders above it.
func (f *FileTree) folder(p string) *TreeNode {
	if p == "." {
		p = ""
	}
	if n := f.nodes[p]; n != nil {
		return n
	}
	parent := f.folder(path.Dir(p))
	n := &TreeNode{Label: path.Base(p), Expanded: !f.collapsed[p], Data: &fileNode{path: p, dir: true}}
	f.nodes[p] = n
	parent.Children = append(parent.Children, n)
	return n
}

// cleanFilePath makes p relative and clean ("" for the root or nothing).
func cleanFilePath(p string) string {
	p = strings.Trim(path.Clean("/"+p), "/")
	return p
}

// sortFileNodes orders folders first, then files, each by name ignoring case.
func sortFileNodes(n *TreeNode) {
	slices.SortStableFunc(n.Children, func(a, b *TreeNode) int {
		da, db := a.Data.(*fileNode).dir, b.Data.(*fileNode).dir
		if da != db {
			if da {
				return -1
			}
			return 1
		}
		if c := strings.Compare(strings.ToLower(a.Label), strings.ToLower(b.Label)); c != 0 {
			return c
		}
		return strings.Compare(a.Label, b.Label)
	})
	for _, c := range n.Children {
		sortFileNodes(c)
	}
}

// exists reports whether p names an entry.
func (f *FileTree) exists(p string) bool {
	n := f.nodes[p]
	return n != nil && !n.Data.(*fileNode).pending
}

// request hands r to the host.
func (f *FileTree) request(r FileRequest) {
	if f.OnRequest != nil {
		f.OnRequest(r)
	}
}

// activate is a click or Enter on a row: a folder opens or closes, a file is
// opened.
func (f *FileTree) activate(n *TreeNode) {
	fn := n.Data.(*fileNode)
	if fn.dir {
		if f.pressing {
			// A press may become a drag of the folder: it opens or closes on
			// the release.
			f.toggle = n
			return
		}
		n.Expanded = !n.Expanded
		return
	}
	if !fn.pending {
		f.request(FileRequest{Action: FileOpen, Path: fn.path})
	}
}

// targetDir is the folder a new entry goes in when the selection is p: p
// itself for a folder, else the file's folder.
func (f *FileTree) targetDir(p string) string {
	if n := f.nodes[p]; n != nil && n.Data.(*fileNode).dir {
		return p
	}
	return parentDir(p)
}

// parentDir is the folder p is in ("" for the root).
func parentDir(p string) string {
	d := path.Dir(p)
	if d == "." {
		return ""
	}
	return d
}

// BeginCreate starts typing the name of a new file (dir false) or folder in
// folder dir ("" is the root), as the Explorer's New File and New Folder do.
func (f *FileTree) BeginCreate(dir string, folder bool) {
	f.ensure()
	f.closeOverlays()
	a := FileCreate
	if folder {
		a = FileCreateDir
	}
	f.edit = &fileEdit{action: a, path: f.targetDir(dir), entry: NewEntry("")}
	f.edit.entry.SetFocused(true)
	f.placePending()
	f.validate()
}

// placePending puts a row for the name being typed at the top of its folder.
func (f *FileTree) placePending() {
	parent := f.nodes[f.edit.path]
	if parent == nil {
		parent = f.root
		f.edit.path = ""
	}
	parent.Expanded = true
	n := &TreeNode{Data: &fileNode{dir: f.edit.action == FileCreateDir, pending: true}}
	at := 0
	if f.edit.action == FileCreate {
		for at < len(parent.Children) && parent.Children[at].Data.(*fileNode).dir {
			at++
		}
	}
	parent.Children = slices.Insert(parent.Children, at, n)
	f.edit.node = n
	f.tree.scrollToRow(f.rowIndex(n))
}

// BeginRename starts renaming p in place; the name is selected but for its
// extension, as in the Explorer.
func (f *FileTree) BeginRename(p string) {
	f.ensure()
	n := f.nodes[p]
	if n == nil || n == f.root {
		return
	}
	f.closeOverlays()
	f.edit = &fileEdit{action: FileRename, path: p, entry: NewEntry(n.Label), node: n}
	f.edit.entry.SetFocused(true)
	f.validate()
}

// AskDelete asks for confirmation to delete p; Enter or the Delete button
// confirms, Escape or Cancel does not.
func (f *FileTree) AskDelete(p string) {
	f.ensure()
	if n := f.nodes[p]; n == nil || n == f.root {
		return
	}
	f.closeOverlays()
	f.confirm = p
}

// Dismiss closes the menu and cancels a rename, a new name or a confirmation:
// the host calls it when a click lands outside the tree.
func (f *FileTree) Dismiss() { f.closeOverlays() }

// closeOverlays ends whatever is in progress.
func (f *FileTree) closeOverlays() {
	f.menu.Close()
	f.confirm = ""
	f.cancelEdit()
}

// cancelEdit drops a name being typed, and its pending row.
func (f *FileTree) cancelEdit() {
	if f.edit == nil {
		return
	}
	if f.edit.action != FileRename {
		f.tree.Remove(f.edit.node)
	}
	f.edit = nil
}

// Editing reports whether a name is being typed in place.
func (f *FileTree) Editing() bool { return f.edit != nil }

// Confirming is the path awaiting a delete confirmation, "" for none.
func (f *FileTree) Confirming() string { return f.confirm }

// Message is the line under the tree: a name's validation message or the
// delete question ("" for none).
func (f *FileTree) Message() string {
	switch {
	case f.confirm != "":
		name := path.Base(f.confirm)
		if f.nodes[f.confirm] != nil && f.nodes[f.confirm].Data.(*fileNode).dir {
			return "Are you sure you want to delete '" + name + "' and its contents?"
		}
		return "Are you sure you want to delete '" + name + "'?"
	case f.edit != nil:
		return f.edit.msg
	}
	return ""
}

// editTarget is the full path the typed name makes.
func (f *FileTree) editTarget() string {
	name := strings.TrimSpace(f.edit.entry.Text().Get())
	dir := f.edit.path
	if f.edit.action == FileRename {
		dir = parentDir(f.edit.path)
	}
	return cleanFilePath(dir + "/" + name)
}

// validate checks the typed name with the Explorer's rules and messages.
func (f *FileTree) validate() {
	e := f.edit
	raw := e.entry.Text().Get()
	name := strings.TrimSpace(raw)
	e.msg, e.block = "", true
	switch {
	case name == "":
		e.msg = "A file or folder name must be provided."
	case strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`):
		e.msg = Tr("filetree.err.slash", "A file or folder name cannot start with a slash.")
	case !validFileName(name):
		e.msg = Trf("filetree.err.invalid", "The name '%s' is not valid as a file or folder name. Please choose a different name.", name)
	case f.edit.action == FileRename && f.editTarget() == f.edit.path:
		e.block = false
	case f.exists(f.editTarget()):
		e.msg = Trf("filetree.err.exists", "A file or folder '%s' already exists at this location. Please choose a different name.", name)
	case f.edit.action == FileRename && strings.HasPrefix(f.editTarget()+"/", f.edit.path+"/"):
		e.msg = Trf("filetree.err.invalid", "The name '%s' is not valid as a file or folder name. Please choose a different name.", name)
	default:
		e.block = false
		if raw != name {
			e.msg = Tr("filetree.warn.space", "Leading or trailing whitespace detected in file or folder name.")
		}
	}
}

// validFileName refuses the segments no file system takes: empty ones, "."
// and "..", control characters, and the characters Windows reserves.
func validFileName(name string) bool {
	for seg := range strings.SplitSeq(name, "/") {
		if seg == "" || seg == "." || seg == ".." || strings.ContainsAny(seg, `\:*?"<>|`) {
			return false
		}
		for _, r := range seg {
			if unicode.IsControl(r) {
				return false
			}
		}
	}
	return true
}

// commitEdit asks for the typed name when it is good; an unchanged rename
// simply ends.
func (f *FileTree) commitEdit() {
	f.validate()
	if f.edit.block {
		return
	}
	e := f.edit
	to := f.editTarget()
	f.cancelEdit()
	switch e.action {
	case FileRename:
		if to != e.path {
			f.request(FileRequest{Action: FileRename, Path: e.path, To: to})
			f.selected.Set(to)
		}
	default:
		f.request(FileRequest{Action: e.action, Path: to})
		f.selected.Set(to)
	}
}

// confirmDelete asks the host to delete the confirmed path.
func (f *FileTree) confirmDelete() {
	p := f.confirm
	f.confirm = ""
	f.request(FileRequest{Action: FileDelete, Path: p})
}

// Context opens the context menu for the entry at widget-local (x, y): the
// row there is selected first; empty space below the rows is the root.
func (f *FileTree) Context(x, y int) {
	f.ensure()
	f.layout()
	f.closeOverlays()
	p, dir := "", true
	if n := f.tree.NodeAt(x, y); n != nil {
		fn := n.Data.(*fileNode)
		p, dir = fn.path, fn.dir
		f.selected.Set(p)
	}
	var items []MenuItem
	if dir {
		items = append(items,
			MenuItem{Label: Tr("filetree.new_file", "New File…"), Action: func() { f.BeginCreate(p, false) }},
			MenuItem{Label: Tr("filetree.new_folder", "New Folder…"), Action: func() { f.BeginCreate(p, true) }})
	}
	if p != "" {
		if len(items) > 0 {
			items = append(items, MenuItem{Separator: true})
		}
		items = append(items,
			MenuItem{Label: Tr("filetree.rename", "Rename…"), Shortcut: "F2", Action: func() { f.BeginRename(p) }},
			MenuItem{Label: Tr("filetree.delete", "Delete"), Shortcut: "Del", Action: func() { f.AskDelete(p) }})
	}
	if f.MenuExtra != nil {
		if extra := f.MenuExtra(p, dir); len(extra) > 0 {
			items = append(items, MenuItem{Separator: true})
			items = append(items, extra...)
		}
	}
	f.menu.Menu = NewMenu(items)
	f.menu.SetBounds(f.Bounds())
	r := f.Bounds()
	f.menu.Popup(r.X+x, r.Y+y)
}

// MenuOpen reports whether the context menu is showing.
func (f *FileTree) MenuOpen() bool { return f.menu.Open().Get() }

// DropDirAt is the folder a drop at widget-local (x, y) goes in: the folder
// under the pointer, a file's folder, or the root ("") below the rows.
func (f *FileTree) DropDirAt(x, y int) string {
	f.ensure()
	f.layout()
	n := f.tree.NodeAt(x, y)
	if n == nil {
		return ""
	}
	fn := n.Data.(*fileNode)
	if fn.dir {
		return fn.path
	}
	return parentDir(fn.path)
}

// DropTarget is the folder a drag in progress would land in, and whether one
// is in progress (a row being moved, or files from outside).
func (f *FileTree) DropTarget() (string, bool) { return f.dropDir, f.dragging || f.dropping }

// FileTree is a DropTarget for files from outside.
var _ DropTarget = (*FileTree)(nil)

// AcceptsDrop accepts any non-empty payload: the host decides what it reads.
func (f *FileTree) AcceptsDrop(payload string) bool { return payload != "" }

// rowH is the tree's row height.
func (f *FileTree) rowH() int { return f.tree.rowHeight() }

// lineH is a footer line's height.
func (f *FileTree) lineH() int { return f.glyphHeight() + scaled(4) }

// footerLines wraps the message to the width.
func (f *FileTree) footerLines() []string {
	return wrapToWidth(f.Message(), f.Bounds().W-2*scaled(6), f.textWidth)
}

// footerH is the height of the strip under the tree: the message, and the
// confirmation's buttons.
func (f *FileTree) footerH() int {
	lines := len(f.footerLines())
	if lines == 0 {
		return 0
	}
	h := lines*f.lineH() + scaled(8)
	if f.confirm != "" {
		h += f.rowH() + scaled(4)
	}
	return min(h, f.Bounds().H)
}

// layout places the tree over the widget, above the footer.
func (f *FileTree) layout() {
	r := f.Bounds()
	f.tree.SetBounds(Rect{X: r.X, Y: r.Y, W: r.W, H: r.H - f.footerH()})
	f.tree.SetFocused(false)
}

// wrapToWidth breaks s into lines no wider than w, at spaces.
func wrapToWidth(s string, w int, measure func(string) int) []string {
	if s == "" {
		return nil
	}
	var lines []string
	line := ""
	for _, word := range strings.Fields(s) {
		if line != "" && measure(line+" "+word) > w {
			lines = append(lines, line)
			line = word
			continue
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	return append(lines, line)
}

// rowIndex is n's index among the visible rows, -1 when hidden.
func (f *FileTree) rowIndex(n *TreeNode) int {
	f.tree.flatten()
	return f.tree.rowOf(n)
}

// rowRect is the surface rectangle of n's row when it is on screen.
func (f *FileTree) rowRect(n *TreeNode) (Rect, int, bool) {
	i := f.rowIndex(n)
	if i < 0 {
		return Rect{}, 0, false
	}
	tr := f.tree.Bounds()
	rh := f.rowH()
	y := tr.Y + (i-f.tree.ScrollRow().Get())*rh
	if y < tr.Y || y+rh > tr.Y+tr.H {
		return Rect{}, 0, false
	}
	return Rect{X: tr.X, Y: y, W: tr.W, H: rh}, f.tree.rows[i].depth, true
}

// iconFor is the glyph drawer of an entry.
func (f *FileTree) iconFor(name string, dir, open bool) func(painter.Painter, Rect, RGBA) {
	if f.Icon != nil {
		if d := f.Icon(name, dir, open); d != nil {
			return d
		}
	}
	if dir {
		return fileTreeFolderIcon
	}
	return fileTreePageIcon
}

// The default glyphs, made once: a row's draw allocates no drawer.
var (
	fileTreeFolderIcon = func(p painter.Painter, r Rect, ink RGBA) { DrawIconoir(p, r, "folder", ink) }
	fileTreePageIcon   = func(p painter.Painter, r Rect, ink RGBA) { DrawIconoir(p, r, "page", ink) }
)

// drawRow paints a row's icon, name and size (TreeView.RowRenderer).
func (f *FileTree) drawRow(p painter.Painter, theme *Theme, r Rect, n *TreeNode, _ bool, ink RGBA) {
	fn := n.Data.(*fileNode)
	is := r.H - scaled(6)
	f.iconFor(n.Label, fn.dir, n.Expanded)(p, Rect{X: r.X, Y: r.Y + (r.H-is)/2, W: is, H: is}, ink)
	if f.edit != nil && f.edit.node == n {
		return // the Entry is drawn over it
	}
	x := r.X + is + scaled(TreeIconGap)
	ty := r.Y + (r.H-f.glyphHeight())/2
	right := r.X + r.W - scaled(4)
	if f.ShowSizes && !fn.dir {
		s := FileSizeText(fn.size)
		right -= f.textWidth(s)
		f.drawText(p, right, ty, s, ink)
		right -= scaled(8)
	}
	f.drawText(p, x, ty, elideToWidth(n.Label, right-x, f.textWidth), ink)
}

// elideToWidth shortens s with an ellipsis until it fits w.
func elideToWidth(s string, w int, measure func(string) int) string {
	if measure(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && measure(string(r)+"…") > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// FileSizeText writes a size the way a file panel shows it: 512 B, 12.5 KiB,
// 3.0 MiB.
func FileSizeText(n int64) string {
	switch {
	case n < 1024:
		return strconv.FormatInt(n, 10) + " B"
	case n < 1<<20:
		return strconv.FormatFloat(float64(n)/1024, 'f', 1, 64) + " KiB"
	}
	return strconv.FormatFloat(float64(n)/(1<<20), 'f', 1, 64) + " MiB"
}

// strongInk is the outline colour of a drop target: the accent when it keeps
// 3:1 against the surface, else the popup border, which does by construction.
func strongInk(theme *Theme) RGBA {
	if contrastRatio(theme.Accent, theme.Surface) >= PopupBorderMinContrast &&
		contrastRatio(theme.Accent, theme.Background) >= PopupBorderMinContrast {
		return theme.Accent
	}
	return PopupBorder(theme)
}

// Draw paints the tree, the name being typed, the drop target's outline, the
// footer and the context menu.
func (f *FileTree) Draw(p painter.Painter, theme *Theme) {
	f.ensure()
	f.layout()
	r := f.Bounds()
	f.tree.Draw(p, theme)
	if f.edit != nil {
		if rr, depth, ok := f.rowRect(f.edit.node); ok {
			x := rr.X + depth*scaled(TreeIndentW) + scaled(TreeChevronW) + rr.H - scaled(6) + scaled(TreeIconGap)
			f.edit.entry.SetBounds(Rect{X: x, Y: rr.Y + 1, W: max(rr.X+rr.W-x-scaled(2), 0), H: rr.H - 2})
			f.edit.entry.Draw(p, theme)
		}
	}
	if dir, on := f.DropTarget(); on {
		f.drawDropOutline(p, theme, dir)
	}
	f.drawFooter(p, theme)
	f.menu.Draw(p, theme)
	f.drawFocusRing(p, theme, r)
}

// drawDropOutline outlines the drop folder: its row and the rows of what it
// holds, or the whole tree for the root.
func (f *FileTree) drawDropOutline(p painter.Painter, theme *Theme, dir string) {
	tr := f.tree.Bounds()
	ink := strongInk(theme)
	w := scaled(2)
	box := tr
	if n := f.nodes[dir]; n != nil && n != f.root {
		i := f.rowIndex(n)
		if i < 0 {
			return
		}
		j := i + 1
		for j < len(f.tree.rows) && f.tree.rows[j].depth > f.tree.rows[i].depth {
			j++
		}
		rh, top := f.rowH(), f.tree.ScrollRow().Get()
		y0 := max(tr.Y+(i-top)*rh, tr.Y)
		y1 := min(tr.Y+(j-top)*rh, tr.Y+tr.H)
		if y1 <= y0 {
			return
		}
		box = Rect{X: tr.X, Y: y0, W: tr.W, H: y1 - y0}
	}
	fillRect(p, box.X, box.Y, box.W, w, ink)
	fillRect(p, box.X, box.Y+box.H-w, box.W, w, ink)
	fillRect(p, box.X, box.Y, w, box.H, ink)
	fillRect(p, box.X+box.W-w, box.Y, w, box.H, ink)
}

// drawFooter paints the message strip, and the confirmation's buttons.
func (f *FileTree) drawFooter(p painter.Painter, theme *Theme) {
	h := f.footerH()
	if h == 0 {
		return
	}
	r := f.Bounds()
	y := r.Y + r.H - h
	fillRect(p, r.X, y, r.W, h, theme.SurfaceAlt)
	fillRect(p, r.X, y, r.W, 1, PopupBorder(theme))
	ink := theme.OnSurface
	if f.edit != nil && f.edit.block {
		ink = strongError(theme)
	}
	ly := y + scaled(4)
	for _, l := range f.footerLines() {
		f.drawText(p, r.X+scaled(6), ly+scaled(2), l, ink)
		ly += f.lineH()
	}
	f.footerButton = [2]Rect{}
	if f.confirm == "" {
		return
	}
	x := r.X + scaled(6)
	for i, label := range []string{Tr("filetree.delete", "Delete"), Tr("filetree.cancel", "Cancel")} {
		b := NewButton(label, nil)
		b.Selected().Set(i == 0)
		br := Rect{X: x, Y: ly, W: b.PreferredWidth() + scaled(12), H: f.rowH()}
		b.SetBounds(br)
		b.Draw(p, theme)
		f.footerButton[i] = br
		x += br.W + scaled(6)
	}
}

// strongError is the ink of a blocking message: a red that keeps 4.5:1 on the
// footer in light and dark, else the text colour.
func strongError(theme *Theme) RGBA {
	for _, c := range []RGBA{RGB(0xb4, 0x23, 0x18), RGB(0xff, 0x8a, 0x80)} {
		if contrastRatio(c, theme.SurfaceAlt) >= 4.5 {
			return c
		}
	}
	return theme.OnSurface
}

// OnEvent routes an event (widget-local coordinates): to the menu when it is
// open, to the name being typed, to the confirmation, then to the tree, adding
// the Explorer's keys, the drag that moves an entry and the drop from outside.
func (f *FileTree) OnEvent(ev Event) {
	if f.Disabled().Get() {
		return
	}
	f.ensure()
	f.layout()
	r := f.Bounds()
	if f.menu.Open().Get() {
		ev.X += r.X
		ev.Y += r.Y
		if ev.Kind == EventKeyDown && ev.Code == "Escape" {
			f.menu.Close()
			return
		}
		f.menu.OnEvent(ev)
		return
	}
	switch ev.Kind {
	case EventDragStart, EventDragMove:
		f.dropping = true
		f.dropDir = f.DropDirAt(ev.X, ev.Y)
		return
	case EventDragLeave:
		f.dropping = false
		return
	case EventDrop:
		f.dropping = false
		if names := SplitDropPayload(ev.Code); len(names) > 0 {
			f.request(FileRequest{Action: FileImport, To: f.DropDirAt(ev.X, ev.Y), Names: names})
		}
		return
	case EventKeyDown, EventChar:
		f.key(ev)
		return
	case EventClick:
		if f.click(ev) {
			return
		}
		f.pressing = true
		f.tree.OnEvent(ev)
		f.pressing = false
		return
	case EventMouseDrag:
		if f.dragMove(ev) {
			return
		}
	case EventMouseUp:
		f.drop()
	}
	f.tree.OnEvent(ev)
}

// click handles a press: on the footer, beside the name being typed (which
// commits it), or on a row (where a drag may start). It reports whether the
// press was taken.
func (f *FileTree) click(ev Event) bool {
	r := f.Bounds()
	sx, sy := r.X+ev.X, r.Y+ev.Y
	if f.confirm != "" {
		switch {
		case f.footerButton[0].Contains(sx, sy):
			f.confirmDelete()
		case f.footerButton[1].Contains(sx, sy):
			f.confirm = ""
		}
		if sy >= f.tree.Bounds().Y+f.tree.Bounds().H {
			return true
		}
		f.confirm = ""
	}
	if f.edit != nil {
		if f.edit.entry.Bounds().Contains(sx, sy) {
			eb := f.edit.entry.Bounds()
			f.edit.entry.OnEvent(Event{Kind: EventClick, X: sx - eb.X, Y: sy - eb.Y})
			return true
		}
		f.commitEdit()
		if f.edit != nil {
			f.cancelEdit()
		}
	}
	f.press = f.tree.NodeAt(ev.X, ev.Y)
	f.pressX, f.pressY = ev.X, ev.Y
	f.dragging = false
	return false
}

// dragMove follows a press that became a drag of a row; it reports whether the
// event was taken (a drag of the scrollbar is the tree's).
func (f *FileTree) dragMove(ev Event) bool {
	if f.press == nil {
		return false
	}
	dx, dy := ev.X-f.pressX, ev.Y-f.pressY
	if !f.dragging && dx*dx+dy*dy < scaled(4)*scaled(4) {
		return true
	}
	f.dragging = true
	f.dropDir = f.DropDirAt(ev.X, ev.Y)
	return true
}

// drop ends a press: a row dragged onto another folder moves there.
func (f *FileTree) drop() {
	n, dragging, toggle := f.press, f.dragging, f.toggle
	f.press, f.dragging, f.toggle = nil, false, nil
	if !dragging {
		if toggle != nil {
			toggle.Expanded = !toggle.Expanded
		}
		return
	}
	from := n.Data.(*fileNode).path
	to := cleanFilePath(f.dropDir + "/" + path.Base(from))
	switch {
	case f.dropDir == parentDir(from):
	case strings.HasPrefix(f.dropDir+"/", from+"/"):
	case f.exists(to):
	default:
		f.request(FileRequest{Action: FileMove, Path: from, To: to})
		f.selected.Set(to)
	}
}

// key handles a key: the name being typed takes it first, then the
// confirmation, then the Explorer's keys, then the tree's.
func (f *FileTree) key(ev Event) {
	if f.edit != nil {
		switch {
		case ev.Kind == EventKeyDown && ev.Code == "Enter":
			f.commitEdit()
		case ev.Kind == EventKeyDown && ev.Code == "Escape":
			f.cancelEdit()
		default:
			f.edit.entry.OnEvent(ev)
			f.validate()
		}
		return
	}
	if f.confirm != "" {
		if ev.Kind == EventKeyDown && ev.Code == "Enter" {
			f.confirmDelete()
			return
		}
		f.confirm = ""
		return
	}
	sel := f.selected.Get()
	if ev.Kind == EventChar {
		f.typeAhead(ev.Code)
		return
	}
	switch ev.Code {
	case "F2":
		f.BeginRename(sel)
	case "Delete":
		f.AskDelete(sel)
	case "Backspace":
		if ev.Meta {
			f.AskDelete(sel)
		}
	case "ContextMenu", "Shift+F10":
		x, y := scaled(TreeChevronW), 0
		if rr, _, ok := f.rowRect(f.nodes[sel]); ok {
			y = rr.Y - f.Bounds().Y + rr.H/2
		}
		f.Context(x, y)
	default:
		f.tree.OnEvent(ev)
	}
}

// typeAhead moves to the next row whose name starts with what was typed: one
// character steps through the matches, characters typed quickly match as a
// string (APG treeview). "*" expands the selection's sibling folders.
func (f *FileTree) typeAhead(ch string) {
	now := fileTreeNow()
	if ch == "*" {
		f.expandSiblings()
		return
	}
	if now.Sub(f.typedAt) > fileTypeAheadGap {
		f.typed = ""
	}
	f.typedAt = now
	f.typed += strings.ToLower(ch)
	f.tree.flatten()
	rows := f.tree.rows
	start := f.tree.cursorRow()
	if len([]rune(f.typed)) == 1 {
		start++ // one character: the next match, not this one
	}
	for k := range len(rows) {
		i := (max(start, 0) + k) % len(rows)
		if strings.HasPrefix(strings.ToLower(rows[i].node.Label), f.typed) {
			f.tree.setCursorRow(i)
			return
		}
	}
}

// expandSiblings expands every folder at the selection's level.
func (f *FileTree) expandSiblings() {
	// The selection always names a node (or "", the root), so its folder is
	// one too.
	for _, c := range f.nodes[parentDir(f.selected.Get())].Children {
		if c.Data.(*fileNode).dir {
			c.Expanded = true
		}
	}
}

// A11y describes the tree: its name and the selected path.
func (f *FileTree) A11y() A11yInfo {
	return A11yInfo{Role: RoleTree, Name: f.Label, Value: f.selected.Get()}
}
