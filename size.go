// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// Size is a width/height pair in painter units (pixels for a PixelPainter,
// cells for a CellPainter). It is the dimensional companion of Rect for the
// places a widget needs an extent without a position — e.g. the fixed cell
// footprint a VirtualGrid reflows its items into. Kept a plain value type
// (no methods, no state) so it costs nothing and composes freely.
type Size struct{ W, H int }
