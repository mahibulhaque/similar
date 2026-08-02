package similar

import "fmt"

// Algorithm selects the diff algorithm. In v0.x the only value is Myers; the
// type exists so call sites stay stable as more algorithms ship.
//
// engine.go holds the single switch that turns a value of this type into an
// implementation, so adding an algorithm means adding a constant here and a case
// there.
type Algorithm int

const (
	// Myers is Eugene W. Myers' shortest-edit-script algorithm.
	Myers Algorithm = iota
)

// String returns the algorithm's name.
func (a Algorithm) String() string {
	switch a {
	case Myers:
		return "myers"
	default:
		return fmt.Sprintf("Algorithm(%d)", int(a))
	}
}

// valid reports whether a names an algorithm this release implements.
//
// It is unexported because there is exactly one place an Algorithm can enter
// the package — WithAlgorithm — and that is the only caller. It was public
// while the rule was spread across the entry points that could report a bad
// value as an error and those that had to panic; with one gate, a caller has
// nothing to check that WithAlgorithm does not check for them.
func (a Algorithm) valid() bool {
	switch a {
	case Myers:
		return true
	default:
		return false
	}
}
