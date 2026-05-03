package diffmatchpatch

import (
	"fmt"
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
		t.Errorf("Patch.String:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestPatchFromText(t *testing.T) {
	dmp := New()

	patches, err := dmp.PatchFromText("")
	if err != nil {
		t.Fatalf("PatchFromText: empty: unexpected error: %v", err)
	}
	if len(patches) != 0 {
		t.Errorf("PatchFromText: empty: want 0 patches, got %d", len(patches))
	}

	strp := "@@ -21,18 +22,17 @@\n jump\n-s\n+ed\n  over \n-the\n+a\n %0Alaz\n"
	patches, err = dmp.PatchFromText(strp)
	if err != nil {
		t.Fatalf("PatchFromText #1: unexpected error: %v", err)
	}
	if got := patches[0].String(); got != strp {
		t.Errorf("PatchFromText #1:\nwant: %q\ngot:  %q", strp, got)
	}

	patches, err = dmp.PatchFromText("@@ -1 +1 @@\n-a\n+b\n")
	if err != nil {
		t.Fatalf("PatchFromText #2: unexpected error: %v", err)
	}
	if got := patches[0].String(); got != "@@ -1 +1 @@\n-a\n+b\n" {
		t.Errorf("PatchFromText #2: got %q", got)
	}

	patches, err = dmp.PatchFromText("@@ -1,3 +0,0 @@\n-abc\n")
	if err != nil {
		t.Fatalf("PatchFromText #3: unexpected error: %v", err)
	}
	if got := patches[0].String(); got != "@@ -1,3 +0,0 @@\n-abc\n" {
		t.Errorf("PatchFromText #3: got %q", got)
	}

	patches, err = dmp.PatchFromText("@@ -0,0 +1,3 @@\n+abc\n")
	if err != nil {
		t.Fatalf("PatchFromText #4: unexpected error: %v", err)
	}
	if got := patches[0].String(); got != "@@ -0,0 +1,3 @@\n+abc\n" {
		t.Errorf("PatchFromText #4: got %q", got)
	}

	if _, err := dmp.PatchFromText("Bad\nPatch\n"); err == nil {
		t.Error("PatchFromText #5: expected error for bad input")
	}
}

func TestPatchToText(t *testing.T) {
	dmp := New()

	strp := "@@ -21,18 +22,17 @@\n jump\n-s\n+ed\n  over \n-the\n+a\n  laz\n"
	patches, err := dmp.PatchFromText(strp)
	if err != nil {
		t.Fatalf("PatchToText: Single: PatchFromText error: %v", err)
	}
	if got := dmp.PatchToText(patches); got != strp {
		t.Errorf("PatchToText: Single:\nwant: %q\ngot:  %q", strp, got)
	}

	strp = "@@ -1,9 +1,9 @@\n-f\n+F\n oo+fooba\n@@ -7,9 +7,9 @@\n obar\n-,\n+.\n  tes\n"
	patches, err = dmp.PatchFromText(strp)
	if err != nil {
		t.Fatalf("PatchToText: Dual: PatchFromText error: %v", err)
	}
	if got := dmp.PatchToText(patches); got != strp {
		t.Errorf("PatchToText: Dual:\nwant: %q\ngot:  %q", strp, got)
	}
}

func TestPatchAddContext(t *testing.T) {
	dmp := New()
	dmp.PatchMargin = 4

	patches, _ := dmp.PatchFromText("@@ -21,4 +21,10 @@\n-jump\n+somersault\n")
	p := patches[0]
	dmp.patchAddContext(&p, "The quick brown fox jumps over the lazy dog.")
	if got := p.String(); got != "@@ -17,12 +17,18 @@\n fox \n-jump\n+somersault\n s ov\n" {
		t.Errorf("patch_addContext: Simple case: got %q", got)
	}

	patches, _ = dmp.PatchFromText("@@ -21,4 +21,10 @@\n-jump\n+somersault\n")
	p = patches[0]
	dmp.patchAddContext(&p, "The quick brown fox jumps.")
	if got := p.String(); got != "@@ -17,10 +17,16 @@\n fox \n-jump\n+somersault\n s.\n" {
		t.Errorf("patch_addContext: Not enough trailing context: got %q", got)
	}

	patches, _ = dmp.PatchFromText("@@ -3 +3,2 @@\n-e\n+at\n")
	p = patches[0]
	dmp.patchAddContext(&p, "The quick brown fox jumps.")
	if got := p.String(); got != "@@ -1,7 +1,8 @@\n Th\n-e\n+at\n  qui\n" {
		t.Errorf("patch_addContext: Not enough leading context: got %q", got)
	}

	patches, _ = dmp.PatchFromText("@@ -3 +3,2 @@\n-e\n+at\n")
	p = patches[0]
	dmp.patchAddContext(&p, "The quick brown fox jumps.  The quick brown fox crashes.")
	if got := p.String(); got != "@@ -1,27 +1,28 @@\n Th\n-e\n+at\n  quick brown fox jumps. \n" {
		t.Errorf("patch_addContext: Ambiguity: got %q", got)
	}
}

func TestPatchMake(t *testing.T) {
	dmp := New()

	patches := dmp.PatchMake("", "")
	if got := dmp.PatchToText(patches); got != "" {
		t.Errorf("patch_make: Null case: want empty, got %q", got)
	}

	text1 := "The quick brown fox jumps over the lazy dog."
	text2 := "That quick brown fox jumped over a lazy dog."

	expectedPatch := "@@ -1,8 +1,7 @@\n Th\n-at\n+e\n  qui\n@@ -21,17 +21,18 @@\n jump\n-ed\n+s\n  over \n-a\n+the\n  laz\n"
	patches = dmp.PatchMake(text2, text1)
	if got := dmp.PatchToText(patches); got != expectedPatch {
		t.Errorf("patch_make: Text2+Text1 inputs:\nwant: %q\ngot:  %q", expectedPatch, got)
	}

	expectedPatch = "@@ -1,11 +1,12 @@\n Th\n-e\n+at\n  quick b\n@@ -22,18 +22,17 @@\n jump\n-s\n+ed\n  over \n-the\n+a\n  laz\n"
	patches = dmp.PatchMake(text1, text2)
	if got := dmp.PatchToText(patches); got != expectedPatch {
		t.Errorf("patch_make: Text1+Text2 inputs:\nwant: %q\ngot:  %q", expectedPatch, got)
	}

	diffs := dmp.DiffMain(text1, text2, false)
	patches = dmp.PatchMakeFromDiffs(diffs)
	if got := dmp.PatchToText(patches); got != expectedPatch {
		t.Errorf("patch_make: Diff input:\nwant: %q\ngot:  %q", expectedPatch, got)
	}

	patches = dmp.PatchMakeFromTextAndDiffs(text1, diffs)
	if got := dmp.PatchToText(patches); got != expectedPatch {
		t.Errorf("patch_make: Text1+Diff inputs:\nwant: %q\ngot:  %q", expectedPatch, got)
	}

	patches = dmp.PatchMake("`1234567890-=[]\\;',./", "~!@#$%^&*()_+{}|:\"<>?")
	wantEncoded := "@@ -1,21 +1,21 @@\n-%601234567890-=%5B%5D%5C;',./\n+~!@#$%25%5E&*()_+%7B%7D%7C:%22%3C%3E?\n"
	if got := dmp.PatchToText(patches); got != wantEncoded {
		t.Errorf("patch_toText: Character encoding:\nwant: %q\ngot:  %q", wantEncoded, got)
	}

	patches, _ = dmp.PatchFromText("@@ -1,21 +1,21 @@\n-%601234567890-=%5B%5D%5C;',./\n+~!@#$%25%5E&*()_+%7B%7D%7C:%22%3C%3E?\n")
	wantDiffs := makeDiffs(Delete, "`1234567890-=[]\\;',./", Insert, "~!@#$%^&*()_+{}|:\"<>?")
	assertDiffsEqual(t, wantDiffs, patches[0].Diffs, "patch_fromText: Character decoding")

	text1 = ""
	for x := 0; x < 100; x++ {
		text1 += "abcdef"
	}
	text2 = text1 + "123"
	expectedPatch = "@@ -573,28 +573,31 @@\n cdefabcdefabcdefabcdefabcdef\n+123\n"
	patches = dmp.PatchMake(text1, text2)
	if got := dmp.PatchToText(patches); got != expectedPatch {
		t.Errorf("patch_make: Long string with repeats:\nwant: %q\ngot:  %q", expectedPatch, got)
	}
}

func TestPatchSplitMax(t *testing.T) {
	dmp := New()

	patches := dmp.PatchMake("abcdefghijklmnopqrstuvwxyz01234567890", "XabXcdXefXghXijXklXmnXopXqrXstXuvXwxXyzX01X23X45X67X89X0")
	patches = dmp.PatchSplitMax(patches)
	want := "@@ -1,32 +1,46 @@\n+X\n ab\n+X\n cd\n+X\n ef\n+X\n gh\n+X\n ij\n+X\n kl\n+X\n mn\n+X\n op\n+X\n qr\n+X\n st\n+X\n uv\n+X\n wx\n+X\n yz\n+X\n 012345\n@@ -25,13 +39,18 @@\n zX01\n+X\n 23\n+X\n 45\n+X\n 67\n+X\n 89\n+X\n 0\n"
	if got := dmp.PatchToText(patches); got != want {
		t.Errorf("patch_splitMax #1:\nwant: %q\ngot:  %q", want, got)
	}

	patches = dmp.PatchMake("abcdef1234567890123456789012345678901234567890123456789012345678901234567890uvwxyz", "abcdefuvwxyz")
	oldToText := dmp.PatchToText(patches)
	patches = dmp.PatchSplitMax(patches)
	if got := dmp.PatchToText(patches); got != oldToText {
		t.Errorf("patch_splitMax #2:\nwant: %q\ngot:  %q", oldToText, got)
	}

	patches = dmp.PatchMake("1234567890123456789012345678901234567890123456789012345678901234567890", "abc")
	patches = dmp.PatchSplitMax(patches)
	want = "@@ -1,32 +1,4 @@\n-1234567890123456789012345678\n 9012\n@@ -29,32 +1,4 @@\n-9012345678901234567890123456\n 7890\n@@ -57,14 +1,3 @@\n-78901234567890\n+abc\n"
	if got := dmp.PatchToText(patches); got != want {
		t.Errorf("patch_splitMax #3:\nwant: %q\ngot:  %q", want, got)
	}

	patches = dmp.PatchMake("abcdefghij , h : 0 , t : 1 abcdefghij , h : 0 , t : 1 abcdefghij , h : 0 , t : 1", "abcdefghij , h : 1 , t : 1 abcdefghij , h : 1 , t : 1 abcdefghij , h : 0 , t : 1")
	patches = dmp.PatchSplitMax(patches)
	want = "@@ -2,32 +2,32 @@\n bcdefghij , h : \n-0\n+1\n  , t : 1 abcdef\n@@ -29,32 +29,32 @@\n bcdefghij , h : \n-0\n+1\n  , t : 1 abcdef\n"
	if got := dmp.PatchToText(patches); got != want {
		t.Errorf("patch_splitMax #4:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestPatchAddPadding(t *testing.T) {
	dmp := New()

	patches := dmp.PatchMake("", "test")
	if got := dmp.PatchToText(patches); got != "@@ -0,0 +1,4 @@\n+test\n" {
		t.Errorf("patch_addPadding: Both edges full (before): got %q", got)
	}
	patches, _ = dmp.PatchAddPadding(patches)
	if got := dmp.PatchToText(patches); got != "@@ -1,8 +1,12 @@\n %01%02%03%04\n+test\n %01%02%03%04\n" {
		t.Errorf("patch_addPadding: Both edges full (after): got %q", got)
	}

	patches = dmp.PatchMake("XY", "XtestY")
	if got := dmp.PatchToText(patches); got != "@@ -1,2 +1,6 @@\n X\n+test\n Y\n" {
		t.Errorf("patch_addPadding: Both edges partial (before): got %q", got)
	}
	patches, _ = dmp.PatchAddPadding(patches)
	if got := dmp.PatchToText(patches); got != "@@ -2,8 +2,12 @@\n %02%03%04X\n+test\n Y%01%02%03\n" {
		t.Errorf("patch_addPadding: Both edges partial (after): got %q", got)
	}

	patches = dmp.PatchMake("XXXXYYYY", "XXXXtestYYYY")
	if got := dmp.PatchToText(patches); got != "@@ -1,8 +1,12 @@\n XXXX\n+test\n YYYY\n" {
		t.Errorf("patch_addPadding: Both edges none (before): got %q", got)
	}
	patches, _ = dmp.PatchAddPadding(patches)
	if got := dmp.PatchToText(patches); got != "@@ -5,8 +5,12 @@\n XXXX\n+test\n YYYY\n" {
		t.Errorf("patch_addPadding: Both edges none (after): got %q", got)
	}
}

func TestPatchApply(t *testing.T) {
	dmp := New()
	dmp.MatchDistance = 1000
	dmp.MatchThreshold = 0.5
	dmp.PatchDeleteThreshold = 0.5

	check := func(msg string, patches []Patch, text, want string) {
		t.Helper()
		result, applied := dmp.PatchApply(patches, text)
		got := result
		for _, b := range applied {
			got += fmt.Sprintf("\t%v", b)
		}
		if got != want {
			t.Errorf("patch_apply: %s:\nwant: %q\ngot:  %q", msg, want, got)
		}
	}

	patches := dmp.PatchMake("", "")
	result, applied := dmp.PatchApply(patches, "Hello world.")
	if result != "Hello world." || len(applied) != 0 {
		t.Errorf("patch_apply: Null case: got %q with %d results", result, len(applied))
	}

	patches = dmp.PatchMake("The quick brown fox jumps over the lazy dog.", "That quick brown fox jumped over a lazy dog.")
	check("Exact match", patches, "The quick brown fox jumps over the lazy dog.", "That quick brown fox jumped over a lazy dog.\ttrue\ttrue")
	check("Partial match", patches, "The quick red rabbit jumps over the tired tiger.", "That quick red rabbit jumped over a tired tiger.\ttrue\ttrue")
	check("Failed match", patches, "I am the very model of a modern major general.", "I am the very model of a modern major general.\tfalse\tfalse")

	patches = dmp.PatchMake("x1234567890123456789012345678901234567890123456789012345678901234567890y", "xabcy")
	check("Big delete, small change", patches, "x123456789012345678901234567890-----++++++++++-----123456789012345678901234567890y", "xabcy\ttrue\ttrue")
	check("Big delete, big change 1", patches, "x12345678901234567890---------------++++++++++---------------12345678901234567890y", "xabc12345678901234567890---------------++++++++++---------------12345678901234567890y\tfalse\ttrue")

	dmp.PatchDeleteThreshold = 0.6
	check("Big delete, big change 2", patches, "x12345678901234567890---------------++++++++++---------------12345678901234567890y", "xabcy\ttrue\ttrue")
	dmp.PatchDeleteThreshold = 0.5

	dmp.MatchThreshold = 0.0
	dmp.MatchDistance = 0
	patches = dmp.PatchMake("abcdefghijklmnopqrstuvwxyz--------------------1234567890", "abcXXXXXXXXXXdefghijklmnopqrstuvwxyz--------------------1234567YYYYYYYYYY890")
	check("Compensate for failed patch", patches, "ABCDEFGHIJKLMNOPQRSTUVWXYZ--------------------1234567890", "ABCDEFGHIJKLMNOPQRSTUVWXYZ--------------------1234567YYYYYYYYYY890\tfalse\ttrue")
	dmp.MatchThreshold = 0.5
	dmp.MatchDistance = 1000

	// No side effects.
	patches = dmp.PatchMake("", "test")
	patchStr := dmp.PatchToText(patches)
	dmp.PatchApply(patches, "")
	if got := dmp.PatchToText(patches); got != patchStr {
		t.Errorf("patch_apply: No side effects: patch text changed")
	}

	patches = dmp.PatchMake("The quick brown fox jumps over the lazy dog.", "Woof")
	patchStr = dmp.PatchToText(patches)
	dmp.PatchApply(patches, "The quick brown fox jumps over the lazy dog.")
	if got := dmp.PatchToText(patches); got != patchStr {
		t.Errorf("patch_apply: No side effects with major delete: patch text changed")
	}

	patches = dmp.PatchMake("", "test")
	check("Edge exact match", patches, "", "test\ttrue")

	patches = dmp.PatchMake("XY", "XtestY")
	check("Near edge exact match", patches, "XY", "XtestY\ttrue")

	patches = dmp.PatchMake("y", "y123")
	check("Edge partial match", patches, "x", "x123\ttrue")
}
