package similar

import (
	"iter"
	"sync"
)

// TextDiff is a captured text diff: the tokenized old and new sides plus the
// diff ops between them. Construct it with DiffText and a Tokenizer, with the
// DiffLines, DiffWords, and DiffChars conveniences, or — for input that is
// already tokenized — with DiffSlices.
//
// A TextDiff also knows its own source text, reconstructed from its tokens, so
// SliceOld, SliceNew, and RemappedChanges map ops back onto connected runs of
// the original strings without being handed them again.
type TextDiff struct {
	old               []string
	new               []string
	ops               []DiffOp
	newlineTerminated bool
	algorithm         Algorithm

	remapOnce sync.Once
	remapOld  sideRemapper
	remapNew  sideRemapper
}

// DiffText diffs old and new split by tok. Pass one of Lines, Words, Chars, or
// LinesAndNewlines, or any Tokenizer of your own. The newline-terminated flag
// is what tok reports; wrap tok in NewlineTerminated to change it.
//
// It panics if tok is nil: this returns a *TextDiff and no error, so an
// unusable argument is rejected where the caller can see which one was wrong.
func DiffText(old, new string, tok Tokenizer, opts ...Option) *TextDiff {
	if tok == nil {
		panic("text: nil tokenizer")
	}
	return build(tok.Split(old), tok.Split(new), tok.NewlineTerminated(), opts)
}

// DiffLines diffs old and new split into lines (newlines attached). The
// newline-terminated flag defaults to true for line diffs.
func DiffLines(old, new string, opts ...Option) *TextDiff {
	return DiffText(old, new, Lines, opts...)
}

// DiffWords diffs old and new split into words (whitespace runs and
// non-whitespace runs).
func DiffWords(old, new string, opts ...Option) *TextDiff {
	return DiffText(old, new, Words, opts...)
}

// DiffChars diffs old and new split into characters (rune boundaries).
func DiffChars(old, new string, opts ...Option) *TextDiff {
	return DiffText(old, new, Chars, opts...)
}

// NewlinePolicy states whether a slice of tokens is newline-terminated. Every
// other entry point learns this from its Tokenizer; DiffSlices is handed tokens
// with no tokenizer attached, so only the caller can say.
type NewlinePolicy bool

const (
	// PlainTokens marks tokens that are not newline-terminated.
	PlainTokens NewlinePolicy = false
	// NewlineTerminatedTokens marks tokens that carry their trailing newline,
	// as the tokens Lines produces do.
	NewlineTerminatedTokens NewlinePolicy = true
)

// DiffSlices diffs two already-tokenized slices. The slices are copied, so the
// caller may reuse them afterwards.
//
// The remapping methods reconstruct their source text by joining these tokens,
// so if the tokens do not account for every byte of some original string, the
// joined tokens — not that string — are what gets remapped.
func DiffSlices(old, new []string, nl NewlinePolicy, opts ...Option) *TextDiff {
	return build(cloneStrings(old), cloneStrings(new), bool(nl), opts)
}

func build(oldToks, newToks []string, newlineTerminated bool, opts []Option) *TextDiff {
	c := resolve(opts)
	// WithAlgorithm rejects an unknown value when the option is applied, and the
	// standard hook stack never fails, so Capture cannot error here.
	ops, err := captureOps(c.ctx, c.algorithm, oldToks, newToks)
	if err != nil {
		panic("text: " + err.Error())
	}

	return &TextDiff{
		old:               oldToks,
		new:               newToks,
		ops:               ops,
		newlineTerminated: newlineTerminated,
		algorithm:         c.algorithm,
	}
}

// Algorithm returns the algorithm that produced the diff.
func (d *TextDiff) Algorithm() Algorithm { return d.algorithm }

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
	return DiffRatio(d.ops, len(d.old), len(d.new))
}

// Ops returns the captured diff ops. The returned slice is owned by the
// TextDiff and must not be modified.
func (d *TextDiff) Ops() []DiffOp { return d.ops }

// GroupedOps isolates change clusters with n items of surrounding context.
func (d *TextDiff) GroupedOps(n int) [][]DiffOp {
	return GroupDiffOps(d.ops, n)
}

// remap builds the byte-offset tables for both sides on first use.
//
// Building them costs O(bytes), and most diffs are never remapped — notably the
// ones GetCloseMatches creates per candidate, which are read only for a ratio —
// so the cost is deferred until a remapping method is actually called.
func (d *TextDiff) remap() (*sideRemapper, *sideRemapper) {
	d.remapOnce.Do(func() {
		d.remapOld = newSideRemapper(d.old)
		d.remapNew = newSideRemapper(d.new)
	})
	return &d.remapOld, &d.remapNew
}

// SliceOld returns the run of old-side text covered by token indices
// [start, end) and whether the range is valid. An empty range yields "".
func (d *TextDiff) SliceOld(start, end int) (string, bool) {
	old, _ := d.remap()
	return old.slice(start, end)
}

// SliceNew returns the run of new-side text covered by token indices
// [start, end) and whether the range is valid. An empty range yields "".
func (d *TextDiff) SliceNew(start, end int) (string, bool) {
	_, new := d.remap()
	return new.slice(start, end)
}

// RemappedChanges returns the runs of original text an op encodes. Unlike
// Changes, which yields one change per token, this yields one per connected
// run — useful for word or character diffs where the tokens are tiny. A Replace
// yields a delete run followed by an insert run.
//
// It panics if op holds indices out of range for this diff's tokens, matching
// the upstream crate.
func (d *TextDiff) RemappedChanges(op DiffOp) []RemappedChange {
	switch op.Tag {
	case Equal:
		return []RemappedChange{{ChangeEqual, d.mustOld(op.OldIndex, op.OldIndex+op.OldLen)}}
	case Delete:
		return []RemappedChange{{ChangeDelete, d.mustOld(op.OldIndex, op.OldIndex+op.OldLen)}}
	case Insert:
		return []RemappedChange{{ChangeInsert, d.mustNew(op.NewIndex, op.NewIndex+op.NewLen)}}
	case Replace:
		return []RemappedChange{
			{ChangeDelete, d.mustOld(op.OldIndex, op.OldIndex+op.OldLen)},
			{ChangeInsert, d.mustNew(op.NewIndex, op.NewIndex+op.NewLen)},
		}
	default:
		return nil
	}
}

// AllRemappedChanges flattens every op into a single lazy stream of runs,
// mirroring AllChanges.
func (d *TextDiff) AllRemappedChanges() iter.Seq[RemappedChange] {
	return func(yield func(RemappedChange) bool) {
		for _, op := range d.ops {
			for _, rc := range d.RemappedChanges(op) {
				if !yield(rc) {
					return
				}
			}
		}
	}
}

func (d *TextDiff) mustOld(start, end int) string {
	s, ok := d.SliceOld(start, end)
	if !ok {
		panic("text: remapped old slice out of bounds")
	}
	return s
}

func (d *TextDiff) mustNew(start, end int) string {
	s, ok := d.SliceNew(start, end)
	if !ok {
		panic("text: remapped new slice out of bounds")
	}
	return s
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
