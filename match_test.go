package diffmatchpatch

import (
	"testing"
)

func TestMatchAlphabet(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		want    map[rune]int
	}{
		{"Unique", "abc", map[rune]int{'a': 4, 'b': 2, 'c': 1}},
		{"Duplicates", "abcaba", map[rune]int{'a': 37, 'b': 18, 'c': 8}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := matchAlphabet(c.pattern)
			if len(got) != len(c.want) {
				t.Errorf("map length: want %d, got %d", len(c.want), len(got))
				return
			}
			for k, v := range c.want {
				if got[k] != v {
					t.Errorf("map[%q]: want %d, got %d", k, v, got[k])
				}
			}
		})
	}
}

func TestMatchBitap(t *testing.T) {
	cases := []struct {
		name      string
		text      string
		pattern   string
		loc       int
		want      int
		threshold float32
		distance  int
	}{
		{"Exact match #1", "abcdefghijk", "fgh", 5, 5, 0.5, 100},
		{"Exact match #2", "abcdefghijk", "fgh", 0, 5, 0.5, 100},
		{"Fuzzy match #1", "abcdefghijk", "efxhi", 0, 4, 0.5, 100},
		{"Fuzzy match #2", "abcdefghijk", "cdefxyhijk", 5, 2, 0.5, 100},
		{"Fuzzy match #3", "abcdefghijk", "bxy", 1, -1, 0.5, 100},
		{"Overflow", "123456789xx0", "3456789x0", 2, 2, 0.5, 100},
		{"Before start match", "abcdef", "xxabc", 4, 0, 0.5, 100},
		{"Beyond end match", "abcdef", "defyy", 4, 3, 0.5, 100},
		{"Oversized pattern", "abcdef", "xabcdefy", 0, 0, 0.5, 100},
		{"Threshold #1", "abcdefghijk", "efxyhi", 1, 4, 0.4, 100},
		{"Threshold #2", "abcdefghijk", "efxyhi", 1, -1, 0.3, 100},
		{"Threshold #3", "abcdefghijk", "bcdef", 1, 1, 0.0, 100},
		{"Multiple select #1", "abcdexyzabcde", "abccde", 3, 0, 0.5, 100},
		{"Multiple select #2", "abcdexyzabcde", "abccde", 5, 8, 0.5, 100},
		{"Distance test #1", "abcdefghijklmnopqrstuvwxyz", "abcdefg", 24, -1, 0.5, 10},
		{"Distance test #2", "abcdefghijklmnopqrstuvwxyz", "abcdxxefg", 1, 0, 0.5, 10},
		{"Distance test #3", "abcdefghijklmnopqrstuvwxyz", "abcdefg", 24, 0, 0.5, 1000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := Matcher{Threshold: c.threshold, Distance: c.distance}
			if got := m.bitap(c.text, c.pattern, c.loc); got != c.want {
				t.Errorf("want %d, got %d", c.want, got)
			}
		})
	}
}

func TestMatchMain(t *testing.T) {
	cases := []struct {
		name      string
		text      string
		pattern   string
		loc       int
		want      int
		threshold float32
	}{
		{"Equality", "abcdef", "abcdef", 1000, 0, 0.5},
		{"Null text", "", "abcdef", 1, -1, 0.5},
		{"Null pattern", "abcdef", "", 3, 3, 0.5},
		{"Exact match", "abcdef", "de", 3, 3, 0.5},
		{"Beyond end match", "abcdef", "defy", 4, 3, 0.5},
		{"Oversized pattern", "abcdef", "abcdefy", 0, 0, 0.5},
		{"Complex match", "I am the very model of a modern major general.", " that berry ", 5, 4, 0.7},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := Matcher{Threshold: c.threshold, Distance: 1000}
			if got := m.Match(c.text, c.pattern, c.loc); got != c.want {
				t.Errorf("want %d, got %d", c.want, got)
			}
		})
	}
}
