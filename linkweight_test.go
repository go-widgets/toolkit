// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func listDeps(t *testing.T, pkg string) map[string]bool {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", pkg)
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("go list unavailable: %v", err)
	}
	m := map[string]bool{}
	for _, l := range strings.Fields(string(out)) {
		m[l] = true
	}
	return m
}

// TestRootImportsNoInitHeavyPackage keeps package toolkit free of imports whose
// init functions drag code into every program: the linker treats each imported
// package's init task as a root (cmd/link/internal/ld/deadcode.go), so an
// import is linked weight whether or not a widget using it is ever built.
// collabui is the planted positive — it must show the session package, or this
// check could not see it.
func TestRootImportsNoInitHeavyPackage(t *testing.T) {
	heavy := []string{
		"github.com/go-crdt/collab",     // transports: net/http, crypto/tls (gRPC natively)
		"github.com/go-images/images",   // every decoder self-registers in init
		"github.com/go-gfx/gfx/codec",   // the decoder registry itself
		"github.com/go-images/jpeg2000", // ~350 KB of wasm code via its init
	}
	if !listDeps(t, "./collabui")["github.com/go-crdt/collab"] {
		t.Fatal("control failed: collabui does not import go-crdt/collab, so this check sees nothing")
	}
	root := listDeps(t, ".")
	for _, h := range heavy {
		if root[h] {
			t.Errorf("package toolkit imports %s: every toolkit program would link it", h)
		}
	}
}
