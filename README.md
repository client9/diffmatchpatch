# diffmatchpatch

A Go implementation of the [Diff Match Patch](https://github.com/google/diff-match-patch) algorithms by Neil Fraser — computing differences between texts, fuzzy matching, and applying patches.

[![Go Reference](https://pkg.go.dev/badge/github.com/client9/diffmatchpatch.svg)](https://pkg.go.dev/github.com/client9/diffmatchpatch)
[![Build Status](https://github.com/client9/diffmatchpatch/actions/workflows/go.yml/badge.svg)](https://github.com/client9/diffmatchpatch/actions)

## Install

```
go get github.com/client9/diffmatchpatch
```

## Usage

### Diff

```go
import (
    "context"
    "github.com/client9/diffmatchpatch"
)

diffs := diffmatchpatch.DiffStrings(context.Background(), "Hello, world!", "Goodbye, world!")

for _, d := range diffs {
    switch d.Type {
    case diffmatchpatch.Insert:
        fmt.Printf("+%s", d)
    case diffmatchpatch.Delete:
        fmt.Printf("-%s", d)
    case diffmatchpatch.Equal:
        fmt.Printf(" %s", d)
    }
}
```

Use `context.WithTimeout` to limit how long the diff computation runs:

```go
ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
defer cancel()
diffs := diffmatchpatch.DiffStrings(ctx, text1, text2)
```

Three diff entry points are available:

| Function | Granularity |
|----------|-------------|
| `DiffStrings(ctx, s1, s2)` | character |
| `DiffRunes(ctx, r1, r2)` | rune slice |
| `DiffLines(ctx, s1, s2)` | line, then character within changed blocks |

### Cleanup

Raw diffs can be semantically or operationally refined:

```go
diffs = diffmatchpatch.CleanupSemantic(diffs)       // align edits to word/line boundaries
diffs = diffmatchpatch.CleanupEfficiency(diffs, 4)  // eliminate cheap equalities
```

### Match

Locate the best approximate match for a pattern within a text:

```go
m := diffmatchpatch.Matcher{
    Threshold: 0.5,  // 0 = exact only, 1 = match anything
    Distance:  1000, // how far from loc to search
}
loc := m.Match(text, pattern, expectedLoc)
// returns -1 if no match found within threshold
```

### Patch

```go
p := diffmatchpatch.Patcher{
    DeleteThreshold: 0.5,
    Margin:          4,
    EditCost:        4,
    Matcher: diffmatchpatch.Matcher{
        Threshold: 0.5,
        Distance:  1000,
    },
}

// Create patches
patches := p.Make(context.Background(), original, revised)

// Serialize / deserialize (serial subpackage — see Serialization section)
text := serial.PatchToText(patches)
patches, err := serial.PatchFromText(text)

// Apply
result, applied := p.Apply(context.Background(), patches, target)
```

`applied` is a `[]bool` with one entry per patch (large patches are split to fit within the platform's Bitap word size, so `len(applied)` may exceed `len(patches)`).

## Utilities

```go
diffmatchpatch.TranslateIndex(diffs, i)  // map index from text1 to text2
```

## Serialization

The `serial` subpackage provides text serialization for both diffs and patches.
These formats use URI encoding for compatibility with the original JavaScript
implementation and other diff-match-patch ports.

```go
import "github.com/client9/diffmatchpatch/serial"

// Compact cross-language diff encoding
encoded := serial.ToDelta(diffs)
diffs, err := serial.FromDelta(text1, encoded)

// Patch text format (unified-diff-like, but character-granularity)
text := serial.PatchToText(patches)
patches, err := serial.PatchFromText(text)
```

## License

This code is licensed under [MIT](LICENSE)

The [original code](https://github.com/google/diff-match-patch) is licensed under [Apache 2.0](APACHE-LICENSE-2.0).
