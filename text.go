package similar

import (
	"iter"
	"maps"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
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
	ops := captureOps(c.ctx, c.algorithm, oldToks, newToks)

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

// Changes returns the changes a single op expands to. An Equal, Delete, or
// Insert maps directly; a Replace expands to all its deletes followed by all
// its inserts, so a Change only ever carries the equal/delete/insert tags.
//
// It panics if op holds indices out of range for this diff's tokens, matching
// the upstream crate.
func (d *TextDiff) Changes(op DiffOp) iter.Seq[Change] {
	return func(yield func(Change) bool) {
		d.emitChanges(op, yield)
	}
}

// AllChanges flattens every op into a single lazy stream of changes.
func (d *TextDiff) AllChanges() iter.Seq[Change] {
	return func(yield func(Change) bool) {
		for _, op := range d.ops {
			if !d.emitChanges(op, yield) {
				return
			}
		}
	}
}

// emitChanges yields the changes for op and reports whether iteration should
// continue (false once yield has asked to stop).
func (d *TextDiff) emitChanges(op DiffOp, yield func(Change) bool) bool {
	switch op.Tag {
	case Equal:
		for k := 0; k < op.OldLen; k++ {
			oi, ni := op.OldIndex+k, op.NewIndex+k
			if !yield(equalChange(d.old[oi], oi, ni)) {
				return false
			}
		}
	case Delete:
		for k := 0; k < op.OldLen; k++ {
			oi := op.OldIndex + k
			if !yield(deleteChange(d.old[oi], oi)) {
				return false
			}
		}
	case Insert:
		for k := 0; k < op.NewLen; k++ {
			ni := op.NewIndex + k
			if !yield(insertChange(d.new[ni], ni)) {
				return false
			}
		}
	case Replace:
		for k := 0; k < op.OldLen; k++ {
			oi := op.OldIndex + k
			if !yield(deleteChange(d.old[oi], oi)) {
				return false
			}
		}
		for k := 0; k < op.NewLen; k++ {
			ni := op.NewIndex + k
			if !yield(insertChange(d.new[ni], ni)) {
				return false
			}
		}
	}
	return true
}

// RemappedChange is a tagged run of the original text: the value is a connected
// substring of the old or new input, not a single token.
type RemappedChange struct {
	Tag   ChangeTag
	Value string
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

// sideRemapper reconstructs one side's source text from its tokens and maps a
// half-open token range onto a substring of it.
//
// The tokens are the only source of truth. Every tokenizer here partitions its
// input, so joining them reproduces the text the diff was built from and no
// caller-supplied string can disagree with the token offsets.
//
// starts has one more entry than there are tokens: starts[i] is token i's byte
// offset and starts[len(tokens)] is the total length, so mapping a range needs
// no second table.
type sideRemapper struct {
	source string
	starts []int
}

// newSideRemapper joins the tokens and records where each one begins.
func newSideRemapper(tokens []string) sideRemapper {
	var b strings.Builder
	starts := make([]int, len(tokens)+1)
	for i, t := range tokens {
		starts[i] = b.Len()
		b.WriteString(t)
	}
	starts[len(tokens)] = b.Len()
	return sideRemapper{source: b.String(), starts: starts}
}

// slice returns the source substring covered by token indices [start, end) and
// whether the range is valid. An empty range is valid and yields "". The result
// shares the source's backing array, so no bytes are copied.
func (r sideRemapper) slice(start, end int) (string, bool) {
	if start < 0 || end < start || end >= len(r.starts) {
		return "", false
	}
	return r.source[r.starts[start]:r.starts[end]], true
}

// GetCloseMatches returns up to n of the possibilities whose character-level
// similarity to word is at least cutoff, ordered by descending similarity with
// a lexicographic tie-break. It mirrors Python's difflib.get_close_matches.
//
// Candidates are cheaply prefiltered with an upper-bound ratio before the full
// character diff is run, so obvious non-matches are skipped.
func GetCloseMatches(word string, possibilities []string, n int, cutoff float64) []string {
	seq1 := tokenizeChars(word)
	quick := newQuickSeqRatio(seq1)

	type scored struct {
		ratio float64
		word  string
	}
	var matches []scored
	for _, p := range possibilities {
		// The length bound needs a count, not a token slice, so it runs before
		// the candidate is tokenized: a candidate rejected on length alone
		// costs one pass over its bytes and no allocation.
		if upperLenRatio(len(seq1), utf8.RuneCountInString(p)) < cutoff {
			continue
		}
		seq2 := tokenizeChars(p)
		if quick.calc(seq2) < cutoff {
			continue
		}
		ratio := DiffSlices(seq1, seq2, PlainTokens).Ratio()
		if ratio >= cutoff {
			matches = append(matches, scored{ratio: ratio, word: p})
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].ratio != matches[j].ratio {
			return matches[i].ratio > matches[j].ratio
		}
		return matches[i].word < matches[j].word
	})

	if n < 0 {
		n = 0
	}
	rv := make([]string, 0, min(n, len(matches)))
	for i := 0; i < len(matches) && i < n; i++ {
		rv = append(rv, matches[i].word)
	}
	return rv
}

// upperLenRatio is a cheap upper bound on the similarity of two token
// sequences: 2*min(len)/(len1+len2), or 1.0 when both are empty. Even a perfect
// match of every token on the shorter side cannot beat it, so a candidate below
// the cutoff here cannot reach it and is discarded before any diff runs.
//
// It takes lengths rather than the sequences themselves because the caller may
// not have tokenized the candidate yet — for text that is a rune count, which
// costs no allocation.
func upperLenRatio(len1, len2 int) float64 {
	n := len1 + len2
	if n == 0 {
		return 1.0
	}
	min := len1
	if len2 < min {
		min = len2
	}
	return 2.0 * float64(min) / float64(n)
}

// quickSeqRatio computes an order-independent upper-bound ratio by treating the
// sequences as multisets, following Python's difflib. Because Go strings are
// comparable, counting uses a plain map rather than a hashed bucket table.
//
// calc needs to spend the counts as it walks a candidate, so it works on a
// scratch copy. That copy is a field rather than a fresh map per call: one
// GetCloseMatches spends it once per candidate, and cloning the map each time
// was the single largest allocation in that loop. calc is therefore not safe
// for concurrent use on one quickSeqRatio — nothing shares one, since
// newQuickSeqRatio is called per GetCloseMatches.
type quickSeqRatio struct {
	counts  map[string]int
	unique  int
	scratch map[string]int
}

func newQuickSeqRatio(seq []string) quickSeqRatio {
	counts := make(map[string]int, len(seq))
	unique := 0
	for _, w := range seq {
		if counts[w] == 0 {
			unique++
		}
		counts[w]++
	}
	return quickSeqRatio{
		counts:  counts,
		unique:  unique,
		scratch: make(map[string]int, len(counts)),
	}
}

// calc returns the multiset match ratio of seq against the sequence this ratio
// was built from. It overwrites the scratch map, so the result must be read
// before calling it again.
func (q quickSeqRatio) calc(seq []string) float64 {
	n := q.unique + len(seq)
	if n == 0 {
		return 1.0
	}
	avail := q.scratch
	if avail == nil {
		// A zero-value quickSeqRatio has no scratch map to reuse.
		avail = make(map[string]int, len(q.counts))
	}
	clear(avail)
	maps.Copy(avail, q.counts)
	matches := 0
	for _, w := range seq {
		if avail[w] > 0 {
			matches++
		}
		avail[w]--
	}
	return 2.0 * float64(matches) / float64(n)
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
