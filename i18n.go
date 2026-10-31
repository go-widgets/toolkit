// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"sync"
)

// The words the toolkit itself writes (a file tree's context menu, a find bar's
// "No results", a ribbon's accessible name…) are English by default and can be
// said in another language by the host: every one goes through Tr with a stable
// KEY and its English text, and SetLocalizer gives the host the place to answer.
//
//	toolkit.SetLocalizer(func(key, english string) string {
//		if s, ok := french[key]; ok {
//			return s
//		}
//		return english // a key the host does not know stays English
//	})
//
// The keys are the toolkit's API for this: they are listed in MessageKeys with
// their English text, and a key is never renamed in a minor release. A message
// with arguments carries fmt verbs (Trf). With no localizer set, nothing changes.

var (
	localizerMu sync.RWMutex
	localizer   func(key, english string) string
)

// SetLocalizer sets the function that says the toolkit's own words in the
// host's language; nil restores English.
func SetLocalizer(f func(key, english string) string) {
	localizerMu.Lock()
	localizer = f
	localizerMu.Unlock()
}

// Tr is the message key in the host's language: english when no localizer is
// set, or when it answers "".
func Tr(key, english string) string {
	localizerMu.RLock()
	f := localizer
	localizerMu.RUnlock()
	if f == nil {
		return english
	}
	if s := f(key, english); s != "" {
		return s
	}
	return english
}

// Trf is Tr followed by fmt.Sprintf with args.
func Trf(key, english string, args ...any) string {
	return fmt.Sprintf(Tr(key, english), args...)
}

// MessageKeys are the keys the toolkit writes, with their English text.
var MessageKeys = map[string]string{
	"filetree.new_file":    "New File…",
	"filetree.new_folder":  "New Folder…",
	"filetree.rename":      "Rename…",
	"filetree.delete":      "Delete",
	"filetree.cancel":      "Cancel",
	"filetree.err.slash":   "A file or folder name cannot start with a slash.",
	"filetree.err.invalid": "The name '%s' is not valid as a file or folder name. Please choose a different name.",
	"filetree.err.exists":  "A file or folder '%s' already exists at this location. Please choose a different name.",
	"filetree.warn.space":  "Leading or trailing whitespace detected in file or folder name.",
	"find.replace":         "Replace",
	"find.replace_all":     "All",
	"find.bad_pattern":     "Bad pattern",
	"find.no_results":      "No results",
	"find.count":           "%d of %d",
	"find.a11y":            "Find and replace",
	"ribbon.a11y":          "Ribbon",
	"ribbon.a11y_tabs":     "Ribbon tabs",
	"filechooser.open":     "Open",
	"filechooser.cancel":   "Cancel",
	"dialog.ok":            "OK",
}
