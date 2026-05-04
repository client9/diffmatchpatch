package diffmatchpatch

func diffCommonPrefixRunes(r1, r2 []rune) int {
	n := min(len(r1), len(r2))
	for i := range n {
		if r1[i] != r2[i] {
			return i
		}
	}
	return n
}

func diffCommonSuffixRunes(r1, r2 []rune) int {
	n1, n2 := len(r1), len(r2)
	n := min(n1, n2)
	for i := 1; i <= n; i++ {
		if r1[n1-i] != r2[n2-i] {
			return i - 1
		}
	}
	return n
}

// diffCommonOverlap returns the length (runes) of the longest overlap
// between the suffix of r1 and the prefix of r2.
func diffCommonOverlap(r1, r2 []rune) int {
	len1, len2 := len(r1), len(r2)
	if len1 == 0 || len2 == 0 {
		return 0
	}
	if len1 > len2 {
		r1 = r1[len1-len2:]
		len1 = len2
	} else if len1 < len2 {
		r2 = r2[:len1]
	}
	textLen := len1
	if runesEqual(r1, r2) {
		return textLen
	}
	best := 0
	length := 1
	for {
		pattern := r1[textLen-length:]
		found := runesIndex(r2, pattern)
		if found == -1 {
			return best
		}
		length += found
		if found == 0 || runesEqual(r1[textLen-length:], r2[:length]) {
			best = length
			length++
		}
	}
}

// runesIndex returns the index of the first occurrence of needle in haystack,
// or -1 if not present. This is a naive O(n·m) subsequence search — a known
// trade-off of the rune-based design. strings.Index uses Rabin-Karp for longer
// needles but operates on bytes; slices.Index finds only single elements.
// Future improvement: implement Rabin-Karp (or similar) directly on []rune.
func runesIndex(haystack, needle []rune) int {
	n := len(needle)
	if n == 0 {
		return 0
	}
	for i := 0; i <= len(haystack)-n; i++ {
		if runesEqual(haystack[i:i+n], needle) {
			return i
		}
	}
	return -1
}

func runesEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func runesIndexFrom(text, pattern []rune, start int) int {
	n := len(pattern)
	for i := start; i <= len(text)-n; i++ {
		if runesEqual(text[i:i+n], pattern) {
			return i
		}
	}
	return -1
}

func runesLastIndexUpTo(text, pattern []rune, end int) int {
	n := len(pattern)
	end = min(end, len(text)-n)
	for i := end; i >= 0; i-- {
		if runesEqual(text[i:i+n], pattern) {
			return i
		}
	}
	return -1
}

func runesHasPrefix(s, prefix []rune) bool {
	return len(s) >= len(prefix) && runesEqual(s[:len(prefix)], prefix)
}

func runesHasSuffix(s, suffix []rune) bool {
	return len(s) >= len(suffix) && runesEqual(s[len(s)-len(suffix):], suffix)
}
