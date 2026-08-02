package similar

import (
	"context"
	"fmt"

	"github.com/mahibulhaque/similar/internal/algorithms"
)

// This file runs a diff algorithm through the library's standard hook stack. It
// owns exactly two facts: which Algorithm value dispatches to which
// implementation, and that materializing a diff means Compact(Replace(Capture)).
//
// Both the raw sequence entry points in similar.go and the text layer's
// construction in text_diff.go go through it, so those two facts are stated once
// and adding an algorithm edits one switch.

// run dispatches the selected algorithm over the sub-ranges old[oldStart:oldEnd]
// and new[newStart:newEnd], streaming operations to hook. It honors ctx's
// deadline and cancellation: a deadline hit yields a valid but possibly
// approximate script and is not an error.
//
// The only error it returns is a hook callback's, which aborts the diff.
//
// It panics on an algorithm it does not know. WithAlgorithm is the only way to
// choose one and it rejects an unusable value where the caller passes it, so
// reaching this is a broken invariant rather than a runtime condition.
func run[T comparable](
	ctx context.Context,
	alg Algorithm,
	hook DiffHook,
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
) error {
	switch alg {
	case Myers:
		return algorithms.DiffDeadline(ctx, hook, old, oldStart, oldEnd, new, newStart, newEnd)
	default:
		panic(fmt.Sprintf("similar: unknown algorithm %d", int(alg)))
	}
}

// captureOps runs alg over the whole of old and new through the standard hook
// stack — Compact for semantic cleanup, Replace to coalesce adjacent
// delete+insert, Capture to accumulate — and returns the resulting operations.
//
// It returns no error because it can encounter none: the hooks in that stack
// never fail, and run panics rather than reporting an unknown algorithm. This
// is the single place that states that, so neither Diff nor the text layer's
// build has to handle an impossible one.
func captureOps[T comparable](
	ctx context.Context,
	alg Algorithm,
	old, new []T,
) []DiffOp {
	capture := NewCapture()
	hook := newCompact[T](NewReplaceHook(capture), old, new)
	if err := run(ctx, alg, hook, old, 0, len(old), new, 0, len(new)); err != nil {
		panic("similar: standard hook stack failed: " + err.Error())
	}
	return capture.Ops()
}
