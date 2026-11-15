// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// ScanAccessibleBaseTypes hands the module-wide completeness scan to the
// external test package, which can import the sub-packages the scan must cover.
var ScanAccessibleBaseTypes = scanAccessibleBaseTypes

// ScanBaseTypesWithoutA11y is the same scan's refusal half: the Base types that
// answer no A11y().
var ScanBaseTypesWithoutA11y = scanBaseTypesWithoutA11y
