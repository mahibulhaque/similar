// Package similar computes a minimal edit script (Myers' diff) between two
// slices of any comparable type.
//
// It is a faithful behavior port of the Rust `similar` crate's Myers
// implementation: the classic divide-and-conquer middle-snake recursion plus
// its heuristics (front-anchor peel, exact small-side fallback, and a disjoint
// fast path), with an optional deadline that bails to an approximate script
// rather than running unbounded.
//
// # Quick start
//
// The common case is a single call:
//
//	ops := similar.Diff([]string{"a", "b", "c"}, []string{"a", "x", "c"})
//	for _, op := range ops {
//		fmt.Println(op.Tag, op.OldRange())
//	}
//
// Diff returns a slice of [DiffOp], each tagged Equal, Delete, Insert, or
// Replace and carrying indices and lengths into the two sequences. Applying the
// operations in order reconstructs the new sequence exactly, and the edit cost
// is minimal (N + M − 2·LCS).
//
// # Deadlines and cancellation
//
// Pass a context with a deadline (or one you can cancel) to bound execution on
// pathological inputs:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
//	defer cancel()
//	ops, err := similar.DiffDeadline(ctx, similar.Myers, old, new)
//
// A deadline hit yields a valid but possibly approximate script; it is not an
// error. The no-deadline path always produces the exact minimal script.
//
// # Algorithm selection
//
// [Algorithm] selects the implementation; Myers is the only value in this
// release. Every entry point routes through one dispatch point, so an unknown
// value is rejected consistently: the functions that return an error report it
// there, while [CaptureDiff] and [WithAlgorithm], which cannot, panic.
//
// # Streaming with a hook
//
// Advanced users can implement [DiffHook] and receive callbacks as the diff is
// produced, avoiding a materialized slice. Embed [NoopHook] to override only
// the callbacks you care about, and wrap a hook in [ReplaceHook] to coalesce
// adjacent delete+insert into replace operations.
//
// # Text diffing
//
// For text, the higher-level [TextDiff] tokenizes two strings and exposes the
// result as tagged [Change] values:
//
//	diff := similar.DiffLines("a\nb\nc", "a\nb\nC")
//	for c := range diff.AllChanges() {
//		fmt.Printf("%s%s", c.Tag(), c)
//	}
//
// How the strings are cut up is a [Tokenizer], passed to [DiffText]: [Lines],
// [Words], [Chars], and [LinesAndNewlines] are the ones shipped here, and a
// caller can implement its own. [DiffLines], [DiffWords], and [DiffChars] are
// conveniences over the first three; [DiffSlices] takes tokens you made
// yourself.
//
// AllChanges and Changes return an iter.Seq[Change] so changes stream lazily and
// early-exit cheaply on large inputs; use slices.Collect to gather them. A
// TextDiff also reports a similarity [TextDiff.Ratio], its raw [TextDiff.Ops],
// and [TextDiff.GroupedOps] clusters. [TextDiff.AllRemappedChanges] maps word or
// character diffs back onto connected runs of the original strings — a TextDiff
// knows its own source text, so the strings need not be passed again — and
// [GetCloseMatches] finds the closest matches to a word from a candidate list.
package similar
