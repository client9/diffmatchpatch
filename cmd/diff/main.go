package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	dmp "github.com/client9/diffmatchpatch"
)

func main() {
	var (
		unified  = flag.Int("u", 3, "output N lines of unified context")
		ignCase  = flag.Bool("i", false, "ignore case differences")
		brief    = flag.Bool("q", false, "report only whether files differ")
		showSame = flag.Bool("s", false, "report when two files are the same")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: diff [options] file1 file2\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}

	file1, file2 := flag.Arg(0), flag.Arg(1)

	text1, err := os.ReadFile(file1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "diff: %v\n", err)
		os.Exit(2)
	}
	text2, err := os.ReadFile(file2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "diff: %v\n", err)
		os.Exit(2)
	}

	s1, s2 := string(text1), string(text2)
	if *ignCase {
		s1, s2 = strings.ToLower(s1), strings.ToLower(s2)
	}

	diffs := diffByLine(context.Background(), s1, s2)

	allEqual := true
	for _, d := range diffs {
		if d.Type != dmp.Equal {
			allEqual = false
			break
		}
	}

	if allEqual {
		if *showSame {
			fmt.Printf("Files %s and %s are identical\n", file1, file2)
		}
		os.Exit(0)
	}

	if *brief {
		fmt.Printf("Files %s and %s differ\n", file1, file2)
		os.Exit(1)
	}

	printUnified(os.Stdout, diffs, file1, file2, *unified)
	os.Exit(1)
}

// diffByLine performs a line-level diff, returning one Diff per line.
// Each Diff.Text holds a single line (including its trailing newline if any).
func diffByLine(ctx context.Context, text1, text2 string) []dmp.Diff {
	lines1 := splitLines(text1)
	lines2 := splitLines(text2)

	// Encode each unique line as a rune for efficient diffing.
	lineToRune := make(map[string]rune)
	var runeToLine []string
	next := rune(1)

	encode := func(lines []string) []rune {
		out := make([]rune, len(lines))
		for i, line := range lines {
			r, ok := lineToRune[line]
			if !ok {
				r = next
				next++
				lineToRune[line] = r
				runeToLine = append(runeToLine, line)
			}
			out[i] = r
		}
		return out
	}

	r1 := encode(lines1)
	r2 := encode(lines2)

	rawDiffs := dmp.DiffRunes(ctx, r1, r2)

	// Expand each multi-line diff into individual per-line diffs.
	var result []dmp.Diff
	for _, d := range rawDiffs {
		for _, r := range d.Text {
			result = append(result, dmp.Diff{
				Type: d.Type,
				Text: []rune(runeToLine[r-1]),
			})
		}
	}
	return result
}

// printUnified renders diffs in unified diff format.
func printUnified(w io.Writer, diffs []dmp.Diff, file1, file2 string, ctx int) {
	n := len(diffs)

	// Assign 1-based line numbers for each file.
	lnum1 := make([]int, n)
	lnum2 := make([]int, n)
	cur1, cur2 := 1, 1
	for i, d := range diffs {
		switch d.Type {
		case dmp.Equal:
			lnum1[i] = cur1
			lnum2[i] = cur2
			cur1++
			cur2++
		case dmp.Delete:
			lnum1[i] = cur1
			cur1++
		case dmp.Insert:
			lnum2[i] = cur2
			cur2++
		}
	}

	// Collect indices of changed lines and group into hunks.
	var changed []int
	for i, d := range diffs {
		if d.Type != dmp.Equal {
			changed = append(changed, i)
		}
	}
	if len(changed) == 0 {
		return
	}

	type hunk struct{ start, end int }
	var hunks []hunk
	hs := max(0, changed[0]-ctx)
	he := min(n-1, changed[0]+ctx)
	for _, ci := range changed[1:] {
		if ci-ctx <= he+1 {
			he = min(n-1, ci+ctx)
		} else {
			hunks = append(hunks, hunk{hs, he})
			hs = max(0, ci-ctx)
			he = min(n-1, ci+ctx)
		}
	}
	hunks = append(hunks, hunk{hs, he})

	fmt.Fprintf(w, "--- %s\n", file1)
	fmt.Fprintf(w, "+++ %s\n", file2)

	for _, h := range hunks {
		countOld, countNew := 0, 0
		for i := h.start; i <= h.end; i++ {
			switch diffs[i].Type {
			case dmp.Equal:
				countOld++
				countNew++
			case dmp.Delete:
				countOld++
			case dmp.Insert:
				countNew++
			}
		}

		// startOld: first file1 line in hunk; if countOld==0, last file1 line before hunk.
		startOld := 0
		for i := h.start; i <= h.end; i++ {
			if diffs[i].Type == dmp.Equal || diffs[i].Type == dmp.Delete {
				startOld = lnum1[i]
				break
			}
		}
		if startOld == 0 {
			for i := h.start - 1; i >= 0; i-- {
				if lnum1[i] > 0 {
					startOld = lnum1[i]
					break
				}
			}
		}

		// startNew: first file2 line in hunk; if countNew==0, last file2 line before hunk.
		startNew := 0
		for i := h.start; i <= h.end; i++ {
			if diffs[i].Type == dmp.Equal || diffs[i].Type == dmp.Insert {
				startNew = lnum2[i]
				break
			}
		}
		if startNew == 0 {
			for i := h.start - 1; i >= 0; i-- {
				if lnum2[i] > 0 {
					startNew = lnum2[i]
					break
				}
			}
		}

		fmt.Fprintf(w, "@@ -%d,%d +%d,%d @@\n", startOld, countOld, startNew, countNew)

		for i := h.start; i <= h.end; i++ {
			line := string(diffs[i].Text)
			switch diffs[i].Type {
			case dmp.Equal:
				fmt.Fprintf(w, " %s", line)
			case dmp.Delete:
				fmt.Fprintf(w, "-%s", line)
			case dmp.Insert:
				fmt.Fprintf(w, "+%s", line)
			}
			if len(line) > 0 && line[len(line)-1] != '\n' {
				fmt.Fprintln(w)
				fmt.Fprintln(w, `\ No newline at end of file`)
			}
		}
	}
}

// splitLines splits text into lines, each retaining its trailing newline.
func splitLines(s string) []string {
	var result []string
	for len(s) > 0 {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			result = append(result, s)
			break
		}
		result = append(result, s[:i+1])
		s = s[i+1:]
	}
	return result
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
