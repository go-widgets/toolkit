// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Command gen writes scalesweep_types_gen_test.go in the toolkit root. Run it
// through `go generate` (see scalesweep_test.go) from the repository root.
package main

import (
	"fmt"
	"os"

	"github.com/go-widgets/toolkit/internal/scalesweep"
)

func main() {
	names, err := scalesweep.Types(".")
	if err == nil {
		err = os.WriteFile("scalesweep_types_gen_test.go", scalesweep.Generate(names), 0o644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
