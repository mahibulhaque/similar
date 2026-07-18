package similar_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

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

func TestDiffDeadlineExceededStillValid(t *testing.T) {
	a := make([]int, 400)
	b := make([]int, 400)
	for i := range a {
		a[i] = i
		b[i] = i * 7 // fully divergent middle
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ops, err := similar.DiffDeadline(ctx, similar.Myers, a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

	for iter := 0; iter < 2000; iter++ {
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
	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			ops := similar.CaptureDiff(similar.Myers, fx.old, fx.new)
			got, err := json.MarshalIndent(ops, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')
			path := filepath.Join("testdata", fx.name+".golden")
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
