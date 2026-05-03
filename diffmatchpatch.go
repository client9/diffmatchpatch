// Package diffmatchpatch implements the Diff Match Patch algorithms for
// computing differences between two texts, fuzzy matching, and applying patches.
// Ported from Neil Fraser's original implementation at https://github.com/google/diff-match-patch.
package diffmatchpatch

import "time"

// Operation defines the type of a diff.
type Operation int

const (
	Delete Operation = iota
	Insert
	Equal
)

// Diff represents a single edit operation on a piece of text.
type Diff struct {
	Type Operation
	Text string
}

// Patch represents a set of changes to apply to a text.
type Patch struct {
	Diffs            []Diff
	Start1, Start2   int
	Length1, Length2 int
}

// DiffMatchPatch holds the configuration for diff, match, and patch operations.
type DiffMatchPatch struct {
	// DiffTimeout is the number of seconds to map a diff before giving up (0 for infinity).
	DiffTimeout float64
	// DiffEditCost is the cost of an empty edit operation for efficiency cleanup.
	DiffEditCost int
	// MatchThreshold is the fuzzy match threshold (0=perfect, 1=very loose).
	MatchThreshold float32
	// MatchDistance is how far to search for a match (0=exact, math.MaxInt32=global).
	MatchDistance int
	// PatchDeleteThreshold is the threshold for deletions in patch_apply (0=strict, 1=loose).
	PatchDeleteThreshold float32
	// PatchMargin is the margin of context to include in each patch.
	PatchMargin int
	// MatchMaxBits is the number of bits in an int (use 64 on 64-bit systems).
	MatchMaxBits int
}

// New returns a DiffMatchPatch with defaults matching the original Java implementation.
func New() *DiffMatchPatch {
	return &DiffMatchPatch{
		DiffTimeout:          1.0,
		DiffEditCost:         4,
		MatchThreshold:       0.5,
		MatchDistance:        1000,
		PatchDeleteThreshold: 0.5,
		PatchMargin:          4,
		MatchMaxBits:         32,
	}
}

// diffDeadline converts DiffTimeout into an absolute time.Time deadline.
// Returns time.Time{} (zero) when DiffTimeout <= 0, meaning no deadline.
func (dmp *DiffMatchPatch) diffDeadline() time.Time {
	if dmp.DiffTimeout <= 0 {
		return time.Time{}
	}
	return time.Now().Add(time.Duration(dmp.DiffTimeout * float64(time.Second)))
}

// linesCharsResult holds the output of diffLinesToRunes.
// Each element of chars1/chars2 is a rune whose integer value is an index into lineArray.
type linesCharsResult struct {
	chars1    []rune
	chars2    []rune
	lineArray []string
}
