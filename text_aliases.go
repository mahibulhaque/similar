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
	// Tokenizer splits a string into the tokens a diff compares. Implement it
	// to diff by a rule this package does not ship. See [text.Tokenizer].
	Tokenizer = text.Tokenizer
	// RemappedChange is a tagged run of the original text, as produced by
	// [TextDiff.RemappedChanges].
	RemappedChange = text.RemappedChange
)

// ChangeTag values.
const (
	ChangeEqual  = diff.ChangeEqual
	ChangeDelete = diff.ChangeDelete
	ChangeInsert = diff.ChangeInsert
)

// The tokenizers this package ships, for use with [DiffText]. Treat them as
// constants — they are variables only because an interface value cannot be one.
var (
	// Lines splits into lines with the trailing newline attached.
	Lines = text.Lines
	// Words splits into alternating whitespace and non-whitespace runs.
	Words = text.Words
	// Chars splits into characters on rune boundaries.
	Chars = text.Chars
	// LinesAndNewlines splits into alternating newline and non-newline runs,
	// keeping terminators as tokens of their own.
	LinesAndNewlines = text.LinesAndNewlines
)

// DiffText diffs old and new split by tok, which may be one of [Lines],
// [Words], [Chars], [LinesAndNewlines], or a [Tokenizer] of your own.
func DiffText(old, new string, tok Tokenizer, opts ...Option) *TextDiff {
	return text.DiffText(old, new, tok, opts...)
}

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
