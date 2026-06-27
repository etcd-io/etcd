package typeutil

import (
	"bytes"
	"go/types"
	"strings"
	"sync"

	"golang.org/x/exp/typeparams"
)

var bufferPool = &sync.Pool{
	New: func() any {
		buf := bytes.NewBuffer(nil)
		buf.Grow(64)
		return buf
	},
}

func FuncName(f *types.Func) string {
	// We don't care about aliases in this function because we use FuncName to check calls
	// to known methods, and method receivers are determined by the method declaration,
	// not the call. Thus, even if a user does 'type Alias = *sync.Mutex' and calls
	// Alias.Lock, we'll still see it as (*sync.Mutex).Lock.

	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	if f.Type() != nil {
		sig := f.Type().(*types.Signature)
		if recv := sig.Recv(); recv != nil {
			buf.WriteByte('(')
			if _, ok := recv.Type().(*types.Interface); ok {
				// gcimporter creates abstract methods of
				// named interfaces using the interface type
				// (not the named type) as the receiver.
				// Don't print it in full.
				buf.WriteString("interface")
			} else {
				types.WriteType(buf, recv.Type(), nil)
			}
			buf.WriteByte(')')
			buf.WriteByte('.')
		} else if f.Pkg() != nil {
			writePackage(buf, f.Pkg())
		}
	}
	buf.WriteString(f.Name())
	s := buf.String()
	bufferPool.Put(buf)
	return s
}

func writePackage(buf *bytes.Buffer, pkg *types.Package) {
	if pkg == nil {
		return
	}
	s := pkg.Path()
	if s != "" {
		buf.WriteString(s)
		buf.WriteByte('.')
	}
}

// Dereference returns a pointer's element type; otherwise it returns
// T.
func Dereference(T types.Type) types.Type {
	if p, ok := T.Underlying().(*types.Pointer); ok {
		return p.Elem()
	}
	return T
}

// DereferenceR returns a pointer's element type; otherwise it returns
// T. If the element type is itself a pointer, DereferenceR will be
// applied recursively.
func DereferenceR(T types.Type) types.Type {
	if p, ok := T.Underlying().(*types.Pointer); ok {
		return DereferenceR(p.Elem())
	}
	return T
}

func IsObject(obj types.Object, name string) bool {
	var path string
	if pkg := obj.Pkg(); pkg != nil {
		path = pkg.Path() + "."
	}
	return path+obj.Name() == name
}

// IsTypeName reports whether obj represents the qualified name. If obj is a type alias,
// IsTypeName checks both the alias and the aliased type, if the aliased type has a type
// name.
func IsTypeName(obj *types.TypeName, name string) bool {
	var qf string
	if idx := strings.LastIndex(name, "."); idx != -1 {
		qf = name[:idx]
		name = name[idx+1:]
	}
	if obj.Name() == name &&
		((qf == "" && obj.Pkg() == nil) || (obj.Pkg() != nil && obj.Pkg().Path() == qf)) {
		return true
	}

	if !obj.IsAlias() {
		return false
	}

	// FIXME(dh): we should peel away one layer of alias at a time; this is blocked on
	// github.com/golang/go/issues/66559
	if typ, ok := types.Unalias(obj.Type()).(interface{ Obj() *types.TypeName }); ok {
		return IsTypeName(typ.Obj(), name)
	}

	return false
}

func IsPointerToTypeWithName(typ types.Type, name string) bool {
	ptr, ok := types.Unalias(typ).(*types.Pointer)
	if !ok {
		return false
	}
	return IsTypeWithName(ptr.Elem(), name)
}

// IsTypeWithName reports whether typ represents a type with the qualified name, If typ is
// a type alias, IsTypeWithName checks both the alias and the aliased type. The following
// types can have names: Basic, Named, Alias.
func IsTypeWithName(typ types.Type, name string) bool {
	switch typ := typ.(type) {
	case *types.Basic:
		return typ.Name() == name
	case *types.Named:
		return IsTypeName(typ.Obj(), name)
	case *types.Alias:
		// FIXME(dh): we should peel away one layer of alias at a time; this is blocked on
		// github.com/golang/go/issues/66559

		// IsTypeName already handles aliases to other aliases or named types; our
		// fallback is required for aliases to basic types.
		return IsTypeName(typ.Obj(), name) || IsTypeWithName(types.Unalias(typ), name)
	default:
		return false
	}
}

// IsPointerLike returns true if type T is like a pointer. This returns true for all nillable types,
// unsafe.Pointer, and type sets where at least one term is pointer-like.
func IsPointerLike(T types.Type) bool {
	switch T := T.Underlying().(type) {
	case *types.Interface:
		if T.IsMethodSet() {
			return true
		} else {
			terms, err := typeparams.NormalTerms(T)
			if err != nil {
				return false
			}
			for _, term := range terms {
				if IsPointerLike(term.Type()) {
					return true
				}
			}
			return false
		}
	case *types.Chan, *types.Map, *types.Signature, *types.Pointer, *types.Slice:
		return true
	case *types.Basic:
		return T.Kind() == types.UnsafePointer
	}
	return false
}

type Field struct {
	Var  *types.Var
	Tag  string
	Path []int
}

// FlattenFields recursively flattens T and embedded structs,
// returning a list of fields. If multiple fields with the same name
// exist, all will be returned.
func FlattenFields(T *types.Struct) []Field {
	return flattenFields(T, nil, nil)
}

func flattenFields(T *types.Struct, path []int, seen map[types.Type]bool) []Field {
	if seen == nil {
		seen = map[types.Type]bool{}
	}
	if seen[T] {
		return nil
	}
	seen[T] = true
	var out []Field
	for i := 0; i < T.NumFields(); i++ {
		field := T.Field(i)
		tag := T.Tag(i)
		np := append(path[:len(path):len(path)], i)
		if field.Anonymous() {
			if s, ok := Dereference(field.Type()).Underlying().(*types.Struct); ok {
				out = append(out, flattenFields(s, np, seen)...)
			} else {
				out = append(out, Field{field, tag, np})
			}
		} else {
			out = append(out, Field{field, tag, np})
		}
	}
	return out
}
