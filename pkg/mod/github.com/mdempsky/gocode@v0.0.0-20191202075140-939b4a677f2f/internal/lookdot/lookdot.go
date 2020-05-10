package lookdot

import "go/types"

type Visitor func(obj types.Object)

func Walk(tv *types.TypeAndValue, v Visitor) bool {
	switch {
	case tv.IsType():
		walk(tv.Type, false, false, v)
	case tv.IsValue():
		walk(tv.Type, tv.Addressable(), true, v)
	default:
		return false
	}
	return true
}

func walk(typ0 types.Type, addable0, value bool, v Visitor) {
	// Enumerating valid selector expression identifiers is
	// surprisingly nuanced.

	// found is a map from selector identifiers to the objects
	// they select. Nil entries are used to track objects that
	// have already been reported to the visitor and to indicate
	// ambiguous identifiers.
	found := make(map[string]types.Object)

	addObj := func(obj types.Object, valid bool) {
		id := obj.Id()
		switch otherObj, isPresent := found[id]; {
		case !isPresent:
			if valid {
				found[id] = obj
			} else {
				found[id] = nil
			}
		case otherObj != nil:
			// Ambiguous selector.
			found[id] = nil
		}
	}

	// visited keeps track of named types that we've already
	// visited. We only need to track named types, because
	// recursion can only happen through embedded struct fields,
	// which must be either a named type or a pointer to a named
	// type.
	visited := make(map[*types.Named]bool)

	type todo struct {
		typ     types.Type
		addable bool
	}

	var cur, next []todo
	cur = []todo{{typ0, addable0}}

	for {
		if len(cur) == 0 {
			// Flush discovered objects to visitor function.
			for id, obj := range found {
				if obj != nil {
					v(obj)
					found[id] = nil
				}
			}

			// Move unvisited types from next to cur.
			// It's important to check between levels to
			// ensure that ambiguous selections are
			// correctly handled.
			cur = next[:0]
			for _, t := range next {
				nt := namedOf(t.typ)
				if nt == nil {
					if _, ok := t.typ.(*types.Basic); ok {
						continue
					}
					panic("panic: embedded struct field without name?")
				}
				if !visited[nt] {
					cur = append(cur, t)
				}
			}
			next = nil

			if len(cur) == 0 {
				break
			}
		}

		now := cur[0]
		cur = cur[1:]

		// Look for methods declared on a named type.
		{
			typ, addable := chasePointer(now.typ)
			if !addable {
				addable = now.addable
			}
			if typ, ok := typ.(*types.Named); ok {
				visited[typ] = true
				for i, n := 0, typ.NumMethods(); i < n; i++ {
					m := typ.Method(i)
					addObj(m, addable || !hasPtrRecv(m))
				}
			}
		}

		// Look for interface methods.
		if typ, ok := now.typ.Underlying().(*types.Interface); ok {
			for i, n := 0, typ.NumMethods(); i < n; i++ {
				addObj(typ.Method(i), true)
			}
		}

		// Look for struct fields.
		{
			typ, addable := chasePointer(now.typ.Underlying())
			if !addable {
				addable = now.addable
			}
			if typ, ok := typ.Underlying().(*types.Struct); ok {
				for i, n := 0, typ.NumFields(); i < n; i++ {
					f := typ.Field(i)
					addObj(f, value)
					if f.Anonymous() {
						next = append(next, todo{f.Type(), addable})
					}
				}
			}
		}
	}
}

// namedOf returns the named type T when given T or *T.
// Otherwise, it returns nil.
func namedOf(typ types.Type) *types.Named {
	if ptr, isPtr := typ.(*types.Pointer); isPtr {
		typ = ptr.Elem()
	}
	res, _ := typ.(*types.Named)
	return res
}

func hasPtrRecv(m *types.Func) bool {
	_, ok := m.Type().(*types.Signature).Recv().Type().(*types.Pointer)
	return ok
}

func chasePointer(typ types.Type) (types.Type, bool) {
	if ptr, isPtr := typ.(*types.Pointer); isPtr {
		return ptr.Elem(), true
	}
	return typ, false
}
