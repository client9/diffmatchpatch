package diffmatchpatch

import (
	"testing"
)

func TestMatchAlphabet(t *testing.T) {
	dmp := New()

	check := func(msg, pattern string, want map[rune]int) {
		t.Helper()
		got := dmp.matchAlphabet(pattern)
		if len(want) != len(got) {
			t.Errorf("match_alphabet: %s: map length mismatch: want %v, got %v", msg, want, got)
			return
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("match_alphabet: %s: map[%q]: want %d, got %d", msg, k, v, got[k])
			}
		}
	}

	check("Unique", "abc", map[rune]int{'a': 4, 'b': 2, 'c': 1})
	check("Duplicates", "abcaba", map[rune]int{'a': 37, 'b': 18, 'c': 8})
}

func TestMatchBitap(t *testing.T) {
	dmp := New()
	dmp.MatchDistance = 100
	dmp.MatchThreshold = 0.5

	check := func(msg, text, pattern string, loc, want int) {
		t.Helper()
		if got := dmp.matchBitap(text, pattern, loc); got != want {
			t.Errorf("match_bitap: %s: want %d, got %d", msg, want, got)
		}
	}

	check("Exact match #1", "abcdefghijk", "fgh", 5, 5)
	check("Exact match #2", "abcdefghijk", "fgh", 0, 5)
	check("Fuzzy match #1", "abcdefghijk", "efxhi", 0, 4)
	check("Fuzzy match #2", "abcdefghijk", "cdefxyhijk", 5, 2)
	check("Fuzzy match #3", "abcdefghijk", "bxy", 1, -1)
	check("Overflow", "123456789xx0", "3456789x0", 2, 2)
	check("Before start match", "abcdef", "xxabc", 4, 0)
	check("Beyond end match", "abcdef", "defyy", 4, 3)
	check("Oversized pattern", "abcdef", "xabcdefy", 0, 0)

	dmp.MatchThreshold = 0.4
	check("Threshold #1", "abcdefghijk", "efxyhi", 1, 4)

	dmp.MatchThreshold = 0.3
	check("Threshold #2", "abcdefghijk", "efxyhi", 1, -1)

	dmp.MatchThreshold = 0.0
	check("Threshold #3", "abcdefghijk", "bcdef", 1, 1)

	dmp.MatchThreshold = 0.5
	check("Multiple select #1", "abcdexyzabcde", "abccde", 3, 0)
	check("Multiple select #2", "abcdexyzabcde", "abccde", 5, 8)

	dmp.MatchDistance = 10
	check("Distance test #1", "abcdefghijklmnopqrstuvwxyz", "abcdefg", 24, -1)
	check("Distance test #2", "abcdefghijklmnopqrstuvwxyz", "abcdxxefg", 1, 0)

	dmp.MatchDistance = 1000
	check("Distance test #3", "abcdefghijklmnopqrstuvwxyz", "abcdefg", 24, 0)
}

func TestMatchMain(t *testing.T) {
	dmp := New()

	check := func(msg, text, pattern string, loc, want int) {
		t.Helper()
		if got := dmp.MatchMain(text, pattern, loc); got != want {
			t.Errorf("match_main: %s: want %d, got %d", msg, want, got)
		}
	}

	check("Equality", "abcdef", "abcdef", 1000, 0)
	check("Null text", "", "abcdef", 1, -1)
	check("Null pattern", "abcdef", "", 3, 3)
	check("Exact match", "abcdef", "de", 3, 3)
	check("Beyond end match", "abcdef", "defy", 4, 3)
	check("Oversized pattern", "abcdef", "abcdefy", 0, 0)

	dmp.MatchThreshold = 0.7
	check("Complex match", "I am the very model of a modern major general.", " that berry ", 5, 4)
	dmp.MatchThreshold = 0.5
}
