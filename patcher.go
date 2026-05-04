package diffmatchpatch

import (
	"context"
	"slices"
	"unicode/utf8"
)

// Patcher holds configuration for computing and applying patches.
type Patcher struct {
	// DeleteThreshold is the max ratio of deleted characters before a patch is rejected (0=strict, 1=loose).
	DeleteThreshold float32
	// Margin is the number of context characters included around each change.
	Margin int
	// EditCost is the threshold used by CleanupEfficiency when cleaning diffs before patching.
	EditCost int
	// Matcher is the fuzzy-match configuration used during patch application.
	Matcher Matcher
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
		(p.Matcher.MaxBits == 0 || len(pattern) < p.Matcher.MaxBits-p.Margin-p.Margin) {
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

// MakeFromTextAndDiffs computes patches from text1 and a diff list.
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

// MakeFromDiffs computes patches from a diff list, deriving text1 from the diffs.
func (p Patcher) MakeFromDiffs(diffs []Diff) []Patch {
	return p.MakeFromTextAndDiffs(Source(diffs), diffs)
}

// Make computes patches to turn text1 into text2.
// Use context.WithTimeout to bound the diff computation time.
func (p Patcher) Make(ctx context.Context, text1, text2 string) []Patch {
	diffs := diffMainRunesFree(ctx, []rune(text1), []rune(text2), true)
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
	patchSize := p.Matcher.MaxBits
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

// Apply applies patches to text. Returns the patched text and a boolean slice
// indicating which patches were successfully applied.
// Use context.WithTimeout to bound the diff computation time for imperfect matches.
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

		if text1Len > p.Matcher.MaxBits {
			startLoc = p.Matcher.Match(text, string(r1[:p.Matcher.MaxBits]), expectedLoc)
			if startLoc != -1 {
				endLoc = p.Matcher.Match(text,
					string(r1[text1Len-p.Matcher.MaxBits:]),
					expectedLoc+text1Len-p.Matcher.MaxBits)
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
				end := min(endLoc+p.Matcher.MaxBits, textLen)
				text2 = string(rText[startLoc:end])
			}
			if text1 == text2 {
				rT2 := []rune(Dest(aPatch.Diffs))
				text = string(rText[:startLoc]) + string(rT2) + string(rText[startLoc+text1Len:])
			} else {
				diffs := diffMainRunesFree(ctx, []rune(text1), []rune(text2), false)
				if text1Len > p.Matcher.MaxBits &&
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
