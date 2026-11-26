// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package i18ntest

import (
	"strings"
	"testing"

	"github.com/go-widgets/toolkit"
)

type fake struct{ errs []string }

func (f *fake) Helper() {}
func (f *fake) Errorf(format string, args ...any) {
	f.errs = append(f.errs, strings.TrimSpace(sprintf(format, args...)))
}

func TestCheckPassesACompleteCatalog(t *testing.T) {
	c := toolkit.NewCatalog("en").
		Add("en", map[string]string{"a": "A", "n": "%d"}).
		Add("fr", map[string]string{"a": "À", "n": "%d"})
	f := &fake{}
	Check(f, c)
	if len(f.errs) != 0 {
		t.Fatalf("unexpected: %v", f.errs)
	}
}

func TestCheckFailsOnAMissingKeyAndAVerbMismatch(t *testing.T) {
	c := toolkit.NewCatalog("en").
		Add("en", map[string]string{"a": "A", "b": "B", "n": "%d"}).
		Add("fr", map[string]string{"a": "À", "n": "%s"})
	f := &fake{}
	Check(f, c)
	all := strings.Join(f.errs, "\n")
	if len(f.errs) != 2 || !strings.Contains(all, "fr") || !strings.Contains(all, "b") || !strings.Contains(all, "fr n") {
		t.Fatalf("errors: %q", f.errs)
	}
}

func TestCheckFailsOnAnEmptyCatalog(t *testing.T) {
	f := &fake{}
	Check(f, toolkit.NewCatalog("en"))
	if len(f.errs) != 1 {
		t.Fatalf("errors: %q", f.errs)
	}
}
