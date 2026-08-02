package similar

import (
	"context"
	"testing"
)

func TestWithAlgorithmIsHonored(t *testing.T) {
	d := DiffLines("a\nb\n", "a\nc\n", WithAlgorithm(Myers))
	if got := d.Algorithm(); got != Myers {
		t.Fatalf("Algorithm = %v, want %v", got, Myers)
	}
	if len(d.Ops()) == 0 {
		t.Fatal("ops = none, want a diff")
	}
}

func TestWithAlgorithmRejectsUnknown(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("WithAlgorithm(99): want panic, got none")
		}
		if want := "text: unknown algorithm 99"; r != want {
			t.Fatalf("panic = %v, want %q", r, want)
		}
	}()
	WithAlgorithm(Algorithm(99))
}

func TestDefaultsAreMyersAndBackground(t *testing.T) {
	c := resolve(nil)
	if c.algorithm != Myers {
		t.Fatalf("algorithm = %v, want myers", c.algorithm)
	}
	if c.ctx != context.Background() {
		t.Fatalf("ctx = %v, want background", c.ctx)
	}
	if c.newlineTerminated != nil {
		t.Fatal("newlineTerminated = set, want unset")
	}
}

func TestNilOptionIsIgnored(t *testing.T) {
	d := DiffLines("a\n", "b\n", nil)
	if len(d.Ops()) == 0 {
		t.Fatal("ops = none, want a diff")
	}
}
