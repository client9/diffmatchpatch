package diffmatchpatch

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// String returns the GNU unified diff format for a Patch.
func (p Patch) String() string {
	var coords1, coords2 string
	if p.Length1 == 0 {
		coords1 = fmt.Sprintf("%d,0", p.Start1)
	} else if p.Length1 == 1 {
		coords1 = strconv.Itoa(p.Start1 + 1)
	} else {
		coords1 = fmt.Sprintf("%d,%d", p.Start1+1, p.Length1)
	}
	if p.Length2 == 0 {
		coords2 = fmt.Sprintf("%d,0", p.Start2)
	} else if p.Length2 == 1 {
		coords2 = strconv.Itoa(p.Start2 + 1)
	} else {
		coords2 = fmt.Sprintf("%d,%d", p.Start2+1, p.Length2)
	}
	var buf strings.Builder
	fmt.Fprintf(&buf, "@@ -%s +%s @@\n", coords1, coords2)
	for _, d := range p.Diffs {
		switch d.Type {
		case Insert:
			buf.WriteByte('+')
		case Delete:
			buf.WriteByte('-')
		case Equal:
			buf.WriteByte(' ')
		}
		buf.WriteString(encodeURI(string(d.Text)))
		buf.WriteByte('\n')
	}
	return buf.String()
}

// patchAddContext expands the patch context until it is unique in text.
func (dmp *DiffMatchPatch) patchAddContext(patch *Patch, text string) {
	if len(text) == 0 {
		return
	}
	rText := []rune(text)
	textLen := len(rText)
	pattern := rText[patch.Start2 : patch.Start2+patch.Length1]
	padding := 0

	for runesCountAtLeast2(rText, pattern) &&
		(dmp.MatchMaxBits == 0 || len(pattern) < dmp.MatchMaxBits-dmp.PatchMargin-dmp.PatchMargin) {
		padding += dmp.PatchMargin
		start := max(0, patch.Start2-padding)
		end := min(textLen, patch.Start2+patch.Length1+padding)
		pattern = rText[start:end]
	}
	padding += dmp.PatchMargin

	prefixStart := max(0, patch.Start2-padding)
	prefix := rText[prefixStart:patch.Start2]
	if len(prefix) > 0 {
		patch.Diffs = append([]Diff{{Equal, slices.Clone(prefix)}}, patch.Diffs...)
	}

	suffixEnd := min(textLen, patch.Start2+patch.Length1+padding)
	suffix := rText[patch.Start2+patch.Length1 : suffixEnd]
	if len(suffix) > 0 {
		patch.Diffs = append(patch.Diffs, Diff{Equal, slices.Clone(suffix)})
	}

	prefixLen := len(prefix)
	suffixLen := len(suffix)
	patch.Start1 -= prefixLen
	patch.Start2 -= prefixLen
	patch.Length1 += prefixLen + suffixLen
	patch.Length2 += prefixLen + suffixLen
}

// runesCountAtLeast2 reports whether pattern appears at least twice in text.
func runesCountAtLeast2(text, pattern []rune) bool {
	first := runesIndex(text, pattern)
	if first == -1 {
		return false
	}
	return runesIndex(text[first+1:], pattern) != -1
}

func patchDeepCopy(patches []Patch) []Patch {
	out := make([]Patch, len(patches))
	for i, p := range patches {
		out[i] = Patch{
			Diffs:   append([]Diff{}, p.Diffs...),
			Start1:  p.Start1,
			Start2:  p.Start2,
			Length1: p.Length1,
			Length2: p.Length2,
		}
	}
	return out
}

// PatchMake computes patches to turn text1 into text2.
func (dmp *DiffMatchPatch) PatchMake(text1, text2 string) []Patch {
	diffs := dmp.DiffMain(text1, text2, true)
	if len(diffs) > 2 {
		diffs = CleanupSemantic(diffs)
		diffs = dmp.DiffCleanupEfficiency(diffs)
	}
	return dmp.PatchMakeFromTextAndDiffs(text1, diffs)
}

// PatchMakeFromDiffs computes patches from a diff list, deriving text1 from the diffs.
func (dmp *DiffMatchPatch) PatchMakeFromDiffs(diffs []Diff) []Patch {
	return dmp.PatchMakeFromTextAndDiffs(Source(diffs), diffs)
}

// PatchMakeFromTextAndDiffs computes patches from text1 and a diff list.
func (dmp *DiffMatchPatch) PatchMakeFromTextAndDiffs(text1 string, diffs []Diff) []Patch {
	var patches []Patch
	if len(diffs) == 0 {
		return patches
	}
	patch := Patch{}
	charCount1 := 0
	charCount2 := 0
	prepatchText := text1
	postpatchText := text1

	for i, d := range diffs {
		if len(patch.Diffs) == 0 && d.Type != Equal {
			patch.Start1 = charCount1
			patch.Start2 = charCount2
		}
		switch d.Type {
		case Insert:
			patch.Diffs = append(patch.Diffs, d)
			patch.Length2 += len(d.Text)
			rPost := []rune(postpatchText)
			postpatchText = string(rPost[:charCount2]) + string(d.Text) + string(rPost[charCount2:])
		case Delete:
			patch.Length1 += len(d.Text)
			patch.Diffs = append(patch.Diffs, d)
			rPost := []rune(postpatchText)
			postpatchText = string(rPost[:charCount2]) + string(rPost[charCount2+len(d.Text):])
		case Equal:
			dLen := len(d.Text)
			if dLen <= 2*dmp.PatchMargin && len(patch.Diffs) != 0 && i != len(diffs)-1 {
				patch.Diffs = append(patch.Diffs, d)
				patch.Length1 += dLen
				patch.Length2 += dLen
			}
			if dLen >= 2*dmp.PatchMargin {
				if len(patch.Diffs) != 0 {
					dmp.patchAddContext(&patch, prepatchText)
					patches = append(patches, patch)
					patch = Patch{}
					prepatchText = postpatchText
					charCount1 = charCount2
				}
			}
		}
		if d.Type != Insert {
			charCount1 += len(d.Text)
		}
		if d.Type != Delete {
			charCount2 += len(d.Text)
		}
	}
	if len(patch.Diffs) != 0 {
		dmp.patchAddContext(&patch, prepatchText)
		patches = append(patches, patch)
	}
	return patches
}

// PatchAddPadding adds padding to the start and end of patches.
func (dmp *DiffMatchPatch) PatchAddPadding(patches []Patch) ([]Patch, string) {
	paddingLen := dmp.PatchMargin
	var nullPadding strings.Builder
	for x := 1; x <= paddingLen; x++ {
		nullPadding.WriteRune(rune(x))
	}
	pad := nullPadding.String()
	padRunes := []rune(pad)

	for i := range patches {
		patches[i].Start1 += paddingLen
		patches[i].Start2 += paddingLen
	}

	first := &patches[0]
	if len(first.Diffs) == 0 || first.Diffs[0].Type != Equal {
		first.Diffs = append([]Diff{{Equal, slices.Clone(padRunes)}}, first.Diffs...)
		first.Start1 -= paddingLen
		first.Start2 -= paddingLen
		first.Length1 += paddingLen
		first.Length2 += paddingLen
	} else if paddingLen > len(first.Diffs[0].Text) {
		extra := paddingLen - len(first.Diffs[0].Text)
		first.Diffs[0].Text = append(slices.Clone(padRunes[len(first.Diffs[0].Text):]), first.Diffs[0].Text...)
		first.Start1 -= extra
		first.Start2 -= extra
		first.Length1 += extra
		first.Length2 += extra
	}

	last := &patches[len(patches)-1]
	if len(last.Diffs) == 0 || last.Diffs[len(last.Diffs)-1].Type != Equal {
		last.Diffs = append(last.Diffs, Diff{Equal, slices.Clone(padRunes)})
		last.Length1 += paddingLen
		last.Length2 += paddingLen
	} else if paddingLen > len(last.Diffs[len(last.Diffs)-1].Text) {
		extra := paddingLen - len(last.Diffs[len(last.Diffs)-1].Text)
		last.Diffs[len(last.Diffs)-1].Text = append(last.Diffs[len(last.Diffs)-1].Text, padRunes[:extra]...)
		last.Length1 += extra
		last.Length2 += extra
	}

	return patches, pad
}

// PatchSplitMax splits patches that are too long for the match algorithm.
func (dmp *DiffMatchPatch) PatchSplitMax(patches []Patch) []Patch {
	patchSize := dmp.MatchMaxBits
	for x := 0; x < len(patches); x++ {
		if patches[x].Length1 <= patchSize {
			continue
		}
		bigpatch := patches[x]
		patches = append(patches[:x], patches[x+1:]...)
		x--
		start1 := bigpatch.Start1
		start2 := bigpatch.Start2
		var precontext []rune

		for len(bigpatch.Diffs) != 0 {
			patch := Patch{}
			empty := true
			patch.Start1 = start1 - len(precontext)
			patch.Start2 = start2 - len(precontext)
			if len(precontext) > 0 {
				patch.Length1 = len(precontext)
				patch.Length2 = len(precontext)
				patch.Diffs = append(patch.Diffs, Diff{Equal, slices.Clone(precontext)})
			}

			for len(bigpatch.Diffs) != 0 && patch.Length1 < patchSize-dmp.PatchMargin {
				diffType := bigpatch.Diffs[0].Type
				diffText := bigpatch.Diffs[0].Text
				if diffType == Insert {
					patch.Length2 += len(diffText)
					start2 += len(diffText)
					patch.Diffs = append(patch.Diffs, bigpatch.Diffs[0])
					bigpatch.Diffs = bigpatch.Diffs[1:]
					empty = false
				} else if diffType == Delete && len(patch.Diffs) == 1 &&
					patch.Diffs[0].Type == Equal && len(diffText) > 2*patchSize {
					patch.Length1 += len(diffText)
					start1 += len(diffText)
					empty = false
					patch.Diffs = append(patch.Diffs, Diff{diffType, diffText})
					bigpatch.Diffs = bigpatch.Diffs[1:]
				} else {
					take := min(len(diffText), patchSize-patch.Length1-dmp.PatchMargin)
					diffText = diffText[:take]
					patch.Length1 += take
					start1 += take
					if diffType == Equal {
						patch.Length2 += take
						start2 += take
					} else {
						empty = false
					}
					patch.Diffs = append(patch.Diffs, Diff{diffType, slices.Clone(diffText)})
					if len(diffText) == len(bigpatch.Diffs[0].Text) {
						bigpatch.Diffs = bigpatch.Diffs[1:]
					} else {
						bigpatch.Diffs[0].Text = bigpatch.Diffs[0].Text[take:]
					}
				}
			}

			precontext = []rune(Dest(patch.Diffs))
			if len(precontext) > dmp.PatchMargin {
				precontext = precontext[len(precontext)-dmp.PatchMargin:]
			}

			postcontext := []rune(Source(bigpatch.Diffs))
			if len(postcontext) > dmp.PatchMargin {
				postcontext = postcontext[:dmp.PatchMargin]
			}
			if len(postcontext) > 0 {
				patch.Length1 += len(postcontext)
				patch.Length2 += len(postcontext)
				if len(patch.Diffs) > 0 && patch.Diffs[len(patch.Diffs)-1].Type == Equal {
					patch.Diffs[len(patch.Diffs)-1].Text = append(patch.Diffs[len(patch.Diffs)-1].Text, postcontext...)
				} else {
					patch.Diffs = append(patch.Diffs, Diff{Equal, slices.Clone(postcontext)})
				}
			}

			if !empty {
				x++
				patches = append(patches[:x], append([]Patch{patch}, patches[x:]...)...)
			}
		}
	}
	return patches
}

// PatchApply applies patches to text. Returns the patched text and a boolean
// slice indicating which patches were applied.
func (dmp *DiffMatchPatch) PatchApply(patches []Patch, text string) (string, []bool) {
	if len(patches) == 0 {
		return text, []bool{}
	}

	patches = patchDeepCopy(patches)
	patches, nullPadding := dmp.PatchAddPadding(patches)
	text = nullPadding + text + nullPadding
	patches = dmp.PatchSplitMax(patches)

	x := 0
	delta := 0
	results := make([]bool, len(patches))

	for _, aPatch := range patches {
		expectedLoc := aPatch.Start2 + delta
		text1 := Source(aPatch.Diffs)
		r1 := []rune(text1)
		text1Len := len(r1)
		var startLoc, endLoc int
		endLoc = -1

		if text1Len > dmp.MatchMaxBits {
			startLoc = dmp.MatchMain(text, string(r1[:dmp.MatchMaxBits]), expectedLoc)
			if startLoc != -1 {
				endLoc = dmp.MatchMain(text,
					string(r1[text1Len-dmp.MatchMaxBits:]),
					expectedLoc+text1Len-dmp.MatchMaxBits)
				if endLoc == -1 || startLoc >= endLoc {
					startLoc = -1
				}
			}
		} else {
			startLoc = dmp.MatchMain(text, text1, expectedLoc)
		}

		if startLoc == -1 {
			results[x] = false
			delta -= aPatch.Length2 - aPatch.Length1
		} else {
			results[x] = true
			delta = startLoc - expectedLoc
			rText := []rune(text)
			textLen := len(rText)
			var text2 string
			if endLoc == -1 {
				end := min(startLoc+text1Len, textLen)
				text2 = string(rText[startLoc:end])
			} else {
				end := min(endLoc+dmp.MatchMaxBits, textLen)
				text2 = string(rText[startLoc:end])
			}
			if text1 == text2 {
				rT2 := []rune(Dest(aPatch.Diffs))
				text = string(rText[:startLoc]) + string(rT2) + string(rText[startLoc+text1Len:])
			} else {
				diffs := dmp.DiffMain(text1, text2, false)
				if text1Len > dmp.MatchMaxBits &&
					float64(Levenshtein(diffs))/float64(text1Len) > float64(dmp.PatchDeleteThreshold) {
					results[x] = false
				} else {
					diffs = CleanupSemanticLossless(diffs)
					index1 := 0
					for _, aDiff := range aPatch.Diffs {
						if aDiff.Type != Equal {
							index2 := TranslateIndex(diffs, index1)
							rText = []rune(text)
							if aDiff.Type == Insert {
								text = string(rText[:startLoc+index2]) + string(aDiff.Text) + string(rText[startLoc+index2:])
							} else if aDiff.Type == Delete {
								end := TranslateIndex(diffs, index1+len(aDiff.Text))
								text = string(rText[:startLoc+index2]) + string(rText[startLoc+end:])
							}
						}
						if aDiff.Type != Delete {
							index1 += len(aDiff.Text)
						}
					}
				}
			}
		}
		x++
	}

	padLen := utf8.RuneCountInString(nullPadding)
	rText := []rune(text)
	text = string(rText[padLen : len(rText)-padLen])
	return text, results
}

// PatchToText serializes a list of patches to a string.
func PatchToText(patches []Patch) string {
	var buf strings.Builder
	for _, p := range patches {
		buf.WriteString(p.String())
	}
	return buf.String()
}

var patchHeaderRe = regexp.MustCompile(`^@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@$`)

// PatchFromText parses a textual representation of patches.
func PatchFromText(textline string) ([]Patch, error) {
	var patches []Patch
	if textline == "" {
		return patches, nil
	}
	lines := strings.Split(textline, "\n")
	textPointer := 0
	for textPointer < len(lines) {
		m := patchHeaderRe.FindStringSubmatch(lines[textPointer])
		if m == nil {
			return nil, fmt.Errorf("invalid patch string: %q", lines[textPointer])
		}
		patch := Patch{}

		patch.Start1, _ = strconv.Atoi(m[1])
		if m[2] == "" {
			patch.Start1--
			patch.Length1 = 1
		} else if m[2] == "0" {
			patch.Length1 = 0
		} else {
			patch.Start1--
			patch.Length1, _ = strconv.Atoi(m[2])
		}

		patch.Start2, _ = strconv.Atoi(m[3])
		if m[4] == "" {
			patch.Start2--
			patch.Length2 = 1
		} else if m[4] == "0" {
			patch.Length2 = 0
		} else {
			patch.Start2--
			patch.Length2, _ = strconv.Atoi(m[4])
		}
		textPointer++

		for textPointer < len(lines) {
			if len(lines[textPointer]) == 0 {
				textPointer++
				continue
			}
			sign := lines[textPointer][0]
			line := lines[textPointer][1:]
			decoded, err := decodeURI(line)
			if err != nil {
				return nil, fmt.Errorf("invalid encoding in patch: %w", err)
			}
			switch sign {
			case '-':
				patch.Diffs = append(patch.Diffs, Diff{Delete, []rune(decoded)})
			case '+':
				patch.Diffs = append(patch.Diffs, Diff{Insert, []rune(decoded)})
			case ' ':
				patch.Diffs = append(patch.Diffs, Diff{Equal, []rune(decoded)})
			case '@':
				goto nextPatch
			default:
				return nil, fmt.Errorf("invalid patch mode %q in: %s", sign, line)
			}
			textPointer++
		}
	nextPatch:
		patches = append(patches, patch)
	}
	return patches, nil
}

