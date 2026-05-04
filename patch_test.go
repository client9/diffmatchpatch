package diffmatchpatch

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func newPatcher() Patcher {
	return Patcher{
		DeleteThreshold: 0.5,
		Margin:          4,
		EditCost:        4,
		Matcher:         Matcher{Threshold: 0.5, Distance: 1000},
	}
}

// parseTestPatch parses a simple ASCII-only patch string for use in tests.
// No URI decoding is performed; only suitable for test inputs that don't need it.
var testPatchHeaderRe = regexp.MustCompile(`^@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@$`)

func parseTestPatch(text string) Patch {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	m := testPatchHeaderRe.FindStringSubmatch(lines[0])
	var p Patch
	p.Start1, _ = strconv.Atoi(m[1])
	if m[2] == "" {
		p.Start1--
		p.Length1 = 1
	} else if m[2] == "0" {
		p.Length1 = 0
	} else {
		p.Start1--
		p.Length1, _ = strconv.Atoi(m[2])
	}
	p.Start2, _ = strconv.Atoi(m[3])
	if m[4] == "" {
		p.Start2--
		p.Length2 = 1
	} else if m[4] == "0" {
		p.Length2 = 0
	} else {
		p.Start2--
		p.Length2, _ = strconv.Atoi(m[4])
	}
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		content := []rune(line[1:])
		switch line[0] {
		case '-':
			p.Diffs = append(p.Diffs, Diff{Delete, content})
		case '+':
			p.Diffs = append(p.Diffs, Diff{Insert, content})
		case ' ':
			p.Diffs = append(p.Diffs, Diff{Equal, content})
		}
	}
	return p
}

// patchToText is a local test helper. The public API lives in serial.PatchToText.
func patchToText(patches []Patch) string {
	var buf strings.Builder
	for _, p := range patches {
		buf.WriteString(patchString(p))
	}
	return buf.String()
}

func patchString(p Patch) string {
	encodeURISafe := func() [256]bool {
		var safe [256]bool
		for c := 'A'; c <= 'Z'; c++ {
			safe[c] = true
		}
		for c := 'a'; c <= 'z'; c++ {
			safe[c] = true
		}
		for c := '0'; c <= '9'; c++ {
			safe[c] = true
		}
		for _, c := range "-_.!~*'();/?:@&=+$,# " {
			safe[c] = true
		}
		return safe
	}()
	encode := func(s string) string {
		var b strings.Builder
		for i := 0; i < len(s); {
			ch := s[i]
			if ch < 128 && encodeURISafe[ch] {
				b.WriteByte(ch)
				i++
			} else {
				_, size := utf8.DecodeRuneInString(s[i:])
				for j := range size {
					fmt.Fprintf(&b, "%%%02X", s[i+j])
				}
				i += size
			}
		}
		return b.String()
	}
	var coords1, coords2 string
	if p.Length1 == 0 {
		coords1 = fmt.Sprintf("%d,0", p.Start1)
	} else if p.Length1 == 1 {
		coords1 = fmt.Sprintf("%d", p.Start1+1)
	} else {
		coords1 = fmt.Sprintf("%d,%d", p.Start1+1, p.Length1)
	}
	if p.Length2 == 0 {
		coords2 = fmt.Sprintf("%d,0", p.Start2)
	} else if p.Length2 == 1 {
		coords2 = fmt.Sprintf("%d", p.Start2+1)
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
		buf.WriteString(encode(string(d.Text)))
		buf.WriteByte('\n')
	}
	return buf.String()
}

func TestPatchAddContext(t *testing.T) {
	pt := Patcher{Margin: 4}
	cases := []struct {
		name       string
		patchText  string
		sourceText string
		want       string
	}{
		{
			"Simple case",
			"@@ -21,4 +21,10 @@\n-jump\n+somersault\n",
			"The quick brown fox jumps over the lazy dog.",
			"@@ -17,12 +17,18 @@\n fox \n-jump\n+somersault\n s ov\n",
		},
		{
			"Not enough trailing context",
			"@@ -21,4 +21,10 @@\n-jump\n+somersault\n",
			"The quick brown fox jumps.",
			"@@ -17,10 +17,16 @@\n fox \n-jump\n+somersault\n s.\n",
		},
		{
			"Not enough leading context",
			"@@ -3 +3,2 @@\n-e\n+at\n",
			"The quick brown fox jumps.",
			"@@ -1,7 +1,8 @@\n Th\n-e\n+at\n  qui\n",
		},
		{
			"Ambiguity",
			"@@ -3 +3,2 @@\n-e\n+at\n",
			"The quick brown fox jumps.  The quick brown fox crashes.",
			"@@ -1,27 +1,28 @@\n Th\n-e\n+at\n  quick brown fox jumps. \n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := parseTestPatch(c.patchText)
			pt.addContext(&p, c.sourceText)
			if got := patchString(p); got != c.want {
				t.Errorf("want: %q\ngot:  %q", c.want, got)
			}
		})
	}
}

func TestPatchMake(t *testing.T) {
	p := newPatcher()
	ctx := context.Background()

	t.Run("Null case", func(t *testing.T) {
		patches := p.Make(ctx, "", "")
		if got := patchToText(patches); got != "" {
			t.Errorf("want empty, got %q", got)
		}
	})

	t.Run("Text2+Text1 inputs", func(t *testing.T) {
		text1 := "The quick brown fox jumps over the lazy dog."
		text2 := "That quick brown fox jumped over a lazy dog."
		want := "@@ -1,8 +1,7 @@\n Th\n-at\n+e\n  qui\n@@ -21,17 +21,18 @@\n jump\n-ed\n+s\n  over \n-a\n+the\n  laz\n"
		patches := p.Make(ctx, text2, text1)
		if got := patchToText(patches); got != want {
			t.Errorf("want: %q\ngot:  %q", want, got)
		}
	})

	t.Run("Text1+Text2 inputs", func(t *testing.T) {
		text1 := "The quick brown fox jumps over the lazy dog."
		text2 := "That quick brown fox jumped over a lazy dog."
		want := "@@ -1,11 +1,12 @@\n Th\n-e\n+at\n  quick b\n@@ -22,18 +22,17 @@\n jump\n-s\n+ed\n  over \n-the\n+a\n  laz\n"
		patches := p.Make(ctx, text1, text2)
		if got := patchToText(patches); got != want {
			t.Errorf("want: %q\ngot:  %q", want, got)
		}
	})

	t.Run("From diffs", func(t *testing.T) {
		text1 := "The quick brown fox jumps over the lazy dog."
		text2 := "That quick brown fox jumped over a lazy dog."
		want := "@@ -1,11 +1,12 @@\n Th\n-e\n+at\n  quick b\n@@ -22,18 +22,17 @@\n jump\n-s\n+ed\n  over \n-the\n+a\n  laz\n"
		diffs := DiffStrings(ctx, text1, text2)
		patches := p.MakeFromDiffs(diffs)
		if got := patchToText(patches); got != want {
			t.Errorf("want: %q\ngot:  %q", want, got)
		}
	})

	t.Run("Text1+Diff inputs", func(t *testing.T) {
		text1 := "The quick brown fox jumps over the lazy dog."
		text2 := "That quick brown fox jumped over a lazy dog."
		want := "@@ -1,11 +1,12 @@\n Th\n-e\n+at\n  quick b\n@@ -22,18 +22,17 @@\n jump\n-s\n+ed\n  over \n-the\n+a\n  laz\n"
		diffs := DiffStrings(ctx, text1, text2)
		patches := p.MakeFromTextAndDiffs(text1, diffs)
		if got := patchToText(patches); got != want {
			t.Errorf("want: %q\ngot:  %q", want, got)
		}
	})

	t.Run("Character encoding", func(t *testing.T) {
		patches := p.Make(ctx, "`1234567890-=[]\\;',./", "~!@#$%^&*()_+{}|:\"<>?")
		want := "@@ -1,21 +1,21 @@\n-%601234567890-=%5B%5D%5C;',./\n+~!@#$%25%5E&*()_+%7B%7D%7C:%22%3C%3E?\n"
		if got := patchToText(patches); got != want {
			t.Errorf("want: %q\ngot:  %q", want, got)
		}
	})

	t.Run("Long string with repeats", func(t *testing.T) {
		var text1 strings.Builder
		for range 100 {
			text1.WriteString("abcdef")
		}
		text2 := text1.String() + "123"
		want := "@@ -541,60 +541,63 @@\n abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdef\n+123\n"
		patches := p.Make(ctx, text1.String(), text2)
		if got := patchToText(patches); got != want {
			t.Errorf("want: %q\ngot:  %q", want, got)
		}
	})
}

func TestPatchSplitMax(t *testing.T) {
	p := newPatcher()
	ctx := context.Background()
	cases := []struct {
		name  string
		text1 string
		text2 string
		want  string
	}{
		{
			"Large diff",
			"abcdefghijklmnopqrstuvwxyz01234567890",
			"XabXcdXefXghXijXklXmnXopXqrXstXuvXwxXyzX01X23X45X67X89X0",
			"@@ -1,37 +1,56 @@\n+X\n ab\n+X\n cd\n+X\n ef\n+X\n gh\n+X\n ij\n+X\n kl\n+X\n mn\n+X\n op\n+X\n qr\n+X\n st\n+X\n uv\n+X\n wx\n+X\n yz\n+X\n 01\n+X\n 23\n+X\n 45\n+X\n 67\n+X\n 89\n+X\n 0\n",
		},
		{
			"Unchanged",
			"abcdef1234567890123456789012345678901234567890123456789012345678901234567890uvwxyz",
			"abcdefuvwxyz",
			// Patch spans 78 runes > bitapMaxBits (64), so splitMax splits it.
			"@@ -3,64 +3,8 @@\n cdef\n-12345678901234567890123456789012345678901234567890123456\n 7890\n@@ -59,22 +3,8 @@\n cdef\n-78901234567890\n uvwx\n",
		},
		{
			"All delete",
			"1234567890123456789012345678901234567890123456789012345678901234567890",
			"abc",
			"@@ -1,64 +1,4 @@\n-123456789012345678901234567890123456789012345678901234567890\n 1234\n@@ -61,10 +1,3 @@\n-1234567890\n+abc\n",
		},
		{
			"Interleaved edits",
			"abcdefghij , h : 0 , t : 1 abcdefghij , h : 0 , t : 1 abcdefghij , h : 0 , t : 1",
			"abcdefghij , h : 1 , t : 1 abcdefghij , h : 1 , t : 1 abcdefghij , h : 0 , t : 1",
			"@@ -1,58 +1,58 @@\n abcdefghij , h : \n-0\n+1\n  , t : 1 abcdefghij , h : 0 , t : 1 abcd\n@@ -29,33 +29,33 @@\n bcdefghij , h : \n-0\n+1\n  , t : 1 abcdefg\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			patches := p.Make(ctx, c.text1, c.text2)
			patches = p.splitMax(patches)
			if got := patchToText(patches); got != c.want {
				t.Errorf("want: %q\ngot:  %q", c.want, got)
			}
		})
	}
}

func TestPatchAddPadding(t *testing.T) {
	p := newPatcher()
	ctx := context.Background()
	cases := []struct {
		name       string
		text1      string
		text2      string
		wantBefore string
		wantAfter  string
	}{
		{
			"Both edges full",
			"", "test",
			"@@ -0,0 +1,4 @@\n+test\n",
			"@@ -1,8 +1,12 @@\n %01%02%03%04\n+test\n %01%02%03%04\n",
		},
		{
			"Both edges partial",
			"XY", "XtestY",
			"@@ -1,2 +1,6 @@\n X\n+test\n Y\n",
			"@@ -2,8 +2,12 @@\n %02%03%04X\n+test\n Y%01%02%03\n",
		},
		{
			"Both edges none",
			"XXXXYYYY", "XXXXtestYYYY",
			"@@ -1,8 +1,12 @@\n XXXX\n+test\n YYYY\n",
			"@@ -5,8 +5,12 @@\n XXXX\n+test\n YYYY\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			patches := p.Make(ctx, c.text1, c.text2)
			if got := patchToText(patches); got != c.wantBefore {
				t.Errorf("before: want: %q\ngot:  %q", c.wantBefore, got)
			}
			patches, _ = p.addPadding(patches)
			if got := patchToText(patches); got != c.wantAfter {
				t.Errorf("after: want: %q\ngot:  %q", c.wantAfter, got)
			}
		})
	}
}

func TestPatchApply(t *testing.T) {
	ctx := context.Background()
	formatResult := func(result string, applied []bool) string {
		var s strings.Builder
		s.WriteString(result)
		for _, b := range applied {
			fmt.Fprintf(&s, "\t%v", b)
		}
		return s.String()
	}

	t.Run("Null case", func(t *testing.T) {
		p := newPatcher()
		patches := p.Make(ctx, "", "")
		result, applied := p.Apply(ctx, patches, "Hello world.")
		if result != "Hello world." || len(applied) != 0 {
			t.Errorf("want %q with 0 applied, got %q with %d", "Hello world.", result, len(applied))
		}
	})

	t.Run("Standard patches", func(t *testing.T) {
		p := newPatcher()
		patches := p.Make(ctx, "The quick brown fox jumps over the lazy dog.", "That quick brown fox jumped over a lazy dog.")
		cases := []struct {
			name    string
			applyTo string
			want    string
		}{
			{"Exact match", "The quick brown fox jumps over the lazy dog.", "That quick brown fox jumped over a lazy dog.\ttrue\ttrue"},
			{"Partial match", "The quick red rabbit jumps over the tired tiger.", "That quick red rabbit jumped over a tired tiger.\ttrue\ttrue"},
			{"Failed match", "I am the very model of a modern major general.", "I am the very model of a modern major general.\tfalse\tfalse"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				result, applied := p.Apply(ctx, patches, c.applyTo)
				if got := formatResult(result, applied); got != c.want {
					t.Errorf("want: %q\ngot:  %q", c.want, got)
				}
			})
		}
	})

	t.Run("Big delete", func(t *testing.T) {
		p := newPatcher()
		patches := p.Make(ctx, "x1234567890123456789012345678901234567890123456789012345678901234567890y", "xabcy")

		t.Run("Small change", func(t *testing.T) {
			want := "xabc1234567890y\ttrue\ttrue"
			result, applied := p.Apply(ctx, patches, "x123456789012345678901234567890-----++++++++++-----123456789012345678901234567890y")
			if got := formatResult(result, applied); got != want {
				t.Errorf("want: %q\ngot:  %q", want, got)
			}
		})

		t.Run("Big change 1", func(t *testing.T) {
			want := "x12345678901234567890---------------++++++++++---------------123456abcy\tfalse\ttrue"
			result, applied := p.Apply(ctx, patches, "x12345678901234567890---------------++++++++++---------------12345678901234567890y")
			if got := formatResult(result, applied); got != want {
				t.Errorf("want: %q\ngot:  %q", want, got)
			}
		})

		t.Run("Big change 2 with higher threshold", func(t *testing.T) {
			p2 := newPatcher()
			p2.DeleteThreshold = 0.6
			want := "x12345678901234567890---------------++++++++++---------------123456abcy\tfalse\ttrue"
			result, applied := p2.Apply(ctx, patches, "x12345678901234567890---------------++++++++++---------------12345678901234567890y")
			if got := formatResult(result, applied); got != want {
				t.Errorf("want: %q\ngot:  %q", want, got)
			}
		})
	})

	t.Run("Compensate for failed patch", func(t *testing.T) {
		p := Patcher{
			DeleteThreshold: 0.5,
			Margin:          4,
			EditCost:        4,
			Matcher:         Matcher{Threshold: 0.0, Distance: 0},
		}
		patches := p.Make(ctx, "abcdefghijklmnopqrstuvwxyz--------------------1234567890", "abcXXXXXXXXXXdefghijklmnopqrstuvwxyz--------------------1234567YYYYYYYYYY890")
		want := "ABCDEFGHIJKLMNOPQRSTUVWXYZ--------------------1234567YYYYYYYYYY890\tfalse\ttrue"
		result, applied := p.Apply(ctx, patches, "ABCDEFGHIJKLMNOPQRSTUVWXYZ--------------------1234567890")
		if got := formatResult(result, applied); got != want {
			t.Errorf("want: %q\ngot:  %q", want, got)
		}
	})

	t.Run("No side effects", func(t *testing.T) {
		p := newPatcher()
		patches := p.Make(ctx, "", "test")
		patchStr := patchToText(patches)
		p.Apply(ctx, patches, "")
		if got := patchToText(patches); got != patchStr {
			t.Error("patch text changed after apply")
		}
	})

	t.Run("No side effects with major delete", func(t *testing.T) {
		p := newPatcher()
		patches := p.Make(ctx, "The quick brown fox jumps over the lazy dog.", "Woof")
		patchStr := patchToText(patches)
		p.Apply(ctx, patches, "The quick brown fox jumps over the lazy dog.")
		if got := patchToText(patches); got != patchStr {
			t.Error("patch text changed after apply")
		}
	})

	t.Run("Edge cases", func(t *testing.T) {
		p := newPatcher()
		cases := []struct {
			name    string
			text1   string
			text2   string
			applyTo string
			want    string
		}{
			{"Edge exact match", "", "test", "", "test\ttrue"},
			{"Near edge exact match", "XY", "XtestY", "XY", "XtestY\ttrue"},
			{"Edge partial match", "y", "y123", "x", "x123\ttrue"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				patches := p.Make(ctx, c.text1, c.text2)
				result, applied := p.Apply(ctx, patches, c.applyTo)
				if got := formatResult(result, applied); got != c.want {
					t.Errorf("want: %q\ngot:  %q", c.want, got)
				}
			})
		}
	})
}
