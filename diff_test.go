package diffmatchpatch

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// makeDiffs builds a []Diff from alternating Operation, string pairs.
func makeDiffs(args ...any) []Diff {
	if len(args)%2 != 0 {
		panic("makeDiffs: odd number of arguments")
	}
	diffs := make([]Diff, 0, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		diffs = append(diffs, Diff{args[i].(Operation), args[i+1].(string)})
	}
	return diffs
}

func assertDiffsEqual(t *testing.T, want, got []Diff, msg string) {
	t.Helper()
	if len(want) != len(got) {
		t.Errorf("%s: length mismatch: want %d, got %d\nwant: %v\ngot:  %v", msg, len(want), len(got), want, got)
		return
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("%s: [%d] want %v, got %v", msg, i, want[i], got[i])
		}
	}
}

func diffRebuildTexts(diffs []Diff) [2]string {
	var texts [2]string
	for _, d := range diffs {
		if d.Type != Insert {
			texts[0] += d.Text
		}
		if d.Type != Delete {
			texts[1] += d.Text
		}
	}
	return texts
}

func assertLinesRunesEqual(t *testing.T, msg string, want, got linesCharsResult) {
	t.Helper()
	if len(want.chars1) != len(got.chars1) {
		t.Errorf("%s: chars1 len: want %d, got %d", msg, len(want.chars1), len(got.chars1))
	} else {
		for i := range want.chars1 {
			if want.chars1[i] != got.chars1[i] {
				t.Errorf("%s: chars1[%d]: want %d, got %d", msg, i, want.chars1[i], got.chars1[i])
			}
		}
	}
	if len(want.chars2) != len(got.chars2) {
		t.Errorf("%s: chars2 len: want %d, got %d", msg, len(want.chars2), len(got.chars2))
	} else {
		for i := range want.chars2 {
			if want.chars2[i] != got.chars2[i] {
				t.Errorf("%s: chars2[%d]: want %d, got %d", msg, i, want.chars2[i], got.chars2[i])
			}
		}
	}
	if len(want.lineArray) != len(got.lineArray) {
		t.Errorf("%s: lineArray len: want %d, got %d", msg, len(want.lineArray), len(got.lineArray))
	} else {
		for i := range want.lineArray {
			if want.lineArray[i] != got.lineArray[i] {
				t.Errorf("%s: lineArray[%d]: want %q, got %q", msg, i, want.lineArray[i], got.lineArray[i])
			}
		}
	}
}

func TestDiffCommonPrefix(t *testing.T) {
	dmp := New()
	if n := dmp.DiffCommonPrefix("abc", "xyz"); n != 0 {
		t.Errorf("Null case: want 0, got %d", n)
	}
	if n := dmp.DiffCommonPrefix("1234abcdef", "1234xyz"); n != 4 {
		t.Errorf("Non-null case: want 4, got %d", n)
	}
	if n := dmp.DiffCommonPrefix("1234", "1234xyz"); n != 4 {
		t.Errorf("Whole case: want 4, got %d", n)
	}
}

func TestDiffCommonSuffix(t *testing.T) {
	dmp := New()
	if n := dmp.DiffCommonSuffix("abc", "xyz"); n != 0 {
		t.Errorf("Null case: want 0, got %d", n)
	}
	if n := dmp.DiffCommonSuffix("abcdef1234", "xyz1234"); n != 4 {
		t.Errorf("Non-null case: want 4, got %d", n)
	}
	if n := dmp.DiffCommonSuffix("1234", "xyz1234"); n != 4 {
		t.Errorf("Whole case: want 4, got %d", n)
	}
}

func TestDiffCommonOverlap(t *testing.T) {
	dmp := New()
	if n := dmp.diffCommonOverlap("", "abcd"); n != 0 {
		t.Errorf("Null case: want 0, got %d", n)
	}
	if n := dmp.diffCommonOverlap("abc", "abcd"); n != 3 {
		t.Errorf("Whole case: want 3, got %d", n)
	}
	if n := dmp.diffCommonOverlap("123456", "abcd"); n != 0 {
		t.Errorf("No overlap: want 0, got %d", n)
	}
	if n := dmp.diffCommonOverlap("123456xxx", "xxxabcd"); n != 3 {
		t.Errorf("Overlap: want 3, got %d", n)
	}
	// Some overly clever languages (C#) may treat ligatures as equal to their
	// component letters. E.g. U+FB01 == 'fi'
	if n := dmp.diffCommonOverlap("fi", "ﬁi"); n != 0 {
		t.Errorf("Unicode: want 0, got %d", n)
	}
}

func TestDiffHalfMatch(t *testing.T) {
	dmp := New()
	dmp.DiffTimeout = 1

	if got := dmp.diffHalfMatch("1234567890", "abcdef"); got != nil {
		t.Errorf("No match #1: want nil, got %v", got)
	}
	if got := dmp.diffHalfMatch("12345", "23"); got != nil {
		t.Errorf("No match #2: want nil, got %v", got)
	}

	check := func(msg, text1, text2 string, want []string) {
		t.Helper()
		got := dmp.diffHalfMatch(text1, text2)
		if len(got) != len(want) {
			t.Errorf("diff_halfMatch: %s: want %v, got %v", msg, want, got)
			return
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("diff_halfMatch: %s [%d]: want %q, got %q", msg, i, want[i], got[i])
			}
		}
	}

	check("Single Match #1", "1234567890", "a345678z", []string{"12", "90", "a", "z", "345678"})
	check("Single Match #2", "a345678z", "1234567890", []string{"a", "z", "12", "90", "345678"})
	check("Single Match #3", "abc56789z", "1234567890", []string{"abc", "z", "1234", "0", "56789"})
	check("Single Match #4", "a23456xyz", "1234567890", []string{"a", "xyz", "1", "7890", "23456"})
	check("Multiple Matches #1", "121231234123451234123121", "a1234123451234z", []string{"12123", "123121", "a", "z", "1234123451234"})
	check("Multiple Matches #2", "x-=-=-=-=-=-=-=-=-=-=-=-=", "xx-=-=-=-=-=-=-=", []string{"", "-=-=-=-=-=", "x", "", "x-=-=-=-=-=-=-="})
	check("Multiple Matches #3", "-=-=-=-=-=-=-=-=-=-=-=-=y", "-=-=-=-=-=-=-=yy", []string{"-=-=-=-=-=", "", "", "y", "-=-=-=-=-=-=-=y"})
	check("Non-optimal halfmatch", "qHilloHelloHew", "xHelloHeHulloy", []string{"qHillo", "w", "x", "Hulloy", "HelloHe"})

	dmp.DiffTimeout = 0
	if got := dmp.diffHalfMatch("qHilloHelloHew", "xHelloHeHulloy"); got != nil {
		t.Errorf("Optimal no halfmatch: want nil, got %v", got)
	}
}

func TestDiffLinesToRunes(t *testing.T) {
	dmp := New()

	assertLinesRunesEqual(t, "Shared lines",
		linesCharsResult{
			chars1:    []rune{1, 2, 1},
			chars2:    []rune{2, 1, 2},
			lineArray: []string{"", "alpha\n", "beta\n"},
		},
		dmp.diffLinesToRunes("alpha\nbeta\nalpha\n", "beta\nalpha\nbeta\n"))

	assertLinesRunesEqual(t, "Empty string and blank lines",
		linesCharsResult{
			chars1:    []rune{},
			chars2:    []rune{1, 2, 3, 3},
			lineArray: []string{"", "alpha\r\n", "beta\r\n", "\r\n"},
		},
		dmp.diffLinesToRunes("", "alpha\r\nbeta\r\n\r\n\r\n"))

	assertLinesRunesEqual(t, "No linebreaks",
		linesCharsResult{
			chars1:    []rune{1},
			chars2:    []rune{2},
			lineArray: []string{"", "a", "b"},
		},
		dmp.diffLinesToRunes("a", "b"))

	// More than 256 to reveal any 8-bit limitations.
	n := 300
	var lineList strings.Builder
	expectedChars := make([]rune, n)
	lineArray := make([]string, n+1)
	lineArray[0] = ""
	for i := 1; i <= n; i++ {
		line := fmt.Sprintf("%d\n", i)
		lineList.WriteString(line)
		lineArray[i] = line
		expectedChars[i-1] = rune(i)
	}
	assertLinesRunesEqual(t, "More than 256",
		linesCharsResult{chars1: expectedChars, chars2: []rune{}, lineArray: lineArray},
		dmp.diffLinesToRunes(lineList.String(), ""))
}

func TestDiffRunesToLines(t *testing.T) {
	dmp := New()

	// Diff equality.
	if (Diff{Equal, "a"}) != (Diff{Equal, "a"}) {
		t.Error("Equality: Diff structs with same fields must be equal")
	}

	// Convert runes up to lines.
	lineArray := []string{"", "alpha\n", "beta\n"}
	diffs := []Diff{
		{Equal, string([]rune{1, 2, 1})},
		{Insert, string([]rune{2, 1, 2})},
	}
	diffs = dmp.diffRunesToLines(diffs, lineArray)
	assertDiffsEqual(t, []Diff{{Equal, "alpha\nbeta\nalpha\n"}, {Insert, "beta\nalpha\nbeta\n"}}, diffs, "Shared lines")

	// More than 256.
	n := 300
	lineArray = make([]string, n+1)
	lineArray[0] = ""
	runeList := make([]rune, n)
	var lineList strings.Builder
	for i := 1; i <= n; i++ {
		line := fmt.Sprintf("%d\n", i)
		lineArray[i] = line
		lineList.WriteString(line)
		runeList[i-1] = rune(i)
	}
	diffs = []Diff{{Delete, string(runeList)}}
	diffs = dmp.diffRunesToLines(diffs, lineArray)
	assertDiffsEqual(t, []Diff{{Delete, lineList.String()}}, diffs, "More than 256")

	// More than 65536 to verify rune range handles large line counts.
	var bigList strings.Builder
	for i := 0; i < 66000; i++ {
		bigList.WriteString(fmt.Sprintf("%d\n", i))
	}
	text := bigList.String()
	result := dmp.diffLinesToRunes(text, "")
	diffs = []Diff{{Insert, string(result.chars1)}}
	diffs = dmp.diffRunesToLines(diffs, result.lineArray)
	if diffs[0].Text != text {
		t.Errorf("More than 65536: round-trip failed, lengths differ: got %d, want %d", len(diffs[0].Text), len(text))
	}
}

func TestDiffCleanupMerge(t *testing.T) {
	dmp := New()

	cases := []struct {
		msg   string
		input []Diff
		want  []Diff
	}{
		{"Null case", makeDiffs(), makeDiffs()},
		{"No change case",
			makeDiffs(Equal, "a", Delete, "b", Insert, "c"),
			makeDiffs(Equal, "a", Delete, "b", Insert, "c")},
		{"Merge equalities",
			makeDiffs(Equal, "a", Equal, "b", Equal, "c"),
			makeDiffs(Equal, "abc")},
		{"Merge deletions",
			makeDiffs(Delete, "a", Delete, "b", Delete, "c"),
			makeDiffs(Delete, "abc")},
		{"Merge insertions",
			makeDiffs(Insert, "a", Insert, "b", Insert, "c"),
			makeDiffs(Insert, "abc")},
		{"Merge interweave",
			makeDiffs(Delete, "a", Insert, "b", Delete, "c", Insert, "d", Equal, "e", Equal, "f"),
			makeDiffs(Delete, "ac", Insert, "bd", Equal, "ef")},
		{"Prefix and suffix detection",
			makeDiffs(Delete, "a", Insert, "abc", Delete, "dc"),
			makeDiffs(Equal, "a", Delete, "d", Insert, "b", Equal, "c")},
		{"Prefix and suffix detection with equalities",
			makeDiffs(Equal, "x", Delete, "a", Insert, "abc", Delete, "dc", Equal, "y"),
			makeDiffs(Equal, "xa", Delete, "d", Insert, "b", Equal, "cy")},
		{"Slide edit left",
			makeDiffs(Equal, "a", Insert, "ba", Equal, "c"),
			makeDiffs(Insert, "ab", Equal, "ac")},
		{"Slide edit right",
			makeDiffs(Equal, "c", Insert, "ab", Equal, "a"),
			makeDiffs(Equal, "ca", Insert, "ba")},
		{"Slide edit left recursive",
			makeDiffs(Equal, "a", Delete, "b", Equal, "c", Delete, "ac", Equal, "x"),
			makeDiffs(Delete, "abc", Equal, "acx")},
		{"Slide edit right recursive",
			makeDiffs(Equal, "x", Delete, "ca", Equal, "c", Delete, "b", Equal, "a"),
			makeDiffs(Equal, "xca", Delete, "cba")},
		{"Empty merge",
			makeDiffs(Delete, "b", Insert, "ab", Equal, "c"),
			makeDiffs(Insert, "a", Equal, "bc")},
		{"Empty equality",
			makeDiffs(Equal, "", Insert, "a", Equal, "b"),
			makeDiffs(Insert, "a", Equal, "b")},
	}
	for _, c := range cases {
		assertDiffsEqual(t, c.want, dmp.DiffCleanupMerge(c.input), "diff_cleanupMerge: "+c.msg)
	}
}

func TestDiffCleanupSemanticLossless(t *testing.T) {
	dmp := New()

	cases := []struct {
		msg   string
		input []Diff
		want  []Diff
	}{
		{"Null case", makeDiffs(), makeDiffs()},
		{"Blank lines",
			makeDiffs(Equal, "AAA\r\n\r\nBBB", Insert, "\r\nDDD\r\n\r\nBBB", Equal, "\r\nEEE"),
			makeDiffs(Equal, "AAA\r\n\r\n", Insert, "BBB\r\nDDD\r\n\r\n", Equal, "BBB\r\nEEE")},
		{"Line boundaries",
			makeDiffs(Equal, "AAA\r\nBBB", Insert, " DDD\r\nBBB", Equal, " EEE"),
			makeDiffs(Equal, "AAA\r\n", Insert, "BBB DDD\r\n", Equal, "BBB EEE")},
		{"Word boundaries",
			makeDiffs(Equal, "The c", Insert, "ow and the c", Equal, "at."),
			makeDiffs(Equal, "The ", Insert, "cow and the ", Equal, "cat.")},
		{"Alphanumeric boundaries",
			makeDiffs(Equal, "The-c", Insert, "ow-and-the-c", Equal, "at."),
			makeDiffs(Equal, "The-", Insert, "cow-and-the-", Equal, "cat.")},
		{"Hitting the start",
			makeDiffs(Equal, "a", Delete, "a", Equal, "ax"),
			makeDiffs(Delete, "a", Equal, "aax")},
		{"Hitting the end",
			makeDiffs(Equal, "xa", Delete, "a", Equal, "a"),
			makeDiffs(Equal, "xaa", Delete, "a")},
		{"Sentence boundaries",
			makeDiffs(Equal, "The xxx. The ", Insert, "zzz. The ", Equal, "yyy."),
			makeDiffs(Equal, "The xxx.", Insert, " The zzz.", Equal, " The yyy.")},
	}
	for _, c := range cases {
		assertDiffsEqual(t, c.want, dmp.DiffCleanupSemanticLossless(c.input), "diff_cleanupSemanticLossless: "+c.msg)
	}
}

func TestDiffCleanupSemantic(t *testing.T) {
	dmp := New()

	cases := []struct {
		msg   string
		input []Diff
		want  []Diff
	}{
		{"Null case", makeDiffs(), makeDiffs()},
		{"No elimination #1",
			makeDiffs(Delete, "ab", Insert, "cd", Equal, "12", Delete, "e"),
			makeDiffs(Delete, "ab", Insert, "cd", Equal, "12", Delete, "e")},
		{"No elimination #2",
			makeDiffs(Delete, "abc", Insert, "ABC", Equal, "1234", Delete, "wxyz"),
			makeDiffs(Delete, "abc", Insert, "ABC", Equal, "1234", Delete, "wxyz")},
		{"Simple elimination",
			makeDiffs(Delete, "a", Equal, "b", Delete, "c"),
			makeDiffs(Delete, "abc", Insert, "b")},
		{"Backpass elimination",
			makeDiffs(Delete, "ab", Equal, "cd", Delete, "e", Equal, "f", Insert, "g"),
			makeDiffs(Delete, "abcdef", Insert, "cdfg")},
		{"Multiple elimination",
			makeDiffs(Insert, "1", Equal, "A", Delete, "B", Insert, "2", Equal, "_", Insert, "1", Equal, "A", Delete, "B", Insert, "2"),
			makeDiffs(Delete, "AB_AB", Insert, "1A2_1A2")},
		{"Word boundaries",
			makeDiffs(Equal, "The c", Delete, "ow and the c", Equal, "at."),
			makeDiffs(Equal, "The ", Delete, "cow and the ", Equal, "cat.")},
		{"No overlap elimination",
			makeDiffs(Delete, "abcxx", Insert, "xxdef"),
			makeDiffs(Delete, "abcxx", Insert, "xxdef")},
		{"Overlap elimination",
			makeDiffs(Delete, "abcxxx", Insert, "xxxdef"),
			makeDiffs(Delete, "abc", Equal, "xxx", Insert, "def")},
		{"Reverse overlap elimination",
			makeDiffs(Delete, "xxxabc", Insert, "defxxx"),
			makeDiffs(Insert, "def", Equal, "xxx", Delete, "abc")},
		{"Two overlap eliminations",
			makeDiffs(Delete, "abcd1212", Insert, "1212efghi", Equal, "----", Delete, "A3", Insert, "3BC"),
			makeDiffs(Delete, "abcd", Equal, "1212", Insert, "efghi", Equal, "----", Delete, "A", Equal, "3", Insert, "BC")},
	}
	for _, c := range cases {
		assertDiffsEqual(t, c.want, dmp.DiffCleanupSemantic(c.input), "diff_cleanupSemantic: "+c.msg)
	}
}

func TestDiffCleanupEfficiency(t *testing.T) {
	dmp := New()
	dmp.DiffEditCost = 4

	cases := []struct {
		msg   string
		input []Diff
		want  []Diff
	}{
		{"Null case", makeDiffs(), makeDiffs()},
		{"No elimination",
			makeDiffs(Delete, "ab", Insert, "12", Equal, "wxyz", Delete, "cd", Insert, "34"),
			makeDiffs(Delete, "ab", Insert, "12", Equal, "wxyz", Delete, "cd", Insert, "34")},
		{"Four-edit elimination",
			makeDiffs(Delete, "ab", Insert, "12", Equal, "xyz", Delete, "cd", Insert, "34"),
			makeDiffs(Delete, "abxyzcd", Insert, "12xyz34")},
		{"Three-edit elimination",
			makeDiffs(Insert, "12", Equal, "x", Delete, "cd", Insert, "34"),
			makeDiffs(Delete, "xcd", Insert, "12x34")},
		{"Backpass elimination",
			makeDiffs(Delete, "ab", Insert, "12", Equal, "xy", Insert, "34", Equal, "z", Delete, "cd", Insert, "56"),
			makeDiffs(Delete, "abxyzcd", Insert, "12xy34z56")},
	}
	for _, c := range cases {
		assertDiffsEqual(t, c.want, dmp.DiffCleanupEfficiency(c.input), "diff_cleanupEfficiency: "+c.msg)
	}

	dmp.DiffEditCost = 5
	assertDiffsEqual(t,
		makeDiffs(Delete, "abwxyzcd", Insert, "12wxyz34"),
		dmp.DiffCleanupEfficiency(makeDiffs(Delete, "ab", Insert, "12", Equal, "wxyz", Delete, "cd", Insert, "34")),
		"diff_cleanupEfficiency: High cost elimination")
	dmp.DiffEditCost = 4
}

func TestDiffPrettyHtml(t *testing.T) {
	dmp := New()
	diffs := makeDiffs(Equal, "a\n", Delete, "<B>b</B>", Insert, "c&d")
	want := `<span>a&para;<br></span><del style="background:#ffe6e6;">&lt;B&gt;b&lt;/B&gt;</del><ins style="background:#e6ffe6;">c&amp;d</ins>`
	if got := dmp.DiffPrettyHtml(diffs); got != want {
		t.Errorf("diff_prettyHtml:\nwant: %s\ngot:  %s", want, got)
	}
}

func TestDiffText(t *testing.T) {
	dmp := New()
	diffs := makeDiffs(Equal, "jump", Delete, "s", Insert, "ed", Equal, " over ", Delete, "the", Insert, "a", Equal, " lazy")
	if got := dmp.DiffText1(diffs); got != "jumps over the lazy" {
		t.Errorf("diff_text1: want %q, got %q", "jumps over the lazy", got)
	}
	if got := dmp.DiffText2(diffs); got != "jumped over a lazy" {
		t.Errorf("diff_text2: want %q, got %q", "jumped over a lazy", got)
	}
}

func TestDiffDelta(t *testing.T) {
	dmp := New()

	diffs := makeDiffs(Equal, "jump", Delete, "s", Insert, "ed", Equal, " over ", Delete, "the", Insert, "a", Equal, " lazy", Insert, "old dog")
	text1 := dmp.DiffText1(diffs)
	if text1 != "jumps over the lazy" {
		t.Fatalf("diff_text1: Base text: want %q, got %q", "jumps over the lazy", text1)
	}

	delta := dmp.DiffToDelta(diffs)
	if delta != "=4\t-1\t+ed\t=6\t-3\t+a\t=5\t+old dog" {
		t.Errorf("diff_toDelta: want %q, got %q", "=4\t-1\t+ed\t=6\t-3\t+a\t=5\t+old dog", delta)
	}

	got, err := dmp.DiffFromDelta(text1, delta)
	if err != nil {
		t.Fatalf("diff_fromDelta: Normal: unexpected error: %v", err)
	}
	assertDiffsEqual(t, diffs, got, "diff_fromDelta: Normal")

	if _, err := dmp.DiffFromDelta(text1+"x", delta); err == nil {
		t.Error("diff_fromDelta: Too long: expected error")
	}
	if _, err := dmp.DiffFromDelta(text1[1:], delta); err == nil {
		t.Error("diff_fromDelta: Too short: expected error")
	}
	if _, err := dmp.DiffFromDelta("", "+%c3%xy"); err == nil {
		t.Error("diff_fromDelta: Invalid character: expected error")
	}

	// Special characters — uppercase URL encoding matching Java.
	diffs = makeDiffs(Equal, "ڀ \x00 \t %", Delete, "ځ \x01 \n ^", Insert, "ڂ \x02 \\ |")
	text1 = dmp.DiffText1(diffs)
	if text1 != "ڀ \x00 \t %ځ \x01 \n ^" {
		t.Errorf("diff_text1: Unicode text: want %q, got %q", "ڀ \x00 \t %ځ \x01 \n ^", text1)
	}
	delta = dmp.DiffToDelta(diffs)
	if delta != "=7\t-7\t+%DA%82 %02 %5C %7C" {
		t.Errorf("diff_toDelta: Unicode: want %q, got %q", "=7\t-7\t+%DA%82 %02 %5C %7C", delta)
	}
	got, err = dmp.DiffFromDelta(text1, delta)
	if err != nil {
		t.Fatalf("diff_fromDelta: Unicode: unexpected error: %v", err)
	}
	assertDiffsEqual(t, diffs, got, "diff_fromDelta: Unicode")

	// Unchanged characters pool.
	diffs = makeDiffs(Insert, "A-Z a-z 0-9 - _ . ! ~ * ' ( ) ; / ? : @ & = + $ , # ")
	text2 := dmp.DiffText2(diffs)
	if text2 != "A-Z a-z 0-9 - _ . ! ~ * ' ( ) ; / ? : @ & = + $ , # " {
		t.Errorf("diff_text2: Unchanged characters: got %q", text2)
	}
	delta = dmp.DiffToDelta(diffs)
	if delta != "+A-Z a-z 0-9 - _ . ! ~ * ' ( ) ; / ? : @ & = + $ , # " {
		t.Errorf("diff_toDelta: Unchanged characters: got %q", delta)
	}
	got, err = dmp.DiffFromDelta("", delta)
	if err != nil {
		t.Fatalf("diff_fromDelta: Unchanged characters: unexpected error: %v", err)
	}
	assertDiffsEqual(t, diffs, got, "diff_fromDelta: Unchanged characters")

	// 160 kb string.
	a := "abcdefghij"
	for i := 0; i < 14; i++ {
		a += a
	}
	diffs = makeDiffs(Insert, a)
	delta = dmp.DiffToDelta(diffs)
	if delta != "+"+a {
		t.Errorf("diff_toDelta: 160kb string: unexpected delta (length %d)", len(delta))
	}
	got, err = dmp.DiffFromDelta("", delta)
	if err != nil {
		t.Fatalf("diff_fromDelta: 160kb string: unexpected error: %v", err)
	}
	assertDiffsEqual(t, diffs, got, "diff_fromDelta: 160kb string")
}

func TestDiffXIndex(t *testing.T) {
	dmp := New()
	diffs := makeDiffs(Delete, "a", Insert, "1234", Equal, "xyz")
	if got := dmp.DiffXIndex(diffs, 2); got != 5 {
		t.Errorf("Translation on equality: want 5, got %d", got)
	}
	diffs = makeDiffs(Equal, "a", Delete, "1234", Equal, "xyz")
	if got := dmp.DiffXIndex(diffs, 3); got != 1 {
		t.Errorf("Translation on deletion: want 1, got %d", got)
	}
}

func TestDiffLevenshtein(t *testing.T) {
	dmp := New()
	diffs := makeDiffs(Delete, "abc", Insert, "1234", Equal, "xyz")
	if got := dmp.DiffLevenshtein(diffs); got != 4 {
		t.Errorf("Trailing equality: want 4, got %d", got)
	}
	diffs = makeDiffs(Equal, "xyz", Delete, "abc", Insert, "1234")
	if got := dmp.DiffLevenshtein(diffs); got != 4 {
		t.Errorf("Leading equality: want 4, got %d", got)
	}
	diffs = makeDiffs(Delete, "abc", Equal, "xyz", Insert, "1234")
	if got := dmp.DiffLevenshtein(diffs); got != 7 {
		t.Errorf("Middle equality: want 7, got %d", got)
	}
}

func TestDiffBisect(t *testing.T) {
	dmp := New()
	a, b := "cat", "map"

	// Normal: deadline far in the future.
	want := makeDiffs(Delete, "c", Insert, "m", Equal, "a", Delete, "t", Insert, "p")
	got := dmp.DiffBisect(a, b, time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC))
	assertDiffsEqual(t, want, got, "diff_bisect: Normal")

	// Timeout: deadline already in the past.
	want = makeDiffs(Delete, "cat", Insert, "map")
	got = dmp.DiffBisect(a, b, time.Unix(0, 1))
	assertDiffsEqual(t, want, got, "diff_bisect: Timeout")
}

func TestDiffMain(t *testing.T) {
	dmp := New()

	// Trivial diffs.
	assertDiffsEqual(t, makeDiffs(), dmp.DiffMain("", "", false), "diff_main: Null case")
	assertDiffsEqual(t, makeDiffs(Equal, "abc"), dmp.DiffMain("abc", "abc", false), "diff_main: Equality")
	assertDiffsEqual(t, makeDiffs(Equal, "ab", Insert, "123", Equal, "c"), dmp.DiffMain("abc", "ab123c", false), "diff_main: Simple insertion")
	assertDiffsEqual(t, makeDiffs(Equal, "a", Delete, "123", Equal, "bc"), dmp.DiffMain("a123bc", "abc", false), "diff_main: Simple deletion")
	assertDiffsEqual(t, makeDiffs(Equal, "a", Insert, "123", Equal, "b", Insert, "456", Equal, "c"), dmp.DiffMain("abc", "a123b456c", false), "diff_main: Two insertions")
	assertDiffsEqual(t, makeDiffs(Equal, "a", Delete, "123", Equal, "b", Delete, "456", Equal, "c"), dmp.DiffMain("a123b456c", "abc", false), "diff_main: Two deletions")

	// Real diff, timeout disabled.
	dmp.DiffTimeout = 0
	assertDiffsEqual(t, makeDiffs(Delete, "a", Insert, "b"), dmp.DiffMain("a", "b", false), "diff_main: Simple case #1")
	assertDiffsEqual(t,
		makeDiffs(Delete, "Apple", Insert, "Banana", Equal, "s are a", Insert, "lso", Equal, " fruit."),
		dmp.DiffMain("Apples are a fruit.", "Bananas are also fruit.", false),
		"diff_main: Simple case #2")
	assertDiffsEqual(t,
		makeDiffs(Delete, "a", Insert, "ڀ", Equal, "x", Delete, "\t", Insert, "\x00"),
		dmp.DiffMain("ax\t", "ڀx\x00", false),
		"diff_main: Simple case #3")
	assertDiffsEqual(t,
		makeDiffs(Delete, "1", Equal, "a", Delete, "y", Equal, "b", Delete, "2", Insert, "xab"),
		dmp.DiffMain("1ayb2", "abxab", false),
		"diff_main: Overlap #1")
	assertDiffsEqual(t,
		makeDiffs(Insert, "xaxcx", Equal, "abc", Delete, "y"),
		dmp.DiffMain("abcy", "xaxcxabc", false),
		"diff_main: Overlap #2")
	assertDiffsEqual(t,
		makeDiffs(Delete, "ABCD", Equal, "a", Delete, "=", Insert, "-", Equal, "bcd", Delete, "=", Insert, "-", Equal, "efghijklmnopqrs", Delete, "EFGHIJKLMNOefg"),
		dmp.DiffMain("ABCDa=bcd=efghijklmnopqrsEFGHIJKLMNOefg", "a-bcd-efghijklmnopqrs", false),
		"diff_main: Overlap #3")
	assertDiffsEqual(t,
		makeDiffs(Insert, " ", Equal, "a", Insert, "nd", Equal, " [[Pennsylvania]]", Delete, " and [[New"),
		dmp.DiffMain("a [[Pennsylvania]] and [[New", " and [[Pennsylvania]]", false),
		"diff_main: Large equality")

	// Timeout.
	dmp.DiffTimeout = 0.1
	longA := "`Twas brillig, and the slithy toves\nDid gyre and gimble in the wabe:\nAll mimsy were the borogoves,\nAnd the mome raths outgrabe.\n"
	longB := "I am the very model of a modern major general,\nI've information vegetable, animal, and mineral,\nI know the kings of England, and I quote the fights historical,\nFrom Marathon to Waterloo, in order categorical.\n"
	for i := 0; i < 10; i++ {
		longA += longA
		longB += longB
	}
	start := time.Now()
	dmp.DiffMain(longA, longB, true)
	elapsed := time.Since(start)
	timeout := time.Duration(dmp.DiffTimeout * float64(time.Second))
	if elapsed < timeout {
		t.Errorf("diff_main: Timeout min: elapsed %v < timeout %v", elapsed, timeout)
	}
	if elapsed > timeout*2 {
		t.Errorf("diff_main: Timeout max: elapsed %v > timeout*2 %v", elapsed, timeout*2)
	}
	dmp.DiffTimeout = 0

	// Line-mode speedup.
	a := "1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n"
	b := "abcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\n"
	assertDiffsEqual(t, dmp.DiffMain(a, b, false), dmp.DiffMain(a, b, true), "diff_main: Simple line-mode")

	a = "1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890"
	b = "abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghij"
	assertDiffsEqual(t, dmp.DiffMain(a, b, false), dmp.DiffMain(a, b, true), "diff_main: Single line-mode")

	a = "1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n"
	b = "abcdefghij\n1234567890\n1234567890\n1234567890\nabcdefghij\n1234567890\n1234567890\n1234567890\nabcdefghij\n1234567890\n1234567890\n1234567890\nabcdefghij\n"
	if diffRebuildTexts(dmp.DiffMain(a, b, true)) != diffRebuildTexts(dmp.DiffMain(a, b, false)) {
		t.Error("diff_main: Overlap line-mode: line-mode and text-mode results diverge")
	}
}
