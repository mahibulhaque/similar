package algorithms

// diffHook is the contract this package emits its edit script through.
//
// It is declared here, rather than imported, so that the algorithms depend on
// nothing but the shape of their consumer. The public statement of the contract
// is similar.DiffHook, whose method set is a superset of this one; an interface
// value of that type is assignable to this one, and the call sites in package
// similar are what type-check the two against each other. If the public
// interface ever grows a method this one lacks, that is harmless; if this one
// grows a method the public interface lacks, package similar stops compiling.
//
// The one method it deliberately lacks is Replace. Nothing in this package can
// produce one: the front anchor only ever peels a one-sided skip, and no other
// site emits a replacement. Leaving Replace off the contract is what makes that
// a compile error rather than a convention — similar.ReplaceHook synthesises
// every Replace a caller sees, by coalescing an adjacent Delete and Insert.
//
// A hook never sees the sequence values, only indices and lengths. Any callback
// may return an error, which aborts the diff and propagates to the caller.
// Finish is always called after the last operation.
type diffHook interface {
	// Equal reports that old[oldIndex:oldIndex+length] equals
	// new[newIndex:newIndex+length].
	Equal(oldIndex, newIndex, length int) error
	// Delete reports that old[oldIndex:oldIndex+oldLen] is removed; newIndex
	// is the position in the new sequence at the point of deletion.
	Delete(oldIndex, oldLen, newIndex int) error
	// Insert reports that new[newIndex:newIndex+newLen] is inserted at
	// oldIndex in the old sequence.
	Insert(oldIndex, newIndex, newLen int) error
	// Finish is called once after the final operation.
	Finish() error
}
