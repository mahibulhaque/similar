package similar

import "github.com/mahibulhaque/similar/internal/diff"

// The shared diff vocabulary is re-exported here so callers need only import
// the root package. The canonical definitions live in internal/diff.
type (
	// DiffOp is a single tagged diff operation. See [diff.DiffOp].
	DiffOp = diff.DiffOp
	// DiffTag identifies the kind of a DiffOp.
	DiffTag = diff.DiffTag
	// DiffHook receives operation callbacks as a diff is produced.
	DiffHook = diff.DiffHook
	// NoopHook is an embeddable DiffHook whose callbacks all return nil.
	NoopHook = diff.NoopHook
	// Capture is a DiffHook that accumulates operations into a slice.
	Capture = diff.Capture
	// ReplaceHook coalesces adjacent delete+insert into Replace operations.
	ReplaceHook = diff.ReplaceHook
	// Change is an expanded diff operation: a tagged value with its old/new
	// position. See [diff.Change].
	Change = diff.Change
	// ChangeTag identifies the kind of a Change (equal/delete/insert).
	ChangeTag = diff.ChangeTag
)

// DiffTag values.
const (
	Equal   = diff.Equal
	Delete  = diff.Delete
	Insert  = diff.Insert
	Replace = diff.Replace
)

// ChangeTag values.
const (
	ChangeEqual  = diff.ChangeEqual
	ChangeDelete = diff.ChangeDelete
	ChangeInsert = diff.ChangeInsert
)

// NewCapture returns an empty Capture hook.
func NewCapture() *Capture { return diff.NewCapture() }

// NewReplaceHook wraps an inner hook to coalesce delete+insert into replace.
func NewReplaceHook(d DiffHook) *ReplaceHook { return diff.NewReplace(d) }

// GroupDiffOps isolates change clusters with n items of context.
func GroupDiffOps(ops []DiffOp, n int) [][]DiffOp { return diff.GroupDiffOps(ops, n) }

// DiffRatio returns the similarity of two sequences from their ops and lengths.
func DiffRatio(ops []DiffOp, oldLen, newLen int) float64 {
	return diff.DiffRatio(ops, oldLen, newLen)
}
