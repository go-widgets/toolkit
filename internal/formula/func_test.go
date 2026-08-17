// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package formula

import "testing"

// numbersEnv is a 3x3 sheet whose column A holds 1,2,3 and whose B1/C1 hold
// 10/100 — the fixture the aggregate tests read.
func numbersEnv() testEnv {
	return testEnv{cols: 3, rows: 3, cells: map[Ref]Value{
		{0, 0}: Number(1), {0, 1}: Number(2), {0, 2}: Number(3),
		{1, 0}: Number(10),
		{2, 0}: Number(100),
	}}
}

func TestAggregateFunctions(t *testing.T) {
	env := numbersEnv()
	wantNum(t, env, "SUM(A1:A3)", 6)
	wantNum(t, env, "SUM(1,2,3)", 6)
	wantNum(t, env, "SUM(A1:A3,B1,C1)", 116)
	wantNum(t, env, "AVG(A1:A3)", 2)
	wantNum(t, env, "AVERAGE(A1:A3)", 2) // alias
	wantNum(t, env, "MIN(A1:C1)", 1)
	wantNum(t, env, "MAX(A1:C1)", 100)
	wantNum(t, env, "COUNT(A1:C3)", 5)
	// Nested functions.
	wantNum(t, env, "SUM(A1:A3)+MAX(B1,C1)", 106)
	wantNum(t, env, "ROUND(AVG(A1:A3),0)", 2)
}

// TestAggregatesSkipNonNumbers covers numbersOf skipping text and blank cells.
func TestAggregatesSkipNonNumbers(t *testing.T) {
	env := testEnv{cols: 1, rows: 3, cells: map[Ref]Value{
		{0, 0}: Number(1),
		{0, 1}: TextValue("label"),
		// {0,2} left blank
	}}
	wantNum(t, env, "SUM(A1:A3)", 1)
	wantNum(t, env, "COUNT(A1:A3)", 1)
}

// TestMinMaxOrdering exercises both the update-min and no-update arms.
func TestMinMaxOrdering(t *testing.T) {
	env := testEnv{cols: 3, rows: 1, cells: map[Ref]Value{
		{0, 0}: Number(3), {1, 0}: Number(1), {2, 0}: Number(2),
	}}
	wantNum(t, env, "MIN(A1:C1)", 1) // 3 -> 1 (update) -> 2 (no update)
	env2 := testEnv{cols: 3, rows: 1, cells: map[Ref]Value{
		{0, 0}: Number(1), {1, 0}: Number(3), {2, 0}: Number(2),
	}}
	wantNum(t, env2, "MAX(A1:C1)", 3) // 1 -> 3 (update) -> 2 (no update)
}

// TestMinMaxEmpty covers the no-numbers arm returning 0.
func TestMinMaxEmpty(t *testing.T) {
	env := testEnv{cols: 1, rows: 1, cells: map[Ref]Value{{0, 0}: TextValue("x")}}
	wantNum(t, env, "MIN(A1:A1)", 0)
	wantNum(t, env, "MAX(A1:A1)", 0)
}

// TestAvgEmpty covers AVG of no numbers -> #DIV/0!.
func TestAvgEmpty(t *testing.T) {
	env := testEnv{cols: 1, rows: 1, cells: map[Ref]Value{{0, 0}: TextValue("x")}}
	wantErr(t, env, "AVG(A1:A1)", ErrDiv0)
}

// TestAggregateErrorPropagation covers every aggregate's collect-error arm:
// a scalar error argument, an out-of-bounds range corner (#REF!), and an error
// value sitting inside a range.
func TestAggregateErrorPropagation(t *testing.T) {
	env := numbersEnv()
	// Scalar error argument.
	wantErr(t, env, "SUM(1/0)", ErrDiv0)
	// Out-of-bounds range corner in each aggregate.
	wantErr(t, env, "SUM(A1:Z9)", ErrRef)
	wantErr(t, env, "AVG(A1:Z9)", ErrRef)
	wantErr(t, env, "MIN(A1:Z9)", ErrRef)
	wantErr(t, env, "MAX(A1:Z9)", ErrRef)
	wantErr(t, env, "COUNT(A1:Z9)", ErrRef)
	// An error value inside a range propagates.
	errEnv := testEnv{cols: 1, rows: 1, cells: map[Ref]Value{{0, 0}: Error(ErrDiv0)}}
	wantErr(t, errEnv, "SUM(A1:A1)", ErrDiv0)
}

// TestRangeCornerNormalisation covers reversed row and column corners.
func TestRangeCornerNormalisation(t *testing.T) {
	env := numbersEnv()
	wantNum(t, env, "SUM(A3:A1)", 6)   // rows reversed
	wantNum(t, env, "SUM(C1:A1)", 111) // columns reversed
}

func TestIf(t *testing.T) {
	env := testEnv{cols: 2, rows: 2}
	wantNum(t, env, "IF(1,10,20)", 10) // truthy
	wantNum(t, env, "IF(0,10,20)", 20) // falsy with else
	wantNum(t, env, "IF(0,10)", 0)     // falsy without else
	wantNum(t, env, "IF(2>1,10,20)", 10)
	wantErr(t, env, "IF(1)", ErrValue)       // too few args
	wantErr(t, env, "IF(1,2,3,4)", ErrValue) // too many args
	wantErr(t, env, "IF(1/0,2,3)", ErrDiv0)  // condition errors
	wantErr(t, env, `IF("a",2,3)`, ErrValue) // non-numeric condition
}

func TestAbs(t *testing.T) {
	env := testEnv{cols: 2, rows: 2}
	wantNum(t, env, "ABS(-5)", 5)
	wantNum(t, env, "ABS(5)", 5)
	wantErr(t, env, "ABS(1,2)", ErrValue) // wrong arity
	wantErr(t, env, "ABS(1/0)", ErrDiv0)  // error operand
	wantErr(t, env, `ABS("a")`, ErrValue) // non-numeric operand
}

func TestRound(t *testing.T) {
	env := testEnv{cols: 2, rows: 2}
	wantNum(t, env, "ROUND(3.14159,2)", 3.14)
	wantNum(t, env, "ROUND(2.5,0)", 3)
	wantErr(t, env, "ROUND(1)", ErrValue)     // wrong arity
	wantErr(t, env, "ROUND(1/0,2)", ErrDiv0)  // first operand errors
	wantErr(t, env, "ROUND(2,1/0)", ErrDiv0)  // second operand errors
	wantErr(t, env, `ROUND("a",2)`, ErrValue) // first not numeric
	wantErr(t, env, `ROUND(2,"a")`, ErrValue) // second not numeric
}

func TestUnknownFunction(t *testing.T) {
	env := testEnv{cols: 2, rows: 2}
	wantErr(t, env, "NOPE(1)", ErrName)
}
