// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package formula

import "testing"

// TestBinaryErrorOperandsShortCircuit covers the left- and right-operand error
// propagation arms of evalBinary.
func TestBinaryErrorOperandsShortCircuit(t *testing.T) {
	env := testEnv{cols: 2, rows: 2}
	wantErr(t, env, "1/0+2", ErrDiv0)  // left operand errors
	wantErr(t, env, "2+1/0", ErrDiv0)  // right operand errors
	wantErr(t, env, "-(1/0)", ErrDiv0) // unary minus over an error operand
}

// TestCompareMixedNumericText covers the branch where the left operand is
// numeric but the right is not, forcing the text comparison path.
func TestCompareMixedNumericText(t *testing.T) {
	env := testEnv{cols: 2, rows: 2}
	wantNum(t, env, `1="a"`, 0) // "1" != "a" via text compare
}

// TestCraftedNodeDefaults reaches the defensive default arms that a
// well-formed parse never produces, by evaluating hand-built nodes with
// out-of-range kinds/operators.
func TestCraftedNodeDefaults(t *testing.T) {
	env := testEnv{cols: 2, rows: 2}
	one := &node{kind: nNum, num: 1}

	if got := eval(&node{kind: nodeKind(99)}, env); got.Err != ErrValue {
		t.Errorf("eval(bogus kind) = %+v, want #VALUE!", got)
	}
	if got := evalUnary(&node{kind: nUnary, op: tStar, x: one}, env); got.Err != ErrValue {
		t.Errorf("evalUnary(bogus op) = %+v, want #VALUE!", got)
	}
	if got := evalBinary(&node{kind: nBin, op: tLParen, l: one, r: one}, env); got.Err != ErrValue {
		t.Errorf("evalBinary(bogus op) = %+v, want #VALUE!", got)
	}
	if compareResult(tLParen, 0) {
		t.Error("compareResult(bogus op) = true, want false")
	}
}

// TestBoolValue covers both spellings of a boolean result.
func TestBoolValue(t *testing.T) {
	if got := boolValue(true); got.Num != 1 {
		t.Errorf("boolValue(true) = %+v, want 1", got)
	}
	if got := boolValue(false); got.Num != 0 {
		t.Errorf("boolValue(false) = %+v, want 0", got)
	}
}
