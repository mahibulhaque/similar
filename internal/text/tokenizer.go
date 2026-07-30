package text

// Tokenizer splits a string into the tokens a diff compares. It is the seam
// between "how text is cut up" and everything else the layer does with the
// pieces: Lines, Words, Chars, and LinesAndNewlines are the tokenizers this
// package ships, and a caller can pass its own to DiffText.
type Tokenizer interface {
	// Split returns s partitioned into tokens.
	//
	// Implementations should account for every byte of s. The remapping methods
	// reconstruct source text by joining the tokens, so if Split drops or
	// rewrites bytes, the joined tokens — not s — are what gets remapped.
	Split(s string) []string

	// NewlineTerminated reports the default for a diff's newline-terminated
	// flag when built with this tokenizer. WithNewlineTerminated overrides it.
	NewlineTerminated() bool
}

// tokenizer is the shape every shipped tokenizer takes: a split function plus
// the newline-terminated default that belongs to it.
type tokenizer struct {
	split             func(string) []string
	newlineTerminated bool
}

func (t tokenizer) Split(s string) []string { return t.split(s) }

func (t tokenizer) NewlineTerminated() bool { return t.newlineTerminated }

var _ Tokenizer = tokenizer{}

// The tokenizers this package ships. Treat them as constants — they are
// variables only because an interface value cannot be one.
var (
	// Lines splits into lines with their trailing newline attached, handling
	// "\r", "\n", and "\r\n". It is the only tokenizer whose tokens are
	// newline-terminated by default.
	Lines Tokenizer = tokenizer{tokenizeLines, true}

	// Words splits into alternating runs of whitespace and non-whitespace.
	Words Tokenizer = tokenizer{tokenizeWords, false}

	// Chars splits into individual characters on rune boundaries.
	Chars Tokenizer = tokenizer{tokenizeChars, false}

	// LinesAndNewlines splits into alternating runs of newline and non-newline
	// characters, so the line terminators are tokens of their own rather than
	// being attached to the preceding line.
	LinesAndNewlines Tokenizer = tokenizer{tokenizeLinesAndNewlines, false}
)
