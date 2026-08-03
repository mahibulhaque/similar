package algorithms

import (
	"context"
	"time"
)

// Default gate values. Each is the threshold at which a heuristic starts paying
// for itself; they are ported from the Rust crate and are not tuned here.
const (
	defaultSmallSideExactMax       = 64
	defaultSmallSideExactMinLarge  = 512
	defaultSmallSideExactMaxWork   = 64_000_000
	defaultDisjointFastPathMinLen  = 512
	defaultDisjointFastPathMinWork = 128 * 1024
	defaultFrontAnchorMaxSkip      = 4
	defaultFrontAnchorMinCommon    = 96
)

// defaultLCSTableMaxWork caps the cells of the LCS table at roughly 64 MiB of
// int32 — the same byte budget defaultSmallSideExactMaxWork gives its uint8
// table. It has no counterpart in the Rust crate, whose table is a map that
// simply grows; the LCS algorithm is O(N*M) in space, so without a cap a large
// enough input is an allocation failure rather than a slow diff. Over the cap
// the table is declined and the middle approximated, exactly as on a deadline.
const defaultLCSTableMaxWork = 16_000_000

// smallSideExactHardMax is the largest small side the exact fallback can
// handle: its dp table stores LCS lengths in a uint8, and the LCS of a range
// cannot exceed the length of the smaller side. The gate is clamped to this, so
// lowering smallSideExactMax is safe and raising it past the table's range
// cannot silently overflow.
const smallSideExactHardMax = 255

// How often the hot loops read the clock. These are not gates: they change how
// promptly a bailout is noticed, never what the algorithm decides, so there is
// nothing for a test to vary and they stay constants.
const (
	smallSideDeadlineCheckInterval    = 1024
	frontAnchorDeadlineCheckStep      = 1024
	disjointFastPathDeadlineCheckStep = 1024
)

// limits bundles everything that bounds one diff run, threaded as a single
// value through the recursion.
//
// Two of its fields are the bailout signals: an absolute wall-clock time
// extracted once from the context, and the context itself for cancellation. A
// zero time means no time limit; a nil ctx means no cancellation.
//
// The rest are the thresholds gating each heuristic. In production they are
// always the defaults above — fromContext is the only constructor, and nothing
// outside this package can name this type. They are fields rather than
// constants so that this package's tests can drive a gate's on/off edge
// directly, instead of having to build inputs large enough to cross a
// hard-coded 512.
type limits struct {
	time time.Time
	ctx  context.Context

	// smallSideExactMax is the largest small side the exact fallback accepts.
	smallSideExactMax int
	// smallSideExactMinLarge is the smallest large side that makes the exact
	// fallback worth running.
	smallSideExactMinLarge int
	// smallSideExactMaxWork caps oldLen*newLen for the exact fallback.
	smallSideExactMaxWork int
	// disjointFastPathMinLen is the shortest side the disjoint fast path
	// considers.
	disjointFastPathMinLen int
	// disjointFastPathMinWork is the oldLen*newLen below which full search is
	// cheap enough that the fast path's scan does not pay for itself.
	disjointFastPathMinWork int
	// frontAnchorMaxSkip is how many leading items the anchor scan may peel
	// from the larger side.
	frontAnchorMaxSkip int
	// frontAnchorMinCommon is the shortest common run that justifies anchoring,
	// and also the shortest range the scan will look at.
	frontAnchorMinCommon int
	// lcsTableMaxWork caps the cells — (oldLen+1)*(newLen+1) — of the LCS table.
	lcsTableMaxWork int
}

// fromContext builds the limits for one run: the production gates, plus ctx's
// deadline (if any) read once here and threaded as a plain time.Time through
// the recursion.
func fromContext(ctx context.Context) limits {
	lim := limits{
		ctx:                     ctx,
		smallSideExactMax:       defaultSmallSideExactMax,
		smallSideExactMinLarge:  defaultSmallSideExactMinLarge,
		smallSideExactMaxWork:   defaultSmallSideExactMaxWork,
		disjointFastPathMinLen:  defaultDisjointFastPathMinLen,
		disjointFastPathMinWork: defaultDisjointFastPathMinWork,
		frontAnchorMaxSkip:      defaultFrontAnchorMaxSkip,
		frontAnchorMinCommon:    defaultFrontAnchorMinCommon,
		lcsTableMaxWork:         defaultLCSTableMaxWork,
	}
	if ctx != nil {
		if t, ok := ctx.Deadline(); ok {
			lim.time = t
		}
	}
	return lim
}

// smallSideCap is smallSideExactMax clamped to what the dp table can represent.
func (l limits) smallSideCap() int {
	return min(l.smallSideExactMax, smallSideExactHardMax)
}

// exceeded reports whether the diff should bail: either the context has been
// cancelled or the wall-clock deadline has passed.
func (l limits) exceeded() bool {
	if l.ctx != nil {
		select {
		case <-l.ctx.Done():
			return true
		default:
		}
	}
	return !l.time.IsZero() && !time.Now().Before(l.time)
}
