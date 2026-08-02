# Spec 0001 — Myers Diff Port (similar-go v0.1.0)

Status: ready-for-agent
Module: `github.com/mahibulhaque/similar`
Source of truth: `C:\Source\similar/src/algorithms/myers.rs` (Rust crate `similar`)

## Problem Statement

Go developers who need a high-quality diff have no faithful equivalent of the Rust
`similar` crate's Myers implementation. Existing Go diff libraries either produce
non-minimal edit scripts, lack deadline/cancellation control on pathological inputs,
or expose awkward APIs. A developer wanting "compute the minimal edit script between
two sequences, fast, with a way to bound worst-case time" has to hand-roll Myers or
accept a weaker library.

## Solution

A new Go module `github.com/mahibulhaque/similar` that ports the `similar` crate's
Myers' diff algorithm — behavior-complete, including its heuristics (front-anchor
peel, small-side-exact fallback, disjoint fast-path) and deadline bailout — behind a
small idiomatic Go facade. For v0.1.0 the module ships exactly one algorithm (Myers).

A user calls a single entry point, selecting the algorithm via an enum, and receives a
slice of typed diff operations (`Equal`/`Delete`/`Insert`/`Replace`) carrying indices
and lengths into the original sequences. Advanced users can stream operations by
implementing a hook interface instead of collecting a slice. Users needing to bound
execution time pass a `context.Context` with a deadline; the algorithm periodically
checks it and falls back to an approximate script rather than running unbounded.

The port is faithful in *behavior*, not in *syntax*: every Rust construct with no Go
analog (cross-type equality, trait default methods, operator-overloaded negative
indexing, hash-keyed maps) is replaced by the idiomatic Go mechanism that yields the
same observable result.

## User Stories

1. As a Go developer, I want to diff two `[]T` where `T` is any comparable type, so that I can compare slices of strings, runes, bytes, ints, or my own comparable structs without adapters.
2. As a Go developer, I want the diff to return a minimal edit script (cost `N + M − 2·LCS`), so that the output is as small and meaningful as the classic Myers guarantee.
3. As a Go developer, I want each operation as a tagged struct with explicit index and length fields, so that I can iterate and apply them without decoding an opaque format.
4. As a Go developer, I want the diff operations to be JSON-serializable with stable field names, so that I can persist them or send them over the wire.
5. As a Go developer, I want to select the algorithm through an `Algorithm` enum, so that when more algorithms ship later my call sites stay stable.
6. As a Go developer, I want a one-call convenience function `Diff(old, new)` that returns all operations, so that the common case is a single line.
7. As a Go developer, I want a `CaptureDiff` variant that records operations into a slice, so that I do not have to write a hook for the common case.
8. As an advanced Go developer, I want to implement a `DiffHook` interface and receive operation callbacks as the diff is produced, so that I can stream operations without materializing a slice.
9. As an advanced Go developer, I want to embed a no-op base hook and override only the callbacks I care about, so that I do not have to implement every method.
10. As an advanced Go developer, I want a `Replace` wrapper hook that coalesces adjacent delete+insert into a single replace, so that I can get replace semantics without the core emitting them.
11. As an advanced Go developer, I want a `Capture` hook that accumulates operations, so that I can reuse the same instrument the library and its tests use.
12. As a Go developer, I want to pass a `context.Context` with a deadline, so that pathological inputs cannot make the diff run unbounded.
13. As a Go developer, I want the diff to honor context cancellation, so that I can abort an in-flight diff from another goroutine.
14. As a Go developer, I want a deadline hit to yield a valid (if approximate) edit script rather than an error or a panic, so that my code always gets usable output.
15. As a Go developer, I want the no-deadline path (background context) to always produce the exact minimal script, so that correctness is the default.
16. As a Go developer diffing large, clearly-unrelated sequences, I want the library to short-circuit to a straight replace, so that I do not pay full search cost when there is no common content.
17. As a Go developer diffing a tiny sequence against a large one, I want an exact small-side fallback, so that the result stays minimal and fast in that shape.
18. As a Go developer diffing sequences that differ mainly by a large prepend/append, I want the front-anchor heuristic to peel the shared run early, so that unbalanced shifts diff quickly and exactly.
19. As a Go developer, I want operations that are contiguous, non-overlapping, and fully cover both sequences, so that applying them in order reconstructs the new sequence exactly.
20. As a Go developer, I want a `finish` signal after the last operation, so that a streaming consumer knows the diff is complete.
21. As a Go developer, I want hook methods to return `error` and have that error abort and propagate, so that a failing consumer stops the diff cleanly.
22. As a Go developer, I want empty-input cases (empty vs empty, empty vs non-empty, non-empty vs empty) handled correctly, so that edge inputs do not panic or misbehave.
23. As a Go developer, I want identical inputs to produce a single equal operation, so that the no-change case is trivially cheap.
24. As a Go developer, I want to diff sub-ranges of sequences (non-zero start), so that I can diff windows without copying slices.
25. As a maintainer, I want the algorithm implementation hidden in `internal/`, so that I can refactor it freely without breaking external users.
26. As a maintainer, I want the public API to be a thin facade, so that the supported surface is small and stable.
27. As a maintainer, I want an oracle-driven test suite (brute-force LCS + reconstruct), so that correctness and minimality are verifiable without hand-authored expected output.
28. As a maintainer, I want continuous fuzzing against the invariants, so that random inputs cannot silently break correctness.
29. As a maintainer, I want golden-file regression fixtures, so that trusted output is pinned against accidental change.
30. As a maintainer, I want the module to depend only on the standard library, so that it stays a clean, easy-to-vendor leaf dependency.
31. As a Go developer, I want runnable example functions in the docs, so that I can copy a working usage snippet from pkg.go.dev.

## Implementation Decisions

- **Single generic type parameter `[T comparable]`; both sides are `[]T`.** The Rust cross-type `PartialEq<Old::Output>` path (`diff_deadline_raw`) and the `type_name` cross-type guard in the disjoint fast-path are dropped — unportable and non-idiomatic. `comparable` supplies both equality and map-key usage, replacing Rust's `Hash + Eq` bound.
- **Public API is a facade only.** Users select an algorithm through an `Algorithm` enum (single value `Myers` in v0.1.0) and call `Diff` / `DiffDeadline` / `CaptureDiff`, or implement the `DiffHook` interface. There is deliberately **no** exported per-algorithm entry point (Rust exposes `algorithms::myers::diff`; this port does not). All algorithm code lives under `internal/`.
- **`DiffHook` is an interface whose methods return `error`.** Rust trait default methods are emulated by an embeddable `NoopHook` struct (all methods return nil) that custom hooks embed and selectively override. Rust's associated `type Error` becomes plain `error`; the `Infallible` case is "always return nil". Custom error conditions use structs implementing the `error` interface.
- **`replace` fan-out is owned by a `Replace` wrapper hook, not the interface.** Go interfaces cannot provide a default that dispatches back to the concrete type, so the core emits only equal/delete/insert; the `Replace` wrapper coalesces adjacent delete+insert into a replace. This matches the crate's existing `Replace` composition.
- **`DiffOp` is a tagged struct**, not a sum type:
  ```go
  type DiffTag int
  const ( Equal DiffTag = iota; Delete; Insert; Replace )
  type DiffOp struct {
      Tag      DiffTag `json:"tag"`
      OldIndex int     `json:"old_index"`
      NewIndex int     `json:"new_index"`
      OldLen   int     `json:"old_len,omitempty"`
      NewLen   int     `json:"new_len,omitempty"`
  }
  ```
  For an `Equal`, `OldLen == NewLen == len` (an equal span is the same length on both sides); `Len()` returns `OldLen`. Methods `OldRange()`/`NewRange()` mirror the crate's higher-level `DiffOp`. Chosen over an interface sum type for JSON-friendliness and zero boxing (developer preference).
- **Deadline is a hybrid `context.Context` → `time.Time`.** The public API takes a `context.Context`; at entry the code extracts `ctx.Deadline()` once into a `time.Time` and threads that value through the hot loops (cheap `!deadline.IsZero() && !time.Now().Before(deadline)`, faithful to Rust's `deadline_exceeded`). Context cancellation (`ctx.Done()`) is honored at the same periodic checkpoints (every 1024 iterations). `Diff(old, new)` uses `context.Background()` (never expires). The deadline-hit fallback emits the same approximate script (raw delete+insert of the un-diffed middle) as the Rust implementation.
- **Numeric policy:** signed `int` everywhere (indices, lengths, ranges, diagonal `k`, `delta`, `x`, `y`) to avoid underflow on the many `end − len` subtractions; `uint8` only for the small-side-exact dynamic-programming buffer (values are bounded by the small-side cap of 64, matching Rust's `u8` memory profile); a `satMul` helper guards the `work = oldLen * newLen` heuristic gates against overflow.
- **The FNV-1a hash layer is removed entirely.** Rust's `stable_hash` + hash-collision bucketing exist only because it keys maps by `u64`. Go maps key on `T comparable` directly, so the disjoint fast-path's "has common item" check is a `map[T]struct{}` membership test. No `uint64` hashing, no collision handling, no hash utility.
- **The Myers `V` vector becomes a `reachVector` struct** (`offset int; data []int`) with `at(k)`/`set(k, x)` methods, replacing Rust's operator-overloaded `Index<isize>`; the two instances are named `fwd` and `bwd` (was `vf`/`vb`).
- **Package layout** *(superseded by [ADR 0001](../adr/0001-vocabulary-at-the-root.md): the
  vocabulary is now declared in `package similar` and the alias files are gone. The
  description below records the v0.1.0/v0.2.0 layout.)* follows go.dev/doc/modules/layout: root package `similar` is the facade (`Algorithm`, `Diff`, `DiffDeadline`, `CaptureDiff`, plus `aliases.go` re-exporting the shared types via `type X = diff.X` for single-import ergonomics). `internal/diff` is a leaf holding the canonical `DiffOp`, `DiffTag`, `DiffHook`, `NoopHook`, `Capture`, `Replace`, `Compact`. `internal/diffutil` is a leaf holding `commonPrefixLen`, `commonSuffixLen`, `isEmptyRange`, `satMul`. `internal/algorithms` holds the Myers implementation and heuristics with colocated tests. Dependency DAG is acyclic: `diff` ← `algorithms` ← `similar`, `diff` ← `similar`. `docs/` holds prose; `doc.go` + example functions live in-package. Optional `cmd/` reserved for a future demo.
- **Heuristics are layered on a proven-correct classic core** and are pure optimizations: front-anchor peel, small-side-exact (both directions), and disjoint fast-path must each preserve the minimal-script invariant. Only the deadline path may produce a non-minimal (approximate) script, and only when the deadline is actually hit.

## Testing Decisions

- **Good tests assert external behavior, not implementation details.** The two load-bearing behaviors are (1) applying the operations to `old` reconstructs `new`, and (2) the edit cost is minimal. Tests assert those, not the internal shape of the search.
- **Oracle-driven correctness.** Test-only helpers form the oracle: `bruteLCS` (an O(N·M) longest-common-subsequence DP), `reconstruct(old, ops)` (applies operations), and `editCost(ops)` (sums delete+insert lengths). The oracle is self-tested on trivial known inputs before it is trusted to judge Myers.
- **Invariant tests are the red drivers.** For any input: `reconstruct(old, Diff(old, new)) == new`; `editCost(Diff(old, new)) == len(old) + len(new) − 2·bruteLCS(old, new)`; operations are contiguous, non-overlapping, and fully cover both sequences. These fail before the algorithm exists and pass once the classic core is implemented — no hand-authored expected operations are ever needed.
- **Modules under test:** `internal/diff` (op struct JSON round-trip; `NoopHook` returns nil; `Capture` accumulates; `Replace` coalesces), `internal/diffutil` (prefix/suffix/empty-range/satMul, porting the crate's `common_prefix_len`/`common_suffix_len` cases), and `internal/algorithms` (the full invariant + edge-case + deadline + heuristic suite). The root `similar` package gets integration tests, golden fixtures, and example functions.
- **Ported Rust tests** (as behavioral fixtures, judged by the oracle): `test_find_middle_snake` (pins the middle snake at x=4, y=1), `test_diff`, `test_contiguous`, `test_pat`, `test_small_side_exact_variants` (all three sub-cases including sparse overlap at index 500), `test_deadline_reached`, `test_heuristic_deadline_guards` (asserts zero element accesses when the deadline is already exceeded), `test_front_anchor_regressions_stay_exact` (Myers cost equals LCS cost), and `test_finish_called` (finish invoked for all inputs including empty).
- **Golden files** (`testdata/*.golden` with an `-update` flag) are the Go stand-in for the crate's `insta` snapshots — regression guards only, added after the invariants are green. No snapshot library.
- **Native fuzzing** (`testing.F`) continuously feeds random `[]int`/`[]byte` inputs and re-checks the reconstruct and minimality invariants on every seed.
- **Edge-case matrix (every case included):** both empty, pure insert, pure delete, identical, fully disjoint, single-element (equal and differing), common prefix only / suffix only / prefix+suffix with changed middle, single differing middle element, reversed sequences, interleaved edits, non-zero start ranges, all-identical repeated elements, and the `i % 2` repetitive pattern; heuristic threshold boundaries tested at N−1 / N / N+1 for the constants 64, 512, 64M, 96, 4, `large ≥ small·2`, 512, and 128K; small-side-exact in both directions; deadline cases (already-exceeded, mid-run, commit-after-first-emit, nil, context-cancel); hook-contract cases (finish always called including empty, Replace coalescing, error propagation).
- **Test execution:** `go test ./... -race -cover`; a short fuzz run in CI and a longer nightly fuzz run.

## Out of Scope

- Any algorithm other than Myers (Patience, Histogram, Hunt, LCS) — reserved for later versions; the `Algorithm` enum is shaped to accept them.
- Cross-type diffing (old and new being different element types related only by equality) — dropped from the port.
- The raw, non-heuristic entry point (`diff_deadline_raw`) that accepts non-comparable values.
- Any exported per-algorithm entry point; only the facade is public.
- Higher-level text/line/word/char diffing, unified-diff formatting, and change-tagging conveniences from the broader `similar` crate.
- Sequence adapters (`CachedLookup`, `IdentifyDistinct`) and the `unique` utility.
- A CLI (`cmd/`) — reserved, not built in v0.1.0.

## Further Notes

- Go 1.26 toolchain; standard library only (no third-party dependencies; empty `go.sum`).
- Tooling: `gofumpt` formatting; `golangci-lint` with `govet`, `staticcheck`, `errcheck`, `revive`, `gofumpt`, `ineffassign`, `unconvert`, `misspell`; a Makefile with `fmt`, `lint`, `test`, `fuzz`, `cover`, `ci` targets; GitHub Actions running `make ci` plus a golden-file check (no `-update`) and a nightly fuzz job. Semantic-version tag `v0.1.0` cut once the suite is green and fuzzing is clean. Conventional-commit messages (`feat:`, `test:`, `ref:`), mirroring the source repo.
- Build order (each step is its own commit with test and implementation together, and each commit compiles and passes): (1) diff types, (2) hooks (Capture/Replace/Compact), (3) diffutil, (4) test oracle, (5) classic Myers core — correct and minimal with zero heuristics, the v0.1 backbone, (6) front-anchor peel, (7) small-side-exact both directions, (8) disjoint fast-path, (9) deadline plumbing, (10) root facade with golden fixtures and an example, (11) fuzz + CI + tag.
- Faithfulness principle: the port matches observable behavior, not Rust syntax. Where Go and Rust diverge (cross-type equality, trait defaults, operator overloading, hash-keyed maps), the Go-idiomatic mechanism is used so that end-user results are identical.
