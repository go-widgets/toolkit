// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// must is what a test says to a reader that can refuse.
//
// OpsSince returns an error below what a replica has collected, and nothing
// here collects, so it can never be seen. Saying so once keeps every call site
// from saying it, and turns the case that cannot happen into a failure rather
// than a value quietly thrown away.
func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
