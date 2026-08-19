// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
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

// TestRunWritesToOutDir exercises the success path: run generates the five
// showcase PNGs into -out and prints their paths, returning exit code 0.
func TestRunWritesToOutDir(t *testing.T) {
	dir := t.TempDir()
	var out, errb bytes.Buffer
	if code := run([]string{"-out", dir}, &out, &errb); code != 0 {
		t.Fatalf("run exit = %d, stderr=%q", code, errb.String())
	}
	lines := strings.Fields(out.String())
	if len(lines) != 5 {
		t.Fatalf("printed %d paths, want 5: %q", len(lines), out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "scene.png")); err != nil {
		t.Fatalf("scene.png not written: %v", err)
	}
}

// TestRunFlagError covers the flag-parse failure branch (exit 2).
func TestRunFlagError(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"-nope"}, &out, &errb); code != 2 {
		t.Fatalf("bad flag exit = %d, want 2", code)
	}
}

// TestRunGenerateError covers the generation-failure branch (exit 1) by pointing
// -out at a directory that does not exist.
func TestRunGenerateError(t *testing.T) {
	var out, errb bytes.Buffer
	missing := filepath.Join(t.TempDir(), "no-such-dir")
	if code := run([]string{"-out", missing}, &out, &errb); code != 1 {
		t.Fatalf("generate-error exit = %d, want 1", code)
	}
	if !strings.Contains(errb.String(), "isodemo:") {
		t.Fatalf("stderr missing diagnostic: %q", errb.String())
	}
}

// TestMainSeam runs main() with the exit seam stubbed and os.Args pointed at a
// temp output dir, so the real entry point is covered without exiting the test
// process.
func TestMainSeam(t *testing.T) {
	dir := t.TempDir()
	oldArgs, oldExit := os.Args, osExit
	defer func() { os.Args, osExit = oldArgs, oldExit }()

	got := -1
	osExit = func(code int) { got = code }
	os.Args = []string{"isodemo", "-out", dir}
	main()
	if got != 0 {
		t.Fatalf("main exit = %d, want 0", got)
	}
}
