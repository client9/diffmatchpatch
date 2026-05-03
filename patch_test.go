package diffmatchpatch

import (
	"fmt"
	"slices"
	"testing"
)

func TestPatchObj(t *testing.T) {
	p := Patch{
		Diffs:   makeDiffs(Equal, "jump", Delete, "s", Insert, "ed", Equal, " over ", Delete, "the", Insert, "a", Equal, "\nlaz"),
		Start1:  20,
		Start2:  21,
		Length1: 18,
		Length2: 17,
	}
	want := "@@ -21,18 +22,17 @@\n jump\n-s\n+ed\n  over \n-the\n+a\n %0Alaz\n"
	if got := p.String(); got != want {
		t.Errorf("want: %q\ngot:  %q", want, got)
	}
}

func TestPatchFromText(t *testing.T) {
	dmp := New()
	cases := []struct {
		name      string
		input     string
		wantPatch string
		wantErr   bool
	}{
		{"Empty", "", "", false},
		{"Single patch", "@@ -21,18 +22,17 @@\n jump\n-s\n+ed\n  over \n-the\n+a\n %0Alaz\n", "@@ -21,18 +22,17 @@\n jump\n-s\n+ed\n  over \n-the\n+a\n %0Alaz\n", false},
		{"No length #1", "@@ -1 +1 @@\n-a\n+b\n", "@@ -1 +1 @@\n-a\n+b\n", false},
		{"No length #2", "@@ -1,3 +0,0 @@\n-abc\n", "@@ -1,3 +0,0 @@\n-abc\n", false},
		{"No length #3", "@@ -0,0 +1,3 @@\n+abc\n", "@@ -0,0 +1,3 @@\n+abc\n", false},
		{"Bad input", "Bad\nPatch\n", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			patches, err := dmp.PatchFromText(c.input)
			if c.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.wantPatch == "" {
				if len(patches) != 0 {
					t.Errorf("want 0 patches, got %d", len(patches))
				}
				return
			}
			if got := patches[0].String(); got != c.wantPatch {
				t.Errorf("want: %q\ngot:  %q", c.wantPatch, got)
			}
		})
	}
}

func TestPatchToText(t *testing.T) {
	dmp := New()
	cases := []struct {
		name  string
		input string
	}{
		{"Single", "@@ -21,18 +22,17 @@\n jump\n-s\n+ed\n  over \n-the\n+a\n  laz\n"},
		{"Dual", "@@ -1,9 +1,9 @@\n-f\n+F\n oo+fooba\n@@ -7,9 +7,9 @@\n obar\n-,\n+.\n  tes\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			patches, err := dmp.PatchFromText(c.input)
			if err != nil {
				t.Fatalf("PatchFromText error: %v", err)
			}
			if got := dmp.PatchToText(patches); got != c.input {
				t.Errorf("want: %q\ngot:  %q", c.input, got)
			}
		})
	}
}

func TestPatchAddContext(t *testing.T) {
	dmp := New()
	dmp.PatchMargin = 4
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
			patches, _ := dmp.PatchFromText(c.patchText)
			p := patches[0]
			dmp.patchAddContext(&p, c.sourceText)
			if got := p.String(); got != c.want {
				t.Errorf("want: %q\ngot:  %q", c.want, got)
			}
		})
	}
}

func TestPatchMake(t *testing.T) {
	dmp := New()

	t.Run("Null case", func(t *testing.T) {
		patches := dmp.PatchMake("", "")
		if got := dmp.PatchToText(patches); got != "" {
			t.Errorf("want empty, got %q", got)
		}
	})

	t.Run("Text2+Text1 inputs", func(t *testing.T) {
		text1 := "The quick brown fox jumps over the lazy dog."
		text2 := "That quick brown fox jumped over a lazy dog."
		want := "@@ -1,8 +1,7 @@\n Th\n-at\n+e\n  qui\n@@ -21,17 +21,18 @@\n jump\n-ed\n+s\n  over \n-a\n+the\n  laz\n"
		patches := dmp.PatchMake(text2, text1)
		if got := dmp.PatchToText(patches); got != want {
			t.Errorf("want: %q\ngot:  %q", want, got)
		}
	})

	t.Run("Text1+Text2 inputs", func(t *testing.T) {
		text1 := "The quick brown fox jumps over the lazy dog."
		text2 := "That quick brown fox jumped over a lazy dog."
		want := "@@ -1,11 +1,12 @@\n Th\n-e\n+at\n  quick b\n@@ -22,18 +22,17 @@\n jump\n-s\n+ed\n  over \n-the\n+a\n  laz\n"
		patches := dmp.PatchMake(text1, text2)
		if got := dmp.PatchToText(patches); got != want {
			t.Errorf("want: %q\ngot:  %q", want, got)
		}
	})

	t.Run("From diffs", func(t *testing.T) {
		text1 := "The quick brown fox jumps over the lazy dog."
		text2 := "That quick brown fox jumped over a lazy dog."
		want := "@@ -1,11 +1,12 @@\n Th\n-e\n+at\n  quick b\n@@ -22,18 +22,17 @@\n jump\n-s\n+ed\n  over \n-the\n+a\n  laz\n"
		diffs := dmp.DiffMain(text1, text2, false)
		patches := dmp.PatchMakeFromDiffs(diffs)
		if got := dmp.PatchToText(patches); got != want {
			t.Errorf("want: %q\ngot:  %q", want, got)
		}
	})

	t.Run("Text1+Diff inputs", func(t *testing.T) {
		text1 := "The quick brown fox jumps over the lazy dog."
		text2 := "That quick brown fox jumped over a lazy dog."
		want := "@@ -1,11 +1,12 @@\n Th\n-e\n+at\n  quick b\n@@ -22,18 +22,17 @@\n jump\n-s\n+ed\n  over \n-the\n+a\n  laz\n"
		diffs := dmp.DiffMain(text1, text2, false)
		patches := dmp.PatchMakeFromTextAndDiffs(text1, diffs)
		if got := dmp.PatchToText(patches); got != want {
			t.Errorf("want: %q\ngot:  %q", want, got)
		}
	})

	t.Run("Character encoding", func(t *testing.T) {
		patches := dmp.PatchMake("`1234567890-=[]\\;',./", "~!@#$%^&*()_+{}|:\"<>?")
		want := "@@ -1,21 +1,21 @@\n-%601234567890-=%5B%5D%5C;',./\n+~!@#$%25%5E&*()_+%7B%7D%7C:%22%3C%3E?\n"
		if got := dmp.PatchToText(patches); got != want {
			t.Errorf("want: %q\ngot:  %q", want, got)
		}
	})

	t.Run("Character decoding", func(t *testing.T) {
		patches, _ := dmp.PatchFromText("@@ -1,21 +1,21 @@\n-%601234567890-=%5B%5D%5C;',./\n+~!@#$%25%5E&*()_+%7B%7D%7C:%22%3C%3E?\n")
		want := makeDiffs(Delete, "`1234567890-=[]\\;',./", Insert, "~!@#$%^&*()_+{}|:\"<>?")
		if !slices.Equal(patches[0].Diffs, want) {
			t.Errorf("want %v, got %v", want, patches[0].Diffs)
		}
	})

	t.Run("Long string with repeats", func(t *testing.T) {
		text1 := ""
		for x := 0; x < 100; x++ {
			text1 += "abcdef"
		}
		text2 := text1 + "123"
		want := "@@ -573,28 +573,31 @@\n cdefabcdefabcdefabcdefabcdef\n+123\n"
		patches := dmp.PatchMake(text1, text2)
		if got := dmp.PatchToText(patches); got != want {
			t.Errorf("want: %q\ngot:  %q", want, got)
		}
	})
}

func TestPatchSplitMax(t *testing.T) {
	dmp := New()
	cases := []struct {
		name  string
		text1 string
		text2 string
		want  string // empty string means result must equal pre-split PatchToText
	}{
		{
			"Large diff",
			"abcdefghijklmnopqrstuvwxyz01234567890",
			"XabXcdXefXghXijXklXmnXopXqrXstXuvXwxXyzX01X23X45X67X89X0",
			"@@ -1,32 +1,46 @@\n+X\n ab\n+X\n cd\n+X\n ef\n+X\n gh\n+X\n ij\n+X\n kl\n+X\n mn\n+X\n op\n+X\n qr\n+X\n st\n+X\n uv\n+X\n wx\n+X\n yz\n+X\n 012345\n@@ -25,13 +39,18 @@\n zX01\n+X\n 23\n+X\n 45\n+X\n 67\n+X\n 89\n+X\n 0\n",
		},
		{
			"Unchanged",
			"abcdef1234567890123456789012345678901234567890123456789012345678901234567890uvwxyz",
			"abcdefuvwxyz",
			"",
		},
		{
			"All delete",
			"1234567890123456789012345678901234567890123456789012345678901234567890",
			"abc",
			"@@ -1,32 +1,4 @@\n-1234567890123456789012345678\n 9012\n@@ -29,32 +1,4 @@\n-9012345678901234567890123456\n 7890\n@@ -57,14 +1,3 @@\n-78901234567890\n+abc\n",
		},
		{
			"Interleaved edits",
			"abcdefghij , h : 0 , t : 1 abcdefghij , h : 0 , t : 1 abcdefghij , h : 0 , t : 1",
			"abcdefghij , h : 1 , t : 1 abcdefghij , h : 1 , t : 1 abcdefghij , h : 0 , t : 1",
			"@@ -2,32 +2,32 @@\n bcdefghij , h : \n-0\n+1\n  , t : 1 abcdef\n@@ -29,32 +29,32 @@\n bcdefghij , h : \n-0\n+1\n  , t : 1 abcdef\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			patches := dmp.PatchMake(c.text1, c.text2)
			want := c.want
			if want == "" {
				want = dmp.PatchToText(patches)
			}
			patches = dmp.PatchSplitMax(patches)
			if got := dmp.PatchToText(patches); got != want {
				t.Errorf("want: %q\ngot:  %q", want, got)
			}
		})
	}
}

func TestPatchAddPadding(t *testing.T) {
	dmp := New()
	cases := []struct {
		name        string
		text1       string
		text2       string
		wantBefore  string
		wantAfter   string
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
			patches := dmp.PatchMake(c.text1, c.text2)
			if got := dmp.PatchToText(patches); got != c.wantBefore {
				t.Errorf("before: want: %q\ngot:  %q", c.wantBefore, got)
			}
			patches, _ = dmp.PatchAddPadding(patches)
			if got := dmp.PatchToText(patches); got != c.wantAfter {
				t.Errorf("after: want: %q\ngot:  %q", c.wantAfter, got)
			}
		})
	}
}

func TestPatchApply(t *testing.T) {
	formatResult := func(result string, applied []bool) string {
		s := result
		for _, b := range applied {
			s += fmt.Sprintf("\t%v", b)
		}
		return s
	}

	t.Run("Null case", func(t *testing.T) {
		dmp := New()
		patches := dmp.PatchMake("", "")
		result, applied := dmp.PatchApply(patches, "Hello world.")
		if result != "Hello world." || len(applied) != 0 {
			t.Errorf("want %q with 0 applied, got %q with %d", "Hello world.", result, len(applied))
		}
	})

	t.Run("Standard patches", func(t *testing.T) {
		dmp := New()
		dmp.MatchDistance = 1000
		dmp.MatchThreshold = 0.5
		dmp.PatchDeleteThreshold = 0.5
		patches := dmp.PatchMake("The quick brown fox jumps over the lazy dog.", "That quick brown fox jumped over a lazy dog.")
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
				result, applied := dmp.PatchApply(patches, c.applyTo)
				if got := formatResult(result, applied); got != c.want {
					t.Errorf("want: %q\ngot:  %q", c.want, got)
				}
			})
		}
	})

	t.Run("Big delete", func(t *testing.T) {
		dmp := New()
		dmp.MatchDistance = 1000
		dmp.MatchThreshold = 0.5
		dmp.PatchDeleteThreshold = 0.5
		patches := dmp.PatchMake("x1234567890123456789012345678901234567890123456789012345678901234567890y", "xabcy")

		t.Run("Small change", func(t *testing.T) {
			want := "xabcy\ttrue\ttrue"
			result, applied := dmp.PatchApply(patches, "x123456789012345678901234567890-----++++++++++-----123456789012345678901234567890y")
			if got := formatResult(result, applied); got != want {
				t.Errorf("want: %q\ngot:  %q", want, got)
			}
		})

		t.Run("Big change 1", func(t *testing.T) {
			want := "xabc12345678901234567890---------------++++++++++---------------12345678901234567890y\tfalse\ttrue"
			result, applied := dmp.PatchApply(patches, "x12345678901234567890---------------++++++++++---------------12345678901234567890y")
			if got := formatResult(result, applied); got != want {
				t.Errorf("want: %q\ngot:  %q", want, got)
			}
		})

		t.Run("Big change 2 with higher threshold", func(t *testing.T) {
			dmp2 := New()
			dmp2.MatchDistance = 1000
			dmp2.MatchThreshold = 0.5
			dmp2.PatchDeleteThreshold = 0.6
			want := "xabcy\ttrue\ttrue"
			result, applied := dmp2.PatchApply(patches, "x12345678901234567890---------------++++++++++---------------12345678901234567890y")
			if got := formatResult(result, applied); got != want {
				t.Errorf("want: %q\ngot:  %q", want, got)
			}
		})
	})

	t.Run("Compensate for failed patch", func(t *testing.T) {
		dmp := New()
		dmp.MatchThreshold = 0.0
		dmp.MatchDistance = 0
		dmp.PatchDeleteThreshold = 0.5
		patches := dmp.PatchMake("abcdefghijklmnopqrstuvwxyz--------------------1234567890", "abcXXXXXXXXXXdefghijklmnopqrstuvwxyz--------------------1234567YYYYYYYYYY890")
		want := "ABCDEFGHIJKLMNOPQRSTUVWXYZ--------------------1234567YYYYYYYYYY890\tfalse\ttrue"
		result, applied := dmp.PatchApply(patches, "ABCDEFGHIJKLMNOPQRSTUVWXYZ--------------------1234567890")
		if got := formatResult(result, applied); got != want {
			t.Errorf("want: %q\ngot:  %q", want, got)
		}
	})

	t.Run("No side effects", func(t *testing.T) {
		dmp := New()
		patches := dmp.PatchMake("", "test")
		patchStr := dmp.PatchToText(patches)
		dmp.PatchApply(patches, "")
		if got := dmp.PatchToText(patches); got != patchStr {
			t.Error("patch text changed after apply")
		}
	})

	t.Run("No side effects with major delete", func(t *testing.T) {
		dmp := New()
		patches := dmp.PatchMake("The quick brown fox jumps over the lazy dog.", "Woof")
		patchStr := dmp.PatchToText(patches)
		dmp.PatchApply(patches, "The quick brown fox jumps over the lazy dog.")
		if got := dmp.PatchToText(patches); got != patchStr {
			t.Error("patch text changed after apply")
		}
	})

	t.Run("Edge cases", func(t *testing.T) {
		dmp := New()
		dmp.MatchDistance = 1000
		dmp.MatchThreshold = 0.5
		dmp.PatchDeleteThreshold = 0.5
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
				patches := dmp.PatchMake(c.text1, c.text2)
				result, applied := dmp.PatchApply(patches, c.applyTo)
				if got := formatResult(result, applied); got != c.want {
					t.Errorf("want: %q\ngot:  %q", c.want, got)
				}
			})
		}
	})
}
