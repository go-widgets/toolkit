// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "strings"

// WrapLines breaks text into lines that each measure at most width pixels in
// font f, so no line runs past the edge of the area it is drawn in. It breaks at
// spaces and at "\n"; a word wider than width (a URL, a path, a key) is broken
// between runes rather than left to overflow. Every line holds at least one rune,
// so the result always terminates, even for a width narrower than one glyph.
// Empty text yields no lines; a blank line inside the text is kept.
func WrapLines(f Font, text string, width int) []string {
	if text == "" {
		return nil
	}
	var out []string
	for _, para := range strings.Split(text, "\n") {
		out = appendWrapped(out, f, para, width)
	}
	return out
}

// appendWrapped wraps one paragraph (no newline) onto out.
func appendWrapped(out []string, f Font, para string, width int) []string {
	words := strings.Fields(para)
	if len(words) == 0 {
		return append(out, "")
	}
	cur := ""
	for _, w := range words {
		if cur == "" {
			cur = w
		} else if f.Measure(cur+" "+w) <= width {
			cur += " " + w
			continue
		} else {
			out = append(out, cur)
			cur = w
		}
		// cur is a lone word: break it while it is wider than the line.
		for f.Measure(cur) > width {
			head, rest := splitFit(f, cur, width)
			out = append(out, head)
			cur = rest
			if cur == "" {
				break
			}
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// splitFit cuts the longest prefix of w that fits width (at least one rune).
func splitFit(f Font, w string, width int) (head, rest string) {
	runes := []rune(w)
	n := len(runes)
	for n > 1 && f.Measure(string(runes[:n])) > width {
		n--
	}
	return string(runes[:n]), string(runes[n:])
}

// wrapPitch is the distance between two wrapped lines of font f.
func wrapPitch(f Font) int { return f.Height() + scaled(CardLineSpacing) }

// wrapBlockHeight is the height of n wrapped lines of font f.
func wrapBlockHeight(f Font, n int) int {
	if n <= 0 {
		return 0
	}
	return n*f.Height() + (n-1)*scaled(CardLineSpacing)
}
