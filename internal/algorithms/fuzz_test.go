package algorithms

import (
	"testing"
)

// FuzzMyersInvariants feeds random byte sequences (derived from the fuzz
// inputs) through the raw Myers core and re-checks the reconstruct and
// minimality invariants on every seed.
func FuzzMyersInvariants(f *testing.F) {
	f.Add([]byte("abcabba"), []byte("cbabac"))
	f.Add([]byte(""), []byte("hello"))
	f.Add([]byte("hello"), []byte(""))
	f.Add([]byte("aaaaaa"), []byte("aaaaaa"))
	f.Add([]byte{0, 1, 2, 3, 4}, []byte{0, 1, 2, 9, 4})

	f.Fuzz(func(t *testing.T, a, b []byte) {
		// Cap sizes so the O(N*M) oracle stays cheap.
		if len(a) > 512 {
			a = a[:512]
		}
		if len(b) > 512 {
			b = b[:512]
		}
		ops := captureMyers(a, b)

		got := reconstruct(a, b, ops)
		if !slicesEqual(got, b) {
			t.Fatalf("reconstruct mismatch: a=%v b=%v got=%v", a, b, got)
		}
		wantCost := len(a) + len(b) - 2*bruteLCS(a, b)
		if c := editCost(ops); c != wantCost {
			t.Fatalf("edit cost = %d, want %d (a=%v b=%v)", c, wantCost, a, b)
		}
		checkContiguous(t, a, b, ops)
	})
}

// FuzzLCSInvariants is the same for the LCS core. Both algorithms claim
// minimality, so it also cross-checks that they agree on the cost — a
// disagreement means one of them is wrong, whichever the oracle blames.
func FuzzLCSInvariants(f *testing.F) {
	f.Add([]byte("abcabba"), []byte("cbabac"))
	f.Add([]byte(""), []byte("hello"))
	f.Add([]byte("hello"), []byte(""))
	f.Add([]byte("aaaaaa"), []byte("aaaaaa"))
	f.Add([]byte{0, 1, 2, 3, 4}, []byte{0, 1, 2, 9, 4})

	f.Fuzz(func(t *testing.T, a, b []byte) {
		// Cap sizes so the O(N*M) oracle and table stay cheap.
		if len(a) > 512 {
			a = a[:512]
		}
		if len(b) > 512 {
			b = b[:512]
		}
		ops := captureLCS(a, b)

		got := reconstruct(a, b, ops)
		if !slicesEqual(got, b) {
			t.Fatalf("reconstruct mismatch: a=%v b=%v got=%v", a, b, got)
		}
		wantCost := len(a) + len(b) - 2*bruteLCS(a, b)
		cost := editCost(ops)
		if cost != wantCost {
			t.Fatalf("edit cost = %d, want %d (a=%v b=%v)", cost, wantCost, a, b)
		}
		if myersCost := editCost(captureMyers(a, b)); myersCost != cost {
			t.Fatalf("lcs cost %d, myers cost %d (a=%v b=%v)", cost, myersCost, a, b)
		}
		checkContiguous(t, a, b, ops)
	})
}
