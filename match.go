package diffmatchpatch

import "math/bits"

// matchAlphabet computes the alphabet bitmask for Bitap's pattern.
func matchAlphabet(pattern string) map[rune]int {
	s := make(map[rune]int)
	runes := []rune(pattern)
	for _, c := range runes {
		s[c] = 0
	}
	for i, c := range runes {
		s[c] |= 1 << (len(runes) - i - 1)
	}
	return s
}

// Matcher performs fuzzy text matching with configurable accuracy.
type Matcher struct {
	// Threshold controls how loosely to match (0.0 = perfect, 1.0 = very loose).
	Threshold float32
	// Distance is how far from loc to search (0 = exact location only).
	Distance int
}

// bitapMaxBits is the maximum pattern length Bitap can handle, determined by
// the platform's integer word size (bits.UintSize). The JS original used 32
// because JavaScript bitwise ops are 32-bit; Go uses native int width (64 on
// modern platforms).
const bitapMaxBits = bits.UintSize

func (m Matcher) bitapScore(e, x, loc, patLen int) float64 {
	accuracy := float64(e) / float64(patLen)
	proximity := x - loc
	if proximity < 0 {
		proximity = -proximity
	}
	if m.Distance == 0 {
		if proximity == 0 {
			return accuracy
		}
		return 1.0
	}
	return accuracy + float64(proximity)/float64(m.Distance)
}

func (m Matcher) bitap(text, pattern string, loc int) int {
	rText := []rune(text)
	rPattern := []rune(pattern)
	textLen := len(rText)
	patLen := len(rPattern)

	s := matchAlphabet(pattern)
	scoreThreshold := float64(m.Threshold)

	bestLoc := runesIndexFrom(rText, rPattern, loc)
	if bestLoc != -1 {
		scoreThreshold = min(m.bitapScore(0, bestLoc, loc, patLen), scoreThreshold)
		end := min(loc+patLen, textLen)
		bl2 := runesLastIndexUpTo(rText, rPattern, end)
		if bl2 != -1 {
			scoreThreshold = min(m.bitapScore(0, bl2, loc, patLen), scoreThreshold)
		}
	}

	matchmask := 1 << (patLen - 1)
	bestLoc = -1
	binMax := patLen + textLen

	var lastRd []int
	for d := range patLen {
		binMin := 0
		binMid := binMax
		for binMin < binMid {
			if m.bitapScore(d, loc+binMid, loc, patLen) <= scoreThreshold {
				binMin = binMid
			} else {
				binMax = binMid
			}
			binMid = (binMax-binMin)/2 + binMin
		}
		binMax = binMid
		start := max(1, loc-binMid+1)
		finish := min(loc+binMid, textLen) + patLen

		rd := make([]int, finish+2)
		rd[finish+1] = (1 << d) - 1
		for j := finish; j >= start; j-- {
			var charMatch int
			if j-1 < textLen {
				if v, ok := s[rText[j-1]]; ok {
					charMatch = v
				}
			}
			if d == 0 {
				rd[j] = ((rd[j+1] << 1) | 1) & charMatch
			} else {
				rd[j] = (((rd[j+1] << 1) | 1) & charMatch) |
					(((lastRd[j+1] | lastRd[j]) << 1) | 1) | lastRd[j+1]
			}
			if rd[j]&matchmask != 0 {
				score := m.bitapScore(d, j-1, loc, patLen)
				if score <= scoreThreshold {
					scoreThreshold = score
					bestLoc = j - 1
					if bestLoc > loc {
						start = max(1, 2*loc-bestLoc)
					} else {
						break
					}
				}
			}
		}
		if m.bitapScore(d+1, loc, loc, patLen) > scoreThreshold {
			break
		}
		lastRd = rd
	}
	return bestLoc
}

// Match locates the best instance of pattern in text near loc and returns
// the rune index of the match, or -1 if no match is found within the
// configured Threshold and Distance.
func (m Matcher) Match(text, pattern string, loc int) int {
	rText := []rune(text)
	rPattern := []rune(pattern)
	textLen := len(rText)
	patLen := len(rPattern)

	loc = max(0, min(loc, textLen))

	if text == pattern {
		return 0
	}
	if textLen == 0 {
		return -1
	}
	if loc+patLen <= textLen && string(rText[loc:loc+patLen]) == pattern {
		return loc
	}
	if patLen > bitapMaxBits {
		return -1
	}
	return m.bitap(text, pattern, loc)
}
