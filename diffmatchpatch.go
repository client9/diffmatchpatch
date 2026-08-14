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

// LinesToRunesResult holds the output of [LinesToRunes]: two texts encoded
// one rune per line, plus the table mapping each rune value back to its
// line. Diffing Text1/Text2 with DiffRunes and expanding the result through
// Lines yields a line-granularity diff.
type LinesToRunesResult struct {
	Text1 []rune   // text1, encoded as one rune per line
	Text2 []rune   // text2, encoded as one rune per line
	Lines []string // Lines[r] is the line text for rune value r
}
