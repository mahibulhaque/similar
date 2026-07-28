# similar

[![Go Reference](https://pkg.go.dev/badge/github.com/mahibulhaque/similar.svg)](https://pkg.go.dev/github.com/mahibulhaque/similar)

A faithful Go port of the [Rust `similar` crate](https://github.com/mitsuhiko/similar)'s
Myers' diff algorithm: a minimal edit script between two slices of any
`comparable` type, with the crate's heuristics and an optional deadline.

- **Minimal edit scripts.** Classic Myers divide-and-conquer, cost `N + M − 2·LCS`.
- **Any comparable element.** `[]string`, `[]rune`, `[]byte`, `[]int`, or your
  own comparable structs — no adapters.
- **Bounded worst case.** Pass a `context.Context` with a deadline; on a hit the
  diff bails to a valid approximate script rather than running unbounded.
- **Streaming or slice.** Collect a `[]DiffOp`, or implement a `DiffHook` and
  receive callbacks as the diff is produced.
- **Text diffing.** `TextDiff` splits strings by line, word, or character and
  yields tagged `Change`s, a similarity ratio, grouped ops, remapping onto
  connected runs of the original strings, and difflib-style `GetCloseMatches`.
- **Standard library only.** A clean leaf dependency.

## Install

```sh
go get github.com/mahibulhaque/similar
```

Requires Go 1.26+.

## Usage

```go
ops := similar.Diff(
    []string{"the", "quick", "brown", "fox"},
    []string{"the", "slow", "brown", "cat"},
)
for _, op := range ops {
    os, oe := op.OldRange()
    ns, ne := op.NewRange()
    fmt.Printf("%s old[%d:%d] new[%d:%d]\n", op.Tag, os, oe, ns, ne)
}
```

Each `DiffOp` is tagged `Equal`, `Delete`, `Insert`, or `Replace` and is
JSON-serializable with stable field names. Applying the operations in order
reconstructs the new sequence exactly.

### Deadlines

```go
ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
defer cancel()
ops, err := similar.DiffDeadline(ctx, similar.Myers, old, new)
```

### Streaming with a hook

Embed `NoopHook` and override only the callbacks you need:

```go
type printer struct{ similar.NoopHook }

func (printer) Delete(oldIndex, oldLen, newIndex int) error {
    fmt.Printf("delete old[%d:%d]\n", oldIndex, oldIndex+oldLen)
    return nil
}

similar.DiffHookDeadline(context.Background(), similar.Myers, printer{}, old, new)
```

Wrap a hook in `NewReplaceHook` to coalesce adjacent delete+insert into
`Replace` operations.

### Text diffing

Diff two strings by line, word, or character and walk the tagged changes:

```go
diff := similar.DiffLines("a\nb\nc", "a\nb\nC")
for c := range diff.AllChanges() {
    fmt.Printf("%s%s", c.Tag(), c) // " a\n", " b\n", "-c\n", "+C\n"
}
fmt.Println(diff.Ratio()) // similarity in [0,1]
```

`AllChanges` and `Changes` return an `iter.Seq[Change]`, so changes stream
lazily and `break` early on large inputs; use `slices.Collect` for a slice.

Map a word or character diff back onto connected runs of the original strings.
A `TextDiff` knows its own source text, so you don't pass the strings again:

```go
diff := similar.DiffWords("foo bar baz", "foo bor baz")
for s := range diff.AllRemappedChanges() {
    fmt.Printf("%s%q\n", s.Tag, s.Value) // " \"foo \"", "-\"bar\"", "+\"bor\"", " \" baz\""
}
```

`RemappedChanges(op)` does one op at a time, and `SliceOld`/`SliceNew` extract an
arbitrary token range as a substring.

Find the closest matches to a word (difflib-style):

```go
similar.GetCloseMatches("appel", []string{"ape", "apple", "peach", "puppy"}, 3, 0.6)
// [apple ape]
```

## Development

```sh
make test   # go test ./... -race -cover
make lint   # golangci-lint
make fuzz   # native fuzzing against the invariants
make ci     # test + lint + golden check
```

## License

Behavior ported from the `similar` crate (Apache-2.0).
