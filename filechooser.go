// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// FileChooser is a directory-tree + file-list + path-entry composite.
// It does NO I/O — the host hands it a virtual root (a TreeNode tree
// representing directories) + a func that lists files in a given
// directory. Selection is reported via OnAccept.
//
// FileChooser is the canonical use-case for TreeView + ListBox + Entry
// composed together. It is what an "Open File…" dialog renders inside
// a wasmbox app that has no JS file picker access.
type FileChooser struct {
	Base
	Root     *TreeNode
	ListFiles func(dir *TreeNode) []string
	OnAccept func(path string)
	OnCancel func()

	tree       *TreeView
	list       *ListBox
	pathEntry  *Entry
	openButton *Button
	cancelButton *Button

	currentDir *TreeNode
	selectedFile string
}

// Sizing constants.
const (
	FileChooserTreeRatio   = 35  // % of width for the tree pane
	FileChooserButtonStripH = 32
	FileChooserPathH       = 24
)

// NewFileChooser builds a FileChooser rooted at root with the given
// directory-listing func.
func NewFileChooser(root *TreeNode, listFiles func(dir *TreeNode) []string) *FileChooser {
	f := &FileChooser{
		Root:      root,
		ListFiles: listFiles,
	}
	f.tree = NewTreeView(root)
	f.list = NewListBox(nil)
	f.pathEntry = NewEntry("")
	f.openButton = NewButton("Open", nil)
	f.cancelButton = NewButton("Cancel", nil)

	f.tree.OnActivate = func(n *TreeNode) {
		f.currentDir = n
		f.list.Items = nil
		if f.ListFiles != nil {
			f.list.Items = f.ListFiles(n)
		}
		f.pathEntry.Text = n.Label
	}
	f.list.OnActivate = func(i int) {
		if i < 0 || i >= len(f.list.Items) {
			return
		}
		f.selectedFile = f.list.Items[i]
		dir := ""
		if f.currentDir != nil {
			dir = f.currentDir.Label
		}
		f.pathEntry.Text = dir + "/" + f.selectedFile
	}
	f.openButton.OnClick = func() {
		if f.OnAccept != nil {
			f.OnAccept(f.pathEntry.Text)
		}
	}
	f.cancelButton.OnClick = func() {
		if f.OnCancel != nil {
			f.OnCancel()
		}
	}
	return f
}

// SetBounds positions the child widgets at the chosen split.
func (f *FileChooser) SetBounds(r Rect) {
	f.Base.SetBounds(r)
	pathY := r.Y + r.H - FileChooserButtonStripH - FileChooserPathH
	stripY := r.Y + r.H - FileChooserButtonStripH
	bodyH := pathY - r.Y
	treeW := r.W * FileChooserTreeRatio / 100
	listW := r.W - treeW
	f.tree.SetBounds(Rect{X: r.X, Y: r.Y, W: treeW, H: bodyH})
	f.list.SetBounds(Rect{X: r.X + treeW, Y: r.Y, W: listW, H: bodyH})
	f.pathEntry.SetBounds(Rect{X: r.X, Y: pathY, W: r.W, H: FileChooserPathH})
	bw := DialogButtonW
	gap := 8
	f.openButton.SetBounds(Rect{X: r.X + r.W - bw - gap, Y: stripY + 4, W: bw, H: FileChooserButtonStripH - 8})
	f.cancelButton.SetBounds(Rect{X: r.X + r.W - 2*bw - 2*gap, Y: stripY + 4, W: bw, H: FileChooserButtonStripH - 8})
}

// Draw paints the composite.
func (f *FileChooser) Draw(surface []byte, surfaceW int, theme *Theme) {
	r := f.Bounds()
	fillRect(surface, surfaceW, r.X, r.Y, r.W, r.H, theme.Background)
	f.tree.Draw(surface, surfaceW, theme)
	f.list.Draw(surface, surfaceW, theme)
	f.pathEntry.Draw(surface, surfaceW, theme)
	f.openButton.Draw(surface, surfaceW, theme)
	f.cancelButton.Draw(surface, surfaceW, theme)
}

// OnEvent dispatches to the child widgets based on which one the event
// falls inside.
func (f *FileChooser) OnEvent(ev Event) {
	switch {
	case insideRect(ev.X, ev.Y, f.tree.Bounds()):
		f.tree.OnEvent(localize(ev, f.tree.Bounds()))
	case insideRect(ev.X, ev.Y, f.list.Bounds()):
		f.list.OnEvent(localize(ev, f.list.Bounds()))
	case insideRect(ev.X, ev.Y, f.pathEntry.Bounds()):
		f.pathEntry.OnEvent(localize(ev, f.pathEntry.Bounds()))
	case insideRect(ev.X, ev.Y, f.openButton.Bounds()):
		f.openButton.OnEvent(localize(ev, f.openButton.Bounds()))
	case insideRect(ev.X, ev.Y, f.cancelButton.Bounds()):
		f.cancelButton.OnEvent(localize(ev, f.cancelButton.Bounds()))
	}
}

// Path returns the entry text — the current effective selection.
func (f *FileChooser) Path() string { return f.pathEntry.Text }

func insideRect(x, y int, r Rect) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

func localize(ev Event, r Rect) Event {
	ev.X -= r.X
	ev.Y -= r.Y
	return ev
}
