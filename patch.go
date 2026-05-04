package diffmatchpatch

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// String serializes the patch in GNU unified diff format, with percent-encoded
// non-ASCII characters. Suitable for storage and transmission; parse with PatchFromText.
func (p Patch) String() string {
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
		case Insert:
			buf.WriteByte('+')
		case Delete:
			buf.WriteByte('-')
		case Equal:
			buf.WriteByte(' ')
		}
		buf.WriteString(encodeURI(string(d.Text)))
		buf.WriteByte('\n')
	}
	return buf.String()
}

// PatchToText serializes a list of patches to a string.
func PatchToText(patches []Patch) string {
	var buf strings.Builder
	for _, p := range patches {
		buf.WriteString(p.String())
	}
	return buf.String()
}

var patchHeaderRe = regexp.MustCompile(`^@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@$`)

// PatchFromText parses a textual representation of patches.
func PatchFromText(textline string) ([]Patch, error) {
	var patches []Patch
	if textline == "" {
		return patches, nil
	}
	lines := strings.Split(textline, "\n")
	textPointer := 0
	for textPointer < len(lines) {
		m := patchHeaderRe.FindStringSubmatch(lines[textPointer])
		if m == nil {
			return nil, fmt.Errorf("invalid patch string: %q", lines[textPointer])
		}
		patch := Patch{}

		patch.Start1, _ = strconv.Atoi(m[1])
		if m[2] == "" {
			patch.Start1--
			patch.Length1 = 1
		} else if m[2] == "0" {
			patch.Length1 = 0
		} else {
			patch.Start1--
			patch.Length1, _ = strconv.Atoi(m[2])
		}

		patch.Start2, _ = strconv.Atoi(m[3])
		if m[4] == "" {
			patch.Start2--
			patch.Length2 = 1
		} else if m[4] == "0" {
			patch.Length2 = 0
		} else {
			patch.Start2--
			patch.Length2, _ = strconv.Atoi(m[4])
		}
		textPointer++

		for textPointer < len(lines) {
			if len(lines[textPointer]) == 0 {
				textPointer++
				continue
			}
			sign := lines[textPointer][0]
			line := lines[textPointer][1:]
			decoded, err := decodeURI(line)
			if err != nil {
				return nil, fmt.Errorf("invalid encoding in patch: %w", err)
			}
			switch sign {
			case '-':
				patch.Diffs = append(patch.Diffs, Diff{Delete, []rune(decoded)})
			case '+':
				patch.Diffs = append(patch.Diffs, Diff{Insert, []rune(decoded)})
			case ' ':
				patch.Diffs = append(patch.Diffs, Diff{Equal, []rune(decoded)})
			case '@':
				goto nextPatch
			default:
				return nil, fmt.Errorf("invalid patch mode %q in: %s", sign, line)
			}
			textPointer++
		}
	nextPatch:
		patches = append(patches, patch)
	}
	return patches, nil
}
