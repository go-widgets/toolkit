// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Command isodemo renders the toolkit's isometric-diagram showcase to PNG files.
//
// It writes five images into the directory named by -out (the working directory
// by default): the full scene, the same scene with a layer hidden, the scene
// with a multi-node selection active, and the two CRDT replicas after
// convergence (which are byte-identical). It opens no window and needs no
// display server — the scenes render through the toolkit's headless capture
// path, so the command runs anywhere Go builds.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/go-widgets/toolkit/examples/isodemo"
)

// osExit is a seam over os.Exit so run's exit code can be asserted in a test.
var osExit = os.Exit

func main() { osExit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// run parses args, generates the showcase PNGs and reports the paths written to
// outw, returning the process exit code. Diagnostics go to errw.
func run(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("isodemo", flag.ContinueOnError)
	fs.SetOutput(errw)
	dir := fs.String("out", ".", "directory to write the showcase PNGs into")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	paths, err := isodemo.Generate(*dir)
	if err != nil {
		fmt.Fprintf(errw, "isodemo: %v\n", err)
		return 1
	}
	for _, p := range paths {
		fmt.Fprintln(outw, p)
	}
	return 0
}
