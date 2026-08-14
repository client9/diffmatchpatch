package diffmatchpatch

import (
	"context"
	"slices"
	"strings"
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

func diffText1(diffs []Diff) string {
	var buf strings.Builder
	for _, d := range diffs {
		if d.Type != Insert {
			buf.WriteString(string(d.Text))
		}
	}
	return buf.String()
}

func diffText2(diffs []Diff) string {
	var buf strings.Builder
	for _, d := range diffs {
		if d.Type != Delete {
			buf.WriteString(string(d.Text))
		}
	}
	return buf.String()
}

func levenshtein(diffs []Diff) int {
	dist := 0
	ins, del := 0, 0
	for _, d := range diffs {
		switch d.Type {
		case Insert:
			ins += len(d.Text)
		case Delete:
			del += len(d.Text)
		case Equal:
			dist += max(ins, del)
			ins, del = 0, 0
		}
	}
	return dist + max(ins, del)
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
	// Clamp against textLen: if diffs are inconsistent with text1 (see the
	// MakeFromTextAndDiffs doc comment), patch.Start2/Length1 can point past
	// the end of text, which would otherwise panic on the slice below.
	start2 := min(max(patch.Start2, 0), textLen)
	end2 := min(max(start2+patch.Length1, start2), textLen)
	pattern := rText[start2:end2]
	padding := 0

	for runesCountAtLeast2(rText, pattern) &&
		len(pattern) < bitapMaxBits-p.Margin-p.Margin {
		padding += p.Margin
		start := max(0, start2-padding)
		end := min(textLen, end2+padding)
		pattern = rText[start:end]
	}
	padding += p.Margin

	prefixStart := max(0, start2-padding)
	prefix := rText[prefixStart:start2]
	if len(prefix) > 0 {
		patch.Diffs = append([]Diff{{Equal, slices.Clone(prefix)}}, patch.Diffs...)
	}

	suffixEnd := min(textLen, end2+padding)
	suffix := rText[end2:suffixEnd]
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
			// charCount2 tracks the caller's claimed offset into postpatchText;
			// clamp it so an inconsistent diffs/text1 pairing (see doc comment)
			// degrades gracefully instead of panicking on a bad slice index.
			at := min(charCount2, len(rPost))
			postpatchText = string(rPost[:at]) + string(d.Text) + string(rPost[at:])
		case Delete:
			patch.Length1 += len(d.Text)
			patch.Diffs = append(patch.Diffs, d)
			rPost := []rune(postpatchText)
			start := min(charCount2, len(rPost))
			end := min(charCount2+len(d.Text), len(rPost))
			postpatchText = string(rPost[:start]) + string(rPost[end:])
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
	return p.MakeFromTextAndDiffs(diffText1(diffs), diffs)
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
	// Clamp the margin used for splitting so patchSize-margin always leaves
	// room for progress: precontext is capped at margin runes, so a margin
	// anywhere near patchSize (or larger) could make each new sub-patch start
	// already at or past the size budget, stalling the loop below forever.
	// Keeping margin under half of patchSize guarantees each outer iteration
	// consumes at least one rune from bigpatch.Diffs.
	margin := p.Margin
	if maxMargin := patchSize/2 - 1; margin > maxMargin {
		margin = maxMargin
	}
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

			for len(bigpatch.Diffs) != 0 && patch.Length1 < patchSize-margin {
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
					take := min(len(diffText), patchSize-patch.Length1-margin)
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

			precontext = []rune(diffText2(patch.Diffs))
			if len(precontext) > margin {
				precontext = precontext[len(precontext)-margin:]
			}

			postcontext := []rune(diffText1(bigpatch.Diffs))
			if len(postcontext) > margin {
				postcontext = postcontext[:margin]
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
		text1 := diffText1(aPatch.Diffs)
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
				rT2 := []rune(diffText2(aPatch.Diffs))
				text = string(rText[:startLoc]) + string(rT2) + string(rText[startLoc+text1Len:])
			} else {
				diffs := diffMainRunes(ctx, []rune(text1), []rune(text2), false)
				if text1Len > bitapMaxBits &&
					float64(levenshtein(diffs))/float64(text1Len) > float64(p.DeleteThreshold) {
					results[x] = false
				} else {
					diffs = CleanupSemanticLossless(diffs)
					index1 := 0
					// Apply every edit to the same []rune buffer in place, rather
					// than reconverting the whole text to []rune and rebuilding a
					// new string after each edit (O(N) per edit; O(K*N) for K
					// edits). rText already reflects prior edits in this loop, so
					// positions here mean exactly what they did when text/rText
					// were re-derived from each other after every step.
					for _, aDiff := range aPatch.Diffs {
						if aDiff.Type != Equal {
							index2 := TranslateIndex(diffs, index1)
							if aDiff.Type == Insert {
								rText = slices.Insert(rText, startLoc+index2, aDiff.Text...)
							} else if aDiff.Type == Delete {
								end := TranslateIndex(diffs, index1+len(aDiff.Text))
								rText = slices.Delete(rText, startLoc+index2, startLoc+end)
							}
						}
						if aDiff.Type != Delete {
							index1 += len(aDiff.Text)
						}
					}
					text = string(rText)
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
