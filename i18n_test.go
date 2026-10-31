// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrDefaultsToEnglish(t *testing.T) {
	SetLocalizer(nil)
	if got := Tr("x", "English"); got != "English" {
		t.Errorf("Tr = %q", got)
	}
	if got := Trf("x", "%d of %d", 1, 2); got != "1 of 2" {
		t.Errorf("Trf = %q", got)
	}
}

func TestLocalizerAnswersAndFallsBack(t *testing.T) {
	t.Cleanup(func() { SetLocalizer(nil) })
	SetLocalizer(func(key, english string) string {
		switch key {
		case "k":
			return "traduit"
		case "n":
			return "%d sur %d"
		}
		return "" // a key the host does not know
	})
	if got := Tr("k", "English"); got != "traduit" {
		t.Errorf("Tr = %q", got)
	}
	if got := Tr("unknown", "English"); got != "English" {
		t.Errorf("an empty answer must leave English: %q", got)
	}
	if got := Trf("n", "%d of %d", 1, 2); got != "1 sur 2" {
		t.Errorf("Trf = %q", got)
	}
}

// the widgets say their words through the localizer
func TestWidgetsUseTheLocalizer(t *testing.T) {
	t.Cleanup(func() { SetLocalizer(nil) })
	SetLocalizer(func(key, english string) string { return "«" + english + "»" })
	if got := formatCount(0, 0, true); got != "«Bad pattern»" {
		t.Errorf("find: %q", got)
	}
	if got := formatCount(0, 0, false); got != "«No results»" {
		t.Errorf("find: %q", got)
	}
	if got := formatCount(3, 1, false); got != "«%d of %d»"[:0]+"«2 of 3»" {
		t.Errorf("find count: %q", got)
	}
	ft := NewFileTree([]FileEntry{{Path: "a", Dir: true}})
	ft.SetBounds(Rect{W: 200, H: 200})
	ft.Context(5, 5)
	var labels []string
	for _, it := range ft.menu.Menu.Items {
		labels = append(labels, it.Label)
	}
	if got := strings.Join(labels, "|"); !strings.Contains(got, "«New File…»") || !strings.Contains(got, "«Delete»") || !strings.Contains(got, "«Rename…»") {
		t.Errorf("file tree menu: %q", got)
	}
}

// every key MessageKeys lists is written by the toolkit, and the toolkit writes
// no key it does not list: a key is part of the API.
func TestMessageKeysAreTheOnesInTheSource(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	var src strings.Builder
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || f == "i18n.go" {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src.Write(b)
	}
	s := src.String()
	for key, english := range MessageKeys {
		if !strings.Contains(s, `"`+key+`", "`) {
			t.Errorf("key %q is listed but not written", key)
		}
		if !strings.Contains(s, english) && !strings.Contains(s, strings.ReplaceAll(english, "'", "'")) {
			t.Errorf("key %q: English %q differs from the source", key, english)
		}
	}
	for _, part := range strings.Split(s, `Tr("`)[1:] {
		key, _, _ := strings.Cut(part, `"`)
		if _, ok := MessageKeys[key]; !ok {
			t.Errorf("key %q is written but not listed in MessageKeys", key)
		}
	}
	for _, part := range strings.Split(s, `Trf("`)[1:] {
		key, _, _ := strings.Cut(part, `"`)
		if _, ok := MessageKeys[key]; !ok {
			t.Errorf("key %q is written but not listed in MessageKeys", key)
		}
	}
}
