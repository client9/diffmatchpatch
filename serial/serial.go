// Package serial provides text serialization formats for diffmatchpatch types.
//
// Two formats are available:
//
//   - Delta: a compact tab-delimited encoding of a []Diff relative to text1,
//     compatible with other diff-match-patch implementations (JavaScript, Python, etc.).
//
//   - Patch text: a unified-diff-like format for []Patch. Note that patches in
//     this library operate at character granularity (not line granularity like GNU
//     patch), so the format is not directly compatible with the GNU patch tool.
//     Non-ASCII and special characters are percent-encoded for safe transmission.
package serial

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	dmp "github.com/client9/diffmatchpatch"
)

// — URI encoding helpers (used by both delta and patch formats) —————————————

// encodeURISafe marks byte values that do NOT need % encoding.
// Matches JavaScript's encodeURI safe set.
var encodeURISafe = func() [256]bool {
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

func encodeURI(s string) string {
	var buf strings.Builder
	for i := 0; i < len(s); {
		b := s[i]
		if b < 128 && encodeURISafe[b] {
			buf.WriteByte(b)
			i++
		} else {
			_, size := utf8.DecodeRuneInString(s[i:])
			for j := range size {
				fmt.Fprintf(&buf, "%%%02X", s[i+j])
			}
			i += size
		}
	}
	return buf.String()
}

func decodeURI(s string) (string, error) {
	return url.PathUnescape(s)
}

// — Delta format ——————————————————————————————————————————————————————————————

// ToDelta encodes a diff as a compact delta string relative to text1.
// The format uses tab-separated tokens: "+text" for inserts (URI-encoded),
// "-N" for deletes, and "=N" for equalities, where N is a rune count.
// Compatible with other diff-match-patch implementations.
func ToDelta(diffs []dmp.Diff) string {
	var buf strings.Builder
	for i, d := range diffs {
		switch d.Type {
		case dmp.Insert:
			buf.WriteByte('+')
			buf.WriteString(encodeURI(string(d.Text)))
		case dmp.Delete:
			buf.WriteByte('-')
			fmt.Fprintf(&buf, "%d", len(d.Text))
		case dmp.Equal:
			buf.WriteByte('=')
			fmt.Fprintf(&buf, "%d", len(d.Text))
		}
		if i < len(diffs)-1 {
			buf.WriteByte('\t')
		}
	}
	return buf.String()
}

// FromDelta reconstructs a diff from text1 and a delta string produced by ToDelta.
// Returns an error if the delta is malformed or inconsistent with the length of text1.
func FromDelta(text1, delta string) ([]dmp.Diff, error) {
	var diffs []dmp.Diff
	r1 := []rune(text1)
	pointer := 0
	for token := range strings.SplitSeq(delta, "\t") {
		if token == "" {
			continue
		}
		param := token[1:]
		switch token[0] {
		case '+':
			decoded, err := url.PathUnescape(param)
			if err != nil {
				return nil, fmt.Errorf("illegal escape in FromDelta: %w", err)
			}
			diffs = append(diffs, dmp.Diff{Type: dmp.Insert, Text: []rune(decoded)})
		case '-', '=':
			n, err := strconv.Atoi(param)
			if err != nil || n <= 0 {
				return nil, fmt.Errorf("invalid number in FromDelta: %s", param)
			}
			if pointer+n > len(r1) {
				return nil, fmt.Errorf("delta length (%d) larger than source text length (%d)", pointer+n, len(r1))
			}
			chunk := append([]rune(nil), r1[pointer:pointer+n]...)
			pointer += n
			if token[0] == '=' {
				diffs = append(diffs, dmp.Diff{Type: dmp.Equal, Text: chunk})
			} else {
				diffs = append(diffs, dmp.Diff{Type: dmp.Delete, Text: chunk})
			}
		default:
			return nil, fmt.Errorf("invalid diff operation in FromDelta: %c", token[0])
		}
	}
	if pointer != len(r1) {
		return nil, fmt.Errorf("delta length (%d) smaller than source text length (%d)", pointer, len(r1))
	}
	return diffs, nil
}

// — Patch text format —————————————————————————————————————————————————————————

var patchHeaderRe = regexp.MustCompile(`^@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@$`)

// PatchString serializes a single patch to text. Non-ASCII and special
// characters in diff content are percent-encoded. Parse with PatchFromText.
func PatchString(p dmp.Patch) string {
	var coords1, coords2 string
	if p.Length1 == 0 {
		coords1 = fmt.Sprintf("%d,0", p.Start1)
	} else if p.Length1 == 1 {
		coords1 = strconv.Itoa(p.Start1 + 1)
	} else {
		coords1 = fmt.Sprintf("%d,%d", p.Start1+1, p.Length1)
	}
	if p.Length2 == 0 {
		coords2 = fmt.Sprintf("%d,0", p.Start2)
	} else if p.Length2 == 1 {
		coords2 = strconv.Itoa(p.Start2 + 1)
	} else {
		coords2 = fmt.Sprintf("%d,%d", p.Start2+1, p.Length2)
	}
	var buf strings.Builder
	fmt.Fprintf(&buf, "@@ -%s +%s @@\n", coords1, coords2)
	for _, d := range p.Diffs {
		switch d.Type {
		case dmp.Insert:
			buf.WriteByte('+')
		case dmp.Delete:
			buf.WriteByte('-')
		case dmp.Equal:
			buf.WriteByte(' ')
		}
		buf.WriteString(encodeURI(string(d.Text)))
		buf.WriteByte('\n')
	}
	return buf.String()
}

// PatchToText serializes a slice of patches to text. Parse with PatchFromText.
func PatchToText(patches []dmp.Patch) string {
	var buf strings.Builder
	for _, p := range patches {
		buf.WriteString(PatchString(p))
	}
	return buf.String()
}

// PatchFromText parses a patch text produced by PatchToText.
func PatchFromText(text string) ([]dmp.Patch, error) {
	var patches []dmp.Patch
	if text == "" {
		return patches, nil
	}
	lines := strings.Split(text, "\n")
	i := 0
	for i < len(lines) {
		m := patchHeaderRe.FindStringSubmatch(lines[i])
		if m == nil {
			return nil, fmt.Errorf("invalid patch string: %q", lines[i])
		}
		var p dmp.Patch
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
		i++
		for i < len(lines) {
			if len(lines[i]) == 0 {
				i++
				continue
			}
			sign := lines[i][0]
			decoded, err := decodeURI(lines[i][1:])
			if err != nil {
				return nil, fmt.Errorf("invalid encoding in patch: %w", err)
			}
			switch sign {
			case '-':
				p.Diffs = append(p.Diffs, dmp.Diff{Type: dmp.Delete, Text: []rune(decoded)})
			case '+':
				p.Diffs = append(p.Diffs, dmp.Diff{Type: dmp.Insert, Text: []rune(decoded)})
			case ' ':
				p.Diffs = append(p.Diffs, dmp.Diff{Type: dmp.Equal, Text: []rune(decoded)})
			case '@':
				goto nextPatch
			default:
				return nil, fmt.Errorf("invalid patch mode %q in: %s", sign, lines[i][1:])
			}
			i++
		}
	nextPatch:
		patches = append(patches, p)
	}
	return patches, nil
}
