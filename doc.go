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
// # Streaming with a hook
//
// Advanced users can implement [DiffHook] and receive callbacks as the diff is
// produced, avoiding a materialized slice. Embed [NoopHook] to override only
// the callbacks you care about, and wrap a hook in [ReplaceHook] to coalesce
// adjacent delete+insert into replace operations.
//
// # Text diffing
//
// For text, the higher-level [TextDiff] tokenizes two strings by line, word, or
// character and exposes the result as tagged [Change] values:
//
//	diff := similar.DiffLines("a\nb\nc", "a\nb\nC")
//	for c := range diff.AllChanges() {
//		fmt.Printf("%s%s", c.Tag(), c)
//	}
//
// AllChanges and Changes return an iter.Seq[Change] so changes stream lazily and
// early-exit cheaply on large inputs; use slices.Collect to gather them. A
// TextDiff also reports a similarity [TextDiff.Ratio], its raw [TextDiff.Ops],
// and [TextDiff.GroupedOps] clusters. [NewTextDiffRemapper] maps word or
// character diffs back onto connected runs of the original strings, and
// [GetCloseMatches] finds the closest matches to a word from a candidate list.
package similar
