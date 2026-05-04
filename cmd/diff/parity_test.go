package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestUnifiedMatchesDiff(t *testing.T) {
	cases := []struct {
		name string
		old  string
		new  string
		ctx  int
		want string
	}{
		{
			name: "single line",
			old:  "a\n",
			new:  "b\n",
			ctx:  3,
			want: "--- old\n+++ new\n@@ -1 +1 @@\n-a\n+b\n",
		},
		{
			name: "insertion with context",
			old:  "hello\nworld\nfoo\n",
			new:  "hello\nearth\nfoo\nbar\n",
			ctx:  3,
			want: "--- old\n+++ new\n@@ -1,3 +1,4 @@\n hello\n-world\n+earth\n foo\n+bar\n",
		},
		{
			name: "no newline at end",
			old:  "hello",
			new:  "world",
			ctx:  3,
			want: "--- old\n+++ new\n@@ -1 +1 @@\n-hello\n\\ No newline at end of file\n+world\n\\ No newline at end of file\n",
		},
		{
			name: "identical files",
			old:  "hello\nworld\n",
			new:  "hello\nworld\n",
			ctx:  3,
			want: "",
		},
		{
			name: "insert at start",
			old:  "line1\nline2\n",
			new:  "line0\nline1\nline2\n",
			ctx:  3,
			want: "--- old\n+++ new\n@@ -1,2 +1,3 @@\n+line0\n line1\n line2\n",
		},
		{
			name: "zero context",
			old:  "hello\nworld\nfoo\n",
			new:  "hello\nearth\nfoo\nbar\n",
			ctx:  0,
			want: "--- old\n+++ new\n@@ -2 +2 @@\n-world\n+earth\n@@ -3,0 +4 @@\n+bar\n",
		},
		{
			name: "two hunks",
			old:  "a\nb\nc\nd\ne\nf\ng\nh\n",
			new:  "A\nb\nc\nd\ne\nf\ng\nH\n",
			ctx:  1,
			want: "--- old\n+++ new\n@@ -1,2 +1,2 @@\n-a\n+A\n b\n@@ -7,2 +7,2 @@\n g\n-h\n+H\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := unified(tc.old, tc.new, "old", "new", tc.ctx)
			if got != tc.want {
				t.Fatalf("got:\n%s\nwant:\n%s", got, tc.want)
			}

			// Secondary cross-check: verify our output matches the system diff binary.
			if _, err := exec.LookPath("diff"); err != nil {
				t.Skip("diff not in PATH, skipping cross-check")
			}
			dir := t.TempDir()
			oldPath := filepath.Join(dir, "old")
			newPath := filepath.Join(dir, "new")
			if err := os.WriteFile(oldPath, []byte(tc.old), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(newPath, []byte(tc.new), 0o600); err != nil {
				t.Fatal(err)
			}
			if system := runDiff(t, tc.ctx, oldPath, newPath); got != system {
				t.Fatalf("output differs from system diff:\ngot:\n%s\nsystem:\n%s", got, system)
			}
		})
	}
}

func runDiff(t *testing.T, ctx int, oldPath, newPath string) string {
	t.Helper()
	args := []string{fmt.Sprintf("-U%d", ctx), "-L", "old", "-L", "new", oldPath, newPath}
	out, err := exec.Command("diff", args...).CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() > 1 {
			t.Fatalf("diff failed: %v\n%s", err, string(out))
		}
	}
	return string(out)
}
