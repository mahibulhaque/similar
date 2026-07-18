package diff

// triple is an optional (a, b, c) tuple; ok reports whether it is set.
type triple struct {
	a, b, c int
	ok      bool
}

// ReplaceHook is a DiffHook wrapper that coalesces adjacent deletions and
// insertions into Replace operations (and merges runs of like operations),
// forwarding the result to an inner hook.
//
// The core algorithm emits only equal/delete/insert; wrap it in a ReplaceHook
// to obtain replace semantics without the core producing them. (It is named
// ReplaceHook rather than Replace to avoid clashing with the Replace DiffTag
// constant in this package.)
type ReplaceHook struct {
	d   DiffHook
	del triple // (oldIndex, oldLen, newIndex)
	ins triple // (oldIndex, newIndex, newLen)
	eq  triple // (oldIndex, newIndex, len)
}

// NewReplace wraps an inner hook.
func NewReplace(d DiffHook) *ReplaceHook {
	return &ReplaceHook{d: d}
}

// Inner returns the wrapped hook.
func (r *ReplaceHook) Inner() DiffHook { return r.d }

func (r *ReplaceHook) flushEq() error {
	if r.eq.ok {
		eq := r.eq
		r.eq = triple{}
		return r.d.Equal(eq.a, eq.b, eq.c)
	}
	return nil
}

func (r *ReplaceHook) flushDelIns() error {
	if r.del.ok {
		del := r.del
		r.del = triple{}
		if r.ins.ok {
			ins := r.ins
			r.ins = triple{}
			return r.d.Replace(del.a, del.b, ins.b, ins.c)
		}
		return r.d.Delete(del.a, del.b, del.c)
	}
	if r.ins.ok {
		ins := r.ins
		r.ins = triple{}
		return r.d.Insert(ins.a, ins.b, ins.c)
	}
	return nil
}

func (r *ReplaceHook) Equal(oldIndex, newIndex, length int) error {
	if err := r.flushDelIns(); err != nil {
		return err
	}
	if r.eq.ok {
		r.eq.c += length
	} else {
		r.eq = triple{a: oldIndex, b: newIndex, c: length, ok: true}
	}
	return nil
}

func (r *ReplaceHook) Delete(oldIndex, oldLen, newIndex int) error {
	if err := r.flushEq(); err != nil {
		return err
	}
	if r.del.ok {
		r.del.b += oldLen
	} else {
		r.del = triple{a: oldIndex, b: oldLen, c: newIndex, ok: true}
	}
	return nil
}

func (r *ReplaceHook) Insert(oldIndex, newIndex, newLen int) error {
	if err := r.flushEq(); err != nil {
		return err
	}
	if r.ins.ok {
		r.ins.c += newLen
	} else {
		r.ins = triple{a: oldIndex, b: newIndex, c: newLen, ok: true}
	}
	return nil
}

func (r *ReplaceHook) Replace(oldIndex, oldLen, newIndex, newLen int) error {
	if err := r.flushEq(); err != nil {
		return err
	}
	return r.d.Replace(oldIndex, oldLen, newIndex, newLen)
}

func (r *ReplaceHook) Finish() error {
	if err := r.flushEq(); err != nil {
		return err
	}
	if err := r.flushDelIns(); err != nil {
		return err
	}
	return r.d.Finish()
}

var _ DiffHook = (*ReplaceHook)(nil)
