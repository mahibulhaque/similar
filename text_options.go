package similar

import (
	"context"
	"fmt"
)

// Option configures a TextDiff construction. Options are applied in order; a
// later option overrides an earlier one.
type Option func(*config)

// config holds resolved construction settings. newlineTerminated is a pointer
// so "unset" (auto per diff kind) is distinguishable from an explicit false.
type config struct {
	ctx               context.Context
	algorithm         Algorithm
	newlineTerminated *bool
}

// WithContext sets the context whose deadline and cancellation bound the diff.
// The default is context.Background() (no deadline).
func WithContext(ctx context.Context) Option {
	return func(c *config) { c.ctx = ctx }
}

// WithAlgorithm selects the diff algorithm. The default (and only value in this
// release) is Myers.
//
// It panics if alg names no known algorithm. The diff constructors return a
// *TextDiff with no error, so an unusable value has to be rejected here, where
// the caller can see which argument was wrong.
func WithAlgorithm(alg Algorithm) Option {
	if !alg.Valid() {
		panic(fmt.Sprintf("text: unknown algorithm %d", int(alg)))
	}
	return func(c *config) { c.algorithm = alg }
}

// WithNewlineTerminated forces the newline-terminated flag, overriding the
// automatic default (true for line diffs, false otherwise). The flag controls
// how downstream renderers treat trailing newlines.
func WithNewlineTerminated(yes bool) Option {
	return func(c *config) { c.newlineTerminated = &yes }
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
