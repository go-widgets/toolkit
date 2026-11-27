// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package scalesweep

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, src string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTypesFindsExportedStructsAndConstructors(t *testing.T) {
	d := t.TempDir()
	write(t, d, "a.go", `package p
type Foo struct{}
type bar struct{}
type Alias = Foo
type Gen[T any] struct{}
type Iface interface{}
func NewFoo() *Foo { return nil }
type Baz struct{}
func NewBaz(x int) *Baz { return nil }
type Lst struct{}
func NewLst(items []string, a, b string, on bool, v ...int) *Lst { return nil }
type Own struct{}
func NewOwn(m Mode, f func(), p *Foo, i any, mp map[int]int, c chan int, w interface{}) *Own { return nil }
type Ext struct{}
func NewExt(c other.Thing) *Ext { return nil }
type Mode int
`)
	write(t, d, "a_test.go", "package p\ntype InTest struct{}\n")
	write(t, d, "b.go", "//go:build linux\n\npackage p\ntype Constrained struct{}\n")
	got, err := Types(d)
	if err != nil {
		t.Fatal(err)
	}
	want := []Type{
		{"Baz", true, "0", false},
		{"Ext", false, "", true},
		{"Foo", true, "", false},
		{"Lst", true, `nil, "", "", false`, false},
		{"Own", true, "zero[Mode](), nil, nil, zero[any](), nil, nil, nil", false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Types = %v, want %v", got, want)
	}
	gen := string(Generate(got))
	for _, s := range []string{
		`"Foo", func() any { return NewFoo() }`,
		`"Baz", func() any { return NewBaz(0) }`,
		`"Lst", func() any { return NewLst(nil, "", "", false) }`,
		`"Ext", func() any { return new(Ext) }`,
		`var sweepNeedsArgs = []string{"Ext"}`,
	} {
		if !strings.Contains(gen, s) {
			t.Errorf("Generate lacks %q:\n%s", s, gen)
		}
	}
}

func TestTypesReportsEachFailure(t *testing.T) {
	if _, err := Types("["); err == nil {
		t.Error("a malformed glob pattern must be an error")
	}
	d := t.TempDir()
	if err := os.Mkdir(filepath.Join(d, "dir.go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Types(d); err == nil {
		t.Error("an unreadable .go entry must be an error")
	} else if strings.Contains(err.Error(), "expected") {
		t.Errorf("the read failure was reported as a parse failure: %v", err)
	}
	d = t.TempDir()
	write(t, d, "bad.go", "package p\nfunc (")
	if _, err := Types(d); err == nil {
		t.Error("a file that does not parse must be an error")
	}
}
