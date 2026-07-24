package text

import "github.com/mahibulhaque/similar/internal/diff"

// RemappedChange is a tagged run of the original text: the value is a connected
// substring of the old or new input, not a single token.
type RemappedChange struct {
	Tag   diff.ChangeTag
	Value string
}

// sliceRemapper maps a half-open token-index range back to the byte range it
// covers in the original source string.
type sliceRemapper struct {
	source string
	starts []int
	ends   []int
}

// newSliceRemapper builds a remapper from the source string and the byte length
// of each successive token.
func newSliceRemapper(source string, lengths []int) sliceRemapper {
	starts := make([]int, len(lengths))
	ends := make([]int, len(lengths))
	offset := 0
	for i, l := range lengths {
		starts[i] = offset
		offset += l
		ends[i] = offset
	}
	return sliceRemapper{source: source, starts: starts, ends: ends}
}

// slice returns the source substring covered by token indices [start, end) and
// whether the range is valid.
func (r sliceRemapper) slice(start, end int) (string, bool) {
	if start < 0 || start >= len(r.starts) {
		return "", false
	}
	if end-1 < 0 || end-1 >= len(r.ends) {
		return "", false
	}
	return r.source[r.starts[start]:r.ends[end-1]], true
}

// TextDiffRemapper maps a TextDiff's token-level ops back onto connected runs of
// the original old and new strings. It is useful for word or character diffs
// where the tokens are tiny but you want large consecutive substrings.
type TextDiffRemapper struct {
	old sliceRemapper
	new sliceRemapper
}

// NewTextDiffRemapper builds a remapper from a diff and the original strings the
// diff was created from.
func NewTextDiffRemapper(d *TextDiff, old, new string) *TextDiffRemapper {
	return &TextDiffRemapper{
		old: newSliceRemapper(old, tokenLengths(d.old)),
		new: newSliceRemapper(new, tokenLengths(d.new)),
	}
}

// NewTextDiffRemapperFromTokens builds a remapper directly from token slices and
// the original strings, for callers who tokenized themselves.
func NewTextDiffRemapperFromTokens(oldTokens, newTokens []string, old, new string) *TextDiffRemapper {
	return &TextDiffRemapper{
		old: newSliceRemapper(old, tokenLengths(oldTokens)),
		new: newSliceRemapper(new, tokenLengths(newTokens)),
	}
}

// SliceOld returns the old-string substring covered by token indices
// [start, end) and whether the range is valid.
func (r *TextDiffRemapper) SliceOld(start, end int) (string, bool) {
	return r.old.slice(start, end)
}

// SliceNew returns the new-string substring covered by token indices
// [start, end) and whether the range is valid.
func (r *TextDiffRemapper) SliceNew(start, end int) (string, bool) {
	return r.new.slice(start, end)
}

// IterSlices returns the change(s) an op encodes against the original strings.
// A Replace yields a delete run followed by an insert run.
//
// It panics if op's ranges are out of bounds for the strings passed to the
// constructor, matching the upstream crate.
func (r *TextDiffRemapper) IterSlices(op diff.DiffOp) []RemappedChange {
	switch op.Tag {
	case diff.Equal:
		return []RemappedChange{{diff.ChangeEqual, r.mustOld(op.OldIndex, op.OldIndex+op.OldLen)}}
	case diff.Delete:
		return []RemappedChange{{diff.ChangeDelete, r.mustOld(op.OldIndex, op.OldIndex+op.OldLen)}}
	case diff.Insert:
		return []RemappedChange{{diff.ChangeInsert, r.mustNew(op.NewIndex, op.NewIndex+op.NewLen)}}
	case diff.Replace:
		return []RemappedChange{
			{diff.ChangeDelete, r.mustOld(op.OldIndex, op.OldIndex+op.OldLen)},
			{diff.ChangeInsert, r.mustNew(op.NewIndex, op.NewIndex+op.NewLen)},
		}
	default:
		return nil
	}
}

func (r *TextDiffRemapper) mustOld(start, end int) string {
	s, ok := r.old.slice(start, end)
	if !ok {
		panic("text: remapper old slice out of bounds")
	}
	return s
}

func (r *TextDiffRemapper) mustNew(start, end int) string {
	s, ok := r.new.slice(start, end)
	if !ok {
		panic("text: remapper new slice out of bounds")
	}
	return s
}

func tokenLengths(tokens []string) []int {
	lengths := make([]int, len(tokens))
	for i, t := range tokens {
		lengths[i] = len(t)
	}
	return lengths
}
