// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"regexp"
	"testing"
)

// --- Required --------------------------------------------------------------

func TestRequiredPassesOnNonEmpty(t *testing.T) {
	if err := Required("required")("x"); err != nil {
		t.Fatalf("Required(\"x\") = %v, want nil", err)
	}
}

func TestRequiredFailsOnEmpty(t *testing.T) {
	err := Required("required")("")
	if err == nil {
		t.Fatal("Required(\"\") = nil, want error")
	}
	if err.Error() != "required" {
		t.Fatalf("Required error = %q, want %q", err.Error(), "required")
	}
}

// --- MinLen ------------------------------------------------------------

func TestMinLenPassesAtBoundary(t *testing.T) {
	if err := MinLen(3, "too short")("abc"); err != nil {
		t.Fatalf("MinLen(3)(\"abc\") = %v, want nil", err)
	}
}

func TestMinLenFailsBelowBoundary(t *testing.T) {
	err := MinLen(3, "too short")("ab")
	if err == nil {
		t.Fatal("MinLen(3)(\"ab\") = nil, want error")
	}
	if err.Error() != "too short" {
		t.Fatalf("MinLen error = %q, want %q", err.Error(), "too short")
	}
}

// --- MaxLen ------------------------------------------------------------

func TestMaxLenPassesAtBoundary(t *testing.T) {
	if err := MaxLen(3, "too long")("abc"); err != nil {
		t.Fatalf("MaxLen(3)(\"abc\") = %v, want nil", err)
	}
}

func TestMaxLenFailsAboveBoundary(t *testing.T) {
	err := MaxLen(3, "too long")("abcd")
	if err == nil {
		t.Fatal("MaxLen(3)(\"abcd\") = nil, want error")
	}
	if err.Error() != "too long" {
		t.Fatalf("MaxLen error = %q, want %q", err.Error(), "too long")
	}
}

// --- Pattern -----------------------------------------------------------

func TestPatternMatches(t *testing.T) {
	re := regexp.MustCompile(`^[0-9]+$`)
	if err := Pattern(re, "digits only")("12345"); err != nil {
		t.Fatalf("Pattern match = %v, want nil", err)
	}
}

func TestPatternNoMatch(t *testing.T) {
	re := regexp.MustCompile(`^[0-9]+$`)
	err := Pattern(re, "digits only")("12a45")
	if err == nil {
		t.Fatal("Pattern no-match = nil, want error")
	}
	if err.Error() != "digits only" {
		t.Fatalf("Pattern error = %q, want %q", err.Error(), "digits only")
	}
}

// --- Email ---------------------------------------------------------------

func TestEmailMatches(t *testing.T) {
	if err := Email("bad email")("a@b.com"); err != nil {
		t.Fatalf("Email(\"a@b.com\") = %v, want nil", err)
	}
}

func TestEmailNoMatch(t *testing.T) {
	err := Email("bad email")("not-an-email")
	if err == nil {
		t.Fatal("Email(\"not-an-email\") = nil, want error")
	}
	if err.Error() != "bad email" {
		t.Fatalf("Email error = %q, want %q", err.Error(), "bad email")
	}
}

// --- All -----------------------------------------------------------------

// All returns the first failing rule's error, short-circuiting the rest.
func TestAllReturnsFirstFailure(t *testing.T) {
	rule := All(
		Required("required"),
		MinLen(5, "too short"),
	)
	err := rule("ab")
	if err == nil {
		t.Fatal("All(...)(\"ab\") = nil, want error")
	}
	if err.Error() != "too short" {
		t.Fatalf("All error = %q, want %q", err.Error(), "too short")
	}
}

// All returns nil when every combined rule passes.
func TestAllPassesWhenAllRulesPass(t *testing.T) {
	rule := All(
		Required("required"),
		MinLen(2, "too short"),
		MaxLen(5, "too long"),
	)
	if err := rule("abc"); err != nil {
		t.Fatalf("All(...)(\"abc\") = %v, want nil", err)
	}
}

// --- Validate --------------------------------------------------------------

// Validate stops at + returns the first failing rule's error.
func TestValidateReturnsFirstFailure(t *testing.T) {
	err := Validate("",
		MinLen(1, "empty"),
		MaxLen(3, "too long"),
	)
	if err == nil {
		t.Fatal("Validate(\"\") = nil, want error")
	}
	if err.Error() != "empty" {
		t.Fatalf("Validate error = %q, want %q", err.Error(), "empty")
	}
}

// Validate returns nil when every rule passes.
func TestValidatePassesWhenAllRulesPass(t *testing.T) {
	err := Validate("abc",
		Required("required"),
		MaxLen(5, "too long"),
	)
	if err != nil {
		t.Fatalf("Validate(\"abc\") = %v, want nil", err)
	}
}

// Validate with no rules always passes.
func TestValidateWithNoRulesPasses(t *testing.T) {
	if err := Validate("anything"); err != nil {
		t.Fatalf("Validate with no rules = %v, want nil", err)
	}
}
