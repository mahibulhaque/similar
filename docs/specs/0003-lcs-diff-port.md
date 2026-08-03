# Spec 0003 — LCS Diff Port (similar-go v1.0.0)

Status: shipped
Module: `github.com/mahibulhaque/similar`
Source of truth: `src/algorithms/lcs.rs` (Rust crate `similar`)

## Problem Statement

The module ships one algorithm. `Algorithm` exists so a second one can land — spec
0001 lists LCS as reserved for a later version and ADR 0001 keeps
`internal/algorithms` alive as the one boundary that varies — but until one does,
that shape is a promise rather than a fact. Two things are missing as a result. A
caller who wants to reproduce another difflib-style implementation's output, or who
wants a second implementation to check the first against, has nothing to select. And
the seam itself is untested: an `Algorithm` type with one value, a switch with one
arm, and a heuristic layer written as if it were shared cannot show that adding an
algorithm costs a constant, a case, and a file.

## Solution

Port the crate's classic LCS-table algorithm as the second value of `Algorithm`,
reachable through the existing option in both layers:

```go
ops := similar.Diff(old, new, similar.WithAlgorithm(similar.LCS))
```

The algorithm peels the common prefix and suffix, builds one table of
longest-common-subsequence lengths over what is left, and walks it forward emitting
one operation per element; the standard hook stack compacts those into spans. It is
O(N·M) in time and space, and minimal — the same `N + M − 2·LCS` guarantee Myers
gives, which is what lets the existing oracle judge it unchanged.

Nothing else moves. `WithAlgorithm` remains the only place a value is validated,
`run` remains the only switch, the disjoint fast path and the deadline plumbing are
shared as written, and Myers stays the default and the algorithm for large inputs.

## User Stories

1. As a Go developer, I want to select an LCS table diff instead of Myers, so that I can reproduce the output of difflib-style tools that use one.
2. As a Go developer, I want the choice to be one option on the call I already make, so that switching algorithms does not mean switching entry points.
3. As a Go developer, I want the option to apply to the text layer too, so that `DiffLines`, `DiffWords` and `DiffChars` honour it exactly as `Diff` does.
4. As a Go developer, I want the LCS script to be minimal, so that choosing it costs me speed and never quality.
5. As a Go developer, I want a bad `Algorithm` value to be rejected where I pass it, so that adding a second value does not add a second way to fail.
6. As a Go developer, I want the LCS algorithm to state its cost in its doc comment, so that I learn it is O(N·M) before I run it over two large files rather than after.
7. As a Go developer diffing something large under LCS, I want a bounded failure mode, so that an oversized input degrades to an approximate script instead of exhausting memory.
8. As a maintainer, I want both algorithms judged by one oracle, so that a regression in either shows up as a broken invariant rather than as a stale snapshot.
9. As a maintainer, I want the two algorithms cross-checked on cost, so that a disagreement about what "minimal" means cannot pass unnoticed.
10. As a maintainer, I want every divergence from the crate recorded at its site, so that the next person to compare the two files can tell a decision from a mistake.
11. As a maintainer, I want each algorithm to own its entry-point names, so that no call site has to guess which implementation a generic `Diff` means.

## Implementation Decisions

- **One entry-point pair per algorithm.** The package's `Diff`/`DiffDeadline` were generic names for one specific algorithm, so they became `DiffMyers`/`DiffMyersDeadline`, joined by `DiffLCS`/`DiffLCSDeadline`. The package doc moved to `doc.go` with the rename, since it described the package as the Myers implementation.
- **The table is a flat `[]int32`, not a map.** The crate keys a `BTreeMap` by `(i, j)` and stores only non-zero cells, falling back on a zero for a missing one. One slice of `(newLen+1)*(oldLen+1)` cells, with the trailing row and column left zero, holds the same values — and holds them in an amount of memory predictable from the range lengths alone, which is what the gate below needs.
- **`lcsTableMaxWork` gates the table.** Roughly 64 MiB of cells, the byte budget `smallSideExactMaxWork` already gives its `uint8` table. The crate has no such bound; this algorithm is O(N·M) in space, so without one a large enough input is an allocation failure rather than a slow diff. Over the gate the table is declined and the changed middle becomes one Delete plus one Insert — the same approximation Myers makes on a deadline. It is a field on `limits` rather than a constant so a test can drive its edge.
- **A declined table emits its replacement once.** The crate emits the Delete/Insert pair and then leaves its cursors unadvanced, so its own tail emits repeat the pair and the script reconstructs `new` twice over. Here the walk is skipped with both cursors at zero and the tail emits do the work, which produces the pair exactly once.
- **The all-equal shortcut emits at the range starts.** The crate hard-codes `equal(0, 0, len)`, which reports the wrong span for a sub-range diff. Every operation this package emits is in absolute coordinates.
- **Two empty ranges emit nothing.** The crate emits a zero-length delete for that case. Emitting nothing keeps a zero-length operation out of every consumer and matches Myers.
- **The deadline is honoured per table row, not per cell.** A row is O(oldLen) work, which is what makes reading the clock unconditionally at the top of one cheap. A deadline is a poor fit for this algorithm either way — the table is built before any operation is emitted, so a deadline costs the whole table or nothing — and the crate says as much. The bound worth having here is on size, and that one is not the caller's to pass.
- **The shared heuristics stay shared, and stay as they were.** LCS runs the disjoint fast path, exactly as the crate's does. Front-anchor peeling and the small-side-exact fallback are Myers': they are steps inside its recursion, not preflight checks.
- **`GetCloseMatches` keeps its hard-wired Myers.** It takes no options and its ratio is a similarity number, not a script.

## Testing Decisions

- **The oracle is unchanged and now judges both.** Both algorithms are minimal, so `reconstruct`, `editCost == len(old)+len(new)−2·bruteLCS`, and cursor-contiguity apply to LCS as written. `TestInvariantMatrix` ranges over `everyAlgorithm` and runs all twenty shapes through each.
- **Ported Rust fixtures, judged by the oracle** where the crate uses an `insta` snapshot (`test_diff`, `test_contiguous`, `test_pat`, `test_same`) and pinned operation-for-operation where the crate pins them (`test_issue44_swapped_regression`, `test_subrange_regression`, `test_bad_range_regression`, `test_table`, `test_finish_called`). The two exact-op regressions are the port's fidelity anchors.
- **Each divergence has a test.** `TestLCSIdenticalSubRange` covers the absolute-coordinate fix, `TestLCSEmptyRangesEmitNothing` the empty case, and `TestLCSTableGate` both sides of the gate — at the cap the script is minimal, one cell over it reconstructs, stays contiguous, and contains the replacement once.
- **Fuzzing cross-checks the two.** `FuzzLCSInvariants` mirrors the Myers target and adds an assertion that both algorithms report the same edit cost, since both claim minimality.
- **Golden files are per algorithm.** Myers keeps the original names, LCS gets an `lcs_` prefix. The two sets are identical today, which is the point of keeping them separate: a future divergence shows up as a diff in one file rather than as a judgement about which snapshot is right.
- **The public seam is tested at the surface.** `TestWithAlgorithmLCSMatchesMyers` runs both through the whole hook stack, `TestTextLayerHonorsWithAlgorithm` covers both in the text layer, and `TestPublicAPI` records the new constant.

## Out of Scope

- Patience and Histogram diffing — Patience is the natural next spec; it needs the crate's `unique` utility and delegates its gap diffs to Myers.
- The crate's `diff_deadline_raw` entry points, which accept values that are only `PartialEq`. `T comparable` is the module's constraint.
- Any exported per-algorithm entry point in package `similar`; `DiffLCS` is internal, reachable only through `WithAlgorithm`.
- Tuning `GetCloseMatches` or `Ratio` to honour the algorithm option.
- Raising the LCS table gate, or making it configurable. It is not part of the public API.

## Further Notes

- Go 1.26; standard library only. Conventional-commit messages, one commit per build step, each compiling and passing.
- Build order: (1) `ref` the Myers entry points and move the package doc, (2) `feat` the LCS core with its gate and the ported fixtures, (3) `test` the fuzz target and the cross-check, (4) `feat` the public `LCS` constant with goldens and API snapshot, (5) `docs` — this spec, `CONTEXT.md`, `README.md`, and the CI fuzz targets.
- Faithfulness principle, as in spec 0001: observable behavior, not Rust syntax. The four divergences above are each a case where faithfulness would have cost correctness (three of them) or bounded memory (one), and each is marked at its site in `internal/algorithms/lcs.go`.
