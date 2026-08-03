package similar_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mahibulhaque/similar"
)

func ExampleDiff() {
	old := []string{"foo", "bar", "baz"}
	new := []string{"foo", "bar", "blah"}

	for _, op := range similar.Diff(old, new) {
		os, oe := op.OldRange()
		ns, ne := op.NewRange()
		fmt.Printf("%s old[%d:%d] new[%d:%d]\n", op.Tag, os, oe, ns, ne)
	}
	// Output:
	// equal old[0:2] new[0:2]
	// replace old[2:3] new[2:3]
}

func ExampleDiff_options() {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// A deadline bounds the work. If it expires the script is still valid, only
	// approximate, so there is nothing to check.
	ops := similar.Diff([]rune("kitten"), []rune("sitting"),
		similar.WithContext(ctx),
		similar.WithAlgorithm(similar.Myers),
	)
	fmt.Println(len(ops) > 0)
	// Output:
	// true
}

// countHook embeds NoopHook and overrides only the callbacks it needs.
type countHook struct {
	similar.NoopHook
	equal, changed int
}

func (h *countHook) Equal(int, int, int) error  { h.equal++; return nil }
func (h *countHook) Delete(int, int, int) error { h.changed++; return nil }
func (h *countHook) Insert(int, int, int) error { h.changed++; return nil }

func ExampleDiffTo() {
	h := &countHook{}
	if err := similar.DiffTo(h, []int{1, 2, 3, 4}, []int{1, 9, 3, 4}); err != nil {
		panic(err)
	}
	fmt.Println("equal runs:", h.equal, "changes:", h.changed)
	// Output:
	// equal runs: 2 changes: 2
}

func ExampleDiffRangeTo() {
	old := []int{9, 1, 2, 3, 9}
	new := []int{8, 1, 2, 3, 8}

	// Diff the middles only. Indices reported to the hook stay absolute, which
	// is what slicing the inputs would cost you.
	capture := similar.NewCapture()
	if err := similar.DiffRangeTo(capture, old, 1, 4, new, 1, 4); err != nil {
		panic(err)
	}
	for _, op := range capture.Ops() {
		os, oe := op.OldRange()
		fmt.Printf("%s old[%d:%d]\n", op.Tag, os, oe)
	}
	// Output:
	// equal old[1:4]
}

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
