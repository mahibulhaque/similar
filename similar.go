package similar

import (
	"context"
	"fmt"

	"github.com/mahibulhaque/similar/internal/algorithms"
	"github.com/mahibulhaque/similar/internal/diff"
)

// Algorithm selects the diff algorithm. In v0.x the only value is Myers; the
// type exists so call sites stay stable as more algorithms ship.
//
// It is an alias for the type in internal/algorithms so that the text layer can
// reference the same type without an import cycle.
type Algorithm = algorithms.Algorithm

// Myers is Eugene W. Myers' shortest-edit-script algorithm.
const Myers = algorithms.Myers

// Diff computes the diff between old and new using Myers' algorithm and returns
// all operations. It never expires (background context) and so always produces
// the exact minimal script.
func Diff[T comparable](old, new []T) []DiffOp {
	ops, _ := DiffDeadline(context.Background(), Myers, old, new)
	return ops
}

// CaptureDiff computes the diff with the given algorithm and returns all
// operations, using a background context (no deadline).
func CaptureDiff[T comparable](alg Algorithm, old, new []T) []DiffOp {
	ops, _ := DiffDeadline(context.Background(), alg, old, new)
	return ops
}

// DiffDeadline computes the diff honoring ctx's deadline and cancellation. If
// the deadline is hit the returned script is valid but may be approximate; the
// error is non-nil only if the algorithm or a hook fails.
func DiffDeadline[T comparable](ctx context.Context, alg Algorithm, old, new []T) ([]DiffOp, error) {
	capture := diff.NewCapture()
	hook := diff.NewCompact[T](diff.NewReplace(capture), old, new)
	if err := runHook(ctx, alg, hook, old, 0, len(old), new, 0, len(new)); err != nil {
		return nil, err
	}
	return capture.Ops(), nil
}

// DiffHookDeadline streams operations to a custom DiffHook instead of
// collecting them, honoring ctx's deadline and cancellation.
func DiffHookDeadline[T comparable](ctx context.Context, alg Algorithm, hook DiffHook, old, new []T) error {
	return runHook(ctx, alg, hook, old, 0, len(old), new, 0, len(new))
}

// DiffRangeHookDeadline is like DiffHookDeadline but diffs sub-ranges of old
// and new, so windows can be diffed without copying slices.
func DiffRangeHookDeadline[T comparable](
	ctx context.Context,
	alg Algorithm,
	hook DiffHook,
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
) error {
	return runHook(ctx, alg, hook, old, oldStart, oldEnd, new, newStart, newEnd)
}

func runHook[T comparable](
	ctx context.Context,
	alg Algorithm,
	hook diff.DiffHook,
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
