package serial

import (
	"slices"
	"strings"
	"testing"

	dmp "github.com/client9/diffmatchpatch"
)

// — helpers ———————————————————————————————————————————————————————————————————

func makeDiffs(args ...any) []dmp.Diff {
	if len(args)%2 != 0 {
		panic("makeDiffs: odd number of arguments")
	}
	out := make([]dmp.Diff, 0, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		out = append(out, dmp.Diff{Type: args[i].(dmp.Operation), Text: []rune(args[i+1].(string))})
	}
	return out
}

func diffsEqual(a, b []dmp.Diff) bool {
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

// — Delta tests ———————————————————————————————————————————————————————————————

func TestToDelta(t *testing.T) {
	cases := []struct {
		name      string
		text1     string
		diffs     []dmp.Diff
		wantDelta string
	}{
		{
			name:      "Normal",
			text1:     "jumps over the lazy",
			diffs:     makeDiffs(dmp.Equal, "jump", dmp.Delete, "s", dmp.Insert, "ed", dmp.Equal, " over ", dmp.Delete, "the", dmp.Insert, "a", dmp.Equal, " lazy", dmp.Insert, "old dog"),
			wantDelta: "=4\t-1\t+ed\t=6\t-3\t+a\t=5\t+old dog",
		},
		{
			name:      "Unicode",
			text1:     "ڀ \x00 \t %ځ \x01 \n ^",
			diffs:     makeDiffs(dmp.Equal, "ڀ \x00 \t %", dmp.Delete, "ځ \x01 \n ^", dmp.Insert, "ڂ \x02 \\ |"),
			wantDelta: "=7\t-7\t+%DA%82 %02 %5C %7C",
		},
		{
			name:      "Unchanged characters",
			text1:     "",
			diffs:     makeDiffs(dmp.Insert, "A-Z a-z 0-9 - _ . ! ~ * ' ( ) ; / ? : @ & = + $ , # "),
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
		diffs := makeDiffs(dmp.Insert, a.String())
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
		if _, err := FromDelta("", "=-1"); err == nil {
			t.Error("Negative count: expected error")
		}
	})

	// A zero-length Equal/Delete diff produces a bare "=0"/"-0" token. Other
	// diff-match-patch ports (and this package's own ToDelta) accept these on
	// round-trip; FromDelta must not reject them as invalid.
	t.Run("zero-length tokens", func(t *testing.T) {
		text1 := "hello"
		diffs := []dmp.Diff{
			{Type: dmp.Equal, Text: []rune("hello")},
			{Type: dmp.Equal, Text: []rune{}},
		}
		delta := ToDelta(diffs)
		if delta != "=5\t=0" {
			t.Errorf("ToDelta: want %q, got %q", "=5\t=0", delta)
		}
		got, err := FromDelta(text1, delta)
		if err != nil {
			t.Fatalf("FromDelta: unexpected error on self-produced zero-length token: %v", err)
		}
		if !diffsEqual(got, diffs) {
			t.Errorf("FromDelta: want %v, got %v", diffs, got)
		}

		// A hand-built "-0" token, as another port might emit, must also parse.
		got, err = FromDelta(text1, "=5\t-0")
		if err != nil {
			t.Fatalf("FromDelta: unexpected error on foreign -0 token: %v", err)
		}
		want := []dmp.Diff{
			{Type: dmp.Equal, Text: []rune("hello")},
			{Type: dmp.Delete, Text: []rune{}},
		}
		if !diffsEqual(got, want) {
			t.Errorf("FromDelta: want %v, got %v", want, got)
		}
	})
}

// — Patch text tests ——————————————————————————————————————————————————————————

func TestPatchString(t *testing.T) {
	p := dmp.Patch{
		Diffs:   makeDiffs(dmp.Equal, "jump", dmp.Delete, "s", dmp.Insert, "ed", dmp.Equal, " over ", dmp.Delete, "the", dmp.Insert, "a", dmp.Equal, "\nlaz"),
		Start1:  20,
		Start2:  21,
		Length1: 18,
		Length2: 17,
	}
	want := "@@ -21,18 +22,17 @@\n jump\n-s\n+ed\n  over \n-the\n+a\n %0Alaz\n"
	if got := PatchString(p); got != want {
		t.Errorf("want: %q\ngot:  %q", want, got)
	}
}

func TestPatchFromText(t *testing.T) {
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
			patches, err := PatchFromText(c.input)
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
			if got := PatchString(patches[0]); got != c.wantPatch {
				t.Errorf("want: %q\ngot:  %q", c.wantPatch, got)
			}
		})
	}
}

func TestPatchToText(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"Single", "@@ -21,18 +22,17 @@\n jump\n-s\n+ed\n  over \n-the\n+a\n  laz\n"},
		{"Dual", "@@ -1,9 +1,9 @@\n-f\n+F\n oo+fooba\n@@ -7,9 +7,9 @@\n obar\n-,\n+.\n  tes\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			patches, err := PatchFromText(c.input)
			if err != nil {
				t.Fatalf("PatchFromText error: %v", err)
			}
			if got := PatchToText(patches); got != c.input {
				t.Errorf("want: %q\ngot:  %q", c.input, got)
			}
		})
	}
}

func TestCharacterEncoding(t *testing.T) {
	t.Run("encoding", func(t *testing.T) {
		p := dmp.Patch{
			Diffs:   makeDiffs(dmp.Delete, "`1234567890-=[]\\;',./", dmp.Insert, "~!@#$%^&*()_+{}|:\"<>?"),
			Start1:  0,
			Start2:  0,
			Length1: 21,
			Length2: 21,
		}
		want := "@@ -1,21 +1,21 @@\n-%601234567890-=%5B%5D%5C;',./\n+~!@#$%25%5E&*()_+%7B%7D%7C:%22%3C%3E?\n"
		if got := PatchString(p); got != want {
			t.Errorf("want: %q\ngot:  %q", want, got)
		}
	})

	t.Run("decoding", func(t *testing.T) {
		patches, err := PatchFromText("@@ -1,21 +1,21 @@\n-%601234567890-=%5B%5D%5C;',./\n+~!@#$%25%5E&*()_+%7B%7D%7C:%22%3C%3E?\n")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := makeDiffs(dmp.Delete, "`1234567890-=[]\\;',./", dmp.Insert, "~!@#$%^&*()_+{}|:\"<>?")
		if !diffsEqual(patches[0].Diffs, want) {
			t.Errorf("want %v, got %v", want, patches[0].Diffs)
		}
	})
}
