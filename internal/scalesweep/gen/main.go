// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Command gen writes scalesweep_types_gen_test.go in the toolkit root. Run it
// through `go generate` (see scalesweep_test.go) from the repository root.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/go-widgets/toolkit/internal/scalesweep"
)

var osExit = os.Exit

func run(dir, out string, stderr io.Writer) int {
	names, err := scalesweep.Types(dir)
	if err == nil {
		err = os.WriteFile(out, scalesweep.Generate(names), 0o644)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func main() {
	if code := run(".", "scalesweep_types_gen_test.go", os.Stderr); code != 0 {
		osExit(code)
	}
}
