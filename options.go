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

// WithAlgorithm selects the diff algorithm. The default (and only value in this
// release) is Myers.
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
