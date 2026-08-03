package similar

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDiffLinesOps(t *testing.T) {
	d := DiffLines("a\nb\nc", "a\nb\nC")
	want := []DiffOp{
		{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 2, NewLen: 2},
		{Tag: Replace, OldIndex: 2, NewIndex: 2, OldLen: 1, NewLen: 1},
	}
	if !reflect.DeepEqual(d.Ops(), want) {
		t.Fatalf("ops = %v, want %v", d.Ops(), want)
	}
	if !d.NewlineTerminated() {
		t.Errorf("line diff should be newline-terminated by default")
	}
}

func TestDiffWordsOps(t *testing.T) {
	d := DiffWords("foo bar baz", "foo BAR baz")
	want := []DiffOp{
		{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 2, NewLen: 2},
		{Tag: Replace, OldIndex: 2, NewIndex: 2, OldLen: 1, NewLen: 1},
		{Tag: Equal, OldIndex: 3, NewIndex: 3, OldLen: 2, NewLen: 2},
	}
	if !reflect.DeepEqual(d.Ops(), want) {
		t.Fatalf("ops = %v, want %v", d.Ops(), want)
	}
	if d.NewlineTerminated() {
		t.Errorf("word diff should not be newline-terminated by default")
	}
}

func TestDiffCharsOps(t *testing.T) {
	d := DiffChars("abcdef", "abcDDf")
	want := []DiffOp{
		{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 3, NewLen: 3},
		{Tag: Replace, OldIndex: 3, NewIndex: 3, OldLen: 2, NewLen: 2},
		{Tag: Equal, OldIndex: 5, NewIndex: 5, OldLen: 1, NewLen: 1},
	}
	if !reflect.DeepEqual(d.Ops(), want) {
		t.Fatalf("ops = %v, want %v", d.Ops(), want)
	}
}

func TestDiffSlicesOps(t *testing.T) {
	old := []string{"foo", "bar", "baz"}
	new := []string{"foo", "BAR", "baz"}
	d := DiffSlices(old, new, PlainTokens)
	want := []DiffOp{
		{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 1, NewLen: 1},
		{Tag: Replace, OldIndex: 1, NewIndex: 1, OldLen: 1, NewLen: 1},
		{Tag: Equal, OldIndex: 2, NewIndex: 2, OldLen: 1, NewLen: 1},
	}
	if !reflect.DeepEqual(d.Ops(), want) {
		t.Fatalf("ops = %v, want %v", d.Ops(), want)
	}
	// Mutating the caller's slice must not affect the diff.
	old[1] = "mutated"
	if tok, _ := d.OldToken(1); tok != "bar" {
		t.Errorf("DiffSlices did not copy input: OldToken(1) = %q, want %q", tok, "bar")
	}
}

func TestRatio(t *testing.T) {
	if got := DiffChars("abcd", "bcde").Ratio(); got != 0.75 {
		t.Errorf("Ratio = %v, want 0.75", got)
	}
	if got := DiffChars("", "").Ratio(); got != 1.0 {
		t.Errorf("empty Ratio = %v, want 1.0", got)
	}
}

func TestTokenAccessors(t *testing.T) {
	d := DiffLines("a\nb\nc", "a\nb\nC")
	if d.OldLen() != 3 || d.NewLen() != 3 {
		t.Fatalf("lens = (%d,%d), want (3,3)", d.OldLen(), d.NewLen())
	}
	if tok, ok := d.OldToken(0); !ok || tok != "a\n" {
		t.Errorf("OldToken(0) = (%q,%v), want (\"a\\n\",true)", tok, ok)
	}
	if _, ok := d.OldToken(3); ok {
		t.Errorf("OldToken(3) should be out of range")
	}
	got := slices.Collect(d.OldTokens())
	if want := []string{"a\n", "b\n", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("OldTokens = %q, want %q", got, want)
	}
}

func TestGroupedOpsFromTextDiff(t *testing.T) {
	// A diff with a long equal run should split into two groups.
	oldLines := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\nX\n"
	newLines := "A\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\nX\n"
	d := DiffLines(oldLines, newLines)
	groups := d.GroupedOps(1)
	if len(groups) == 0 {
		t.Fatalf("expected at least one group, got none (ops=%v)", d.Ops())
	}
}

// tagValue is a compact view of a change for comparison.
type tagValue struct {
	tag      ChangeTag
	value    string
	oldIndex int // -1 when absent
	newIndex int // -1 when absent
}

func view(c Change) tagValue {
	oi, ok := c.OldIndex()
	if !ok {
		oi = -1
	}
	ni, ok := c.NewIndex()
	if !ok {
		ni = -1
	}
	return tagValue{c.Tag(), c.Value(), oi, ni}
}

func viewAll(seq func(func(Change) bool)) []tagValue {
	var out []tagValue
	for c := range seq {
		out = append(out, view(c))
	}
	return out
}

func TestAllChangesLines(t *testing.T) {
	d := DiffLines("a\nb\nc", "a\nb\nC")
	got := viewAll(d.AllChanges())
	want := []tagValue{
		{ChangeEqual, "a\n", 0, 0},
		{ChangeEqual, "b\n", 1, 1},
		{ChangeDelete, "c", 2, -1},
		{ChangeInsert, "C", -1, 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changes = %+v, want %+v", got, want)
	}
}

func TestAllChangesCharsReplaceExpands(t *testing.T) {
	d := DiffChars("abcdef", "abcDDf")
	got := viewAll(d.AllChanges())
	want := []tagValue{
		{ChangeEqual, "a", 0, 0},
		{ChangeEqual, "b", 1, 1},
		{ChangeEqual, "c", 2, 2},
		{ChangeDelete, "d", 3, -1},
		{ChangeDelete, "e", 4, -1},
		{ChangeInsert, "D", -1, 3},
		{ChangeInsert, "D", -1, 4},
		{ChangeEqual, "f", 5, 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changes = %+v, want %+v", got, want)
	}
}

func TestChangesSingleOp(t *testing.T) {
	d := DiffChars("abcdef", "abcDDf")
	// The replace op is the second op.
	ops := d.Ops()
	var replace DiffOp
	found := false
	for _, op := range ops {
		if op.Tag == Replace {
			replace = op
			found = true
		}
	}
	if !found {
		t.Fatalf("no replace op in %v", ops)
	}
	got := viewAll(d.Changes(replace))
	want := []tagValue{
		{ChangeDelete, "d", 3, -1},
		{ChangeDelete, "e", 4, -1},
		{ChangeInsert, "D", -1, 3},
		{ChangeInsert, "D", -1, 4},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changes = %+v, want %+v", got, want)
	}
}

func TestAllChangesEarlyBreak(t *testing.T) {
	d := DiffChars("abcdef", "abcDDf")
	count := 0
	for range d.AllChanges() {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Fatalf("early break yielded %d changes, want 2", count)
	}
}

func TestAllChangesCollect(t *testing.T) {
	d := DiffChars("abcdef", "abcDDf")
	all := slices.Collect(d.AllChanges())
	if len(all) != 8 {
		t.Fatalf("collected %d changes, want 8", len(all))
	}
}

func TestRemappedChangesWordDiff(t *testing.T) {
	d := DiffWords("foo bar baz", "foo bor baz")

	got := slices.Collect(d.AllRemappedChanges())
	want := []RemappedChange{
		{ChangeEqual, "foo "},
		{ChangeDelete, "bar"},
		{ChangeInsert, "bor"},
		{ChangeEqual, " baz"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("remapped = %+v, want %+v", got, want)
	}
}

func TestRemappedChangesCharDiffConnectsRuns(t *testing.T) {
	d := DiffChars("foobarbaz", "fooBARbaz")

	got := slices.Collect(d.AllRemappedChanges())
	want := []RemappedChange{
		{ChangeEqual, "foo"},
		{ChangeDelete, "bar"},
		{ChangeInsert, "BAR"},
		{ChangeEqual, "baz"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("remapped = %+v, want %+v", got, want)
	}
}

// RemappedChanges is the per-op form of AllRemappedChanges; a Replace expands to
// a delete run followed by an insert run.
func TestRemappedChangesPerOp(t *testing.T) {
	d := DiffWords("foo bar baz", "foo bor baz")

	var got []RemappedChange
	for _, op := range d.Ops() {
		got = append(got, d.RemappedChanges(op)...)
	}
	if want := slices.Collect(d.AllRemappedChanges()); !reflect.DeepEqual(got, want) {
		t.Fatalf("per-op = %+v, want %+v", got, want)
	}
}

func TestAllRemappedChangesStopsEarly(t *testing.T) {
	d := DiffChars("foobarbaz", "fooBARbaz")

	count := 0
	for range d.AllRemappedChanges() {
		count++
		break
	}
	if count != 1 {
		t.Fatalf("yielded %d changes after break, want 1", count)
	}
}

func TestRemappedChangesPanicsOutOfBounds(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("RemappedChanges with an out-of-range op: want panic, got none")
		}
	}()
	d := DiffWords("foo", "foo")
	d.RemappedChanges(DiffOp{Tag: Equal, OldIndex: 0, NewIndex: 0, OldLen: 99, NewLen: 99})
}

func TestSliceBounds(t *testing.T) {
	d := DiffWords("foo bar baz", "foo bor baz")

	if s, ok := d.SliceOld(0, 2); !ok || s != "foo " {
		t.Errorf("SliceOld(0,2) = (%q,%v), want (\"foo \",true)", s, ok)
	}
	if s, ok := d.SliceNew(2, 3); !ok || s != "bor" {
		t.Errorf("SliceNew(2,3) = (%q,%v), want (\"bor\",true)", s, ok)
	}
	if s, ok := d.SliceOld(1, 1); !ok || s != "" {
		t.Errorf("SliceOld(1,1) = (%q,%v), want (\"\",true)", s, ok)
	}
	if _, ok := d.SliceOld(0, 100); ok {
		t.Error("SliceOld past the last token should report !ok")
	}
	if _, ok := d.SliceOld(-1, 1); ok {
		t.Error("SliceOld with a negative start should report !ok")
	}
	if _, ok := d.SliceOld(2, 1); ok {
		t.Error("SliceOld with end < start should report !ok")
	}
}

// A diff built from tokens the caller supplied remaps against those tokens, so
// no original string has to be threaded back in.
func TestSlicesFromCallerTokens(t *testing.T) {
	d := DiffSlices(tokenizeWords("foo bar baz"), tokenizeWords("foo bor baz"), PlainTokens)

	if s, ok := d.SliceNew(2, 3); !ok || s != "bor" {
		t.Errorf("SliceNew(2,3) = (%q,%v), want (\"bor\",true)", s, ok)
	}
	got := slices.Collect(d.AllRemappedChanges())
	want := []RemappedChange{
		{ChangeEqual, "foo "},
		{ChangeDelete, "bar"},
		{ChangeInsert, "bor"},
		{ChangeEqual, " baz"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("remapped = %+v, want %+v", got, want)
	}
}

// Joining the tokens reproduces the input byte for byte, so the source a remap
// slices against is the source the diff was built from.
func TestReconstructedSourceMatchesInput(t *testing.T) {
	inputs := []string{
		"foo bar baz",
		"a\nb\r\nc\rd",
		"héllo wörld",
		"",
		"   leading and trailing   ",
	}
	for _, in := range inputs {
		diffs := map[string]*TextDiff{
			"lines":            DiffLines(in, in),
			"words":            DiffWords(in, in),
			"chars":            DiffChars(in, in),
			"linesAndNewlines": DiffText(in, in, LinesAndNewlines),
		}
		for name, d := range diffs {
			old, new := d.remap()
			if old.source != in {
				t.Errorf("%s(%q): reconstructed old = %q", name, in, old.source)
			}
			if new.source != in {
				t.Errorf("%s(%q): reconstructed new = %q", name, in, new.source)
			}
		}
	}
}

func TestGetCloseMatches(t *testing.T) {
	got := GetCloseMatches("appel", []string{"ape", "apple", "peach", "puppy"}, 3, 0.6)
	want := []string{"apple", "ape"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCloseMatches = %v, want %v", got, want)
	}
}

func TestGetCloseMatchesRespectsN(t *testing.T) {
	got := GetCloseMatches("appel", []string{"ape", "apple", "peach", "puppy"}, 1, 0.6)
	want := []string{"apple"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCloseMatches(n=1) = %v, want %v", got, want)
	}
}

func TestGetCloseMatchesCutoff(t *testing.T) {
	// A high cutoff should exclude everything but a near-exact match.
	got := GetCloseMatches("apple", []string{"ape", "apple", "aple"}, 3, 0.99)
	want := []string{"apple"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetCloseMatches(cutoff=0.99) = %v, want %v", got, want)
	}
}

func TestGetCloseMatchesTieBreakLexicographic(t *testing.T) {
	// Two candidates equally similar to the word should come back in
	// lexicographic order.
	got := GetCloseMatches("ab", []string{"cb", "ba", "xb"}, 3, 0.4)
	// "cb", "ba", "xb" each share one char with "ab" -> ratio 0.5; order asc.
	want := []string{"ba", "cb", "xb"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tie-break = %v, want %v", got, want)
	}
}

func TestGetCloseMatchesEmpty(t *testing.T) {
	if got := GetCloseMatches("x", nil, 3, 0.6); len(got) != 0 {
		t.Fatalf("empty possibilities = %v, want none", got)
	}
	if got := GetCloseMatches("apple", []string{"apple"}, 0, 0.6); len(got) != 0 {
		t.Fatalf("n=0 = %v, want none", got)
	}
}

func TestUpperLenRatio(t *testing.T) {
	if r := upperLenRatio(0, 0); r != 1.0 {
		t.Errorf("upperLenRatio(empty) = %v, want 1.0", r)
	}
	if r := upperLenRatio(2, 4); r != 2.0*2/6 {
		t.Errorf("upperLenRatio = %v, want %v", r, 2.0*2/6)
	}
	if r := upperLenRatio(4, 2); r != 2.0*2/6 {
		t.Errorf("upperLenRatio is not symmetric: %v", r)
	}
}

// The prefilter compares a token count against a candidate's length, so that
// length has to be counted in runes. "ééé" is three tokens in six bytes: a
// byte count would bound it at 2*3/(3+6) = 0.667 and discard an exact match.
func TestUpperLenRatioUsesRuneCounts(t *testing.T) {
	const multibyte = "ééé"
	if r := upperLenRatio(len(tokenizeChars(multibyte)), len(multibyte)); r >= 0.9 {
		t.Fatalf("byte length should bound below the cutoff, got %v", r)
	}
	if r := upperLenRatio(len(tokenizeChars(multibyte)), utf8.RuneCountInString(multibyte)); r != 1.0 {
		t.Errorf("rune-count bound = %v, want 1.0", r)
	}
	if got := GetCloseMatches(multibyte, []string{multibyte}, 3, 0.9); len(got) != 1 {
		t.Errorf("exact multi-byte match was filtered out: got %v", got)
	}
}

func TestTokenizeLines(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{
			"first\nsecond\rthird\r\nfourth\nlast",
			[]string{"first\n", "second\r", "third\r\n", "fourth\n", "last"},
		},
		{"\n\n", []string{"\n", "\n"}},
		{"\n", []string{"\n"}},
		{"", nil},
	}
	for _, c := range cases {
		if got := tokenizeLines(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("tokenizeLines(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTokenizeLinesAndNewlines(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"foo\n\nbar", []string{"foo", "\n\n", "bar"}},
		{"a\r\nb", []string{"a", "\r\n", "b"}},
		{"\n\n", []string{"\n\n"}},
		{"abc", []string{"abc"}},
		{"", nil},
	}
	for _, c := range cases {
		if got := tokenizeLinesAndNewlines(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("tokenizeLinesAndNewlines(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTokenizeWords(t *testing.T) {
	got := tokenizeWords("foo    bar baz\n\n  aha")
	want := []string{"foo", "    ", "bar", " ", "baz", "\n\n  ", "aha"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokenizeWords = %q, want %q", got, want)
	}
}

func TestTokenizeChars(t *testing.T) {
	// The snowflake "❄️" is U+2744 followed by the variation selector U+FE0F, so
	// it splits into two separate character tokens. Codepoints are spelled out
	// to keep the expectation unambiguous.
	snowman := string(rune(0x2744))
	vs := string(rune(0xfe0f))
	input := "abcfö" + snowman + vs
	got := tokenizeChars(input)
	want := []string{"a", "b", "c", "f", "ö", snowman, vs}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokenizeChars = %q, want %q", got, want)
	}
}

// Every shipped tokenizer splits exactly like the function behind it, and only
// line tokens are newline-terminated by default.
func TestShippedTokenizers(t *testing.T) {
	in := "first line\nsecond line\r\nthird"
	cases := []struct {
		name    string
		tok     Tokenizer
		split   func(string) []string
		newline bool
	}{
		{"Lines", Lines, tokenizeLines, true},
		{"Words", Words, tokenizeWords, false},
		{"Chars", Chars, tokenizeChars, false},
		{"LinesAndNewlines", LinesAndNewlines, tokenizeLinesAndNewlines, false},
	}
	for _, c := range cases {
		if got, want := c.tok.Split(in), c.split(in); !reflect.DeepEqual(got, want) {
			t.Errorf("%s.Split(%q) = %q, want %q", c.name, in, got, want)
		}
		if got := c.tok.NewlineTerminated(); got != c.newline {
			t.Errorf("%s.NewlineTerminated() = %v, want %v", c.name, got, c.newline)
		}
	}
}

// DiffText with a shipped tokenizer is what the matching convenience does.
func TestDiffTextMatchesConveniences(t *testing.T) {
	old, new := "foo bar\nbaz qux\n", "foo bor\nbaz qux\n"
	cases := []struct {
		name       string
		tok        Tokenizer
		convenient func(old, new string, opts ...Option) *TextDiff
	}{
		{"Lines", Lines, DiffLines},
		{"Words", Words, DiffWords},
		{"Chars", Chars, DiffChars},
	}
	for _, c := range cases {
		got := DiffText(old, new, c.tok)
		want := c.convenient(old, new)
		if !reflect.DeepEqual(got.Ops(), want.Ops()) {
			t.Errorf("DiffText(%s) ops = %+v, want %+v", c.name, got.Ops(), want.Ops())
		}
		if !reflect.DeepEqual(slices.Collect(got.OldTokens()), slices.Collect(want.OldTokens())) {
			t.Errorf("DiffText(%s) old tokens differ from the convenience", c.name)
		}
		if got.NewlineTerminated() != want.NewlineTerminated() {
			t.Errorf("DiffText(%s) newlineTerminated = %v, want %v",
				c.name, got.NewlineTerminated(), want.NewlineTerminated())
		}
	}
}

// LinesAndNewlines has no convenience constructor; the seam is the only way in.
func TestDiffTextLinesAndNewlines(t *testing.T) {
	d := DiffText("a\nb", "a\n\nb", LinesAndNewlines)

	if got, want := slices.Collect(d.OldTokens()), []string{"a", "\n", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("old tokens = %q, want %q", got, want)
	}
	if got, want := slices.Collect(d.NewTokens()), []string{"a", "\n\n", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("new tokens = %q, want %q", got, want)
	}
	if d.NewlineTerminated() {
		t.Error("NewlineTerminated() = true, want false for lines-and-newlines")
	}
	got := slices.Collect(d.AllRemappedChanges())
	want := []RemappedChange{
		{ChangeEqual, "a"},
		{ChangeDelete, "\n"},
		{ChangeInsert, "\n\n"},
		{ChangeEqual, "b"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("remapped = %+v, want %+v", got, want)
	}
}

// commaTokenizer is a caller-supplied tokenizer: it partitions on commas,
// keeping each separator as its own token so the joined tokens are the input.
type commaTokenizer struct{}

func (commaTokenizer) Split(s string) []string {
	var rv []string
	for i, field := range strings.Split(s, ",") {
		if i > 0 {
			rv = append(rv, ",")
		}
		if field != "" {
			rv = append(rv, field)
		}
	}
	return rv
}

func (commaTokenizer) NewlineTerminated() bool { return false }

func TestDiffTextCustomTokenizer(t *testing.T) {
	d := DiffText("a,b,c", "a,B,c", commaTokenizer{})

	if got, want := slices.Collect(d.OldTokens()), []string{"a", ",", "b", ",", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("old tokens = %q, want %q", got, want)
	}
	if s, ok := d.SliceOld(0, 3); !ok || s != "a,b" {
		t.Errorf("SliceOld(0,3) = (%q,%v), want (\"a,b\",true)", s, ok)
	}
	got := slices.Collect(d.AllRemappedChanges())
	want := []RemappedChange{
		{ChangeEqual, "a,"},
		{ChangeDelete, "b"},
		{ChangeInsert, "B"},
		{ChangeEqual, ",c"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("remapped = %+v, want %+v", got, want)
	}
}

func TestDiffTextNilTokenizerPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("DiffText with a nil tokenizer: want panic, got none")
		}
		if got, want := r.(string), "text: nil tokenizer"; got != want {
			t.Fatalf("panic = %q, want %q", got, want)
		}
	}()
	DiffText("a", "b", nil)
}

func TestNewlineTerminatedDecorator(t *testing.T) {
	t.Run("overrides the answer in both directions", func(t *testing.T) {
		if NewlineTerminated(Lines, false).NewlineTerminated() {
			t.Error("Lines forced false: got true")
		}
		if !NewlineTerminated(Words, true).NewlineTerminated() {
			t.Error("Words forced true: got false")
		}
	})

	t.Run("leaves splitting alone", func(t *testing.T) {
		const s = "one\ntwo\n"
		got := NewlineTerminated(Lines, false).Split(s)
		want := Lines.Split(s)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Split = %q, want %q", got, want)
		}
	})

	t.Run("reaches the diff it is built with", func(t *testing.T) {
		if d := DiffText("a\n", "a\n", NewlineTerminated(Lines, false)); d.NewlineTerminated() {
			t.Error("Lines forced false: diff reports true")
		}
		if d := DiffText("a", "a", NewlineTerminated(Words, true)); !d.NewlineTerminated() {
			t.Error("Words forced true: diff reports false")
		}
	})

	t.Run("wraps a caller-supplied tokenizer", func(t *testing.T) {
		d := DiffText("a,b", "a,b", NewlineTerminated(commaTokenizer{}, true))
		if !d.NewlineTerminated() {
			t.Error("commaTokenizer forced true: diff reports false")
		}
		if got, want := slices.Collect(d.OldTokens()), []string{"a", ",", "b"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("old tokens = %q, want %q", got, want)
		}
	})

	t.Run("nil tokenizer panics", func(t *testing.T) {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("NewlineTerminated(nil, true): want panic, got none")
			}
			if got, want := r.(string), "similar: nil tokenizer"; got != want {
				t.Fatalf("panic = %q, want %q", got, want)
			}
		}()
		NewlineTerminated(nil, true)
	})
}

// The tokenizer is the only source of the flag; wrapping it is what overrides.
func TestNewlineTerminatedOverridesTokenizer(t *testing.T) {
	if d := DiffText("a\n", "a\n", NewlineTerminated(Lines, false)); d.NewlineTerminated() {
		t.Error("Lines wrapped false: got true, want false")
	}
	if d := DiffText("a", "a", NewlineTerminated(Words, true)); !d.NewlineTerminated() {
		t.Error("Words wrapped true: got false, want true")
	}
}
