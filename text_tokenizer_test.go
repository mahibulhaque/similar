package similar

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

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
