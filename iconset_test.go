// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"flag"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-icons/iconoir"
	"github.com/go-widgets/toolkit/internal/iconsetgen"
)

var updateIconset = flag.Bool("update-iconset", false, "rewrite "+iconsetgen.OutFile+" from the current sources")

// TestBuiltinIconsetIsCurrent regenerates the built-in icon subset in memory and
// compares it byte for byte with the committed file: a name added to the source
// without a regeneration, a name removed, or an SVG changed by an iconoir bump
// all fail here. Comparing keys alone could not see a stale SVG.
func TestBuiltinIconsetIsCurrent(t *testing.T) {
	want, err := iconsetgen.Generate(".", "toolkit")
	if err != nil {
		t.Fatal(err)
	}
	if *updateIconset {
		if err := os.WriteFile(iconsetgen.OutFile, want, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(iconsetgen.OutFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale; run `go generate` in the module root", iconsetgen.OutFile)
	}
}

// TestBuiltinIconsetMatchesIconoir: every built-in glyph is byte-identical to the
// iconoir pack's own, and every name the toolkit's widgets draw is in the set.
func TestBuiltinIconsetMatchesIconoir(t *testing.T) {
	names, err := iconsetgen.Names(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != builtinIconCount {
		t.Fatalf("Names found %d icons, the generated set holds %d", len(names), builtinIconCount)
	}
	for _, n := range names {
		if builtinIcon(n) != iconoir.Icon(n) {
			t.Errorf("builtinIcon(%q) differs from iconoir.Icon", n)
		}
	}
	// The names the widgets draw (dialog, closable pane, file tree, paged
	// view, rich-editor toolbar), spelled out so a regression in Names'
	// literal scan cannot also hide them.
	drawn := []string{"xmark", "expand", "collapse", "minus", "plus", "folder", "page",
		"multiple-pages", "zoom-out", "zoom-in", "scale-frame-reduce", "nav-arrow-left",
		"nav-arrow-right", "bold", "italic", "strikethrough", "code", "text", "quote",
		"code-brackets", "list", "numbered-list-left"}
	for _, n := range drawn {
		if builtinIcon(n) == "" {
			t.Errorf("built-in set lacks %q, which a widget draws", n)
		}
	}
	// And it is a subset: an Iconoir name no toolkit source mentions is absent.
	if builtinIcon("airplane") != "" {
		t.Error(`builtinIcon("airplane") is non-empty; the set is not a subset`)
	}
	if builtinIconCount >= len(iconoir.Names())/10 {
		t.Errorf("built-in set holds %d of %d icons; expected a small subset", builtinIconCount, len(iconoir.Names()))
	}
}

// TestToolkitDoesNotImportIconoir: no non-test file of package toolkit imports
// go-icons/iconoir, whose single embed.FS the linker cannot trim (#588).
func TestToolkitDoesNotImportIconoir(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	scanned := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		for _, imp := range af.Imports {
			if p, _ := strconv.Unquote(imp.Path.Value); p == "github.com/go-icons/iconoir" {
				t.Errorf("%s imports %s", f, p)
			}
		}
	}
	if scanned < 100 {
		t.Fatalf("scanned only %d files; the glob did not read the package", scanned)
	}
}
