package similar

import (
	"context"

	"github.com/mahibulhaque/similar/internal/diff"
	"github.com/mahibulhaque/similar/internal/text"
)

// The text-diffing layer is re-exported here so callers need only import the
// root package. The canonical definitions live in internal/text (and, for the
// shared vocabulary, internal/diff).
type (
	// TextDiff is a captured text diff over string tokens. See [text.TextDiff].
	TextDiff = text.TextDiff
	// Change is an expanded diff operation: a tagged value with its old/new
	// position. See [diff.Change].
	Change = diff.Change
	// ChangeTag identifies the kind of a Change (equal/delete/insert).
	ChangeTag = diff.ChangeTag
	// Option configures a TextDiff construction.
	Option = text.Option
	// TextDiffRemapper maps token-level ops back onto the original strings.
	TextDiffRemapper = text.TextDiffRemapper
	// RemappedChange is a tagged run of the original text.
	RemappedChange = text.RemappedChange
)

// ChangeTag values.
const (
	ChangeEqual  = diff.ChangeEqual
	ChangeDelete = diff.ChangeDelete
	ChangeInsert = diff.ChangeInsert
)

// DiffLines diffs old and new split into lines (newlines attached).
func DiffLines(old, new string, opts ...Option) *TextDiff {
	return text.DiffLines(old, new, opts...)
}

// DiffWords diffs old and new split into words.
func DiffWords(old, new string, opts ...Option) *TextDiff {
	return text.DiffWords(old, new, opts...)
}

// DiffChars diffs old and new split into characters.
func DiffChars(old, new string, opts ...Option) *TextDiff {
	return text.DiffChars(old, new, opts...)
}

// DiffSlices diffs two already-tokenized string slices.
func DiffSlices(old, new []string, opts ...Option) *TextDiff {
	return text.DiffSlices(old, new, opts...)
}

// WithContext sets the context whose deadline and cancellation bound the diff.
func WithContext(ctx context.Context) Option { return text.WithContext(ctx) }

// WithAlgorithm selects the diff algorithm (Myers is the only value for now).
func WithAlgorithm(alg Algorithm) Option { return text.WithAlgorithm(alg) }

// WithNewlineTerminated forces the newline-terminated flag.
func WithNewlineTerminated(yes bool) Option { return text.WithNewlineTerminated(yes) }

// NewTextDiffRemapper builds a remapper from a diff and its original strings.
func NewTextDiffRemapper(d *TextDiff, old, new string) *TextDiffRemapper {
	return text.NewTextDiffRemapper(d, old, new)
}

// NewTextDiffRemapperFromTokens builds a remapper from token slices and strings.
func NewTextDiffRemapperFromTokens(oldTokens, newTokens []string, old, new string) *TextDiffRemapper {
	return text.NewTextDiffRemapperFromTokens(oldTokens, newTokens, old, new)
}

// GetCloseMatches returns up to n possibilities most similar to word.
func GetCloseMatches(word string, possibilities []string, n int, cutoff float64) []string {
	return text.GetCloseMatches(word, possibilities, n, cutoff)
}

// GroupDiffOps isolates change clusters with n items of context.
func GroupDiffOps(ops []DiffOp, n int) [][]DiffOp { return diff.GroupDiffOps(ops, n) }

// DiffRatio returns the similarity of two sequences from their ops and lengths.
func DiffRatio(ops []DiffOp, oldLen, newLen int) float64 {
	return diff.DiffRatio(ops, oldLen, newLen)
}
