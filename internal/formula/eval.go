// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package formula

import "math"

// Env is the sheet an expression reads cell values from. Get returns the
// current value of an in-bounds cell (a blank for a never-written one);
// InBounds reports whether a ref lies inside the grid, so an out-of-range
// reference or range corner can be turned into #REF!.
type Env interface {
	Get(r Ref) Value
	InBounds(r Ref) bool
}

// eval evaluates an AST node against env. An error operand always short-circuits
// to that error (Excel-style propagation), so a #DIV/0! deep in a sum surfaces
// unchanged at the top.
func eval(n *node, env Env) Value {
	switch n.kind {
	case nNum:
		return Number(n.num)
	case nStr:
		return TextValue(n.str)
	case nName:
		return Error(ErrName)
	case nRef:
		if !env.InBounds(n.ref) {
			return Error(ErrRef)
		}
		return env.Get(n.ref)
	case nRange:
		return Error(ErrValue) // a range is only valid as a function argument
	case nUnary:
		return evalUnary(n, env)
	case nBin:
		return evalBinary(n, env)
	case nCall:
		return evalCall(n, env)
	}
	return Error(ErrValue)
}

// evalUnary applies a prefix + or - to a numeric operand.
func evalUnary(n *node, env Env) Value {
	x := eval(n.x, env)
	if x.Kind == KindError {
		return x
	}
	num, ok := asNumber(x)
	if !ok {
		return Error(ErrValue)
	}
	switch n.op {
	case tMinus:
		return Number(-num)
	case tPlus:
		return Number(num)
	}
	return Error(ErrValue)
}

// evalBinary applies a binary operator. Comparisons go through evalCompare;
// arithmetic coerces both operands to numbers (a non-numeric operand is
// #VALUE!), with an explicit #DIV/0! guard and #NUM! for a blown-up result.
func evalBinary(n *node, env Env) Value {
	l := eval(n.l, env)
	if l.Kind == KindError {
		return l
	}
	r := eval(n.r, env)
	if r.Kind == KindError {
		return r
	}
	switch n.op {
	case tEq, tNe, tLt, tGt, tLe, tGe:
		return evalCompare(n.op, l, r)
	}
	ln, lok := asNumber(l)
	rn, rok := asNumber(r)
	if !lok || !rok {
		return Error(ErrValue)
	}
	switch n.op {
	case tPlus:
		return numResult(ln + rn)
	case tMinus:
		return numResult(ln - rn)
	case tStar:
		return numResult(ln * rn)
	case tSlash:
		if rn == 0 {
			return Error(ErrDiv0)
		}
		return numResult(ln / rn)
	case tCaret:
		return numResult(math.Pow(ln, rn))
	}
	return Error(ErrValue)
}

// evalCompare returns 1 (true) or 0 (false) for a comparison. When both
// operands coerce to numbers it compares them numerically; otherwise it
// compares their text forms, so both A1>5 and A1="yes" work.
func evalCompare(op tokKind, l, r Value) Value {
	cmp := 0
	if ln, lok := asNumber(l); lok {
		if rn, rok := asNumber(r); rok {
			cmp = compareFloat(ln, rn)
			return boolValue(compareResult(op, cmp))
		}
	}
	cmp = compareString(asText(l), asText(r))
	return boolValue(compareResult(op, cmp))
}

// compareFloat is the three-way comparison of two floats.
func compareFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// compareString is the three-way comparison of two strings.
func compareString(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// compareResult maps a three-way comparison result onto the boolean the
// comparison operator asks for.
func compareResult(op tokKind, cmp int) bool {
	switch op {
	case tEq:
		return cmp == 0
	case tNe:
		return cmp != 0
	case tLt:
		return cmp < 0
	case tGt:
		return cmp > 0
	case tLe:
		return cmp <= 0
	case tGe:
		return cmp >= 0
	}
	return false
}

// boolValue is the numeric spreadsheet spelling of a boolean: 1 or 0.
func boolValue(b bool) Value {
	if b {
		return Number(1)
	}
	return Number(0)
}
