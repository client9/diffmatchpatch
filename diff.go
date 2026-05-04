package diffmatchpatch

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

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

// CommonPrefix returns the number of runes common to the start of text1 and text2.
func CommonPrefix(text1, text2 string) int {
	return diffCommonPrefixRunes([]rune(text1), []rune(text2))
}

// CommonSuffix returns the number of runes common to the end of text1 and text2.
func CommonSuffix(text1, text2 string) int {
	return diffCommonSuffixRunes([]rune(text1), []rune(text2))
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

// diffHalfMatch checks whether the two rune slices share a common substring
// at least half the length of the longer slice. Returns five sub-slices
// [prefix1, suffix1, prefix2, suffix2, common], or nil if no useful split
// is found. The optimization is skipped when ctx has no deadline, since it
// trades accuracy for speed.
func diffHalfMatch(ctx context.Context, r1, r2 []rune) [][]rune {
	_, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		return nil
	}
	var long, short []rune
	if len(r1) > len(r2) {
		long, short = r1, r2
	} else {
		long, short = r2, r1
	}
	if len(long) < 4 || len(short)*2 < len(long) {
		return nil
	}
	hm1 := diffHalfMatchI(long, short, (len(long)+3)/4)
	hm2 := diffHalfMatchI(long, short, (len(long)+1)/2)
	var hm [][]rune
	switch {
	case hm1 == nil && hm2 == nil:
		return nil
	case hm2 == nil:
		hm = hm1
	case hm1 == nil:
		hm = hm2
	default:
		if len(hm1[4]) > len(hm2[4]) {
			hm = hm1
		} else {
			hm = hm2
		}
	}
	if len(r1) > len(r2) {
		return hm
	}
	return [][]rune{hm[2], hm[3], hm[0], hm[1], hm[4]}
}

// diffHalfMatchI checks a single candidate midpoint i within long for a
// half-match with short. Returns [longA, longB, shortA, shortB, common] or nil.
func diffHalfMatchI(long, short []rune, i int) [][]rune {
	seed := long[i : i+len(long)/4]
	j := -1
	var bestCommon, bestLongA, bestLongB, bestShortA, bestShortB []rune
	for {
		j++
		idx := runesIndex(short[j:], seed)
		if idx == -1 {
			break
		}
		j += idx
		prefix := diffCommonPrefixRunes(long[i:], short[j:])
		suffix := diffCommonSuffixRunes(long[:i], short[:j])
		if len(bestCommon) < suffix+prefix {
			bestCommon = make([]rune, suffix+prefix)
			copy(bestCommon, short[j-suffix:j])
			copy(bestCommon[suffix:], short[j:j+prefix])
			bestLongA = long[:i-suffix]
			bestLongB = long[i+prefix:]
			bestShortA = short[:j-suffix]
			bestShortB = short[j+prefix:]
		}
	}
	if len(bestCommon)*2 >= len(long) {
		return [][]rune{bestLongA, bestLongB, bestShortA, bestShortB, bestCommon}
	}
	return nil
}

func diffBisect(ctx context.Context, r1, r2 []rune) []Diff {
	len1, len2 := len(r1), len(r2)
	maxD := (len1 + len2 + 1) / 2
	vOffset := maxD
	vLen := 2 * maxD
	v1 := make([]int, vLen)
	v2 := make([]int, vLen)
	for i := range v1 {
		v1[i] = -1
		v2[i] = -1
	}
	v1[vOffset+1] = 0
	v2[vOffset+1] = 0
	delta := len1 - len2
	front := delta%2 != 0
	k1start, k1end, k2start, k2end := 0, 0, 0, 0
	for d := range maxD {
		if ctx.Err() != nil {
			break
		}
		for k1 := -d + k1start; k1 <= d-k1end; k1 += 2 {
			k1off := vOffset + k1
			var x1 int
			if k1 == -d || (k1 != d && v1[k1off-1] < v1[k1off+1]) {
				x1 = v1[k1off+1]
			} else {
				x1 = v1[k1off-1] + 1
			}
			y1 := x1 - k1
			for x1 < len1 && y1 < len2 && r1[x1] == r2[y1] {
				x1++
				y1++
			}
			v1[k1off] = x1
			if x1 > len1 {
				k1end += 2
			} else if y1 > len2 {
				k1start += 2
			} else if front {
				k2off := vOffset + delta - k1
				if k2off >= 0 && k2off < vLen && v2[k2off] != -1 {
					if x1 >= len1-v2[k2off] {
						a := diffMainRunes(ctx, r1[:x1], r2[:y1], false)
						return append(a, diffMainRunes(ctx, r1[x1:], r2[y1:], false)...)
					}
				}
			}
		}
		for k2 := -d + k2start; k2 <= d-k2end; k2 += 2 {
			k2off := vOffset + k2
			var x2 int
			if k2 == -d || (k2 != d && v2[k2off-1] < v2[k2off+1]) {
				x2 = v2[k2off+1]
			} else {
				x2 = v2[k2off-1] + 1
			}
			y2 := x2 - k2
			for x2 < len1 && y2 < len2 && r1[len1-x2-1] == r2[len2-y2-1] {
				x2++
				y2++
			}
			v2[k2off] = x2
			if x2 > len1 {
				k2end += 2
			} else if y2 > len2 {
				k2start += 2
			} else if !front {
				k1off := vOffset + delta - k2
				if k1off >= 0 && k1off < vLen && v1[k1off] != -1 {
					x1 := v1[k1off]
					y1 := vOffset + x1 - k1off
					if x1 >= len1-v2[k2off] {
						a := diffMainRunes(ctx, r1[:x1], r2[:y1], false)
						return append(a, diffMainRunes(ctx, r1[x1:], r2[y1:], false)...)
					}
				}
			}
		}
	}
	return []Diff{{Delete, r1}, {Insert, r2}}
}

func diffRunesToLines(diffs []Diff, lineArray []string) []Diff {
	out := make([]Diff, len(diffs))
	for i, d := range diffs {
		var r []rune
		for _, idx := range d.Text {
			r = append(r, []rune(lineArray[idx])...)
		}
		out[i] = Diff{d.Type, r}
	}
	return out
}

func diffLineMode(ctx context.Context, text1, text2 string) []Diff {
	lineArray := []string{""}
	lineHash := make(map[string]int)
	chars1 := diffLinesToRunesMunge(text1, &lineArray, lineHash, 40000)
	chars2 := diffLinesToRunesMunge(text2, &lineArray, lineHash, 65535)
	diffs := diffMainRunes(ctx, chars1, chars2, false)
	diffs = diffRunesToLines(diffs, lineArray)
	diffs = CleanupSemantic(diffs)
	diffs = append(diffs, Diff{Equal, nil})
	pointer := 0
	countDel, countIns := 0, 0
	var textDel, textIns []rune
	for pointer < len(diffs) {
		switch diffs[pointer].Type {
		case Insert:
			countIns++
			textIns = append(textIns, diffs[pointer].Text...)
		case Delete:
			countDel++
			textDel = append(textDel, diffs[pointer].Text...)
		case Equal:
			if countDel >= 1 && countIns >= 1 {
				start := pointer - countDel - countIns
				sub := diffMainRunes(ctx, textDel, textIns, false)
				tail := make([]Diff, len(diffs[pointer:]))
				copy(tail, diffs[pointer:])
				diffs = append(diffs[:start], append(sub, tail...)...)
				pointer = start + len(sub)
			}
			countIns, countDel = 0, 0
			textDel, textIns = nil, nil
		}
		pointer++
	}
	return diffs[:len(diffs)-1]
}

func diffComputeRunes(ctx context.Context, r1, r2 []rune, checklines bool) []Diff {
	if len(r1) == 0 {
		return []Diff{{Insert, r2}}
	}
	if len(r2) == 0 {
		return []Diff{{Delete, r1}}
	}
	var long, short []rune
	if len(r1) > len(r2) {
		long, short = r1, r2
	} else {
		long, short = r2, r1
	}
	var op Operation
	if len(r1) > len(r2) {
		op = Delete
	} else {
		op = Insert
	}
	if idx := runesIndex(long, short); idx != -1 {
		return []Diff{
			{op, long[:idx]},
			{Equal, short},
			{op, long[idx+len(short):]},
		}
	}
	if len(short) == 1 {
		return []Diff{{Delete, r1}, {Insert, r2}}
	}
	if hm := diffHalfMatch(ctx, r1, r2); hm != nil {
		diffsA := diffMainRunes(ctx, hm[0], hm[2], checklines)
		diffsB := diffMainRunes(ctx, hm[1], hm[3], checklines)
		return append(append(diffsA, Diff{Equal, hm[4]}), diffsB...)
	}
	if checklines && len(r1) > 100 && len(r2) > 100 {
		return diffLineMode(ctx, string(r1), string(r2))
	}
	return diffBisect(ctx, r1, r2)
}

func diffMainRunes(ctx context.Context, r1, r2 []rune, checklines bool) []Diff {
	if runesEqual(r1, r2) {
		if len(r1) == 0 {
			return []Diff{}
		}
		return []Diff{{Equal, slices.Clone(r1)}}
	}
	pfxLen := diffCommonPrefixRunes(r1, r2)
	var prefix []rune
	if pfxLen > 0 {
		prefix = r1[:pfxLen]
		r1 = r1[pfxLen:]
		r2 = r2[pfxLen:]
	}
	sfxLen := diffCommonSuffixRunes(r1, r2)
	var suffix []rune
	if sfxLen > 0 {
		suffix = r1[len(r1)-sfxLen:]
		r1 = r1[:len(r1)-sfxLen]
		r2 = r2[:len(r2)-sfxLen]
	}
	diffs := diffComputeRunes(ctx, r1, r2, checklines)
	if len(prefix) > 0 {
		diffs = append([]Diff{{Equal, prefix}}, diffs...)
	}
	if len(suffix) > 0 {
		diffs = append(diffs, Diff{Equal, suffix})
	}
	return CleanupMerge(diffs)
}

// DiffRunes computes the differences between two rune slices.
// Use context.WithTimeout to bound execution time; context.Background() for no limit.
func DiffRunes(ctx context.Context, r1, r2 []rune) []Diff {
	return diffMainRunes(ctx, r1, r2, false)
}

// DiffStrings computes character-level differences between two strings.
// Use context.WithTimeout to bound execution time; context.Background() for no limit.
func DiffStrings(ctx context.Context, s1, s2 string) []Diff {
	return diffMainRunes(ctx, []rune(s1), []rune(s2), false)
}

// DiffLines diffs two strings using a two-pass algorithm. The first pass
// operates at line granularity to quickly locate changed regions; the second
// pass re-diffs each changed region at character level to produce precise
// intra-line edits. The returned diffs therefore contain character-level
// operations, not whole-line ones. To diff at line granularity only, encode
// lines as runes with [DiffRunes].
// Use context.WithTimeout to bound execution time; context.Background() for no limit.
func DiffLines(ctx context.Context, s1, s2 string) []Diff {
	return diffLineMode(ctx, s1, s2)
}

// diffLinesToRunes encodes two texts into rune sequences where each rune value
// is an index into a shared lineArray.
func diffLinesToRunes(text1, text2 string) linesCharsResult {
	lineArray := []string{""}
	lineHash := make(map[string]int)
	chars1 := diffLinesToRunesMunge(text1, &lineArray, lineHash, 40000)
	chars2 := diffLinesToRunesMunge(text2, &lineArray, lineHash, 65535)
	return linesCharsResult{chars1: chars1, chars2: chars2, lineArray: lineArray}
}

func diffLinesToRunesMunge(text string, lineArray *[]string, lineHash map[string]int, maxLines int) []rune {
	var chars []rune
	lineStart := 0
	lineEnd := -1
	for lineEnd < len(text)-1 {
		idx := strings.IndexByte(text[lineStart:], '\n')
		if idx == -1 {
			lineEnd = len(text) - 1
		} else {
			lineEnd = lineStart + idx
		}
		line := text[lineStart : lineEnd+1]
		if n, ok := lineHash[line]; ok {
			chars = append(chars, rune(n))
		} else {
			if len(*lineArray) == maxLines {
				line = text[lineStart:]
				lineEnd = len(text)
			}
			*lineArray = append(*lineArray, line)
			lineHash[line] = len(*lineArray) - 1
			chars = append(chars, rune(len(*lineArray)-1))
		}
		lineStart = lineEnd + 1
	}
	return chars
}

// CleanupMerge reorders and merges like edit sections.
func CleanupMerge(diffs []Diff) []Diff {
	diffs = append(append([]Diff{}, diffs...), Diff{Equal, nil})
	pointer := 0
	countDel, countIns := 0, 0
	var textDel, textIns []rune

	for pointer < len(diffs) {
		switch diffs[pointer].Type {
		case Insert:
			countIns++
			textIns = append(textIns, diffs[pointer].Text...)
			pointer++
		case Delete:
			countDel++
			textDel = append(textDel, diffs[pointer].Text...)
			pointer++
		case Equal:
			if countDel+countIns > 1 {
				if countDel != 0 && countIns != 0 {
					pfxLen := diffCommonPrefixRunes(textIns, textDel)
					if pfxLen != 0 {
						base := pointer - countDel - countIns
						if base > 0 && diffs[base-1].Type == Equal {
							diffs[base-1].Text = append(diffs[base-1].Text, textIns[:pfxLen]...)
						} else {
							diffs = append([]Diff{{Equal, slices.Clone(textIns[:pfxLen])}}, diffs...)
							pointer++
						}
						textIns = textIns[pfxLen:]
						textDel = textDel[pfxLen:]
					}
					sfxLen := diffCommonSuffixRunes(textIns, textDel)
					if sfxLen != 0 {
						sfx := textIns[len(textIns)-sfxLen:]
						diffs[pointer].Text = append(slices.Clone(sfx), diffs[pointer].Text...)
						textIns = textIns[:len(textIns)-sfxLen]
						textDel = textDel[:len(textDel)-sfxLen]
					}
				}
				pointer -= countDel + countIns
				diffs = append(diffs[:pointer], diffs[pointer+countDel+countIns:]...)
				if len(textDel) > 0 {
					ins := make([]Diff, len(diffs[pointer:]))
					copy(ins, diffs[pointer:])
					diffs = append(diffs[:pointer], append([]Diff{{Delete, textDel}}, ins...)...)
					pointer++
				}
				if len(textIns) > 0 {
					ins := make([]Diff, len(diffs[pointer:]))
					copy(ins, diffs[pointer:])
					diffs = append(diffs[:pointer], append([]Diff{{Insert, textIns}}, ins...)...)
					pointer++
				}
				pointer++
			} else if pointer > 0 && diffs[pointer-1].Type == Equal {
				diffs[pointer-1].Text = append(diffs[pointer-1].Text, diffs[pointer].Text...)
				diffs = append(diffs[:pointer], diffs[pointer+1:]...)
			} else {
				pointer++
			}
			countDel, countIns = 0, 0
			textDel, textIns = nil, nil
		}
	}
	if len(diffs[len(diffs)-1].Text) == 0 {
		diffs = diffs[:len(diffs)-1]
	}

	changes := false
	pointer = 1
	for pointer < len(diffs)-1 {
		if diffs[pointer-1].Type == Equal && diffs[pointer+1].Type == Equal {
			curr := diffs[pointer].Text
			prev := diffs[pointer-1].Text
			next := diffs[pointer+1].Text
			if runesHasSuffix(curr, prev) {
				diffs[pointer].Text = append(slices.Clone(prev), curr[:len(curr)-len(prev)]...)
				diffs[pointer+1].Text = append(slices.Clone(prev), next...)
				diffs = append(diffs[:pointer-1], diffs[pointer:]...)
				changes = true
			} else if runesHasPrefix(curr, next) {
				diffs[pointer-1].Text = append(slices.Clone(prev), next...)
				diffs[pointer].Text = append(slices.Clone(curr[len(next):]), next...)
				diffs = append(diffs[:pointer+1], diffs[pointer+2:]...)
				changes = true
			}
		}
		pointer++
	}
	if changes {
		return CleanupMerge(diffs)
	}
	return diffs
}

var blankLineEndRe = regexp.MustCompile(`\n\r?\n$`)
var blankLineStartRe = regexp.MustCompile(`^\r?\n\r?\n`)

func diffCleanupSemanticScore(one, two []rune) int {
	if len(one) == 0 || len(two) == 0 {
		return 6
	}
	char1 := one[len(one)-1]
	char2 := two[0]
	nonAlpha1 := !unicode.IsLetter(char1) && !unicode.IsDigit(char1)
	nonAlpha2 := !unicode.IsLetter(char2) && !unicode.IsDigit(char2)
	ws1 := nonAlpha1 && unicode.IsSpace(char1)
	ws2 := nonAlpha2 && unicode.IsSpace(char2)
	lb1 := ws1 && unicode.IsControl(char1)
	lb2 := ws2 && unicode.IsControl(char2)
	blank1 := lb1 && blankLineEndRe.MatchString(string(one))
	blank2 := lb2 && blankLineStartRe.MatchString(string(two))
	if blank1 || blank2 {
		return 5
	} else if lb1 || lb2 {
		return 4
	} else if nonAlpha1 && !ws1 && ws2 {
		return 3
	} else if ws1 || ws2 {
		return 2
	} else if nonAlpha1 || nonAlpha2 {
		return 1
	}
	return 0
}

// CleanupSemanticLossless shifts edits to align on word/line boundaries.
func CleanupSemanticLossless(diffs []Diff) []Diff {
	diffs = append([]Diff{}, diffs...)
	pointer := 1
	for pointer < len(diffs)-1 {
		if diffs[pointer-1].Type == Equal && diffs[pointer+1].Type == Equal {
			eq1 := diffs[pointer-1].Text
			edit := diffs[pointer].Text
			eq2 := diffs[pointer+1].Text

			commonOff := diffCommonSuffixRunes(eq1, edit)
			if commonOff > 0 {
				commonStr := slices.Clone(edit[len(edit)-commonOff:])
				eq1 = eq1[:len(eq1)-commonOff]
				edit = append(commonStr, edit[:len(edit)-commonOff]...)
				eq2 = append(slices.Clone(commonStr), eq2...)
			}

			bestEq1, bestEdit, bestEq2 := eq1, edit, eq2
			bestScore := diffCleanupSemanticScore(eq1, edit) + diffCleanupSemanticScore(edit, eq2)

			re := slices.Clone(edit)
			re2 := slices.Clone(eq2)
			for len(re) > 0 && len(re2) > 0 && re[0] == re2[0] {
				eq1 = append(eq1, re[0])
				shifted := make([]rune, len(re))
				copy(shifted, re[1:])
				shifted[len(re)-1] = re2[0]
				re = shifted
				re2 = re2[1:]
				edit = re
				eq2 = re2
				score := diffCleanupSemanticScore(eq1, edit) + diffCleanupSemanticScore(edit, eq2)
				if score >= bestScore {
					bestScore = score
					bestEq1 = slices.Clone(eq1)
					bestEdit = slices.Clone(edit)
					bestEq2 = slices.Clone(eq2)
				}
			}

			if !slices.Equal(diffs[pointer-1].Text, bestEq1) {
				if len(bestEq1) > 0 {
					diffs[pointer-1].Text = bestEq1
				} else {
					diffs = append(diffs[:pointer-1], diffs[pointer:]...)
					pointer--
				}
				diffs[pointer].Text = bestEdit
				if len(bestEq2) > 0 {
					diffs[pointer+1].Text = bestEq2
				} else {
					diffs = append(diffs[:pointer+1], diffs[pointer+2:]...)
					pointer--
				}
			}
		}
		pointer++
	}
	return diffs
}

// CleanupSemantic reduces diffs by eliminating semantically trivial equalities.
func CleanupSemantic(diffs []Diff) []Diff {
	diffs = append([]Diff{}, diffs...)
	changes := false
	equalities := []int{}
	var lastEquality []rune
	pointer := 0
	ins1, del1, ins2, del2 := 0, 0, 0, 0

	for pointer < len(diffs) {
		if diffs[pointer].Type == Equal {
			equalities = append(equalities, pointer)
			ins1, del1 = ins2, del2
			ins2, del2 = 0, 0
			lastEquality = diffs[pointer].Text
		} else {
			if diffs[pointer].Type == Insert {
				ins2 += len(diffs[pointer].Text)
			} else {
				del2 += len(diffs[pointer].Text)
			}
			leLen := len(lastEquality)
			if len(lastEquality) > 0 &&
				leLen <= max(ins1, del1) &&
				leLen <= max(ins2, del2) {
				idx := equalities[len(equalities)-1]
				tail := make([]Diff, len(diffs[idx:]))
				copy(tail, diffs[idx:])
				diffs = append(diffs[:idx], append([]Diff{{Delete, lastEquality}}, tail...)...)
				diffs[idx+1].Type = Insert
				equalities = equalities[:len(equalities)-1]
				if len(equalities) > 0 {
					equalities = equalities[:len(equalities)-1]
				}
				if len(equalities) > 0 {
					pointer = equalities[len(equalities)-1]
				} else {
					pointer = -1
				}
				ins1, del1, ins2, del2 = 0, 0, 0, 0
				lastEquality = nil
				changes = true
			}
		}
		pointer++
	}

	if changes {
		diffs = CleanupMerge(diffs)
	}
	diffs = CleanupSemanticLossless(diffs)

	pointer = 1
	for pointer < len(diffs) {
		if diffs[pointer-1].Type == Delete && diffs[pointer].Type == Insert {
			del := diffs[pointer-1].Text
			ins := diffs[pointer].Text
			ov1 := diffCommonOverlap(del, ins)
			ov2 := diffCommonOverlap(ins, del)
			if ov1 >= ov2 {
				if float64(ov1) >= float64(len(del))/2.0 ||
					float64(ov1) >= float64(len(ins))/2.0 {
					eq := slices.Clone(ins[:ov1])
					tail := make([]Diff, len(diffs[pointer:]))
					copy(tail, diffs[pointer:])
					diffs = append(diffs[:pointer], append([]Diff{{Equal, eq}}, tail...)...)
					diffs[pointer-1].Text = del[:len(del)-ov1]
					diffs[pointer+1].Text = ins[ov1:]
					pointer++
				}
			} else {
				if float64(ov2) >= float64(len(del))/2.0 ||
					float64(ov2) >= float64(len(ins))/2.0 {
					eq := slices.Clone(del[:ov2])
					tail := make([]Diff, len(diffs[pointer:]))
					copy(tail, diffs[pointer:])
					diffs = append(diffs[:pointer], append([]Diff{{Equal, eq}}, tail...)...)
					diffs[pointer-1].Type = Insert
					diffs[pointer-1].Text = ins[:len(ins)-ov2]
					diffs[pointer+1].Type = Delete
					diffs[pointer+1].Text = del[ov2:]
					pointer++
				}
			}
			pointer++
		}
		pointer++
	}
	return diffs
}

// CleanupEfficiency reduces diffs by eliminating operationally trivial equalities.
// editCost is the minimum rune count of an equality that is worth preserving;
// equalities shorter than this threshold are converted to insert/delete pairs.
// A value of 4 is typical.
func CleanupEfficiency(diffs []Diff, editCost int) []Diff {
	diffs = append([]Diff{}, diffs...)
	changes := false
	equalities := []int{}
	var lastEquality []rune
	pointer := 0
	preIns, preDel, postIns, postDel := false, false, false, false

	for pointer < len(diffs) {
		if diffs[pointer].Type == Equal {
			if len(diffs[pointer].Text) < editCost && (postIns || postDel) {
				equalities = append(equalities, pointer)
				preIns, preDel = postIns, postDel
				lastEquality = diffs[pointer].Text
			} else {
				equalities = equalities[:0]
				lastEquality = nil
			}
			postIns, postDel = false, false
		} else {
			if diffs[pointer].Type == Delete {
				postDel = true
			} else {
				postIns = true
			}
			if len(lastEquality) > 0 &&
				((preIns && preDel && postIns && postDel) ||
					(len(lastEquality) < editCost/2 &&
						boolToInt(preIns)+boolToInt(preDel)+boolToInt(postIns)+boolToInt(postDel) == 3)) {
				idx := equalities[len(equalities)-1]
				tail := make([]Diff, len(diffs[idx:]))
				copy(tail, diffs[idx:])
				diffs = append(diffs[:idx], append([]Diff{{Delete, lastEquality}}, tail...)...)
				diffs[idx+1].Type = Insert
				equalities = equalities[:len(equalities)-1]
				lastEquality = nil
				if preIns && preDel {
					postIns, postDel = true, true
					equalities = equalities[:0]
				} else {
					if len(equalities) > 0 {
						equalities = equalities[:len(equalities)-1]
					}
					if len(equalities) > 0 {
						pointer = equalities[len(equalities)-1]
					} else {
						pointer = -1
					}
					postIns, postDel = false, false
				}
				changes = true
			}
		}
		pointer++
	}

	if changes {
		diffs = CleanupMerge(diffs)
	}
	return diffs
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// PrettyHtml converts a diff to a pretty HTML snippet.
func PrettyHtml(diffs []Diff) string {
	var buf strings.Builder
	for _, d := range diffs {
		text := strings.ReplaceAll(string(d.Text), "&", "&amp;")
		text = strings.ReplaceAll(text, "<", "&lt;")
		text = strings.ReplaceAll(text, ">", "&gt;")
		text = strings.ReplaceAll(text, "\n", "&para;<br>")
		switch d.Type {
		case Insert:
			buf.WriteString(`<ins style="background:#e6ffe6;">`)
			buf.WriteString(text)
			buf.WriteString("</ins>")
		case Delete:
			buf.WriteString(`<del style="background:#ffe6e6;">`)
			buf.WriteString(text)
			buf.WriteString("</del>")
		case Equal:
			buf.WriteString("<span>")
			buf.WriteString(text)
			buf.WriteString("</span>")
		}
	}
	return buf.String()
}

// Source computes the source text from a diff (equalities and deletions).
func Source(diffs []Diff) string {
	var buf strings.Builder
	for _, d := range diffs {
		if d.Type != Insert {
			buf.WriteString(string(d.Text))
		}
	}
	return buf.String()
}

// Dest computes the destination text from a diff (equalities and insertions).
func Dest(diffs []Diff) string {
	var buf strings.Builder
	for _, d := range diffs {
		if d.Type != Delete {
			buf.WriteString(string(d.Text))
		}
	}
	return buf.String()
}

// Levenshtein returns the edit distance of diffs in runes
// (number of inserted plus deleted runes, not counting equalities).
func Levenshtein(diffs []Diff) int {
	levenshtein := 0
	ins, del := 0, 0
	for _, d := range diffs {
		switch d.Type {
		case Insert:
			ins += len(d.Text)
		case Delete:
			del += len(d.Text)
		case Equal:
			levenshtein += max(ins, del)
			ins, del = 0, 0
		}
	}
	return levenshtein + max(ins, del)
}

// TranslateIndex maps a rune index in text1 to the corresponding rune index in
// text2, accounting for insertions and deletions described by diffs.
func TranslateIndex(diffs []Diff, loc int) int {
	chars1, chars2 := 0, 0
	lastChars1, lastChars2 := 0, 0
	var lastDiff *Diff
	for i := range diffs {
		d := &diffs[i]
		if d.Type != Insert {
			chars1 += len(d.Text)
		}
		if d.Type != Delete {
			chars2 += len(d.Text)
		}
		if chars1 > loc {
			lastDiff = d
			break
		}
		lastChars1, lastChars2 = chars1, chars2
	}
	if lastDiff != nil && lastDiff.Type == Delete {
		return lastChars2
	}
	return lastChars2 + (loc - lastChars1)
}
