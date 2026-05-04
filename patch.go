package diffmatchpatch

import (
	"context"
	"slices"
	"unicode/utf8"
)

// Patcher holds configuration for computing and applying patches.
// The zero value is valid but conservative: a DeleteThreshold of 0 rejects any
// imperfect match, and a Margin of 0 includes no context around changes.
// Typical values: DeleteThreshold 0.5, Margin 4, EditCost 4,
// Matcher{Threshold: 0.5, Distance: 1000}.
type Patcher struct {
	// DeleteThreshold is the maximum acceptable edit-distance ratio between the
	// expected and matched text when applying a patch fuzzily. Patches whose
	// ratio exceeds this value are rejected. 0 requires an exact match; 0.5
	// tolerates up to half the source text being different.
	DeleteThreshold float32
	// Margin is the number of context runes included around each change in a
	// patch, and the minimum buffer size used when splitting oversized patches.
	// A value of 4 is typical.
	Margin int
	// EditCost is passed to CleanupEfficiency in Make to convert short equalities
	// into insert/delete pairs before building patches. A value of 4 is typical.
	// It is not used by MakeFromTextAndDiffs or MakeFromDiffs.
	EditCost int
	// Matcher is the fuzzy-match configuration used to locate patch positions
	// during Apply.
	Matcher Matcher
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

func (p Patcher) addContext(patch *Patch, text string) {
	if len(text) == 0 {
		return
	}
	rText := []rune(text)
	textLen := len(rText)
	pattern := rText[patch.Start2 : patch.Start2+patch.Length1]
	padding := 0

	for runesCountAtLeast2(rText, pattern) &&
		len(pattern) < bitapMaxBits-p.Margin-p.Margin {
		padding += p.Margin
		start := max(0, patch.Start2-padding)
		end := min(textLen, patch.Start2+patch.Length1+padding)
		pattern = rText[start:end]
	}
	padding += p.Margin

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

// MakeFromTextAndDiffs computes patches that transform text1 into text2, where
// diffs describes that transformation. text1 must be consistent with the Delete
// and Equal operations in diffs. Returns nil if diffs is empty.
func (p Patcher) MakeFromTextAndDiffs(text1 string, diffs []Diff) []Patch {
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
			if dLen <= 2*p.Margin && len(patch.Diffs) != 0 && i != len(diffs)-1 {
				patch.Diffs = append(patch.Diffs, d)
				patch.Length1 += dLen
				patch.Length2 += dLen
			}
			if dLen >= 2*p.Margin {
				if len(patch.Diffs) != 0 {
					p.addContext(&patch, prepatchText)
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
		p.addContext(&patch, prepatchText)
		patches = append(patches, patch)
	}
	return patches
}

// MakeFromDiffs computes patches from diffs alone, reconstructing text1 from
// the Delete and Equal segments. Prefer MakeFromTextAndDiffs when text1 is
// already available.
func (p Patcher) MakeFromDiffs(diffs []Diff) []Patch {
	return p.MakeFromTextAndDiffs(Source(diffs), diffs)
}

// Make computes patches to transform text1 into text2. It diffs the two texts,
// applies CleanupSemantic and CleanupEfficiency (using EditCost), then builds
// the patch list. Use context.WithTimeout to limit the diff computation time.
func (p Patcher) Make(ctx context.Context, text1, text2 string) []Patch {
	diffs := diffMainRunes(ctx, []rune(text1), []rune(text2), true)
	if len(diffs) > 2 {
		diffs = CleanupSemantic(diffs)
		diffs = CleanupEfficiency(diffs, p.EditCost)
	}
	return p.MakeFromTextAndDiffs(text1, diffs)
}

func (p Patcher) addPadding(patches []Patch) ([]Patch, string) {
	paddingLen := p.Margin
	var nullPadding []rune
	for x := 1; x <= paddingLen; x++ {
		nullPadding = append(nullPadding, rune(x))
	}
	pad := string(nullPadding)

	for i := range patches {
		patches[i].Start1 += paddingLen
		patches[i].Start2 += paddingLen
	}

	first := &patches[0]
	if len(first.Diffs) == 0 || first.Diffs[0].Type != Equal {
		first.Diffs = append([]Diff{{Equal, slices.Clone(nullPadding)}}, first.Diffs...)
		first.Start1 -= paddingLen
		first.Start2 -= paddingLen
		first.Length1 += paddingLen
		first.Length2 += paddingLen
	} else if paddingLen > len(first.Diffs[0].Text) {
		extra := paddingLen - len(first.Diffs[0].Text)
		first.Diffs[0].Text = append(slices.Clone(nullPadding[len(first.Diffs[0].Text):]), first.Diffs[0].Text...)
		first.Start1 -= extra
		first.Start2 -= extra
		first.Length1 += extra
		first.Length2 += extra
	}

	last := &patches[len(patches)-1]
	if len(last.Diffs) == 0 || last.Diffs[len(last.Diffs)-1].Type != Equal {
		last.Diffs = append(last.Diffs, Diff{Equal, slices.Clone(nullPadding)})
		last.Length1 += paddingLen
		last.Length2 += paddingLen
	} else if paddingLen > len(last.Diffs[len(last.Diffs)-1].Text) {
		extra := paddingLen - len(last.Diffs[len(last.Diffs)-1].Text)
		last.Diffs[len(last.Diffs)-1].Text = append(last.Diffs[len(last.Diffs)-1].Text, nullPadding[:extra]...)
		last.Length1 += extra
		last.Length2 += extra
	}

	return patches, pad
}

func (p Patcher) splitMax(patches []Patch) []Patch {
	patchSize := bitapMaxBits
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

			for len(bigpatch.Diffs) != 0 && patch.Length1 < patchSize-p.Margin {
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
					take := min(len(diffText), patchSize-patch.Length1-p.Margin)
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
			if len(precontext) > p.Margin {
				precontext = precontext[len(precontext)-p.Margin:]
			}

			postcontext := []rune(Source(bigpatch.Diffs))
			if len(postcontext) > p.Margin {
				postcontext = postcontext[:p.Margin]
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

// Apply applies patches to text and returns the patched text along with a
// boolean result per patch indicating whether it was applied successfully.
// Patches that span more than bitapMaxBits runes are split internally, so
// the results slice may be longer than the input patches slice. A patch is
// rejected if its location cannot be found within Matcher.Threshold or if the
// fuzzy edit-distance ratio exceeds DeleteThreshold.
// Use context.WithTimeout to limit time spent on fuzzy re-diffing.
func (p Patcher) Apply(ctx context.Context, patches []Patch, text string) (string, []bool) {
	if len(patches) == 0 {
		return text, []bool{}
	}

	patches = patchDeepCopy(patches)
	patches, nullPadding := p.addPadding(patches)
	text = nullPadding + text + nullPadding
	patches = p.splitMax(patches)

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

		if text1Len > bitapMaxBits {
			startLoc = p.Matcher.Match(text, string(r1[:bitapMaxBits]), expectedLoc)
			if startLoc != -1 {
				endLoc = p.Matcher.Match(text,
					string(r1[text1Len-bitapMaxBits:]),
					expectedLoc+text1Len-bitapMaxBits)
				if endLoc == -1 || startLoc >= endLoc {
					startLoc = -1
				}
			}
		} else {
			startLoc = p.Matcher.Match(text, text1, expectedLoc)
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
				end := min(endLoc+bitapMaxBits, textLen)
				text2 = string(rText[startLoc:end])
			}
			if text1 == text2 {
				rT2 := []rune(Dest(aPatch.Diffs))
				text = string(rText[:startLoc]) + string(rT2) + string(rText[startLoc+text1Len:])
			} else {
				diffs := diffMainRunes(ctx, []rune(text1), []rune(text2), false)
				if text1Len > bitapMaxBits &&
					float64(Levenshtein(diffs))/float64(text1Len) > float64(p.DeleteThreshold) {
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
