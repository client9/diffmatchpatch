package diffmatchpatch

import "math"

// matchAlphabet computes the alphabet bitmask for Bitap's pattern.
func (dmp *DiffMatchPatch) matchAlphabet(pattern string) map[rune]int {
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

func (dmp *DiffMatchPatch) matchBitapScore(e, x, loc int, pattern string) float64 {
	accuracy := float64(e) / float64(runeLen(pattern))
	proximity := x - loc
	if proximity < 0 {
		proximity = -proximity
	}
	if dmp.MatchDistance == 0 {
		if proximity == 0 {
			return accuracy
		}
		return 1.0
	}
	return accuracy + float64(proximity)/float64(dmp.MatchDistance)
}

// matchBitap locates the best instance of pattern in text near loc using Bitap.
func (dmp *DiffMatchPatch) matchBitap(text, pattern string, loc int) int {
	rText := []rune(text)
	rPattern := []rune(pattern)
	textLen := len(rText)
	patLen := len(rPattern)

	s := dmp.matchAlphabet(pattern)
	scoreThreshold := float64(dmp.MatchThreshold)

	bestLoc := runesIndexFrom(rText, rPattern, loc)
	if bestLoc != -1 {
		scoreThreshold = math.Min(dmp.matchBitapScore(0, bestLoc, loc, pattern), scoreThreshold)
		end := min(loc+patLen, textLen)
		bl2 := runesLastIndexUpTo(rText, rPattern, end)
		if bl2 != -1 {
			scoreThreshold = math.Min(dmp.matchBitapScore(0, bl2, loc, pattern), scoreThreshold)
		}
	}

	matchmask := 1 << (patLen - 1)
	bestLoc = -1
	binMax := patLen + textLen

	var lastRd []int
	for d := 0; d < patLen; d++ {
		binMin := 0
		binMid := binMax
		for binMin < binMid {
			if dmp.matchBitapScore(d, loc+binMid, loc, pattern) <= scoreThreshold {
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
				score := dmp.matchBitapScore(d, j-1, loc, pattern)
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
		if dmp.matchBitapScore(d+1, loc, loc, pattern) > scoreThreshold {
			break
		}
		lastRd = rd
	}
	return bestLoc
}

// MatchMain locates the best instance of pattern in text near loc.
func (dmp *DiffMatchPatch) MatchMain(text, pattern string, loc int) int {
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
	return dmp.matchBitap(text, pattern, loc)
}
