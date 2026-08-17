// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package formula

import "testing"

// testEnv is a minimal Env for exercising the lexer/parser/evaluator without a
// full Model: a bounded grid backed by a plain map.
type testEnv struct {
	cols, rows int
	cells      map[Ref]Value
}

func (e testEnv) Get(r Ref) Value {
	if v, ok := e.cells[r]; ok {
		return v
	}
	return Blank()
}

func (e testEnv) InBounds(r Ref) bool {
	return r.Col >= 0 && r.Col < e.cols && r.Row >= 0 && r.Row < e.rows
}

// evalOK parses and evaluates body against env, failing the test if it does not
// parse.
func evalOK(t *testing.T, body string, env Env) Value {
	t.Helper()
	n, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse(%q) unexpected error: %v", body, err)
	}
	return eval(n, env)
}

// wantNum asserts body evaluates to the number want.
func wantNum(t *testing.T, env Env, body string, want float64) {
	t.Helper()
	got := evalOK(t, body, env)
	if got.Kind != KindNumber || got.Num != want {
		t.Errorf("%q = %+v, want number %v", body, got, want)
	}
}

// wantErr asserts body evaluates to the error kind want.
func wantErr(t *testing.T, env Env, body string, want ErrKind) {
	t.Helper()
	got := evalOK(t, body, env)
	if !got.IsError() || got.Err != want {
		t.Errorf("%q = %+v, want error %s", body, got, want)
	}
}

func TestArithmeticPrecedence(t *testing.T) {
	env := testEnv{cols: 4, rows: 4}
	wantNum(t, env, "1+2*3", 7)     // * before +
	wantNum(t, env, "(1+2)*3", 9)   // parens override
	wantNum(t, env, "10-2-3", 5)    // - left-associative
	wantNum(t, env, "8/4/2", 1)     // / left-associative
	wantNum(t, env, "2^3^2", 512)   // ^ right-associative
	wantNum(t, env, "-2^2", -4)     // unary minus looser than ^
	wantNum(t, env, "2^-1", 0.5)    // unary in the exponent
	wantNum(t, env, "--1", 1)       // stacked unary minus
	wantNum(t, env, "+1", 1)        // unary plus
	wantNum(t, env, "2*3+4*5", 26)  // both * bind before +
	wantNum(t, env, " 1\t+  2 ", 3) // whitespace + tab ignored
}

func TestComparisons(t *testing.T) {
	env := testEnv{cols: 4, rows: 4}
	wantNum(t, env, "1<2", 1)
	wantNum(t, env, "2<2", 0)
	wantNum(t, env, "2<=2", 1)
	wantNum(t, env, "3>2", 1)
	wantNum(t, env, "2>3", 0)
	wantNum(t, env, "3>=3", 1)
	wantNum(t, env, "1=1", 1)
	wantNum(t, env, "1=2", 0)
	wantNum(t, env, "1<>2", 1)
	wantNum(t, env, "1<>1", 0)
	// Text comparisons take the string path.
	wantNum(t, env, `"a"<"b"`, 1)
	wantNum(t, env, `"b">"a"`, 1)
	wantNum(t, env, `"a"="a"`, 1)
	wantNum(t, env, `"a"="b"`, 0)
	wantNum(t, env, `"a"=1`, 0) // number 1 -> "1" != "a"
}

func TestStringLiteralsAndEscapes(t *testing.T) {
	env := testEnv{cols: 4, rows: 4}
	got := evalOK(t, `"a""b"`, env) // "" is an embedded quote
	if got.Kind != KindText || got.Text != `a"b` {
		t.Errorf(`"a""b" = %+v, want text a"b`, got)
	}
}

func TestNumericErrorResults(t *testing.T) {
	env := testEnv{cols: 4, rows: 4}
	wantErr(t, env, "1/0", ErrDiv0)
	wantErr(t, env, "(0-8)^0.5", ErrNum) // NaN
	wantErr(t, env, "9^999", ErrNum)     // +Inf
	wantErr(t, env, `"x"+1`, ErrValue)   // text in arithmetic
	wantErr(t, env, `-"x"`, ErrValue)    // unary on text
	wantErr(t, env, "FOO", ErrName)      // bare unknown name
	wantErr(t, env, "A1:B2", ErrValue)   // bare range in scalar position
}

func TestReferenceEval(t *testing.T) {
	env := testEnv{cols: 2, rows: 2, cells: map[Ref]Value{
		{0, 0}: Number(10),
		{1, 0}: Number(5),
	}}
	wantNum(t, env, "A1+B1", 15)
	wantNum(t, env, "A2+1", 1)    // A2 is blank -> 0
	wantErr(t, env, "C1", ErrRef) // column beyond the 2-wide sheet
	wantErr(t, env, "A3", ErrRef) // row beyond the 2-tall sheet
}

func TestParseErrors(t *testing.T) {
	bad := []string{
		"1+",     // trailing operator
		"*2",     // leading binary operator
		"(1",     // unclosed paren
		"1 2",    // two atoms, no operator
		"SUM(1",  // unclosed call
		"A1:",    // colon then EOF
		"A1:5",   // colon then a number
		"A1:FOO", // colon then an invalid ref
		"FOO:A1", // invalid left ref of a range
		`"abc`,   // unterminated string
		".",      // malformed number
		"1.2.3",  // malformed number
		"@",      // character that starts no token
		"-*",     // prefix operator then a bad operand (parseUnary error arm)
		"2^",     // caret with no exponent (parsePower error arm)
		"()",     // empty parentheses (parsePrimary sub-expression error arm)
		"SUM(*)", // a call argument that does not parse (parseCall error arm)
	}
	for _, s := range bad {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) = nil error, want syntax error", s)
		}
	}
}

func TestParseWellFormedCalls(t *testing.T) {
	env := testEnv{cols: 4, rows: 4}
	wantNum(t, env, "SUM()", 0)      // empty argument list
	wantNum(t, env, "SUM(1,2,3)", 6) // comma-separated arguments
}
