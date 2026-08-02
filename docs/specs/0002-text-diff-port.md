# Spec 0002 — Text Diff Port (similar-go v0.2.0)

Status: ready-for-agent
Module: `github.com/mahibulhaque/similar`
Source of truth: `C:\Source\similar/src/text/` (plus `src/common.rs`, `src/types.rs`,
`src/utils.rs` for the crate-level helpers the text layer depends on)

## Problem Statement

`similar-go` currently exposes only the raw Myers layer: a consumer hands in two
`[]T comparable` sequences and gets back low-level `DiffOp` op-codes (index/length
tuples). That is powerful but inconvenient for the most common real task — diffing
text. A user who wants to "show me what changed between these two documents" must
themselves split the text into lines/words/characters, feed the tokens to the
algorithm, interpret op-codes, and map the results back onto the original strings. None
of the ergonomic surface that makes the upstream Rust `similar` crate pleasant to use
(`TextDiff`, tagged `Change`s, similarity ratio, grouped ops, fuzzy close-matches)
exists yet in the Go port.

## Solution

Port the Rust crate's text-diffing convenience layer (`src/text/`) plus the crate-level
helpers it depends on, so Go consumers get the same high-level API:

- Construct a diff directly from two strings, tokenized by lines, words, or characters
  (or from pre-split token slices).
- Walk the result as a stream of tagged `Change` values (`Equal`/`Delete`/`Insert`,
  each carrying its value and old/new index).
- Ask a `TextDiff` for its similarity ratio, its raw ops, and grouped ops (change
  clusters with N lines of context).
- Remap token-level diffs back onto connected runs of the original strings.
- Find the closest matches to a word from a list of candidates (difflib-style).

The port is faithful in behavior, not syntax (the principle established by spec 0001).
Rust constructs with no Go analog — the `DiffableStr` trait, `Cow`, borrowed/owned
generics, `?Sized`, lazy iterators — are replaced by idiomatic Go that produces
identical observable results. The library remains standard-library-only, and the
existing flat facade (everything under package `similar`) is preserved.

## User Stories

1. As a Go developer, I want to diff two multi-line strings by line, so that I can show which lines were added, removed, or unchanged.
2. As a Go developer, I want to diff two strings by word, so that I can highlight word-level edits within a paragraph.
3. As a Go developer, I want to diff two strings by character, so that I can show fine-grained intra-word edits.
4. As a Go developer, I want to diff two already-tokenized `[]string` slices, so that I can control tokenization myself and still use the text result API.
5. As a Go developer, I want line diffs to preserve the newline characters attached to each line, so that reconstructed output is byte-faithful to the input.
6. As a Go developer, I want word tokenization to separate whitespace runs from non-whitespace runs, so that word boundaries match the upstream crate's behavior.
7. As a Go developer, I want character tokenization to split on rune boundaries, so that multi-byte characters are never split mid-rune.
8. As a Go developer, I want each diff result exposed as a stream of `Change` values, so that I can iterate changes without decoding raw op-codes myself.
9. As a Go developer, I want each `Change` to report its tag (`Equal`/`Delete`/`Insert`), so that I can style or filter changes by kind.
10. As a Go developer, I want each `Change` to report its old and new index when present, so that I can correlate changes with line/token positions.
11. As a Go developer, I want the "index absent" case (a delete has no new index; an insert has no old index) modeled explicitly, so that I never mistake a missing index for a real position.
12. As a Go developer, I want a `Replace` op expanded into deletes-then-inserts at the change level, so that `Change` only ever carries the three simple tags.
13. As a Go developer, I want to stream changes lazily and stop early, so that diffing a very large file does not force the whole change list into memory at once.
14. As a Go developer, I want to collect all changes into a slice when convenient, so that I can index, count, or re-iterate them.
15. As a Go developer, I want the changes for a single op, so that I can process one hunk at a time.
16. As a Go developer, I want a similarity ratio in `[0,1]` for a diff, so that I can measure how alike two texts are.
17. As a Go developer, I want the raw captured ops, so that I can build custom output formats on top of the text diff.
18. As a Go developer, I want grouped ops with N lines of surrounding context, so that I can render change clusters (the shape a unified diff would consume) without the unchanged bulk.
19. As a Go developer, I want a `Change` to tell me whether it is missing a trailing newline, so that I can correctly render line-based output.
20. As a Go developer, I want a `Change`'s string form to append a newline when one is missing, so that printing changes in a line diff yields well-formed lines.
21. As a Go developer, I want to configure the diff with a `context.Context`, so that I can bound runtime and cancel long diffs — consistent with the existing facade.
22. As a Go developer, I want to override the newline-terminated behavior explicitly, so that I can force or disable the newline handling independently of the diff kind.
23. As a Go developer, I want the algorithm selectable via an option, so that call sites stay stable when future algorithms ship (Myers is the only value now).
24. As a Go developer, I want default construction to require no options, so that the common case is a single clean call.
25. As a Go developer, I want to map a word/char diff back onto connected runs of the original strings, so that I can present readable substrings instead of scattered tokens.
26. As a Go developer, I want the remapper to yield each op as a tagged run against the original text, so that I can render "equal / deleted / inserted" spans directly.
27. As a Go developer, I want to slice into the original old/new string by token range via the remapper, so that I can extract arbitrary spans.
28. As a Go developer, I want to find the closest matches to a word from a candidate list, so that I can build spelling suggestions or fuzzy lookups.
29. As a Go developer, I want `GetCloseMatches` to accept a cutoff and a max count, so that I control result quality and quantity.
30. As a Go developer, I want close-match results ordered by similarity with a stable lexicographic tie-break, so that results are deterministic.
31. As a Go developer, I want all these types re-exported flat under package `similar`, so that I use one import path and one namespace.
32. As a library maintainer, I want the text implementation hidden in `internal/`, so that the public API stays small and I can refactor internals without breaking users. *(Superseded by [ADR 0001](../adr/0001-vocabulary-at-the-root.md) — hiding the implementation behind type aliases left all 54 exported methods undocumented. Unexported identifiers in `package similar` hide it just as well.)*
33. As a library maintainer, I want the diff vocabulary (`Change`/`ChangeTag`, ratio, grouping) to live with the existing diff types, so that the package layering stays coherent and acyclic. *(Superseded by [ADR 0001](../adr/0001-vocabulary-at-the-root.md) — it still lives with them, now at the root.)*
34. As a library maintainer, I want behavior pinned to the upstream crate's documented examples, so that the port is verifiably faithful.
35. As a library maintainer, I want the library to stay standard-library-only, so that adoption carries no dependency cost.
36. As a Go developer, I want runnable example tests, so that the docs stay correct and show real usage.

## Implementation Decisions

### Layering & modules

> **Superseded by [ADR 0001](../adr/0001-vocabulary-at-the-root.md).** The module
> boundaries below (`internal/text`, `internal/diff`, the `aliases.go` convention)
> describe the v0.2.0 layout. The vocabulary and the text layer are now declared in
> `package similar` directly; `internal/algorithms` and `internal/diffutil` remain. The
> string model and the rest of this section still hold.

- **String model:** the text layer operates on concrete Go `string`; tokens are
  `[]string`. The Rust `DiffableStr`/`DiffableStrRef`/`DiffInput`/`IntoDiffInput` trait
  machinery and `Cow`/borrowed-vs-owned generics are dropped — they exist in Rust only
  to abstract `str` vs `[u8]`, which Go's `string` already subsumes.
- **New module `internal/text`** holds: tokenizers, `TextDiff` + its options, change
  expansion, `TextDiffRemapper`, `GetCloseMatches`, and the quick-ratio prefilter.
- **`internal/diff` gains diff vocabulary:** `Change`, `ChangeTag`, `DiffRatio`,
  `GroupDiffOps`. These belong with the existing `DiffOp`/`DiffTag` because they are
  general diff concepts, not text-specific (Rust defines them in `crate::types` /
  `crate::common`, not `crate::text`).
- **Root package `similar`** re-exports the new public surface flat via `aliases.go`,
  matching the existing `internal/diff` → `aliases.go` convention and mirroring how the
  Rust crate re-exports at its crate root.
- Dependency DAG stays acyclic: `diff` ← `algorithms` ← `text`; `text` also imports
  `diff`; the root package imports all three.

### Public API surface (re-exported under `similar`)

- **Rich constructors** (short verbs → `*TextDiff`): `DiffLines`, `DiffWords`,
  `DiffChars`, `DiffSlices`, each `(old, new, opts ...Option)`. No flat one-liner
  shortcuts this release (they are a non-breaking future addition).
  <br>**Amended as shipped:** the general form is `DiffText(old, new string, tok
  Tokenizer, opts ...Option)`, and `DiffLines`/`DiffWords`/`DiffChars` are one-line
  conveniences over it. `DiffSlices` is unchanged and remains the adapter for
  pre-tokenized input. See "Tokenizers" under Behavioral fidelity.
- **Options** (functional): `WithContext(context.Context)`, `WithAlgorithm(Algorithm)`,
  `WithNewlineTerminated(bool)`. Deadlines/cancellation ride the `context.Context`,
  consistent with the existing facade; `newline_terminated` is tri-state (auto unless
  set), modeled internally as `*bool`.
- **`TextDiff` methods:** `Algorithm()`, `NewlineTerminated()`, `OldLen()`/`NewLen()`,
  `OldToken(i)`/`NewToken(i)` (comma-ok), `OldTokens()`/`NewTokens()`
  (`iter.Seq[string]`), `Ratio() float64`, `Ops() []DiffOp`, `GroupedOps(n) [][]DiffOp`,
  `Changes(op) iter.Seq[Change]`, `AllChanges() iter.Seq[Change]`.
- **`Change`:** `Tag() ChangeTag`, `Value() string`, `OldIndex() (int, bool)`,
  `NewIndex() (int, bool)`, `MissingNewline() bool`, `String() string` (value plus a
  trailing `\n` when missing). Optional indices are stored as `*int` so JSON emits
  `null`, matching the upstream serde output. `MarshalJSON` uses snake_case field names.
- **`ChangeTag`:** `Equal`/`Delete`/`Insert` only; `String()` renders `' '`/`'-'`/`'+'`.
  Because package `similar` already exposes `DiffTag` consts named
  `Equal`/`Delete`/`Insert`/`Replace`, the `ChangeTag` values are exposed under distinct
  names: `ChangeEqual`/`ChangeDelete`/`ChangeInsert` (in both `internal/diff` and the
  root re-export).
- **`TextDiffRemapper`:** `NewTextDiffRemapper(d, old, new)` (and a from-tokens
  variant), `SliceOld(start,end)`/`SliceNew(start,end)`, `IterSlices(op) []RemappedChange`
  where `RemappedChange{Tag ChangeTag; Value string}`. Runs are computed by cumulative
  byte offsets into the original string.
  <br>**Superseded as shipped:** remapping lives on `TextDiff` itself
  (`SliceOld`/`SliceNew`/`RemappedChanges`/`AllRemappedChanges`), which reconstructs its
  source text from its own tokens. The separate type required the caller to re-supply the
  two strings, and a mismatch was unchecked. `RemappedChange` is unchanged.
- **`GetCloseMatches(word string, possibilities []string, n int, cutoff float64) []string`.**
- **Also re-exported:** `GroupDiffOps`, `DiffRatio`.

### Behavioral fidelity

- **Op capture** reuses the `capture_diff_deadline` equivalent already in the module: a
  `Compact(Replace(Capture))` hook stack driven by `algorithms.DiffDeadline` over the
  `[]string` tokens. This yields ops including `Replace`, exactly as Rust's
  `TextDiff::ops()` does.
- **`IdentifyDistinct` interning** (Rust's >100-token optimization) is skipped: Go
  `string` is `comparable`, so tokens are diffed directly. Output is identical; only very
  large inputs are slower. Flagged as a known future optimization.
- **Tokenizers** port the `str` implementations from `abstraction.rs` verbatim in
  behavior: lines keep attached newlines and handle `\r`, `\n`, `\r\n`; a
  lines-and-newlines variant separates newline runs; words alternate whitespace-run /
  non-whitespace-run; chars are rune-boundary substrings.
  <br>**Amended as shipped:** the four are public values behind a `Tokenizer` interface
  (`Split(string) []string`, `NewlineTerminated() bool`) — `Lines`, `Words`, `Chars`,
  `LinesAndNewlines` — reached through `DiffText`. Two reasons. First, the
  lines-and-newlines variant this spec requires had no exported path at all: it was
  implemented and unit-tested but unreachable, because every tokenizer was hard-wired
  into its own constructor. Second, the newline-terminated default is a property of the
  tokenizer (true for lines only), and as a positional `bool` on the internal builder it
  travelled separately from the thing that decides it.
  <br>**Deviation from the crate, taken deliberately:** Rust exposes no user-suppliable
  tokenizer, and spec 0001 says the public API is a facade only. Accepting a caller's
  `Tokenizer` adds a capability upstream lacks. It is admitted here because tokenizing is
  a genuine variation point with four in-tree implementations — unlike `Algorithm`, which
  stays a closed enum with one value — and because it makes rules this port explicitly
  put out of scope (Unicode words, graphemes) implementable by the caller without a
  segmentation dependency in this module.
- **Change expansion** ports `TextChangesIter`: `Equal`/`Delete`/`Insert` map directly;
  `Replace` emits all deletes then all inserts, resolving old/new indices identically.
  Out-of-bounds op indices panic (faithful to Rust's `expect`).
- **`DiffRatio`** = `2 * (sum of Equal lengths) / (oldLen + newLen)`, `1.0` when both
  lengths are zero. Returned as `float64` (the Go port is its own golden source of truth).
- **`GroupDiffOps`** ports `group_diff_ops`: trim leading/trailing Equal runs to `n`
  context, split a group whenever an Equal run exceeds `2n`, drop trailing all-Equal
  groups. Operates on the existing `DiffOp` struct representation (an Equal op has
  `OldLen == NewLen == len`).
- **`GetCloseMatches`** ports the difflib flow: char-tokenize the word, prefilter each
  candidate with `upperSeqRatio` and a multiset `quickSeqRatio`, score survivors with a
  char-level `TextDiff` ratio, keep the top `n` at or above `cutoff`, ordered by
  descending ratio with a lexicographic tie-break. The Rust `BinaryHeap` + `Reverse`
  ordering is reproduced with a stable sort; multiset counting uses a `map[string]int`
  instead of a hashed bucket table.

## Testing Decisions

- **What makes a good test here:** assert external behavior — the ops, the change lists
  (tag + value + indices), the ratio, the grouped clusters, the remapped runs, the
  close-match results — not internal data structures. Behavior is pinned to the upstream
  crate's documented examples, which act as the oracle.
- **Seam decision:** per-package in-package tests plus root black-box examples. Each
  internal package is tested in its own package:
  - `internal/diff`: `Change`/`ChangeTag` (tag rendering, comma-ok indices,
    `MissingNewline`, `String`, JSON with null indices), `DiffRatio` (incl. `abcd`/`bcde`
    → `0.75` and empty → `1.0`), `GroupDiffOps` (context trimming, large-Equal splitting).
  - `internal/text`: tokenizers (port every `abstraction.rs` tokenizer unit test as a Go
    table test, e.g. `"first\nsecond\rthird\r\nfourth\nlast"`, `"foo    bar baz\n\n  aha"`,
    `"abcfö❄️"`); `TextDiff` construction and `Ops`/`Ratio` against the ported doc
    examples (`a\nb\nc`/`a\nb\nC`, `foo bar baz`/`foo BAR baz`, `abcdef`/`abcDDf`, slices
    `foo/bar/baz`); change expansion (Replace → delete-then-insert, index correctness,
    both `range` and `slices.Collect`); `TextDiffRemapper` (the `foo bar baz`/`foo bor baz`
    connected-run example); `GetCloseMatches` (`appel` → `["apple","ape"]` plus cutoff /
    `n` / tie-break edges).
  - Root `package similar_test`: runnable `Example…` tests with `// Output:` blocks
    mirroring the existing `example_test.go`, and golden fixtures (JSON of ops/changes
    with a `-update` flag, matching the existing `TestGolden`).
- **Prior art:** table-driven tests with `t.Helper()` assertions and no third-party deps
  (repo-wide); the oracle/self-check pattern in `internal/algorithms/harness_test.go`;
  golden fixtures in `testdata/*.golden`; runnable examples in `example_test.go`;
  fixed-seed determinism for any randomized checks.
- **Optional fuzzing:** extend the existing `testing.F` pattern to a
  tokenize → diff → reconstruct round-trip (reconstruct the original text from the
  changes and assert equality).

## Out of Scope

- **UnifiedDiff formatter** (`crate::udiff`) — the `@@ … @@` textual output. Deferred to a
  later spec; `GroupedOps`/`Ops` are provided so it can be built on top later.
- **Inline highlighting** (`InlineChange`, `InlineChangeOptions`, semantic cleanup) — its
  default refinement algorithm is Patience, which is not ported.
- **Unicode** words/graphemes (`diff_unicode_words`, `diff_graphemes`) — would require a
  non-stdlib segmentation dependency, breaking the stdlib-only constraint.
- **Byte-slice (`[u8]`) diffable support** — Go `string` already holds arbitrary bytes.
- **Flat one-liner shortcut functions** (`utils::diff_words` returning `[](tag,string)`) —
  deferrable, non-breaking to add once the rich API and remapper ship.
- **`IdentifyDistinct` interning optimization** — behavior-neutral; deferred.
- **A CLI.**

## Further Notes

- **Version & process:** ships as v0.2.0. TDD build order below, one commit per step
  (test + implementation together, each compiles and passes), conventional commits
  (`feat:`/`test:`/`ref:`/`docs:`), `gofumpt` + `golangci-lint` clean (respecting the
  repo's intentional `exported`/`redefines-builtin-id` omissions), standard-library-only,
  empty `go.sum`. Tag `v0.2.0` when `make ci` is green.
- **Build order:**
  1. `Change`/`ChangeTag` in `internal/diff` (const names `ChangeEqual`/`ChangeDelete`/`ChangeInsert`).
  2. `DiffRatio`.
  3. `GroupDiffOps`.
  4. String tokenizers.
  5. `TextDiff` core + options (op capture via the `Compact(Replace(Capture))` stack).
  6. Change expansion (`Changes`/`AllChanges`, lazy `iter.Seq`).
  7. `TextDiffRemapper` (+ internal slice remapper).
  8. `GetCloseMatches` (+ `upperSeqRatio` / `quickSeqRatio`).
  9. Root `aliases.go` wiring + runnable examples + golden fixtures.
  10. README / `doc.go` updates, CHANGELOG, tag v0.2.0.
