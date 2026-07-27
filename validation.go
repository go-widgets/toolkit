// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"errors"
	"regexp"
)

// Rule validates a single string value. It returns a non-nil error --
// whose Error() text is the message shown to the user (e.g. via
// FormField.Error) -- when value fails the rule's check, or nil when
// value passes.
type Rule func(value string) error

// Required rejects an empty value.
func Required(msg string) Rule {
	return func(value string) error {
		if value == "" {
			return errors.New(msg)
		}
		return nil
	}
}

// MinLen rejects a value with fewer than n runes.
func MinLen(n int, msg string) Rule {
	return func(value string) error {
		if len([]rune(value)) < n {
			return errors.New(msg)
		}
		return nil
	}
}

// MaxLen rejects a value with more than n runes.
func MaxLen(n int, msg string) Rule {
	return func(value string) error {
		if len([]rune(value)) > n {
			return errors.New(msg)
		}
		return nil
	}
}

// Pattern rejects a value that does not match re.
func Pattern(re *regexp.Regexp, msg string) Rule {
	return func(value string) error {
		if !re.MatchString(value) {
			return errors.New(msg)
		}
		return nil
	}
}

// emailPattern is a reasonable, non-pedantic email shape check: a
// local part, "@", then a domain with at least one dot. It does not
// implement the full RFC 5322 grammar -- that would reject plenty of
// addresses real users type -- it's the same "good enough" heuristic
// most form libraries ship.
var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// Email rejects a value that does not look like an email address.
func Email(msg string) Rule {
	return Pattern(emailPattern, msg)
}

// All combines rules into a single Rule that runs them in order and
// fails on the first one that fails, discarding the rest -- the same
// short-circuit semantics as Validate. Useful for grouping a related
// set of rules (e.g. a password policy) behind one Rule value.
func All(rules ...Rule) Rule {
	return func(value string) error {
		return Validate(value, rules...)
	}
}

// Validate runs rules against value in order and returns the first
// non-nil error. It returns nil when every rule passes, including
// when rules is empty.
func Validate(value string, rules ...Rule) error {
	for _, r := range rules {
		if err := r(value); err != nil {
			return err
		}
	}
	return nil
}
