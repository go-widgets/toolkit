// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package i18ntest is the test that keeps a toolkit.Catalog complete: it fails
// on a key one locale has and another lacks, and on a translation whose fmt
// verbs differ from the fallback's.
//
//	func TestCatalogIsComplete(t *testing.T) { i18ntest.Check(t, catalog) }
package i18ntest

import (
	"strings"

	"github.com/go-widgets/toolkit"
)

// T is the part of *testing.T Check needs (a fake can stand in).
type T interface {
	Helper()
	Errorf(format string, args ...any)
}

// Check fails t once per locale that lacks keys, naming them, and once per
// translation whose verbs differ from the fallback's. A catalog with no
// table at all also fails: an empty catalog is a missing one.
func Check(t T, c *toolkit.Catalog) {
	t.Helper()
	if len(c.Locales()) == 0 {
		t.Errorf("i18n catalog has no locale")
		return
	}
	missing := c.Missing()
	for _, loc := range c.Locales() {
		if keys := missing[loc]; len(keys) > 0 {
			t.Errorf("locale %q lacks %d key(s): %s", loc, len(keys), strings.Join(keys, ", "))
		}
	}
	for _, m := range c.VerbMismatches() {
		t.Errorf("format verbs differ from the %s text: %s", c.Fallback(), m)
	}
}
