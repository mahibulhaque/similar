// This file holds the whole configuration surface: the Option type and the two
// options themselves, plus the Algorithm value they select from. Algorithm lives
// here because WithAlgorithm is the single gate that validates one; similar.go
// holds the switch that turns a value into an implementation.

package similar

import (
	"context"
	"fmt"
)

// Option configures a diff. Options are applied in order; a later option
// overrides an earlier one.
//
// The set is deliberately small, and every option applies to every entry point
// in this package — the sequence diffs as much as the text ones. Anything that
// suits only one of them is a parameter there rather than an option here: a
// hook and a sub-range change what Diff returns, and whether tokens are
// newline-terminated is a property of the tokenizer that produced them.
type Option func(*config)

// config holds resolved construction settings.
type config struct {
	ctx       context.Context
	algorithm Algorithm
}

// WithContext sets the context whose deadline and cancellation bound the diff.
// The default is context.Background() (no deadline).
func WithContext(ctx context.Context) Option {
	return func(c *config) { c.ctx = ctx }
}

// WithAlgorithm selects the diff algorithm. The default is Myers.
//
// It panics if alg names no known algorithm. This is the single point at which
// an Algorithm value is checked, and it is checked when the option is applied
// rather than when the diff runs, so that entry points returning no error
// cannot be handed one they could not report.
func WithAlgorithm(alg Algorithm) Option {
	if !alg.valid() {
		panic(fmt.Sprintf("similar: unknown algorithm %d", int(alg)))
	}
	return func(c *config) { c.algorithm = alg }
}

func resolve(opts []Option) config {
	c := config{ctx: context.Background(), algorithm: Myers}
	for _, o := range opts {
		if o != nil {
			o(&c)
		}
	}
	if c.ctx == nil {
		c.ctx = context.Background()
	}
	return c
}

// Algorithm selects the diff algorithm. Both values produce a minimal edit
// script; they differ in what they cost to get there.
//
// similar.go holds the single switch that turns a value of this type into an
// implementation, so adding an algorithm means adding a constant here and a case
// there.
type Algorithm int

const (
	// Myers is Eugene W. Myers' shortest-edit-script algorithm. It is the
	// default, and the one to use on large inputs: its cost scales with the
	// number of differences rather than with the size of the input.
	Myers Algorithm = iota
	// LCS is the classic longest-common-subsequence table algorithm. It is
	// O(N*M) in both time and space, so it suits small inputs and comparisons
	// against other difflib-style implementations; past a few thousand tokens a
	// side it stops building the table and approximates the changed middle as
	// one replacement.
	LCS
)

// String returns the algorithm's name.
func (a Algorithm) String() string {
	switch a {
	case Myers:
		return "myers"
	case LCS:
		return "lcs"
	default:
		return fmt.Sprintf("Algorithm(%d)", int(a))
	}
}

// valid reports whether a names an algorithm this release implements.
//
// It is unexported because there is exactly one place an Algorithm can enter
// the package — WithAlgorithm — and that is the only caller. It was public
// while the rule was spread across the entry points that could report a bad
// value as an error and those that had to panic; with one gate, a caller has
// nothing to check that WithAlgorithm does not check for them.
func (a Algorithm) valid() bool {
	switch a {
	case Myers, LCS:
		return true
	default:
		return false
	}
}
