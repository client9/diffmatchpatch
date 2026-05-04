package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzUnifiedParity tests that our unified diff output is semantically correct
// by applying it with patch(1) and verifying the result equals the mutated document.
//
// # How it works
//
// Rather than diffing two random strings (which rarely resemble real edits), the
// fuzzer is given a base document and a mutation program encoded as a []byte. The
// program is executed against the base to produce a mutated document that looks
// like a plausible human edit — block moves, insertions, deletions, line edits.
// unified(base, mutated) is then verified with patch(1) as the semantic oracle.
//
// # Mutation micro-program
//
// ops is a sequence of 4-byte instructions processed by applyMutations:
//
//	byte 0  opcode   picks the operation
//	byte 1  start    first line affected
//	byte 2  count    number of lines affected
//	byte 3  dest     destination line (MOVE only)
//
// Operations (opcode % 4):
//
//	0  MOVE    extract lines [start, start+count) and re-insert at dest
//	1  INSERT  insert count new lines ("LINE 1\n", "LINE 2\n", …) at start
//	2  DELETE  remove lines [start, start+count)
//	3  EDIT    prepend "EDIT N " to each line in [start, start+count)
//
// # The mod trick
//
// Every field is reduced modulo the valid range before use, so every byte
// sequence the fuzzer generates is a valid, fully-executable program — no
// instruction is ever skipped due to an out-of-range value. This means the
// fuzzer's byte-level mutations map directly to meaningful document edits
// without wasted coverage budget on rejected inputs.
//
//	opcode → opcode % numOps          (always a known operation)
//	start  → start  % len(lines)      (always an existing line)
//	count  → count  % available + 1   (at least 1, never past end)
//	dest   → dest   % (len(rest)+1)   (anywhere in the remaining lines)
const (
	opMove   = 0
	opInsert = 1
	opDelete = 2
	opEdit   = 3
	numOps   = 4
	maxInstr = 16 // cap instructions per fuzz input to bound runtime
	maxCount = 10 // cap lines per instruction to keep diffs readable
)

// applyMutations executes a mutation program against a slice of lines.
// See the FuzzUnifiedParity doc comment for the instruction encoding.
func applyMutations(lines []string, ops []byte) []string {
	limit := min(len(ops)/4, maxInstr)
	for i := range limit {
		if len(lines) == 0 {
			break
		}
		b := ops[i*4 : i*4+4]
		nL := len(lines)

		switch int(b[0]) % numOps {

		case opDelete:
			start := int(b[1]) % nL
			count := int(b[2])%min(nL-start, maxCount) + 1
			lines = append(lines[:start:start], lines[start+count:]...)

		case opInsert:
			// Allow inserting at the end (% nL+1).
			at    := int(b[1]) % (nL + 1)
			count := int(b[2])%maxCount + 1
			ins := make([]string, count)
			for j := range ins {
				ins[j] = fmt.Sprintf("LINE %d\n", j+1)
			}
			out := make([]string, 0, nL+count)
			out = append(out, lines[:at]...)
			out = append(out, ins...)
			out = append(out, lines[at:]...)
			lines = out

		case opEdit:
			start := int(b[1]) % nL
			count := int(b[2])%min(nL-start, maxCount) + 1
			for j := range count {
				lines[start+j] = fmt.Sprintf("EDIT %d ", j+1) + lines[start+j]
			}

		case opMove:
			start := int(b[1]) % nL
			count := int(b[2])%min(nL-start, maxCount) + 1
			moved := append([]string(nil), lines[start:start+count]...)
			rest  := append(lines[:start:start], lines[start+count:]...)
			dest  := int(b[3]) % (len(rest) + 1)
			out   := make([]string, 0, len(lines))
			out    = append(out, rest[:dest]...)
			out    = append(out, moved...)
			out    = append(out, rest[dest:]...)
			lines  = out
		}
	}
	return lines
}

func FuzzUnifiedParity(f *testing.F) {
	type seed struct {
		base string
		ops  []byte
	}
	seeds := []seed{
		{
			"one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n",
			[]byte{
				opEdit, 2, 1, 0, // edit line 2
				opInsert, 5, 2, 0, // insert 2 lines at position 5
				opDelete, 1, 1, 0, // delete line 1
				opMove, 3, 2, 0, // move 2 lines from position 3 to start
			},
		},
		{
			"package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n\tfmt.Println(\"world\")\n}\n",
			[]byte{
				opEdit, 5, 1, 0, // edit the first Println
				opInsert, 6, 1, 0, // insert before second Println
				opMove, 1, 1, 3, // move import block down
			},
		},
		{
			"[server]\nhost = localhost\nport = 8080\n\n[database]\nhost = db\nport = 5432\nname = mydb\n",
			[]byte{
				opDelete, 3, 1, 0, // delete blank line
				opEdit, 1, 2, 0, // edit host + port
				opInsert, 4, 1, 0, // insert new config key
			},
		},
		{
			"The quick brown fox jumps over the lazy dog.\nPack my box with five dozen liquor jugs.\nHow vexingly quick daft zebras jump.\nThe five boxing wizards jump quickly.\n",
			[]byte{
				opMove, 0, 1, 3, // move first line to end
				opEdit, 1, 2, 0, // edit two middle lines
			},
		},
	}

	for _, s := range seeds {
		f.Add(s.base, s.ops)
	}

	f.Fuzz(func(t *testing.T, base string, ops []byte) {
		// Skip non-text base documents: null bytes or invalid UTF-8.
		// diff/patch are text tools; invalid byte sequences are treated as binary.
		if strings.ContainsRune(base, 0) || !utf8.ValidString(base) {
			t.Skip()
		}
		lines := splitLines(base)
		if len(lines) == 0 {
			t.Skip()
		}

		mutated := strings.Join(applyMutations(lines, ops), "")
		got := unified(base, mutated, "old", "new", 3)

		// Semantic oracle: when files are identical the diff must be empty.
		if base == mutated {
			if got != "" {
				t.Fatalf("expected empty diff for identical files, got:\n%s", got)
			}
			return
		}
		if got == "" {
			t.Fatal("got empty diff but base != mutated")
		}

		// Apply our diff with patch(1) and verify the result equals mutated.
		// This catches wrong output (bad ranges, missing hunks, off-by-one) without
		// failing on Myers tie-breaking, where multiple valid minimal diffs exist.
		if _, err := exec.LookPath("patch"); err != nil {
			t.Skip("patch not in PATH")
		}
		dir := t.TempDir()
		oldPath := filepath.Join(dir, "old")
		patchPath := filepath.Join(dir, "patch")
		resultPath := filepath.Join(dir, "result")
		if err := os.WriteFile(oldPath, []byte(base), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(patchPath, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("patch", "--quiet", "-o", resultPath, oldPath, patchPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("patch rejected our diff: %v\n%s\ndiff:\n%s", err, out, got)
		}
		result, err := os.ReadFile(resultPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(result) != mutated {
			t.Fatalf("patch produced wrong result:\nbase=%q\nops=%v\nmutated=%q\ndiff:\n%sgot after patch:\n%q",
				base, ops, mutated, got, string(result))
		}
	})
}
