// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
	"github.com/go-widgets/toolkit/virtual"
)

// The announce-Disabled rule beyond the root package (#578 item 2).
//
// TestA11yDisabledCatalogueIsComplete reads only the root package, so
// virtual.VirtualList and virtual.VirtualGrid -- which embed toolkit.Base -- sat
// outside it. This file closes that for every package in the module, not for
// the one that was noticed: the scan walks the module tree itself, so a package
// created next year is read the day it appears and its widgets fail here until
// someone catalogues or exempts them. It lives in the external test package
// because the constructors below import sub-packages that import toolkit.
//
// Names are "<dir>.<Type>" (slash-separated, relative to the module root).

// subpackageDisabledCatalogue maps a sub-package type to a constructor for a
// fresh instance. The rule is the root one: disabled, its A11y() says so; enabled,
// it clears. Behaviour is tested in the package that owns the type.
var subpackageDisabledCatalogue = map[string]func() toolkit.Accessible{
	"virtual.VirtualList": func() toolkit.Accessible {
		return virtual.NewVirtualList(mvvm.NewObservableList[int](), nil, func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	},
	"virtual.VirtualGrid": func() toolkit.Accessible {
		return virtual.NewVirtualGrid(mvvm.NewObservableList[int](), toolkit.Size{W: 10, H: 10}, func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	},
	"virtual.CardList": func() toolkit.Accessible {
		return virtual.NewCardList(mvvm.NewObservableList[int](), nil, func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int, virtual.CardState) {})
	},
}

// subpackageDisabledExempt is the root exemption list's counterpart: a type
// that embeds toolkit.Base and answers A11y() but has no inert state, with the
// reason. Empty today.
var subpackageDisabledExempt = map[string]string{}

// moduleDirs lists every directory of the module, the root first, that holds
// non-test Go source. A nested module (its own go.mod), a hidden or underscore
// directory and testdata are not part of this module's API.
func moduleDirs(t *testing.T) []string {
	t.Helper()
	dirs := []string{"."}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() || path == "." {
			return nil
		}
		base := d.Name()
		if strings.HasPrefix(base, ".") || strings.HasPrefix(base, "_") || base == "testdata" {
			return filepath.SkipDir
		}
		if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
			return filepath.SkipDir
		}
		src, _ := filepath.Glob(filepath.Join(path, "*.go"))
		for _, f := range src {
			if !strings.HasSuffix(f, "_test.go") {
				dirs = append(dirs, path)
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return dirs
}

// TestA11yDisabledEveryPackageIsCovered fails when a type of any sub-package
// embeds toolkit.Base (directly or through another type) and answers A11y() but
// is in neither list, when a list names a type the scan does not find, or when
// the two lists overlap.
func TestA11yDisabledEveryPackageIsCovered(t *testing.T) {
	dirs := moduleDirs(t)
	if len(dirs) < 2 {
		t.Fatalf("the module walk found %v: the scan is blind", dirs)
	}
	found := map[string]bool{}
	for _, name := range toolkit.ScanAccessibleBaseTypes(t, dirs...) {
		if strings.Contains(name, ".") { // the root package has its own, deeper audit
			found[name] = true
		}
	}
	for _, name := range toolkit.ScanBaseTypesWithoutA11y(t, dirs...) {
		if strings.Contains(name, ".") {
			t.Errorf("%s embeds toolkit.Base but has no A11y(): give it one", name)
		}
	}
	for name := range found {
		_, cat := subpackageDisabledCatalogue[name]
		_, exempt := subpackageDisabledExempt[name]
		if !cat && !exempt {
			t.Errorf("%s embeds toolkit.Base and answers A11y() but is in neither subpackageDisabledCatalogue nor subpackageDisabledExempt", name)
		}
		if cat && exempt {
			t.Errorf("%s is both in the catalogue and exempt", name)
		}
	}
	for name := range subpackageDisabledCatalogue {
		if !found[name] {
			t.Errorf("subpackageDisabledCatalogue lists %s, which is not an accessible Base type of any sub-package", name)
		}
	}
	for name, reason := range subpackageDisabledExempt {
		if !found[name] {
			t.Errorf("subpackageDisabledExempt lists %s, which is not an accessible Base type of any sub-package", name)
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("subpackageDisabledExempt[%s] has no reason", name)
		}
	}
}

// TestA11yDisabledSubpackageAudit disables each catalogued sub-package widget,
// requires A11y().Disabled, then enables it and requires the flag to clear.
func TestA11yDisabledSubpackageAudit(t *testing.T) {
	names := make([]string, 0, len(subpackageDisabledCatalogue))
	for name := range subpackageDisabledCatalogue {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		w := subpackageDisabledCatalogue[name]()
		d, ok := w.(interface{ Disabled() *mvvm.Observable[bool] })
		if !ok {
			t.Errorf("%s has no Disabled() observable", name)
			continue
		}
		if w.A11y().Disabled {
			t.Errorf("%s: A11y().Disabled is true before it is disabled", name)
		}
		d.Disabled().Set(true)
		if !w.A11y().Disabled {
			t.Errorf("%s: disabled, but A11y().Disabled is false", name)
		}
		d.Disabled().Set(false)
		if w.A11y().Disabled {
			t.Errorf("%s: re-enabled, but A11y().Disabled is still true", name)
		}
	}
}
