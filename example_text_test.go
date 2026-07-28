package similar_test

import (
	"fmt"

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
