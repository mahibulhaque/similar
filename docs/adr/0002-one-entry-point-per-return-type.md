# ADR 0002 — One entry point per return type

- **Status:** accepted
- **Date:** 2026-08-02
- **Supersedes:** the entry-point shape in [spec 0001](../specs/0001-myers-diff-port.md)
  (user story 7 and the *Public API is a facade only* decision, insofar as it names
  `Diff` / `DiffDeadline` / `CaptureDiff`), and the *Options* decision plus user story 22
  in [spec 0002](../specs/0002-text-diff-port.md)

## Context

[ADR 0001](0001-vocabulary-at-the-root.md) moved the public vocabulary to the root so that
every exported method documents itself on the page users land on. It fixed *where* the
API is declared. It did not ask whether the API was the right shape, and the shape it
inherited was five functions over a twelve-line engine:

```go
func Diff[T comparable](old, new []T) []DiffOp                                    // 2-line body
func CaptureDiff[T comparable](alg Algorithm, old, new []T) []DiffOp              // 3-line body
func DiffDeadline[T comparable](ctx, alg, old, new) ([]DiffOp, error)             // 1-line body
func DiffHookDeadline[T comparable](ctx, alg, hook, old, new) error               // 1-line body
func DiffRangeHookDeadline[T comparable](ctx, alg, hook, old, oldStart, oldEnd,
                                         new, newStart, newEnd) error             // 1-line body
```

`DiffHookDeadline` *is* `DiffRangeHookDeadline` with `0, len(old)` and `0, len(new)`.
`Diff` *is* `DiffDeadline` under a background context with the error dropped. None of the
five held any behaviour; the depth was all downstream, in `internal/algorithms` and
`compact.go`, which between them run to roughly 1,100 lines and export nothing.

The cost was paid at the call site. To choose among the five a reader had to hold four
orthogonal concepts — `Algorithm`, deadline-versus-cancellation, "an approximate script is
not an error", and hook-versus-slice — and then work out which combination each *name*
encoded. Four concepts, five names, no signature stating the difference.

**One rule was stated in five places.** "Is this `Algorithm` legal?" was answered by
`Algorithm.Valid` (`algorithm.go`), the `mustBeKnown` panic (`similar.go`), `run`'s
`default:` error (`engine.go`), the `WithAlgorithm` panic (`text_options.go`), and an
impossible-error panic in the text layer's `build` — plus a sixth statement in `doc.go`'s
prose. `Valid` was a public method whose only reason to exist was that split.

**The two layers configured themselves differently.** The text layer had a functional
`Option`; the sequence layer had positional parameters for the same two settings. A reader
of both learned two idioms for `context` and `Algorithm`.

## Decision

Three entry points, one per return type, over an option seam shared by both layers.

```go
func Diff[T comparable](old, new []T, opts ...Option) []DiffOp
func DiffTo[T comparable](hook DiffHook, old, new []T, opts ...Option) error
func DiffRangeTo[T comparable](hook DiffHook, old []T, oldStart, oldEnd int,
                               new []T, newStart, newEnd int, opts ...Option) error
```

**A hook is not an option, and neither is a range.** Options vary *behaviour*; a hook
changes the *return type*, because with one there is no `[]DiffOp` to hand back. An option
that silently nils out the return value would reproduce, in one function, exactly the
vagueness this ADR removes from five. A sub-range is the same case once removed: it is
meaningful only with a hook, since its entire purpose is that reported indices stay
absolute — which is also the only thing distinguishing it from slicing the inputs, Go
slices being free. Both are therefore arguments, and each earns its own signature.

**`Diff` returns no error.** Nothing it can encounter is one. A deadline or cancellation
yields an approximate but valid script, already documented as not-an-error; `WithAlgorithm`
rejects an unusable value where it is passed; and the hooks `Diff` assembles cannot fail.
`engine.go` had said as much for some time — *"the hooks in that stack never fail, so a
non-nil error means alg was unknown"* — and once validation moved to the option, that
remaining case became unreachable. Making it structural is worth more than the comment
was: the `ops, _ :=` idiom disappears, and callers stop being invited to handle a case that
cannot arise. `captureOps` likewise stops returning an error, which collapses two identical
"this cannot happen" panics into one statement in the place that knows why.

**`Option` is `{ctx, algorithm}` and applies everywhere.** Both fields are meaningful at
every entry point in both layers, with nothing silently ignored in either direction. Two
things were removed to make that true:

- `WithNewlineTerminated` is gone. The flag is a property of the tokens, so it belongs to
  whatever produced them — and `NewlineTerminated()` was already on the `Tokenizer` seam.
  A decorator, `NewlineTerminated(tok, yes)`, now carries the override by the same route as
  the default, so `build` reads one source instead of reconciling two. The tri-state
  `*bool` that distinguished "unset" from "explicitly false" went with it: a tokenizer
  always has an answer, so there is no third state to model.
- `DiffSlices` takes a `NewlinePolicy` argument. It is the one entry point with no
  tokenizer to ask, so only its caller can say. A named constant is the point —
  `DiffSlices(a, b, true)` says nothing at a call site; `DiffSlices(a, b, PlainTokens)`
  does.

**The validity rule has one statement.** `WithAlgorithm` is the only place an `Algorithm`
enters the package, and it validates when the option is applied rather than when the diff
runs, so entry points returning no error cannot be handed a value they could not report.
`Algorithm.Valid` is unexported to `valid`, `mustBeKnown` is deleted, and `run`'s
unreachable arm panics rather than returning an error.

**No deprecated shims. Hard break at v0.3.0.**

## Alternatives considered

**Deprecated wrappers for one release.** Keep the four as `// Deprecated:` forwarders,
remove at v0.4.0. Rejected: it forfeits the change's main justification. The four would
still sit in `api.golden`, still render on the package page, and still be the first thing a
reader — or an agent reading the package — encounters. The point is not that the four
functions are expensive to keep; it is that they are what someone learning this API has to
wade through. Pre-1.0 is precisely the window where that is affordable to fix outright.

**One `Diff` with a `WithHook` option.** Smallest surface: a single function. Rejected: the
return value's meaning would depend on an option, so `[]DiffOp` would be nil whenever
`WithHook` was passed. Options cannot change types, and pretending otherwise moves the
ambiguity from the function list into the return value, where it is harder to see.

**Keep the error on `Diff` for forward compatibility**, in case a future algorithm or hook
can fail. Rejected: it costs every caller an `if err != nil` that is unreachable today, to
buy an API change we may never make — and if we do, an algorithm that can fail is a large
enough change to warrant its own entry point or its own major version.

**Two distinct option types**, `Option` for text and `SeqOption` for sequences, so
inapplicable options are a compile error. Rejected: `WithContext` and `WithAlgorithm` are
common to both, so each would need duplicating under a second name in the same package.
That is a worse outcome than the leak it prevents — and moving the newline flag onto the
tokenizer removed the leak anyway.

**Share one `Option` and tolerate the leak** — `Diff` accepting `WithNewlineTerminated`,
`DiffLines` accepting a range, both ignored. Rejected: silently-ignored configuration is
the same defect as a name that does not say what it does.

## Consequences

- Five sequence entry points become three, and each signature states what distinguishes it.
  `similar.Diff(old, new)` — the common call — is unchanged, variadic parameters being
  source-compatible; the other four are a break.
- One configuration idiom across both layers. Learning `WithContext` once is enough.
- `api.golden` loses `CaptureDiff`, `DiffDeadline`, `DiffHookDeadline`,
  `DiffRangeHookDeadline`, `Algorithm.Valid` and `WithNewlineTerminated`; it gains `DiffTo`,
  `DiffRangeTo`, `NewlineTerminated`, `NewlinePolicy` and its two constants. `Diff` and
  `DiffSlices` are re-signatured.
- The failure mode this trades into: `WithAlgorithm` panics where the old error-returning
  entry points did not. That is deliberate — a bad `Algorithm` is a programming error, not
  a runtime condition — and it now happens at the argument, not several frames later.
- Spec 0001's "public API is a facade only" still holds. There is still no exported
  per-algorithm entry point; there are simply fewer facade functions.
