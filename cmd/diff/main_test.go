package main

import (
	"context"
	"strings"
	"testing"

	dmp "github.com/client9/diffmatchpatch"
)

// diffOps returns just the operations from a diff slice for easy assertion.
func diffOps(diffs []dmp.Diff) []dmp.Operation {
	ops := make([]dmp.Operation, len(diffs))
	for i, d := range diffs {
		ops[i] = d.Type
	}
	return ops
}

// diffTexts returns just the text strings from a diff slice.
func diffTexts(diffs []dmp.Diff) []string {
	texts := make([]string, len(diffs))
	for i, d := range diffs {
		texts[i] = string(d.Text)
	}
	return texts
}

func TestDiffByLine_identical(t *testing.T) {
	diffs := diffByLine(context.Background(), "hello\nworld\n", "hello\nworld\n")
	for _, d := range diffs {
		if d.Type != dmp.Equal {
			t.Errorf("expected all Equal, got op=%v text=%q", d.Type, string(d.Text))
		}
	}
}

func TestDiffByLine_basicChange(t *testing.T) {
	diffs := diffByLine(context.Background(), "hello\nworld\nfoo\n", "hello\nearth\nfoo\nbar\n")
	ops := diffOps(diffs)
	texts := diffTexts(diffs)

	wantOps := []dmp.Operation{dmp.Equal, dmp.Delete, dmp.Insert, dmp.Equal, dmp.Insert}
	wantTexts := []string{"hello\n", "world\n", "earth\n", "foo\n", "bar\n"}

	if len(ops) != len(wantOps) {
		t.Fatalf("got %d diffs, want %d: ops=%v texts=%v", len(ops), len(wantOps), ops, texts)
	}
	for i := range ops {
		if ops[i] != wantOps[i] {
			t.Errorf("diff[%d]: got op=%v, want %v", i, ops[i], wantOps[i])
		}
		if texts[i] != wantTexts[i] {
			t.Errorf("diff[%d]: got text=%q, want %q", i, texts[i], wantTexts[i])
		}
	}
}

func TestDiffByLine_insertAtStart(t *testing.T) {
	diffs := diffByLine(context.Background(), "line1\nline2\n", "line0\nline1\nline2\n")
	ops := diffOps(diffs)
	texts := diffTexts(diffs)

	wantOps := []dmp.Operation{dmp.Insert, dmp.Equal, dmp.Equal}
	wantTexts := []string{"line0\n", "line1\n", "line2\n"}

	if len(ops) != len(wantOps) {
		t.Fatalf("got %d diffs, want %d: ops=%v texts=%v", len(ops), len(wantOps), ops, texts)
	}
	for i := range ops {
		if ops[i] != wantOps[i] {
			t.Errorf("diff[%d]: got op=%v, want %v", i, ops[i], wantOps[i])
		}
		if texts[i] != wantTexts[i] {
			t.Errorf("diff[%d]: got text=%q, want %q", i, texts[i], wantTexts[i])
		}
	}
}

func TestDiffByLine_noTrailingNewline(t *testing.T) {
	diffs := diffByLine(context.Background(), "hello", "world")
	ops := diffOps(diffs)
	if len(ops) != 2 || ops[0] != dmp.Delete || ops[1] != dmp.Insert {
		t.Errorf("got ops=%v, want [Delete Insert]", ops)
	}
	if string(diffs[0].Text) != "hello" {
		t.Errorf("got deleted text %q, want %q", string(diffs[0].Text), "hello")
	}
	if string(diffs[1].Text) != "world" {
		t.Errorf("got inserted text %q, want %q", string(diffs[1].Text), "world")
	}
}

func unified(text1, text2, file1, file2 string, ctx int) string {
	diffs := diffByLine(context.Background(), text1, text2)
	var sb strings.Builder
	printUnified(&sb, diffs, file1, file2, ctx)
	return sb.String()
}

func TestPrintUnified_basicChange(t *testing.T) {
	got := unified("hello\nworld\nfoo\n", "hello\nearth\nfoo\nbar\n", "a.txt", "b.txt", 3)
	want := "--- a.txt\n" +
		"+++ b.txt\n" +
		"@@ -1,3 +1,4 @@\n" +
		" hello\n" +
		"-world\n" +
		"+earth\n" +
		" foo\n" +
		"+bar\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrintUnified_identical(t *testing.T) {
	got := unified("hello\nworld\n", "hello\nworld\n", "a.txt", "b.txt", 3)
	if got != "" {
		t.Errorf("expected empty output for identical files, got:\n%s", got)
	}
}

func TestPrintUnified_insertAtStart(t *testing.T) {
	got := unified("line1\nline2\n", "line0\nline1\nline2\n", "a.txt", "b.txt", 3)
	want := "--- a.txt\n" +
		"+++ b.txt\n" +
		"@@ -1,2 +1,3 @@\n" +
		"+line0\n" +
		" line1\n" +
		" line2\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrintUnified_noTrailingNewline(t *testing.T) {
	got := unified("hello", "world", "a.txt", "b.txt", 3)
	want := "--- a.txt\n" +
		"+++ b.txt\n" +
		"@@ -1 +1 @@\n" +
		"-hello\n" +
		`\ No newline at end of file` + "\n" +
		"+world\n" +
		`\ No newline at end of file` + "\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrintUnified_zeroContext(t *testing.T) {
	got := unified("hello\nworld\nfoo\n", "hello\nearth\nfoo\nbar\n", "a.txt", "b.txt", 0)
	want := "--- a.txt\n" +
		"+++ b.txt\n" +
		"@@ -2 +2 @@\n" +
		"-world\n" +
		"+earth\n" +
		"@@ -3,0 +4 @@\n" +
		"+bar\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrintUnified_twoHunks(t *testing.T) {
	// Changes far enough apart to produce two separate hunks with ctx=1.
	text1 := "a\nb\nc\nd\ne\nf\ng\nh\n"
	text2 := "A\nb\nc\nd\ne\nf\ng\nH\n"
	got := unified(text1, text2, "a.txt", "b.txt", 1)
	want := "--- a.txt\n" +
		"+++ b.txt\n" +
		"@@ -1,2 +1,2 @@\n" +
		"-a\n" +
		"+A\n" +
		" b\n" +
		"@@ -7,2 +7,2 @@\n" +
		" g\n" +
		"-h\n" +
		"+H\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSplitLines(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"a\nb\nc\n", []string{"a\n", "b\n", "c\n"}},
		{"a\nb\nc", []string{"a\n", "b\n", "c"}},
		{"", nil},
		{"single", []string{"single"}},
		{"\n", []string{"\n"}},
	}
	for _, c := range cases {
		got := splitLines(c.input)
		if len(got) != len(c.want) {
			t.Errorf("splitLines(%q) = %v, want %v", c.input, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitLines(%q)[%d] = %q, want %q", c.input, i, got[i], c.want[i])
			}
		}
	}
}
