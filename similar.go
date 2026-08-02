package similar

import (
	"context"
	"fmt"
)

// Diff computes the diff between old and new and returns all operations.
//
// With no options it uses Myers' algorithm under a background context, so it
// never expires and the script is the exact minimal one. Pass WithContext to
// bound it: on a deadline or cancellation the script is still valid, but may be
// approximate. That is not an error, and neither is anything else this can
// encounter — WithAlgorithm rejects an unusable algorithm where it is passed,
// and the hooks Diff assembles cannot fail — which is why it returns no error.
//
// Use DiffTo to stream operations to a hook instead of collecting them.
func Diff[T comparable](old, new []T, opts ...Option) []DiffOp {
	c := resolve(opts)
	ops, err := captureOps(c.ctx, c.algorithm, old, new)
	if err != nil {
		// Unreachable: WithAlgorithm is the only way to set one and it
		// validates on the spot.
		panic("similar: " + err.Error())
	}
	return ops
}

// DiffTo streams operations to hook instead of collecting them, and returns the
// first error a hook callback reports, which aborts the diff.
//
// It takes the hook as an argument rather than an option because the hook is
// what makes this a different function: with one, there is no []DiffOp to hand
// back. The same goes for the sub-range in DiffRangeTo.
func DiffTo[T comparable](hook DiffHook, old, new []T, opts ...Option) error {
	return DiffRangeTo(hook, old, 0, len(old), new, 0, len(new), opts...)
}

// DiffRangeTo is DiffTo over sub-ranges of old and new, so a window can be
// diffed without copying slices. Indices reported to hook are absolute — that
// is what distinguishes this from slicing the inputs and calling DiffTo.
func DiffRangeTo[T comparable](
	hook DiffHook,
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
	opts ...Option,
) error {
	c := resolve(opts)
	return run(c.ctx, c.algorithm, hook, old, oldStart, oldEnd, new, newStart, newEnd)
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
	return captureOps(ctx, alg, old, new)
}

// DiffHookDeadline streams operations to a custom DiffHook instead of
// collecting them, honoring ctx's deadline and cancellation.
func DiffHookDeadline[T comparable](ctx context.Context, alg Algorithm, hook DiffHook, old, new []T) error {
	return run(ctx, alg, hook, old, 0, len(old), new, 0, len(new))
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
	return run(ctx, alg, hook, old, oldStart, oldEnd, new, newStart, newEnd)
}
