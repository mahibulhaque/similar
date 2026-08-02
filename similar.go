package similar

// This file is the package's entry surface for diffing sequences: one function
// per return type. Diff collects operations into a slice; DiffTo streams them
// to a hook; DiffRangeTo does the same over a window with absolute indices.
// Everything that varies without changing the return type — the context and the
// algorithm — is an Option, shared with the text layer in options.go.

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
