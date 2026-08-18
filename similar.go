package similar

import (
	"context"
	"fmt"

	"github.com/mahibulhaque/similar/internal/algorithms"
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
	return captureOps(c.ctx, c.algorithm, old, new)
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
		return algorithms.DiffMyersDeadline(ctx, hook, old, oldStart, oldEnd, new, newStart, newEnd)
	case LCS:
		return algorithms.DiffLCSDeadline(ctx, hook, old, oldStart, oldEnd, new, newStart, newEnd)
	case Patience:
		return algorithms.DiffPatienceDeadline(ctx, hook, old, oldStart, oldEnd, new, newStart, newEnd)
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

// captureRatio runs alg over the whole of old and new and returns the
// similarity ratio, without materializing a script.
//
// It deliberately skips the standard hook stack. A ratio reads only the total
// length of the Equal spans, and that total is what the stack leaves alone:
// ReplaceHook merges ops, and compact shifts hunk boundaries and merges
// neighbours, but neither creates nor destroys a matched item. So the number
// here equals DiffRatio over the compacted script — pinned by
// TestCaptureRatioMatchesTheStandardStack — for the cost of a counter.
//
// Like captureOps it returns no error, for the same reason: matchCounter cannot
// fail and run panics on an unknown algorithm.
func captureRatio[T comparable](
	ctx context.Context,
	alg Algorithm,
	old, new []T,
) float64 {
	counter := &matchCounter{}
	if err := run(ctx, alg, counter, old, 0, len(old), new, 0, len(new)); err != nil {
		panic("similar: match counter failed: " + err.Error())
	}
	return ratioFromMatches(counter.matches, len(old), len(new))
}
