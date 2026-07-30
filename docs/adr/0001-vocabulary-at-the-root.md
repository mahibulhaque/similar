# ADR 0001 — The public vocabulary is declared at the root, not aliased into it

- **Status:** accepted
- **Date:** 2026-07-30
- **Supersedes:** the *Package layout* decision in [spec 0001](../specs/0001-myers-diff-port.md),
  and the *Layering & modules* decisions plus user stories 32–33 in
  [spec 0002](../specs/0002-text-diff-port.md)

## Context

Specs 0001 and 0002 put every implementation type in `internal/` and made the root
package a facade that re-exported it:

```go
// aliases.go, text_aliases.go — 132 lines
type TextDiff = text.TextDiff
type Change   = diff.Change
func DiffWords(old, new string, opts ...Option) *TextDiff {
	return text.DiffWords(old, new, opts...)
}
```

The reasoning was sound on its own terms: hide the implementation, keep the supported
surface small, stay free to refactor. Two things were not foreseen.

**A type alias to an unexported package documents nothing.** `go doc` and pkg.go.dev
render the alias line and stop:

```
$ go doc . TextDiff
type TextDiff = text.TextDiff
    TextDiff is a captured text diff over string tokens. See text.TextDiff.
```

No methods. `Ratio`, `AllChanges`, `Ops`, `GroupedOps`, `SliceOld`, `RemappedChanges` —
absent. And "See `text.TextDiff`" points into `internal/`, which pkg.go.dev does not
publish, so the pointer goes nowhere. Across `TextDiff`, `Change`, `DiffOp`, `Capture`,
`ReplaceHook`, `NoopHook`, `DiffTag`, `ChangeTag` and `Algorithm` that was **54 exported
methods, none documented**, on the tagged v0.2.0 release. Four doclinks in `doc.go`'s own
front-page prose pointed at those invisible methods.

That is the same defect one layer up from the one this codebase had already fixed:
`tokenizeLinesAndNewlines` was implemented, tested, and unreachable. `Ratio()` was
implemented, tested, and undiscoverable.

**A symbol was not public until someone hand-wrote its forwarder.** Nothing enforced
that. Adding a method to `internal/text` shipped it invisible until a second, unrelated
edit landed in `text_aliases.go`.

Go's type aliases were introduced for *gradual repackaging* — moving a type between
packages without breaking users mid-migration. Using them as a standing export mechanism
holds the feature sideways, and the missing documentation is what that costs.

## Decision

Declare the public vocabulary in `package similar` itself. `DiffOp`, `DiffTag`, `Change`,
`ChangeTag`, `DiffHook`, `NoopHook`, `Capture`, `ReplaceHook`, `Algorithm`, `TextDiff`,
`Tokenizer`, `Option`, `RemappedChange` and the four shipped tokenizers are defined where
they are exported. `aliases.go` and `text_aliases.go` are deleted. `internal/diff` and
`internal/engine` are gone; `internal/algorithms` and `internal/diffutil` remain.

**`internal/algorithms` survives deliberately.** It is the one boundary in this codebase
that varies something: `DiffHook` has four implementations and `Algorithm` exists so a
second algorithm can land. It now declares its own unexported `diffHook` interface rather
than importing the public one, so it depends on the shape of its consumer and not on the
consumer's package. The two declarations are checked against each other by the compiler
at the single `algorithms.DiffDeadline` call site in `engine.go`.

**Symbols that were exported in `internal/` but never forwarded arrive unexported**, so
the move could not silently grow the supported surface: `Compact`→`compact`,
`NewCompact`→`newCompact`, `EqualChange`/`DeleteChange`/`InsertChange`→lowercase,
`engine.Run`→`run`, `engine.Capture`→`captureOps`.

The public API did not change. `TestPublicAPI` snapshots the exported surface to
`testdata/api.golden`; across the whole move no entry was removed or re-signatured. The
only deltas were alias lines becoming real declarations and 54 methods appearing.

## Alternatives considered

**Un-internal the packages** — rename `internal/text` → `text` and `internal/diff` →
`diff`. Roughly 50 lines of import churn instead of 2000 moved, and it never touches the
Myers test suite. Rejected: it fixes reachability but not presence (the `similar` package
page still lists no methods and asks the reader to click through), it entrenches the alias
layer permanently, and it turns `compact`, `newCompact`, `run` and the three `Change`
constructors into public API across three namespaces that can never be retracted.

**Merge `internal/algorithms` into the root too.** Would avoid declaring the hook
interface twice and would need no test rework. Rejected: it spends the only enforced seam in the
codebase, lets the text layer reach `findMiddleSnake`, collides `algorithms.Diff` with the
root's `Diff`, and breaks both CI fuzz jobs.

**Wrapper types at the root that delegate.** Rejected: 54 delegating methods to keep in
sync, for the same result.

## Consequences

- Every exported method is documented on the package page users actually land on.
- Exporting is now a capital letter at the definition site; there is no forwarder to
  forget. The replacement failure mode — an *accidental* export — is what `TestPublicAPI`
  guards, and it fires on added, removed, re-signatured and reordered declarations alike.
- The root package is larger: ~24 implementation files. File names carry the layer, with
  the text layer under a `text_` prefix.
- White-box tests from the three retired packages now live at the root as `package
  similar`, beside the black-box `package similar_test` files.
- Spec 0001's "public API is a facade only" still holds in substance — no per-algorithm
  entry point is exported — but it is no longer implemented by an alias layer.
