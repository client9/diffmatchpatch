package diffmatchpatch

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func makeDiffs(args ...any) []Diff {
	if len(args)%2 != 0 {
		panic("makeDiffs: odd number of arguments")
	}
	diffs := make([]Diff, 0, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		diffs = append(diffs, Diff{args[i].(Operation), []rune(args[i+1].(string))})
	}
	return diffs
}

func diffRebuildTexts(diffs []Diff) [2]string {
	var texts [2]string
	for _, d := range diffs {
		if d.Type != Insert {
			texts[0] += string(d.Text)
		}
		if d.Type != Delete {
			texts[1] += string(d.Text)
		}
	}
	return texts
}

func diffsEqual(a, b []Diff) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Type != b[i].Type || !slices.Equal(a[i].Text, b[i].Text) {
			return false
		}
	}
	return true
}

func TestDiffCommonPrefix(t *testing.T) {
	cases := []struct {
		name         string
		text1, text2 string
		want         int
	}{
		{"Null case", "abc", "xyz", 0},
		{"Non-null case", "1234abcdef", "1234xyz", 4},
		{"Whole case", "1234", "1234xyz", 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CommonPrefix(c.text1, c.text2); got != c.want {
				t.Errorf("want %d, got %d", c.want, got)
			}
		})
	}
}

func TestDiffCommonSuffix(t *testing.T) {
	cases := []struct {
		name         string
		text1, text2 string
		want         int
	}{
		{"Null case", "abc", "xyz", 0},
		{"Non-null case", "abcdef1234", "xyz1234", 4},
		{"Whole case", "1234", "xyz1234", 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CommonSuffix(c.text1, c.text2); got != c.want {
				t.Errorf("want %d, got %d", c.want, got)
			}
		})
	}
}

func TestDiffCommonOverlap(t *testing.T) {
	cases := []struct {
		name         string
		text1, text2 string
		want         int
	}{
		{"Null case", "", "abcd", 0},
		{"Whole case", "abc", "abcd", 3},
		{"No overlap", "123456", "abcd", 0},
		{"Overlap", "123456xxx", "xxxabcd", 3},
		{"Unicode", "fi", "ﬁi", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := diffCommonOverlap([]rune(c.text1), []rune(c.text2)); got != c.want {
				t.Errorf("want %d, got %d", c.want, got)
			}
		})
	}
}

func TestDiffHalfMatch(t *testing.T) {
	rr := func(strs ...string) [][]rune {
		result := make([][]rune, len(strs))
		for i, s := range strs {
			result[i] = []rune(s)
		}
		return result
	}
	cases := []struct {
		name         string
		text1, text2 string
		hasDeadline  bool
		want         [][]rune
	}{
		{"No match #1", "1234567890", "abcdef", true, nil},
		{"No match #2", "12345", "23", true, nil},
		{"Single match #1", "1234567890", "a345678z", true, rr("12", "90", "a", "z", "345678")},
		{"Single match #2", "a345678z", "1234567890", true, rr("a", "z", "12", "90", "345678")},
		{"Single match #3", "abc56789z", "1234567890", true, rr("abc", "z", "1234", "0", "56789")},
		{"Single match #4", "a23456xyz", "1234567890", true, rr("a", "xyz", "1", "7890", "23456")},
		{"Multiple matches #1", "121231234123451234123121", "a1234123451234z", true, rr("12123", "123121", "a", "z", "1234123451234")},
		{"Multiple matches #2", "x-=-=-=-=-=-=-=-=-=-=-=-=", "xx-=-=-=-=-=-=-=", true, rr("", "-=-=-=-=-=", "x", "", "x-=-=-=-=-=-=-=")},
		{"Multiple matches #3", "-=-=-=-=-=-=-=-=-=-=-=-=y", "-=-=-=-=-=-=-=yy", true, rr("-=-=-=-=-=", "", "", "y", "-=-=-=-=-=-=-=y")},
		{"Non-optimal halfmatch", "qHilloHelloHew", "xHelloHeHulloy", true, rr("qHillo", "w", "x", "Hulloy", "HelloHe")},
		{"Optimal no halfmatch", "qHilloHelloHew", "xHelloHeHulloy", false, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			if c.hasDeadline {
				ctx, _ = context.WithTimeout(context.Background(), time.Hour)
			}
			got := diffHalfMatchFree(ctx, []rune(c.text1), []rune(c.text2))
			if !slices.EqualFunc(got, c.want, slices.Equal) {
				t.Errorf("want %v, got %v", c.want, got)
			}
		})
	}
}

func TestDiffLinesToRunes(t *testing.T) {
	checkResult := func(t *testing.T, want, got linesCharsResult) {
		t.Helper()
		if !slices.Equal(got.chars1, want.chars1) || !slices.Equal(got.chars2, want.chars2) || !slices.Equal(got.lineArray, want.lineArray) {
			t.Errorf("want %v, got %v", want, got)
		}
	}

	t.Run("Shared lines", func(t *testing.T) {
		want := linesCharsResult{
			chars1:    []rune{1, 2, 1},
			chars2:    []rune{2, 1, 2},
			lineArray: []string{"", "alpha\n", "beta\n"},
		}
		checkResult(t, want, diffLinesToRunes("alpha\nbeta\nalpha\n", "beta\nalpha\nbeta\n"))
	})

	t.Run("Empty string and blank lines", func(t *testing.T) {
		want := linesCharsResult{
			chars1:    []rune{},
			chars2:    []rune{1, 2, 3, 3},
			lineArray: []string{"", "alpha\r\n", "beta\r\n", "\r\n"},
		}
		checkResult(t, want, diffLinesToRunes("", "alpha\r\nbeta\r\n\r\n\r\n"))
	})

	t.Run("No linebreaks", func(t *testing.T) {
		want := linesCharsResult{
			chars1:    []rune{1},
			chars2:    []rune{2},
			lineArray: []string{"", "a", "b"},
		}
		checkResult(t, want, diffLinesToRunes("a", "b"))
	})

	t.Run("More than 256", func(t *testing.T) {
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
		want := linesCharsResult{chars1: expectedChars, chars2: []rune{}, lineArray: lineArray}
		checkResult(t, want, diffLinesToRunes(lineList.String(), ""))
	})
}

func TestDiffRunesToLines(t *testing.T) {
	t.Run("Equality", func(t *testing.T) {
		d1 := Diff{Equal, []rune("a")}
		d2 := Diff{Equal, []rune("a")}
		if d1.Type != d2.Type || !slices.Equal(d1.Text, d2.Text) {
			t.Error("Diff structs with same fields must be equal")
		}
	})

	t.Run("Shared lines", func(t *testing.T) {
		lineArray := []string{"", "alpha\n", "beta\n"}
		diffs := []Diff{
			{Equal, []rune{1, 2, 1}},
			{Insert, []rune{2, 1, 2}},
		}
		want := []Diff{{Equal, []rune("alpha\nbeta\nalpha\n")}, {Insert, []rune("beta\nalpha\nbeta\n")}}
		got := diffRunesToLinesFree(diffs, lineArray)
		if !diffsEqual(got, want) {
			t.Errorf("want %v, got %v", want, got)
		}
	})

	t.Run("More than 256", func(t *testing.T) {
		n := 300
		lineArray := make([]string, n+1)
		lineArray[0] = ""
		runeList := make([]rune, n)
		var lineList strings.Builder
		for i := 1; i <= n; i++ {
			line := fmt.Sprintf("%d\n", i)
			lineArray[i] = line
			lineList.WriteString(line)
			runeList[i-1] = rune(i)
		}
		diffs := []Diff{{Delete, runeList}}
		want := []Diff{{Delete, []rune(lineList.String())}}
		got := diffRunesToLinesFree(diffs, lineArray)
		if !diffsEqual(got, want) {
			t.Errorf("want %v, got %v", want, got)
		}
	})

	t.Run("More than 65536", func(t *testing.T) {
		var bigList strings.Builder
		for i := range 66000 {
			fmt.Fprintf(&bigList, "%d\n", i)
		}
		text := bigList.String()
		result := diffLinesToRunes(text, "")
		diffs := []Diff{{Insert, result.chars1}}
		diffs = diffRunesToLinesFree(diffs, result.lineArray)
		if string(diffs[0].Text) != text {
			t.Errorf("round-trip failed: got length %d, want %d", len(diffs[0].Text), len([]rune(text)))
		}
	})
}

func TestDiffCleanupMerge(t *testing.T) {
	cases := []struct {
		name  string
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
		t.Run(c.name, func(t *testing.T) {
			got := CleanupMerge(c.input)
			if !diffsEqual(got, c.want) {
				t.Errorf("want %v, got %v", c.want, got)
			}
		})
	}
}

func TestDiffCleanupSemanticLossless(t *testing.T) {
	cases := []struct {
		name  string
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
		t.Run(c.name, func(t *testing.T) {
			got := CleanupSemanticLossless(c.input)
			if !diffsEqual(got, c.want) {
				t.Errorf("want %v, got %v", c.want, got)
			}
		})
	}
}

func TestDiffCleanupSemantic(t *testing.T) {
	cases := []struct {
		name  string
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
		t.Run(c.name, func(t *testing.T) {
			got := CleanupSemantic(c.input)
			if !diffsEqual(got, c.want) {
				t.Errorf("want %v, got %v", c.want, got)
			}
		})
	}
}

func TestDiffCleanupEfficiency(t *testing.T) {
	cases := []struct {
		name     string
		editCost int
		input    []Diff
		want     []Diff
	}{
		{"Null case", 4, makeDiffs(), makeDiffs()},
		{"No elimination", 4,
			makeDiffs(Delete, "ab", Insert, "12", Equal, "wxyz", Delete, "cd", Insert, "34"),
			makeDiffs(Delete, "ab", Insert, "12", Equal, "wxyz", Delete, "cd", Insert, "34")},
		{"Four-edit elimination", 4,
			makeDiffs(Delete, "ab", Insert, "12", Equal, "xyz", Delete, "cd", Insert, "34"),
			makeDiffs(Delete, "abxyzcd", Insert, "12xyz34")},
		{"Three-edit elimination", 4,
			makeDiffs(Insert, "12", Equal, "x", Delete, "cd", Insert, "34"),
			makeDiffs(Delete, "xcd", Insert, "12x34")},
		{"Backpass elimination", 4,
			makeDiffs(Delete, "ab", Insert, "12", Equal, "xy", Insert, "34", Equal, "z", Delete, "cd", Insert, "56"),
			makeDiffs(Delete, "abxyzcd", Insert, "12xy34z56")},
		{"High cost elimination", 5,
			makeDiffs(Delete, "ab", Insert, "12", Equal, "wxyz", Delete, "cd", Insert, "34"),
			makeDiffs(Delete, "abwxyzcd", Insert, "12wxyz34")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CleanupEfficiency(c.input, c.editCost)
			if !diffsEqual(got, c.want) {
				t.Errorf("want %v, got %v", c.want, got)
			}
		})
	}
}

func TestDiffPrettyHtml(t *testing.T) {
	diffs := makeDiffs(Equal, "a\n", Delete, "<B>b</B>", Insert, "c&d")
	want := `<span>a&para;<br></span><del style="background:#ffe6e6;">&lt;B&gt;b&lt;/B&gt;</del><ins style="background:#e6ffe6;">c&amp;d</ins>`
	if got := PrettyHtml(diffs); got != want {
		t.Errorf("want: %s\ngot:  %s", want, got)
	}
}

func TestDiffText(t *testing.T) {
	diffs := makeDiffs(Equal, "jump", Delete, "s", Insert, "ed", Equal, " over ", Delete, "the", Insert, "a", Equal, " lazy")
	if got := Source(diffs); got != "jumps over the lazy" {
		t.Errorf("Source: want %q, got %q", "jumps over the lazy", got)
	}
	if got := Dest(diffs); got != "jumped over a lazy" {
		t.Errorf("Dest: want %q, got %q", "jumped over a lazy", got)
	}
}

func TestDiffDelta(t *testing.T) {
	cases := []struct {
		name      string
		text1     string
		diffs     []Diff
		wantDelta string
	}{
		{
			name:      "Normal",
			text1:     "jumps over the lazy",
			diffs:     makeDiffs(Equal, "jump", Delete, "s", Insert, "ed", Equal, " over ", Delete, "the", Insert, "a", Equal, " lazy", Insert, "old dog"),
			wantDelta: "=4\t-1\t+ed\t=6\t-3\t+a\t=5\t+old dog",
		},
		{
			name:      "Unicode",
			text1:     "ڀ \x00 \t %ځ \x01 \n ^",
			diffs:     makeDiffs(Equal, "ڀ \x00 \t %", Delete, "ځ \x01 \n ^", Insert, "ڂ \x02 \\ |"),
			wantDelta: "=7\t-7\t+%DA%82 %02 %5C %7C",
		},
		{
			name:      "Unchanged characters",
			text1:     "",
			diffs:     makeDiffs(Insert, "A-Z a-z 0-9 - _ . ! ~ * ' ( ) ; / ? : @ & = + $ , # "),
			wantDelta: "+A-Z a-z 0-9 - _ . ! ~ * ' ( ) ; / ? : @ & = + $ , # ",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			delta := ToDelta(c.diffs)
			if delta != c.wantDelta {
				t.Errorf("ToDelta: want %q, got %q", c.wantDelta, delta)
			}
			got, err := FromDelta(c.text1, delta)
			if err != nil {
				t.Fatalf("FromDelta: unexpected error: %v", err)
			}
			if !diffsEqual(got, c.diffs) {
				t.Errorf("FromDelta: want %v, got %v", c.diffs, got)
			}
		})
	}

	t.Run("160kb string", func(t *testing.T) {
		var a strings.Builder
		a.WriteString("abcdefghij")
		for range 14 {
			a.WriteString(a.String())
		}
		diffs := makeDiffs(Insert, a.String())
		delta := ToDelta(diffs)
		if delta != "+"+a.String() {
			t.Errorf("ToDelta: unexpected delta length %d", len(delta))
		}
		got, err := FromDelta("", delta)
		if err != nil {
			t.Fatalf("FromDelta: unexpected error: %v", err)
		}
		if !diffsEqual(got, diffs) {
			t.Errorf("FromDelta: round-trip failed")
		}
	})

	t.Run("errors", func(t *testing.T) {
		text1 := "jumps over the lazy"
		delta := "=4\t-1\t+ed\t=6\t-3\t+a\t=5\t+old dog"
		if _, err := FromDelta(text1+"x", delta); err == nil {
			t.Error("Too long: expected error")
		}
		if _, err := FromDelta(text1[1:], delta); err == nil {
			t.Error("Too short: expected error")
		}
		if _, err := FromDelta("", "+%c3%xy"); err == nil {
			t.Error("Invalid character: expected error")
		}
	})
}

func TestDiffXIndex(t *testing.T) {
	cases := []struct {
		name  string
		diffs []Diff
		loc   int
		want  int
	}{
		{"Translation on equality", makeDiffs(Delete, "a", Insert, "1234", Equal, "xyz"), 2, 5},
		{"Translation on deletion", makeDiffs(Equal, "a", Delete, "1234", Equal, "xyz"), 3, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := TranslateIndex(c.diffs, c.loc); got != c.want {
				t.Errorf("want %d, got %d", c.want, got)
			}
		})
	}
}

func TestDiffLevenshtein(t *testing.T) {
	cases := []struct {
		name  string
		diffs []Diff
		want  int
	}{
		{"Trailing equality", makeDiffs(Delete, "abc", Insert, "1234", Equal, "xyz"), 4},
		{"Leading equality", makeDiffs(Equal, "xyz", Delete, "abc", Insert, "1234"), 4},
		{"Middle equality", makeDiffs(Delete, "abc", Equal, "xyz", Insert, "1234"), 7},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Levenshtein(c.diffs); got != c.want {
				t.Errorf("want %d, got %d", c.want, got)
			}
		})
	}
}

func TestDiffBisect(t *testing.T) {
	a, b := "cat", "map"

	t.Run("Normal", func(t *testing.T) {
		want := makeDiffs(Delete, "c", Insert, "m", Equal, "a", Delete, "t", Insert, "p")
		got := diffBisectFree(context.Background(), []rune(a), []rune(b))
		if !diffsEqual(got, want) {
			t.Errorf("want %v, got %v", want, got)
		}
	})

	t.Run("Timeout", func(t *testing.T) {
		want := makeDiffs(Delete, "cat", Insert, "map")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		got := diffBisectFree(ctx, []rune(a), []rune(b))
		if !diffsEqual(got, want) {
			t.Errorf("want %v, got %v", want, got)
		}
	})
}

func TestDiffMain(t *testing.T) {
	cases := []struct {
		name       string
		text1      string
		text2      string
		checklines bool
		want       []Diff
	}{
		{"Null case", "", "", false, makeDiffs()},
		{"Equality", "abc", "abc", false, makeDiffs(Equal, "abc")},
		{"Simple insertion", "abc", "ab123c", false, makeDiffs(Equal, "ab", Insert, "123", Equal, "c")},
		{"Simple deletion", "a123bc", "abc", false, makeDiffs(Equal, "a", Delete, "123", Equal, "bc")},
		{"Two insertions", "abc", "a123b456c", false,
			makeDiffs(Equal, "a", Insert, "123", Equal, "b", Insert, "456", Equal, "c")},
		{"Two deletions", "a123b456c", "abc", false,
			makeDiffs(Equal, "a", Delete, "123", Equal, "b", Delete, "456", Equal, "c")},
		{"Simple case #1", "a", "b", false,
			makeDiffs(Delete, "a", Insert, "b")},
		{"Simple case #2", "Apples are a fruit.", "Bananas are also fruit.", false,
			makeDiffs(Delete, "Apple", Insert, "Banana", Equal, "s are a", Insert, "lso", Equal, " fruit.")},
		{"Simple case #3", "ax\t", "ڀx\x00", false,
			makeDiffs(Delete, "a", Insert, "ڀ", Equal, "x", Delete, "\t", Insert, "\x00")},
		{"Overlap #1", "1ayb2", "abxab", false,
			makeDiffs(Delete, "1", Equal, "a", Delete, "y", Equal, "b", Delete, "2", Insert, "xab")},
		{"Overlap #2", "abcy", "xaxcxabc", false,
			makeDiffs(Insert, "xaxcx", Equal, "abc", Delete, "y")},
		{"Overlap #3", "ABCDa=bcd=efghijklmnopqrsEFGHIJKLMNOefg", "a-bcd-efghijklmnopqrs", false,
			makeDiffs(Delete, "ABCD", Equal, "a", Delete, "=", Insert, "-", Equal, "bcd", Delete, "=", Insert, "-", Equal, "efghijklmnopqrs", Delete, "EFGHIJKLMNOefg")},
		{"Large equality", "a [[Pennsylvania]] and [[New", " and [[Pennsylvania]]", false,
			makeDiffs(Insert, " ", Equal, "a", Insert, "nd", Equal, " [[Pennsylvania]]", Delete, " and [[New")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := diffMainRunesFree(context.Background(), []rune(c.text1), []rune(c.text2), c.checklines)
			if !diffsEqual(got, c.want) {
				t.Errorf("want %v, got %v", c.want, got)
			}
		})
	}

	t.Run("Timeout", func(t *testing.T) {
		timeout := 100 * time.Millisecond
		var longA strings.Builder
		longA.WriteString("`Twas brillig, and the slithy toves\nDid gyre and gimble in the wabe:\nAll mimsy were the borogoves,\nAnd the mome raths outgrabe.\n")
		longB := "I am the very model of a modern major general,\nI've information vegetable, animal, and mineral,\nI know the kings of England, and I quote the fights historical,\nFrom Marathon to Waterloo, in order categorical.\n"
		for range 10 {
			longA.WriteString(longA.String())
			longB += longB
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		start := time.Now()
		diffMainRunesFree(ctx, []rune(longA.String()), []rune(longB), true)
		elapsed := time.Since(start)
		if elapsed < timeout {
			t.Errorf("Timeout min: elapsed %v < timeout %v", elapsed, timeout)
		}
		if elapsed > timeout*2 {
			t.Errorf("Timeout max: elapsed %v > timeout*2 %v", elapsed, timeout*2)
		}
	})

	t.Run("Simple line-mode", func(t *testing.T) {
		ctx := context.Background()
		a := "1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n"
		b := "abcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\n"
		if !diffsEqual(diffMainRunesFree(ctx, []rune(a), []rune(b), false), diffMainRunesFree(ctx, []rune(a), []rune(b), true)) {
			t.Error("line-mode and char-mode results differ")
		}
	})

	t.Run("Single line-mode", func(t *testing.T) {
		ctx := context.Background()
		a := "1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890"
		b := "abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghij"
		if !diffsEqual(diffMainRunesFree(ctx, []rune(a), []rune(b), false), diffMainRunesFree(ctx, []rune(a), []rune(b), true)) {
			t.Error("line-mode and char-mode results differ")
		}
	})

	t.Run("Overlap line-mode", func(t *testing.T) {
		ctx := context.Background()
		a := "1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n"
		b := "abcdefghij\n1234567890\n1234567890\n1234567890\nabcdefghij\n1234567890\n1234567890\n1234567890\nabcdefghij\n1234567890\n1234567890\n1234567890\nabcdefghij\n"
		if diffRebuildTexts(diffMainRunesFree(ctx, []rune(a), []rune(b), true)) != diffRebuildTexts(diffMainRunesFree(ctx, []rune(a), []rune(b), false)) {
			t.Error("line-mode and text-mode results diverge")
		}
	})
}
