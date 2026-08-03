// Package algorithms holds the diff implementations and the heuristics they
// share.
//
// It ports the Rust `similar` crate's algorithms behavior-for-behavior. Myers'
// is the default: classic divide-and-conquer middle-snake recursion,
// front-anchor peeling, exact small-side fallback (both directions), the
// disjoint fast path, and a deadline bailout. The LCS table diff is the second:
// one O(N*M) table over the trimmed middle, walked forward. Both share the
// disjoint fast path and the deadline plumbing; the rest of the heuristics are
// Myers'.
//
// Each algorithm exposes one pair of entry points, DiffX and DiffXDeadline. It
// is internal; the public facade lives in package similar.
package algorithms
