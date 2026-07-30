// Package engine runs a diff algorithm through the library's standard hook
// stack. It owns exactly two facts: which Algorithm value dispatches to which
// implementation, and that materializing a diff means Compact(Replace(Capture)).
//
// Both the public facade and the text layer go through it, so those two facts
// are stated once. It sits above internal/diff and internal/algorithms and is
// imported by neither, so nothing here can create an import cycle.
package engine

import (
	"context"
	"fmt"

	"github.com/mahibulhaque/similar/internal/algorithms"
	"github.com/mahibulhaque/similar/internal/diff"
)

// Run dispatches the selected algorithm over the sub-ranges old[oldStart:oldEnd]
// and new[newStart:newEnd], streaming operations to hook. It honors ctx's
// deadline and cancellation: a deadline hit yields a valid but possibly
// approximate script and is not an error.
//
// It returns an error if alg names no known algorithm, or if a hook callback
// fails (which aborts the diff).
func Run[T comparable](
	ctx context.Context,
	alg algorithms.Algorithm,
	hook diff.DiffHook,
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
) error {
	switch alg {
	case algorithms.Myers:
		return algorithms.DiffDeadline(ctx, hook, old, oldStart, oldEnd, new, newStart, newEnd)
	default:
		return fmt.Errorf("similar: unknown algorithm %d", int(alg))
	}
}

// Capture runs alg over the whole of old and new through the standard hook
// stack — Compact for semantic cleanup, Replace to coalesce adjacent
// delete+insert, Capture to accumulate — and returns the resulting operations.
//
// The hooks in that stack never fail, so a non-nil error means alg was unknown.
func Capture[T comparable](
	ctx context.Context,
	alg algorithms.Algorithm,
	old, new []T,
) ([]diff.DiffOp, error) {
	capture := diff.NewCapture()
	hook := diff.NewCompact[T](diff.NewReplace(capture), old, new)
	if err := Run(ctx, alg, hook, old, 0, len(old), new, 0, len(new)); err != nil {
		return nil, err
	}
	return capture.Ops(), nil
}
