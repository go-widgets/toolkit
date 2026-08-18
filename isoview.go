// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// NewIsoDiagramView returns an additional [IsoDiagram] bound to an EXISTING
// document, for showing one model in several synchronised views. Because every
// edit flows through the document's observable lists, a mutation made through
// any view is seen by all of them: each view subscribes to the shared document
// and repaints on change.
//
// Only the MODEL is shared. Everything a view owns is local to that widget: its
// isometric [iso.Projection] (pan / zoom), its selection set, its in-flight
// gesture and its undo/redo stacks all live on the widget, not in the document.
// So two views can be panned, zoomed and selected independently while editing
// the same diagram — panning one never moves the other, and selecting in one
// never changes the other's highlight, yet placing or moving a node in either
// appears in both. It is a thin, intention-revealing alias for
// [NewIsoDiagram]: pass the same [IsoDocument] to as many views as you need.
func NewIsoDiagramView(doc IsoDocument) *IsoDiagram { return NewIsoDiagram(doc) }
