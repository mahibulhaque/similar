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
// It returns an error if alg names no known algorithm, or if a hook callback
// fails (which aborts the diff).
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
		return fmt.Errorf("similar: unknown algorithm %d", int(alg))
	}
}

// captureOps runs alg over the whole of old and new through the standard hook
// stack — Compact for semantic cleanup, Replace to coalesce adjacent
// delete+insert, Capture to accumulate — and returns the resulting operations.
//
// The hooks in that stack never fail, so a non-nil error means alg was unknown.
func captureOps[T comparable](
	ctx context.Context,
	alg Algorithm,
	old, new []T,
) ([]DiffOp, error) {
	capture := NewCapture()
	hook := newCompact[T](NewReplaceHook(capture), old, new)
	if err := run(ctx, alg, hook, old, 0, len(old), new, 0, len(new)); err != nil {
		return nil, err
	}
	return capture.Ops(), nil
}
