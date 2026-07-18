package diffutil

import (
	"math"
	"testing"
)

func TestIsEmptyRange(t *testing.T) {
	cases := []struct {
		start, end int
		want       bool
	}{
		{0, 0, true},
		{3, 3, true},
		{5, 3, true},
		{0, 1, false},
		{2, 5, false},
	}
	for _, tc := range cases {
		if got := IsEmptyRange(tc.start, tc.end); got != tc.want {
			t.Errorf("IsEmptyRange(%d,%d) = %v, want %v", tc.start, tc.end, got, tc.want)
		}
	}
}

func TestCommonPrefixLen(t *testing.T) {
	b := func(s string) []byte { return []byte(s) }
	cases := []struct {
		old, new       string
		os, oe, ns, ne int
		want           int
	}{
		{"", "", 0, 0, 0, 0, 0},
		{"foobarbaz", "foobarblah", 0, 9, 0, 10, 7},
		{"foobarbaz", "blablabla", 0, 9, 0, 9, 0},
		{"foobarbaz", "foobarblah", 3, 9, 3, 10, 4},
	}
	for _, tc := range cases {
		if got := CommonPrefixLen(b(tc.old), tc.os, tc.oe, b(tc.new), tc.ns, tc.ne); got != tc.want {
			t.Errorf("CommonPrefixLen(%q,%q) = %d, want %d", tc.old, tc.new, got, tc.want)
		}
	}
}

func TestCommonSuffixLen(t *testing.T) {
	b := func(s string) []byte { return []byte(s) }
	cases := []struct {
		old, new       string
		os, oe, ns, ne int
		want           int
	}{
		{"", "", 0, 0, 0, 0, 0},
		{"1234", "X0001234", 0, 4, 0, 8, 4},
		{"1234", "Xxxx", 0, 4, 0, 4, 0},
		{"1234", "01234", 2, 4, 2, 5, 2},
	}
	for _, tc := range cases {
		if got := CommonSuffixLen(b(tc.old), tc.os, tc.oe, b(tc.new), tc.ns, tc.ne); got != tc.want {
			t.Errorf("CommonSuffixLen(%q,%q) = %d, want %d", tc.old, tc.new, got, tc.want)
		}
	}
}

func TestSatMul(t *testing.T) {
	cases := []struct {
		a, b, want int
	}{
		{0, 5, 0},
		{5, 0, 0},
		{3, 4, 12},
		{1 << 40, 1 << 40, math.MaxInt},
		{math.MaxInt, 2, math.MaxInt},
	}
	for _, tc := range cases {
		if got := SatMul(tc.a, tc.b); got != tc.want {
			t.Errorf("SatMul(%d,%d) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
