// Package diffmatchpatch implements the Diff Match Patch algorithms for
// computing differences between two texts, fuzzy matching, and applying patches.
// Ported from Neil Fraser's original implementation at https://github.com/google/diff-match-patch.
package diffmatchpatch


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
	Text []rune
}

// String returns the text of the diff as a string.
func (d Diff) String() string { return string(d.Text) }

// Patch represents a set of changes to apply to a text.
type Patch struct {
	Diffs            []Diff
	Start1, Start2   int
	Length1, Length2 int
}



// linesCharsResult holds the output of diffLinesToRunes.
// Each element of chars1/chars2 is a rune whose integer value is an index into lineArray.
type linesCharsResult struct {
	chars1    []rune
	chars2    []rune
	lineArray []string
}
