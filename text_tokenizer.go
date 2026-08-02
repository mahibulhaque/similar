package similar

import (
	"unicode"
	"unicode/utf8"
)

// tokenizeLines splits s into lines with their trailing newline attached,
// handling "\r", "\n", and "\r\n" terminators. A trailing line without a
// newline is included as-is. Newline bytes are ASCII, so scanning by byte never
// splits a multi-byte rune.
func tokenizeLines(s string) []string {
	var lines []string
	lastPos := 0
	for i := 0; i < len(s); {
		switch s[i] {
		case '\r':
			if i+1 < len(s) && s[i+1] == '\n' {
				i += 2
			} else {
				i++
			}
			lines = append(lines, s[lastPos:i])
			lastPos = i
		case '\n':
			i++
			lines = append(lines, s[lastPos:i])
			lastPos = i
		default:
			i++
		}
	}
	if lastPos < len(s) {
		lines = append(lines, s[lastPos:])
	}
	return lines
}

// tokenizeLinesAndNewlines splits s into alternating runs of newline characters
// and non-newline characters, keeping each run as its own token.
func tokenizeLinesAndNewlines(s string) []string {
	var rv []string
	for i := 0; i < len(s); {
		isNewline := s[i] == '\r' || s[i] == '\n'
		start := i
		i++
		for i < len(s) && (s[i] == '\r' || s[i] == '\n') == isNewline {
			i++
		}
		rv = append(rv, s[start:i])
	}
	return rv
}

// tokenizeWords splits s into alternating runs of whitespace and non-whitespace,
// using the Unicode White_Space property (via unicode.IsSpace) to match the
// upstream crate's char.is_whitespace behavior.
func tokenizeWords(s string) []string {
	var rv []string
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		isSpace := unicode.IsSpace(r)
		start := i
		i += size
		for i < len(s) {
			r2, sz := utf8.DecodeRuneInString(s[i:])
			if unicode.IsSpace(r2) != isSpace {
				break
			}
			i += sz
		}
		rv = append(rv, s[start:i])
	}
	return rv
}

// tokenizeChars splits s into individual characters on rune boundaries, so a
// multi-byte rune is never split.
func tokenizeChars(s string) []string {
	var rv []string
	for i := 0; i < len(s); {
		_, size := utf8.DecodeRuneInString(s[i:])
		rv = append(rv, s[i:i+size])
		i += size
	}
	return rv
}

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

	// NewlineTerminated reports whether a diff built with this tokenizer has
	// newline-terminated tokens. The flag controls how downstream renderers
	// treat trailing newlines; wrap a tokenizer in NewlineTerminated to change
	// the answer without changing how it splits.
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

// NewlineTerminated returns tok with its newline-terminated answer replaced by
// yes. Splitting is unchanged: the returned tokenizer defers to tok for that.
//
// The flag is a property of the tokens, so it belongs to whatever produced
// them. Use this to state it for a tokenizer whose default does not suit:
//
//	similar.DiffText(old, new, similar.NewlineTerminated(similar.Words, true))
//
// It panics if tok is nil, matching DiffText, which cannot report the error.
func NewlineTerminated(tok Tokenizer, yes bool) Tokenizer {
	if tok == nil {
		panic("similar: nil tokenizer")
	}
	return newlineTerminated{tok: tok, yes: yes}
}

// newlineTerminated overrides only the newline-terminated answer of the
// tokenizer it wraps.
type newlineTerminated struct {
	tok Tokenizer
	yes bool
}

func (n newlineTerminated) Split(s string) []string { return n.tok.Split(s) }

func (n newlineTerminated) NewlineTerminated() bool { return n.yes }

var _ Tokenizer = newlineTerminated{}
