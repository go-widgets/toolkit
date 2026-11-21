// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"sort"
	"strings"
)

// A Catalog is an application's messages in several languages: one table of
// key -> text per locale, and the locale that answers for the others (the
// fallback, usually "en"). The toolkit's own words stay with SetLocalizer/Tr;
// a Catalog is for the words the APPLICATION writes, and plugs into the same
// hook:
//
//	cat := toolkit.NewCatalog("en").
//		Add("en", map[string]string{"save": "Save", "count": "%d items"}).
//		Add("fr", map[string]string{"save": "Enregistrer", "count": "%d éléments"})
//	loc := cat.Negotiate(browserLanguages) // "fr" for ["fr-CA", "en"]
//	toolkit.SetLocalizer(cat.Localizer(loc))
//
// A key a locale lacks is answered by the fallback's text, so a half-translated
// table degrades to English instead of showing a key; Missing and
// VerbMismatches list what is half-translated, and the i18ntest package turns
// them into a failing test.
type Catalog struct {
	fallback string
	tables   map[string]map[string]string
}

// NewCatalog is an empty catalog whose fallback locale is fallback (a tag such
// as "en"; it is matched without regard to case).
func NewCatalog(fallback string) *Catalog {
	return &Catalog{fallback: canonicalLocale(fallback), tables: map[string]map[string]string{}}
}

// canonicalLocale is a locale tag with "-" separators and lower case, the form
// the catalog compares in ("fr_CA" and "FR-ca" are both "fr-ca").
func canonicalLocale(tag string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tag), "_", "-"))
}

// Add merges msgs into the table of locale (a later key replaces an earlier
// one) and returns the catalog, so calls chain.
func (c *Catalog) Add(locale string, msgs map[string]string) *Catalog {
	loc := canonicalLocale(locale)
	t := c.tables[loc]
	if t == nil {
		t = make(map[string]string, len(msgs))
		c.tables[loc] = t
	}
	for k, v := range msgs {
		t[k] = v
	}
	return c
}

// Fallback is the locale that answers for the others.
func (c *Catalog) Fallback() string { return c.fallback }

// Locales are the locales that have a table, sorted.
func (c *Catalog) Locales() []string {
	out := make([]string, 0, len(c.tables))
	for l := range c.tables {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// Keys are every key of every table, sorted and without repeats.
func (c *Catalog) Keys() []string {
	seen := map[string]bool{}
	for _, t := range c.tables {
		for k := range t {
			seen[k] = true
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Lookup is the text of key in locale, from that locale's own table or, for a
// regional tag with no table ("fr-ca"), from its language's ("fr"). It does NOT
// fall back to the fallback locale: ok is false for a key the locale lacks.
func (c *Catalog) Lookup(locale, key string) (string, bool) {
	loc := canonicalLocale(locale)
	if s, ok := c.tables[loc][key]; ok {
		return s, true
	}
	if base, _, cut := strings.Cut(loc, "-"); cut {
		s, ok := c.tables[base][key]
		return s, ok
	}
	return "", false
}

// Text is the text of key in locale, else in the fallback locale, else "".
func (c *Catalog) Text(locale, key string) string {
	if s, ok := c.Lookup(locale, key); ok {
		return s
	}
	s, _ := c.Lookup(c.fallback, key)
	return s
}

// Localizer is the function SetLocalizer takes for locale: the catalog's text,
// else the fallback locale's, else "" (Tr then answers its English argument).
func (c *Catalog) Localizer(locale string) func(key, english string) string {
	return func(key, _ string) string { return c.Text(locale, key) }
}

// Negotiate picks, among the catalog's locales, the best answer to the
// requested languages (a browser's navigator.languages, most wanted first), or
// the fallback when none matches. See NegotiateLocale.
func (c *Catalog) Negotiate(requested []string) string {
	return NegotiateLocale(requested, c.Locales(), c.fallback)
}

// Missing lists, per locale, the keys of the catalog that locale has no text
// for (its regional fallback to the language table counts as having one). A
// complete catalog returns an empty map.
func (c *Catalog) Missing() map[string][]string {
	out := map[string][]string{}
	keys := c.Keys()
	for _, loc := range c.Locales() {
		for _, k := range keys {
			if _, ok := c.Lookup(loc, k); !ok {
				out[loc] = append(out[loc], k)
			}
		}
	}
	return out
}

// VerbMismatches lists the "locale key" pairs whose text does not carry the
// same fmt verbs, in the same order, as the fallback locale's: a translation
// that drops or reorders a %d would print garbage or panic in Sprintf.
func (c *Catalog) VerbMismatches() []string {
	var out []string
	base := c.tables[c.fallback]
	for _, loc := range c.Locales() {
		if loc == c.fallback {
			continue
		}
		for _, k := range c.Keys() {
			want, ok1 := base[k]
			got, ok2 := c.tables[loc][k]
			if ok1 && ok2 && fmtVerbs(want) != fmtVerbs(got) {
				out = append(out, loc+" "+k)
			}
		}
	}
	return out
}

// fmtVerbs is the sequence of verbs in s ("%d%s" for "%d of %s"), "%%" aside.
func fmtVerbs(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		j := i + 1
		for j < len(s) && strings.IndexByte("+-# 0123456789.*[]", s[j]) >= 0 {
			j++
		}
		if j < len(s) && s[j] != '%' {
			b.WriteByte('%')
			b.WriteByte(s[j])
		}
		i = j
	}
	return b.String()
}

// NegotiateLocale is the entry of available that best answers requested, most
// wanted first (a browser's navigator.languages): for each requested tag, an
// exact match (without regard to case, "_" read as "-"), else an available
// locale of the same language ("fr-CA" is answered by "fr", "fr" by "fr-FR");
// the first requested tag that is answered wins. None: fallback. A tag with a
// ";q=" weight is read without it.
func NegotiateLocale(requested, available []string, fallback string) string {
	for _, r := range requested {
		tag, _, _ := strings.Cut(r, ";")
		tag = canonicalLocale(tag)
		if tag == "" || tag == "*" {
			continue
		}
		for _, a := range available {
			if canonicalLocale(a) == tag {
				return a
			}
		}
		lang, _, _ := strings.Cut(tag, "-")
		for _, a := range available {
			if al, _, _ := strings.Cut(canonicalLocale(a), "-"); al == lang {
				return a
			}
		}
	}
	return fallback
}
