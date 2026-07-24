package text

import (
	"iter"

	"github.com/mahibulhaque/similar/internal/algorithms"
	"github.com/mahibulhaque/similar/internal/diff"
)

// TextDiff is a captured text diff: the tokenized old and new sides plus the
// diff ops between them. Construct it with DiffLines, DiffWords, DiffChars, or
// DiffSlices.
type TextDiff struct {
	old               []string
	new               []string
	ops               []diff.DiffOp
	newlineTerminated bool
	algorithm         algorithms.Algorithm
}

// DiffLines diffs old and new split into lines (newlines attached). The
// newline-terminated flag defaults to true for line diffs.
func DiffLines(old, new string, opts ...Option) *TextDiff {
	return build(tokenizeLines(old), tokenizeLines(new), true, opts)
}

// DiffWords diffs old and new split into words (whitespace runs and
// non-whitespace runs).
func DiffWords(old, new string, opts ...Option) *TextDiff {
	return build(tokenizeWords(old), tokenizeWords(new), false, opts)
}

// DiffChars diffs old and new split into characters (rune boundaries).
func DiffChars(old, new string, opts ...Option) *TextDiff {
	return build(tokenizeChars(old), tokenizeChars(new), false, opts)
}

// DiffSlices diffs two already-tokenized slices. The slices are copied, so the
// caller may reuse them afterwards.
func DiffSlices(old, new []string, opts ...Option) *TextDiff {
	return build(cloneStrings(old), cloneStrings(new), false, opts)
}

func build(oldToks, newToks []string, newlineDefault bool, opts []Option) *TextDiff {
	c := resolve(opts)
	newline := newlineDefault
	if c.newlineTerminated != nil {
		newline = *c.newlineTerminated
	}
	return &TextDiff{
		old:               oldToks,
		new:               newToks,
		ops:               captureOps(c, oldToks, newToks),
		newlineTerminated: newline,
		algorithm:         c.algorithm,
	}
}

// captureOps runs the algorithm over the tokens through the
// Compact(Replace(Capture)) hook stack, matching the crate's
// capture_diff_deadline. The only algorithm in this release is Myers; the hook
// stack never errors, so a returned error (only possible from a failing hook)
// is not expected here.
func captureOps(c config, old, new []string) []diff.DiffOp {
	capture := diff.NewCapture()
	hook := diff.NewCompact[string](diff.NewReplace(capture), old, new)
	_ = algorithms.DiffDeadline(c.ctx, hook, old, 0, len(old), new, 0, len(new))
	return capture.Ops()
}

// Algorithm returns the algorithm that produced the diff.
func (d *TextDiff) Algorithm() algorithms.Algorithm { return d.algorithm }

// NewlineTerminated reports whether tokens are treated as newline-terminated.
func (d *TextDiff) NewlineTerminated() bool { return d.newlineTerminated }

// OldLen returns the number of old-side tokens.
func (d *TextDiff) OldLen() int { return len(d.old) }

// NewLen returns the number of new-side tokens.
func (d *TextDiff) NewLen() int { return len(d.new) }

// OldToken returns the old-side token at index i and whether i is in range.
func (d *TextDiff) OldToken(i int) (string, bool) {
	if i < 0 || i >= len(d.old) {
		return "", false
	}
	return d.old[i], true
}

// NewToken returns the new-side token at index i and whether i is in range.
func (d *TextDiff) NewToken(i int) (string, bool) {
	if i < 0 || i >= len(d.new) {
		return "", false
	}
	return d.new[i], true
}

// OldTokens iterates the old-side tokens.
func (d *TextDiff) OldTokens() iter.Seq[string] { return sliceSeq(d.old) }

// NewTokens iterates the new-side tokens.
func (d *TextDiff) NewTokens() iter.Seq[string] { return sliceSeq(d.new) }

// Ratio returns the similarity of the two sides in the range [0,1].
func (d *TextDiff) Ratio() float64 {
	return diff.DiffRatio(d.ops, len(d.old), len(d.new))
}

// Ops returns the captured diff ops. The returned slice is owned by the
// TextDiff and must not be modified.
func (d *TextDiff) Ops() []diff.DiffOp { return d.ops }

// GroupedOps isolates change clusters with n items of surrounding context.
func (d *TextDiff) GroupedOps(n int) [][]diff.DiffOp {
	return diff.GroupDiffOps(d.ops, n)
}

func sliceSeq(s []string) iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, v := range s {
			if !yield(v) {
				return
			}
		}
	}
}

func cloneStrings(s []string) []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s...)
}
