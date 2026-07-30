// Package text is the text-diffing convenience layer built on top of the diff
// vocabulary and the Myers algorithm. It tokenizes strings, diffs the tokens,
// and exposes the result as tagged changes, a similarity ratio, grouped ops, a
// remapper onto the original strings, and a difflib-style close-match finder.
//
// The functions in this file are the splitting halves of the Tokenizer values
// declared in tokenizer.go; callers reach them through those values, or supply
// a Tokenizer of their own.
//
// It operates on concrete Go strings (tokens are []string); the Rust crate's
// DiffableStr trait machinery is unnecessary because Go's string already covers
// its use.
package text

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
