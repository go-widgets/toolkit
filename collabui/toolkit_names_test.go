// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package collabui

import "github.com/go-widgets/toolkit"

// The tests moved here unchanged from package toolkit; these test-only names
// let them keep addressing the toolkit types unqualified.
type (
	CodeEditor = toolkit.CodeEditor
	RGBA       = toolkit.RGBA
	Decoration = toolkit.Decoration
	Selection  = toolkit.Selection
)

var NewCodeEditor = toolkit.NewCodeEditor

// sel is a keyed Selection literal (vet rejects unkeyed literals of a type from
// another package).
func sel(sl, sc, el, ec int) Selection {
	return Selection{StartLine: sl, StartCol: sc, EndLine: el, EndCol: ec}
}
