package similar_test

import (
	"fmt"
	"strings"

	"github.com/mahibulhaque/similar"
)

func ExampleDiffLines() {
	diff := similar.DiffLines("a\nb\nc", "a\nb\nC")
	for c := range diff.AllChanges() {
		fmt.Printf("%s%s", c.Tag(), c)
	}
	// Output:
	//  a
	//  b
	// -c
	// +C
}

func ExampleDiffWords() {
	diff := similar.DiffWords("foo bar baz", "foo BAR baz")
	for c := range diff.AllChanges() {
		fmt.Printf("%s%q\n", c.Tag(), c.Value())
	}
	// Output:
	//  "foo"
	//  " "
	// -"bar"
	// +"BAR"
	//  " "
	//  "baz"
}

func ExampleDiffText() {
	diff := similar.DiffText("a\nb", "a\n\nb", similar.LinesAndNewlines)
	for c := range diff.AllChanges() {
		fmt.Printf("%s%q\n", c.Tag(), c.Value())
	}
	// Output:
	//  "a"
	// -"\n"
	// +"\n\n"
	//  "b"
}

// A Tokenizer of your own splits by any rule; here, on commas.
func ExampleDiffText_custom() {
	diff := similar.DiffText("a,b,c", "a,B,c", commaTokenizer{})
	for s := range diff.AllRemappedChanges() {
		fmt.Printf("%s%q\n", s.Tag, s.Value)
	}
	// Output:
	//  "a,"
	// -"b"
	// +"B"
	//  ",c"
}

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

func ExampleTextDiff_Ratio() {
	diff := similar.DiffChars("abcd", "bcde")
	fmt.Println(diff.Ratio())
	// Output: 0.75
}

func ExampleTextDiff_AllRemappedChanges() {
	diff := similar.DiffWords("foo bar baz", "foo bor baz")
	for s := range diff.AllRemappedChanges() {
		fmt.Printf("%s%q\n", s.Tag, s.Value)
	}
	// Output:
	//  "foo "
	// -"bar"
	// +"bor"
	//  " baz"
}

func ExampleGetCloseMatches() {
	matches := similar.GetCloseMatches("appel", []string{"ape", "apple", "peach", "puppy"}, 3, 0.6)
	fmt.Println(matches)
	// Output: [apple ape]
}
