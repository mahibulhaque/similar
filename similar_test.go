package similar_test

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/mahibulhaque/similar"
)

var update = flag.Bool("update", false, "update golden files")

func lcsLen[T comparable](a, b []T) int {
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else {
				dp[i][j] = max(dp[i+1][j], dp[i][j+1])
			}
		}
	}
	return dp[0][0]
}

func reconstruct[T comparable](old, new []T, ops []similar.DiffOp) []T {
	out := make([]T, 0, len(new))
	for _, op := range ops {
		switch op.Tag {
		case similar.Equal:
			out = append(out, old[op.OldIndex:op.OldIndex+op.OldLen]...)
		case similar.Insert, similar.Replace:
			out = append(out, new[op.NewIndex:op.NewIndex+op.NewLen]...)
		}
	}
	return out
}

func editCost(ops []similar.DiffOp) int {
	cost := 0
	for _, op := range ops {
		switch op.Tag {
		case similar.Delete:
			cost += op.OldLen
		case similar.Insert:
			cost += op.NewLen
		case similar.Replace:
			cost += op.OldLen + op.NewLen
		}
	}
	return cost
}

func eq[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDiffReconstructsAndMinimal(t *testing.T) {
	cases := []struct {
		name     string
		old, new []int
	}{
		{"empty", nil, nil},
		{"insert", nil, []int{1, 2}},
		{"delete", []int{1, 2}, nil},
		{"replace middle", []int{1, 2, 3, 4, 5}, []int{1, 2, 9, 4, 5}},
		{"disjoint", []int{1, 2, 3}, []int{4, 5, 6}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := similar.Diff(tc.old, tc.new)
			if got := reconstruct(tc.old, tc.new, ops); !eq(got, tc.new) {
				t.Fatalf("reconstruct = %v, want %v", got, tc.new)
			}
			want := len(tc.old) + len(tc.new) - 2*lcsLen(tc.old, tc.new)
			if c := editCost(ops); c != want {
				t.Fatalf("edit cost = %d, want %d", c, want)
			}
		})
	}
}

func TestDiffStringsReplaceCoalesced(t *testing.T) {
	old := []string{"a", "b", "c", "d"}
	new := []string{"a", "x", "y", "d"}
	ops := similar.Diff(old, new)
	// Expect: equal[0:1], replace[1:3], equal[3:4].
	if len(ops) != 3 {
		t.Fatalf("ops = %v, want 3", ops)
	}
	if ops[1].Tag != similar.Replace {
		t.Fatalf("middle op = %v, want replace", ops[1])
	}
}

func TestDeadlineExceededStillValid(t *testing.T) {
	a := make([]int, 400)
	b := make([]int, 400)
	for i := range a {
		a[i] = i
		b[i] = i * 7 // fully divergent middle
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	ops := similar.Diff(a, b, similar.WithContext(ctx))
	if got := reconstruct(a, b, ops); !eq(got, b) {
		t.Fatal("deadline-hit reconstruct did not rebuild new")
	}
}

// TestDiffRandomizedInvariants drives the default facade (Compact + Replace +
// Capture) over many pseudo-random inputs, re-checking reconstruct and
// minimality. This exercises the compaction shift/merge branches.
func TestDiffRandomizedInvariants(t *testing.T) {
	// Deterministic xorshift PRNG (Date/rand-free, reproducible).
	var state uint64 = 0x9e3779b97f4a7c15
	next := func() uint64 {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		return state
	}
	randInt := func(n int) int { return int(next() % uint64(n)) }

	for iter := range 2000 {
		alphabet := 2 + randInt(6)
		oldLen := randInt(40)
		newLen := randInt(40)
		old := make([]int, oldLen)
		new := make([]int, newLen)
		for i := range old {
			old[i] = randInt(alphabet)
		}
		for i := range new {
			new[i] = randInt(alphabet)
		}

		ops := similar.Diff(old, new)
		if got := reconstruct(old, new, ops); !eq(got, new) {
			t.Fatalf("iter %d: reconstruct = %v, want %v (old=%v)", iter, got, new, old)
		}
		want := len(old) + len(new) - 2*lcsLen(old, new)
		if c := editCost(ops); c != want {
			t.Fatalf("iter %d: edit cost = %d, want %d (old=%v new=%v)", iter, c, want, old, new)
		}
		// Facade output must be fully cursor-contiguous.
		oldCursor, newCursor := 0, 0
		for _, op := range ops {
			os, oe := op.OldRange()
			ns, ne := op.NewRange()
			if os != oldCursor || ns != newCursor {
				t.Fatalf("iter %d: non-contiguous op %v at (%d,%d)", iter, op, oldCursor, newCursor)
			}
			oldCursor, newCursor = oe, ne
		}
		if oldCursor != len(old) || newCursor != len(new) {
			t.Fatalf("iter %d: incomplete coverage (%d,%d)", iter, oldCursor, newCursor)
		}
	}
}

// --- The option seam and the hook entry points ---

func TestDiffHonorsOptions(t *testing.T) {
	old := []string{"a", "b", "c", "d"}
	new := []string{"a", "x", "y", "d"}

	t.Run("no options matches an explicit Myers background", func(t *testing.T) {
		bare := similar.Diff(old, new)
		spelt := similar.Diff(old, new,
			similar.WithAlgorithm(similar.Myers),
			similar.WithContext(context.Background()),
		)
		if !reflect.DeepEqual(bare, spelt) {
			t.Fatalf("defaults = %v, spelt out = %v", bare, spelt)
		}
	})

	t.Run("a cancelled context yields a valid script, not an error", func(t *testing.T) {
		a := make([]int, 400)
		b := make([]int, 400)
		for i := range a {
			a[i] = i
			b[i] = i * 7 // fully divergent middle
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		ops := similar.Diff(a, b, similar.WithContext(ctx))
		if got := reconstruct(a, b, ops); !eq(got, b) {
			t.Fatal("cancelled reconstruct did not rebuild new")
		}
	})

	t.Run("a nil option is ignored", func(t *testing.T) {
		if ops := similar.Diff(old, new, nil); len(ops) == 0 {
			t.Fatal("ops = none, want a diff")
		}
	})
}

func TestDiffToStreamsToAHook(t *testing.T) {
	old := []string{"a", "b", "c", "d"}
	new := []string{"a", "x", "y", "d"}

	capture := similar.NewCapture()
	if err := similar.DiffTo(capture, old, new); err != nil {
		t.Fatalf("DiffTo: %v", err)
	}

	// A bare hook sees the raw script: no Compact or Replace stage sits above
	// it, so the middle arrives as a Delete and an Insert rather than coalesced.
	if got := reconstruct(old, new, capture.Ops()); !eq(got, new) {
		t.Fatalf("hook script rebuilt %v, want %v", got, new)
	}
	for _, op := range capture.Ops() {
		if op.Tag == similar.Replace {
			t.Fatalf("bare hook saw a Replace: %v", capture.Ops())
		}
	}
}

// errHook fails on its first Equal, standing in for any failing callback.
type errHook struct {
	similar.NoopHook
	err error
}

func (h *errHook) Equal(oldIndex, newIndex, length int) error { return h.err }

func TestDiffToPropagatesHookError(t *testing.T) {
	boom := errors.New("boom")
	err := similar.DiffTo(&errHook{err: boom}, []int{1, 2}, []int{1, 2})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

// TestDiffRangeToReportsAbsoluteIndices is the reason the sub-range form exists:
// slicing the inputs and calling DiffTo would report indices relative to the
// window, and the caller would have to add the offsets back.
func TestDiffRangeToReportsAbsoluteIndices(t *testing.T) {
	old := []int{9, 1, 2, 3, 9}
	new := []int{8, 1, 2, 3, 8}

	capture := similar.NewCapture()
	if err := similar.DiffRangeTo(capture, old, 1, 4, new, 1, 4); err != nil {
		t.Fatalf("DiffRangeTo: %v", err)
	}

	want := []similar.DiffOp{{Tag: similar.Equal, OldIndex: 1, NewIndex: 1, OldLen: 3, NewLen: 3}}
	if !reflect.DeepEqual(capture.Ops(), want) {
		t.Fatalf("ops = %v, want %v", capture.Ops(), want)
	}
}

// --- Golden fixtures (regression guards) ---

func TestGolden(t *testing.T) {
	type fixture struct {
		name     string
		old, new []string
	}
	fixtures := []fixture{
		{"words", []string{"the", "quick", "brown", "fox"}, []string{"the", "slow", "brown", "cat"}},
		{"prefix_suffix", []string{"a", "b", "c", "d", "e"}, []string{"a", "b", "X", "d", "e"}},
		{"pure_insert", []string{"x"}, []string{"x", "y", "z"}},
	}
	// Myers keeps the original file names; a second algorithm gets its own set,
	// so a divergence between the two shows up as a diff in one file rather than
	// as a choice of which snapshot is right.
	algorithms := []struct {
		alg    similar.Algorithm
		prefix string
	}{
		{similar.Myers, ""},
		{similar.LCS, "lcs_"},
		{similar.Patience, "patience_"},
	}
	for _, alg := range algorithms {
		for _, fx := range fixtures {
			t.Run(alg.alg.String()+"/"+fx.name, func(t *testing.T) {
				goldenOps(t, alg.prefix+fx.name, similar.Diff(fx.old, fx.new, similar.WithAlgorithm(alg.alg)))
			})
		}
	}
}

// goldenOps compares ops against testdata/<name>.golden, or rewrites it under
// -update.
func goldenOps(t *testing.T, name string, ops []similar.DiffOp) {
	t.Helper()
	got, err := json.MarshalIndent(ops, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update): %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("golden mismatch for %s:\ngot:\n%s\nwant:\n%s", name, got, want)
	}
}

// TestTextGolden snapshots the expanded change lists of representative text
// diffs. Run with -update to regenerate the fixtures.
func TestTextGolden(t *testing.T) {
	fixtures := []struct {
		name     string
		old, new string
		changes  func(old, new string) []similar.Change
	}{
		{
			name: "lines_replace",
			old:  "Hello World\nsome stuff here\nsome more stuff here\n",
			new:  "Hello World\nsome amazing stuff here\nsome more stuff here\n",
			changes: func(old, new string) []similar.Change {
				return slices.Collect(similar.DiffLines(old, new).AllChanges())
			},
		},
		{
			name: "words_replace",
			old:  "the quick brown fox",
			new:  "the slow brown cat",
			changes: func(old, new string) []similar.Change {
				return slices.Collect(similar.DiffWords(old, new).AllChanges())
			},
		},
		{
			name: "chars_replace",
			old:  "abcdef",
			new:  "abcDDf",
			changes: func(old, new string) []similar.Change {
				return slices.Collect(similar.DiffChars(old, new).AllChanges())
			},
		},
		{
			name: "lines_and_newlines_replace",
			old:  "Hello World\nsome stuff here\n\nsome more stuff here\n",
			new:  "Hello World\n\nsome amazing stuff here\nsome more stuff here\n",
			changes: func(old, new string) []similar.Change {
				return slices.Collect(similar.DiffText(old, new, similar.LinesAndNewlines).AllChanges())
			},
		},
	}
	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			got, err := json.MarshalIndent(fx.changes(fx.old, fx.new), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')
			path := filepath.Join("testdata", "text_"+fx.name+".golden")
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run with -update): %v", err)
			}
			if string(got) != string(want) {
				t.Fatalf("golden mismatch for %s:\ngot:\n%s\nwant:\n%s", fx.name, got, want)
			}
		})
	}
}
