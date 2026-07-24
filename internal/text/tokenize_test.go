package text

import (
	"reflect"
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
