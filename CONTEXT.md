# Domain model

The words this codebase uses, and what they mean here. Where a term has a
common meaning elsewhere that differs, the difference is called out — those are
the ones worth reading.

Architectural decisions live in [docs/adr](docs/adr); the original ports are
specified in [docs/specs](docs/specs). Where an ADR supersedes a spec, the ADR
wins.

## The two layers

**Sequence layer** — diffs any `[]T` where `T` is comparable. `Diff`, `DiffTo`,
`DiffRangeTo`. Knows nothing about text.

**Text layer** — diffs strings by tokenizing them and handing the tokens to the
sequence layer. `DiffText` and its conveniences, `TextDiff`. Files carry a
`text_` prefix; anything without one belongs to the sequence layer or to both
(ADR 0001).

## Core terms

**Token** — one element of a sequence. For the text layer, one piece a
**Tokenizer** cut a string into. Not necessarily a word: `Chars` makes each rune
a token, `Lines` makes each line one.

**Op** (`DiffOp`) — one tagged span of an edit script, carrying indices and
lengths into both sequences. Tagged `Equal`, `Delete`, `Insert` or `Replace`.
Every op uses the same `(index, len)` layout on both sides regardless of tag,
which is what lets `OldRange` and `NewRange` be computed uniformly.

**Script** — the ordered `[]DiffOp` for one diff. Two properties hold of every
script this package produces: applying it to `old` reconstructs `new` exactly,
and its ops are *cursor-contiguous* — each op starts where the previous one
ended, on both sides, together spanning both sequences.

**Change** — a *per-token* view of a diff, as opposed to an Op's *span* view.
Where one Op says "delete old[3:7]", four Changes say "delete this token", each
carrying the token's value. `TextDiff.Changes` expands ops into them lazily.

**RemappedChange** — a *byte-run* view: a tag plus the substring of the original
text an op covers, obtained by joining tokens. The three views — Op, Change,
RemappedChange — are the same diff at three granularities.

## Hooks

**Hook** (`DiffHook`) — the streaming interface a diff emits through. A hook
sees indices and lengths, never the values. Any callback may return an error,
which aborts the diff; `Finish` is called once at the end.

**Hook stack** — the composition every materializing entry point runs:
`Compact(Replace(Capture))`, assembled in exactly one place (`captureOps`).
Read outside-in: **Compact** buffers and cleans up the script semantically,
**ReplaceHook** coalesces an adjacent Delete+Insert into a Replace, and
**Capture** accumulates the result. A caller who supplies their own hook via
`DiffTo` gets none of this — they see the raw script.

**Replace is a synthesis, never an observation.** This is the term most likely
to mislead. Neither core can produce a `Replace`: their internal contract has
no such callback, and writing one is a compile error (ADR 0002, and the comment
on `internal/algorithms.diffHook`). Every Replace a caller sees was manufactured
by `ReplaceHook` from a Delete immediately followed by an Insert. So "did this
op come from the algorithm?" and "is this op a Replace?" have opposite answers,
and whether a Replace is visible at all depends on the hook stack, not the
input.

## Configuration

**Option** — `WithContext` and `WithAlgorithm`, and deliberately nothing else.
Every option applies to every entry point in both layers. Anything that suits
only one is an argument to that function: a hook and a sub-range change what is
returned, not how it is computed (ADR 0002).

**Algorithm** — selects the implementation: `Myers`, the default, or `LCS`, the
classic table algorithm. Both produce a minimal script, so the choice is one of
cost, not quality. It is validated in exactly one place, `WithAlgorithm`, at the
moment the option is built rather than when the diff runs.

**Approximate script** — what a diff yields when its context deadline expires or
is cancelled: still valid, still reconstructs `new`, but no longer guaranteed
minimal. This is **not an error**, which is why `Diff` returns none.

## Text layer

**Tokenizer seam** — the `Tokenizer` interface, and the one boundary in the text
layer that genuinely varies. It carries policy, not just a function: alongside
`Split` it answers `NewlineTerminated`. Callers can implement their own; the
package ships `Lines`, `Words`, `Chars` and `LinesAndNewlines`, the last of
which is reachable *only* through the seam.

**Split must account for every byte.** The remapping methods rebuild source text
by joining tokens, so if a tokenizer drops or rewrites bytes, the joined tokens
— not the original string — are what gets remapped.

**Newline policy** — whether tokens carry their trailing newline, which is what
downstream renderers consult. It belongs to whatever produced the tokens: every
entry point takes it from its Tokenizer (wrap one in `NewlineTerminated` to
change the answer), except `DiffSlices`, which has no tokenizer and so takes a
`NewlinePolicy` argument.

**Group** — a window of ops around the changed regions, with long Equal runs
trimmed. What a "unified diff with N lines of context" needs.

## Inside `internal/algorithms`

Not public, but these words appear throughout the package and its commits.

**Gate** — a threshold deciding whether a heuristic runs at all
(`smallSideExactMinLarge`, `disjointFastPathMinLen`, …). Gates live on
**`limits`**, the value threaded through the recursion alongside the deadline,
so tests can move one rather than building an input large enough to cross it.
Distinct from the **deadline-poll intervals**, which govern how often the clock
is read and never what the algorithm decides — those stay constants.

**Heuristic** — a shortcut for an input shape where full search is wasteful: the
**disjoint fast path** (two large ranges sharing no element), the
**small-side-exact** walk (one side tiny, the other large), and **front-anchor
peeling** (a heavily unbalanced shift). All three are *exactness-preserving* —
they produce the same script full search would, only faster. That is why no test
can identify which one ran by looking at the output. Only the first is shared:
the other two are steps inside Myers' recursion, not preflight checks, so the
LCS diff runs the disjoint fast path and nothing else.

**Table** — the LCS algorithm's whole method: one cell per pair of suffixes,
holding the length of their longest common subsequence, filled backwards and then
walked forward to emit one op per element. Two words about it mislead if read
loosely. It is not the small-side-exact heuristic's dp table, which is a
different table for a different purpose. And a **declined** table — refused for
crossing `lcsTableMaxWork`, or abandoned on a deadline — is not an error: the
changed middle becomes one Delete plus one Insert, which is what "approximate
script" means for this algorithm.
