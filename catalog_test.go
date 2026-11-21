// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"reflect"
	"testing"
)

func testCatalog() *Catalog {
	return NewCatalog("EN").
		Add("en", map[string]string{"save": "Save", "count": "%d of %d", "pct": "100%% sure", "only_en": "English only"}).
		Add("fr", map[string]string{"save": "Enregistrer", "count": "%d sur %d", "pct": "sûr à 100%%"}).
		Add("fr_CA", map[string]string{"save": "Sauvegarder"})
}

func TestCatalogLookupAndFallback(t *testing.T) {
	c := testCatalog()
	if c.Fallback() != "en" {
		t.Fatalf("fallback %q", c.Fallback())
	}
	if got := c.Locales(); !reflect.DeepEqual(got, []string{"en", "fr", "fr-ca"}) {
		t.Fatalf("locales %v", got)
	}
	if got := c.Keys(); !reflect.DeepEqual(got, []string{"count", "only_en", "pct", "save"}) {
		t.Fatalf("keys %v", got)
	}
	if s, ok := c.Lookup("fr-CA", "save"); !ok || s != "Sauvegarder" {
		t.Fatalf("own table: %q %v", s, ok)
	}
	if s, ok := c.Lookup("fr-BE", "save"); !ok || s != "Enregistrer" {
		t.Fatalf("regional tag reads its language: %q %v", s, ok)
	}
	if _, ok := c.Lookup("fr", "only_en"); ok {
		t.Fatal("Lookup must not fall back")
	}
	if _, ok := c.Lookup("fr-BE", "only_en"); ok {
		t.Fatal("a regional tag must not fall back either")
	}
	if _, ok := c.Lookup("de", "save"); ok {
		t.Fatal("no table, no answer")
	}
	if got := c.Text("fr", "only_en"); got != "English only" {
		t.Fatalf("Text falls back to en: %q", got)
	}
	if got := c.Text("fr", "nope"); got != "" {
		t.Fatalf("unknown key: %q", got)
	}
	if got := c.Text("fr", "save"); got != "Enregistrer" {
		t.Fatalf("Text: %q", got)
	}
}

func TestCatalogLocalizerPlugsIntoTr(t *testing.T) {
	c := testCatalog()
	SetLocalizer(c.Localizer("fr"))
	t.Cleanup(func() { SetLocalizer(nil) })
	if got := Tr("save", "Save"); got != "Enregistrer" {
		t.Fatalf("Tr: %q", got)
	}
	if got := Trf("count", "%d of %d", 1, 2); got != "1 sur 2" {
		t.Fatalf("Trf: %q", got)
	}
	if got := Tr("unknown", "Unknown"); got != "Unknown" {
		t.Fatalf("an unknown key stays English: %q", got)
	}
}

func TestCatalogAddMergesAndReplaces(t *testing.T) {
	c := NewCatalog("en").Add("en", map[string]string{"a": "1"}).Add("EN", map[string]string{"a": "2", "b": "3"})
	if c.Text("en", "a") != "2" || c.Text("en", "b") != "3" {
		t.Fatalf("merge: %q %q", c.Text("en", "a"), c.Text("en", "b"))
	}
}

func TestCatalogMissingAndVerbMismatches(t *testing.T) {
	c := testCatalog()
	want := map[string][]string{"fr": {"only_en"}, "fr-ca": {"count", "only_en", "pct"}}
	// fr-ca reads fr for the keys it lacks, so only what fr lacks too is missing.
	want["fr-ca"] = []string{"only_en"}
	if got := c.Missing(); !reflect.DeepEqual(got, want) {
		t.Fatalf("missing %v, want %v", got, want)
	}
	if got := c.VerbMismatches(); len(got) != 0 {
		t.Fatalf("no mismatch expected: %v", got)
	}
	c.Add("fr", map[string]string{"count": "%d sur", "pct": "sûr à 100%d"})
	c.Add("de", map[string]string{"save": "Speichern", "count": "%s von %d", "pct": "100%% sicher"})
	got := c.VerbMismatches()
	if !reflect.DeepEqual(got, []string{"de count", "fr count", "fr pct"}) {
		t.Fatalf("mismatches %v", got)
	}
}

func TestFmtVerbs(t *testing.T) {
	for in, want := range map[string]string{
		"":               "",
		"plain":          "",
		"%d of %s":       "%d%s",
		"100%% %5.1f":    "%f",
		"%[1]d %-8s %+v": "%d%s%v",
		"trailing %":     "",
		"%%":             "",
		"a%dz%%%s":       "%d%s",
		"%*d":            "%d",
	} {
		if got := fmtVerbs(in); got != want {
			t.Errorf("fmtVerbs(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNegotiateLocale(t *testing.T) {
	avail := []string{"en", "fr", "de-CH"}
	cases := []struct {
		req  []string
		want string
	}{
		{nil, "en"},
		{[]string{"fr-CA", "en"}, "fr"},
		{[]string{"FR_fr"}, "fr"},
		{[]string{"de"}, "de-CH"},
		{[]string{"de-ch;q=0.8"}, "de-CH"},
		{[]string{"", "*", "ja", "fr"}, "fr"},
		{[]string{"ja", "zh-TW"}, "en"},
		{[]string{"es", "de-AT", "fr"}, "de-CH"},
	}
	for _, c := range cases {
		if got := NegotiateLocale(c.req, avail, "en"); got != c.want {
			t.Errorf("NegotiateLocale(%v) = %q, want %q", c.req, got, c.want)
		}
	}
	if got := testCatalog().Negotiate([]string{"fr-BE"}); got != "fr" {
		t.Fatalf("Catalog.Negotiate: %q", got)
	}
	if got := testCatalog().Negotiate([]string{"ja"}); got != "en" {
		t.Fatalf("Catalog.Negotiate fallback: %q", got)
	}
}
