package similar

import (
	"context"
	"fmt"

	"github.com/mahibulhaque/similar/internal/algorithms"
	"github.com/mahibulhaque/similar/internal/engine"
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
//
// It panics if alg names no known algorithm: the signature has no error to
// return, and a bad Algorithm value is a programming error rather than a
// runtime condition. Use DiffDeadline to receive that as an error instead.
func CaptureDiff[T comparable](alg Algorithm, old, new []T) []DiffOp {
	mustBeKnown(alg)
	ops, _ := DiffDeadline(context.Background(), alg, old, new)
	return ops
}

// mustBeKnown panics unless alg names an algorithm this release implements. It
// guards the entry points that cannot report the failure as an error.
func mustBeKnown(alg Algorithm) {
	if !alg.Valid() {
		panic(fmt.Sprintf("similar: unknown algorithm %d", int(alg)))
	}
}

// DiffDeadline computes the diff honoring ctx's deadline and cancellation. If
// the deadline is hit the returned script is valid but may be approximate; the
// error is non-nil only if the algorithm or a hook fails.
func DiffDeadline[T comparable](ctx context.Context, alg Algorithm, old, new []T) ([]DiffOp, error) {
	return engine.Capture(ctx, alg, old, new)
}

// DiffHookDeadline streams operations to a custom DiffHook instead of
// collecting them, honoring ctx's deadline and cancellation.
func DiffHookDeadline[T comparable](ctx context.Context, alg Algorithm, hook DiffHook, old, new []T) error {
	return engine.Run(ctx, alg, hook, old, 0, len(old), new, 0, len(new))
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
	return engine.Run(ctx, alg, hook, old, oldStart, oldEnd, new, newStart, newEnd)
}
