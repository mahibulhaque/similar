package algorithms

import (
	"context"
	"time"
)

// deadline bundles the two bailout signals threaded through the hot loops: an
// absolute wall-clock time extracted once from the context, and the context
// itself for cancellation. A zero time means "no time limit"; a nil ctx means
// "no cancellation".
type deadline struct {
	time time.Time
	ctx  context.Context
}

// fromContext extracts a deadline from ctx. The context's Deadline (if any) is
// read once here and threaded as a plain time.Time through the recursion.
func fromContext(ctx context.Context) deadline {
	dl := deadline{ctx: ctx}
	if ctx != nil {
		if t, ok := ctx.Deadline(); ok {
			dl.time = t
		}
	}
	return dl
}

// exceeded reports whether the diff should bail: either the context has been
// cancelled or the wall-clock deadline has passed.
func (d deadline) exceeded() bool {
	if d.ctx != nil {
		select {
		case <-d.ctx.Done():
			return true
		default:
		}
	}
	return !d.time.IsZero() && !time.Now().Before(d.time)
}
