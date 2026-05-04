// Package diffmatchpatch implements the Diff Match Patch algorithms for
// computing differences between two texts, fuzzy matching, and applying patches.
// Ported from Neil Fraser's original implementation at https://github.com/google/diff-match-patch.
package diffmatchpatch

// Operation is the type of a diff edit operation.
type Operation int

const (
	Delete Operation = iota // Delete marks text present in text1 but absent from text2.
	Insert                  // Insert marks text absent from text1 but present in text2.
	Equal                   // Equal marks text identical in both text1 and text2.
)

// Diff represents a single edit operation on a piece of text.
type Diff struct {
	Type Operation
	Text []rune
}

// String returns the text of the diff as a string.
func (d Diff) String() string { return string(d.Text) }

// Patch represents a set of diffs to apply to a text.
type Patch struct {
	Diffs   []Diff
	Start1  int // start position in text1 (source)
	Start2  int // start position in text2 (target)
	Length1 int // length of the affected region in text1
	Length2 int // length of the affected region in text2
}

// linesCharsResult holds the output of diffLinesToRunes.
// Each element of chars1/chars2 is a rune whose integer value is an index into lineArray.
type linesCharsResult struct {
	chars1    []rune
	chars2    []rune
	lineArray []string
}
