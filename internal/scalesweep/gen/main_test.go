// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWritesTheTable(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "a.go"), []byte("package p\ntype Foo struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(d, "out_test.go")
	var se bytes.Buffer
	if code := run(d, out, &se); code != 0 {
		t.Fatalf("exit %d: %s", code, se.String())
	}
	b, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(b), `"Foo"`) {
		t.Fatalf("table not written: %v %s", err, b)
	}
}

func TestRunReportsFailures(t *testing.T) {
	var se bytes.Buffer
	if code := run("[", "x", &se); code != 1 || se.Len() == 0 {
		t.Fatalf("bad dir: code %d, stderr %q", code, se.String())
	}
	se.Reset()
	d := t.TempDir()
	if code := run(d, filepath.Join(d, "no", "such", "x.go"), &se); code != 1 || se.Len() == 0 {
		t.Fatalf("unwritable output: code %d, stderr %q", code, se.String())
	}
}

func TestMainExitsNonZeroOnFailure(t *testing.T) {
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("bad.go", []byte("package p\nfunc ("), 0o644); err != nil {
		t.Fatal(err)
	}
	var got int
	osExit = func(c int) { got = c }
	defer func() { osExit = os.Exit }()
	main()
	if got != 1 {
		t.Fatalf("exit %d, want 1", got)
	}
	os.Remove("bad.go")
	main() // success path writes the table here and does not exit
	if _, err := os.Stat("scalesweep_types_gen_test.go"); err != nil {
		t.Fatal(err)
	}
}
