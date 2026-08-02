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

// Valid reports whether a names an algorithm this release implements. It is the
// single source of truth consulted both by the entry points that return an
// error on a bad value and by those that must reject it at construction time.
func (a Algorithm) Valid() bool {
	switch a {
	case Myers:
		return true
	default:
		return false
	}
}
