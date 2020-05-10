// Copyright (c) 2017 David R. Jenni. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Plundered from go/types; customized.

// Copyright (c) 2009 The Go Authors. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// This file implements printing of types.

package main

import (
	"bytes"
	"fmt"
	"go/types"
)

type typeWriter struct {
	buf         *bytes.Buffer
	pkg         *types.Package
	hasError    bool
	importNames map[string]string
}

func typeString(pkg *types.Package, importNames map[string]string, typ types.Type) (string, bool) {
	w := typeWriter{
		buf:         &bytes.Buffer{},
		pkg:         pkg,
		importNames: importNames,
	}
	w.writeType(typ, make([]types.Type, 0, 8))
	return w.buf.String(), !w.hasError
}

func (w *typeWriter) writeType(typ types.Type, visited []types.Type) {
	// Theoretically, this is a quadratic lookup algorithm, but in
	// practice deeply nested composite types with unnamed component
	// types are uncommon. This code is likely more efficient than
	// using a map.
	for _, t := range visited {
		if t == typ {
			fmt.Fprintf(w.buf, "○%T", typ) // cycle to typ
			return
		}
	}
	visited = append(visited, typ)

	switch t := typ.(type) {
	case nil:
		w.buf.WriteString("nil")

	case *types.Basic:
		switch t.Kind() {
		case types.Invalid:
			w.hasError = true
		case types.UnsafePointer:
			w.buf.WriteString("unsafe.")
		}
		w.buf.WriteString(t.Name())

	case *types.Array:
		fmt.Fprintf(w.buf, "[%d]", t.Len())
		w.writeType(t.Elem(), visited)

	case *types.Slice:
		w.buf.WriteString("[]")
		w.writeType(t.Elem(), visited)

	case *types.Struct:
		w.buf.WriteString("struct{")
		for i := 0; i < t.NumFields(); i++ {
			f := t.Field(i)
			if i > 0 {
				w.buf.WriteString("; ")
			}
			if !f.Anonymous() {
				w.buf.WriteString(f.Name())
				w.buf.WriteByte(' ')
			}
			w.writeType(f.Type(), visited)
			if tag := t.Tag(i); tag != "" {
				fmt.Fprintf(w.buf, " %q", tag)
			}
		}
		w.buf.WriteByte('}')

	case *types.Pointer:
		w.buf.WriteByte('*')
		w.writeType(t.Elem(), visited)

	case *types.Tuple:
		w.writeTuple(t, false, visited)

	case *types.Signature:
		w.buf.WriteString("func")
		w.writeSignature(t, visited)

	case *types.Interface:
		// We write the source-level methods and embedded types rather
		// than the actual method set since resolved method signatures
		// may have non-printable cycles if parameters have anonymous
		// interface types that (directly or indirectly) embed the
		// current interface. For instance, consider the result type
		// of m:
		//
		//     type T interface{
		//         m() interface{ T }
		//     }
		//
		w.buf.WriteString("interface{")
		// print explicit interface methods and embedded types
		for i := 0; i < t.NumMethods(); i++ {
			m := t.Method(i)
			if i > 0 {
				w.buf.WriteString("; ")
			}
			w.buf.WriteString(m.Name())
			w.writeSignature(m.Type().(*types.Signature), visited)
		}
		for i := 0; i < t.NumEmbeddeds(); i++ {
			if i > 0 || t.NumMethods() > 0 {
				w.buf.WriteString("; ")
			}
			w.writeType(t.EmbeddedType(i), visited)
		}
		w.buf.WriteByte('}')

	case *types.Map:
		w.buf.WriteString("map[")
		w.writeType(t.Key(), visited)
		w.buf.WriteByte(']')
		w.writeType(t.Elem(), visited)

	case *types.Chan:
		var s string
		var parens bool
		switch t.Dir() {
		case types.SendRecv:
			s = "chan "
			// chan (<-chan T) requires parentheses
			if c, _ := t.Elem().(*types.Chan); c != nil && c.Dir() == types.RecvOnly {
				parens = true
			}
		case types.SendOnly:
			s = "chan<- "
		case types.RecvOnly:
			s = "<-chan "
		default:
			panic("unreachable")
		}
		w.buf.WriteString(s)
		if parens {
			w.buf.WriteByte('(')
		}
		w.writeType(t.Elem(), visited)
		if parens {
			w.buf.WriteByte(')')
		}

	case *types.Named:
		if isImported(w.pkg, t) && t.Obj().Pkg() != nil {
			pkg := t.Obj().Pkg()
			if name, ok := w.importNames[pkg.Path()]; ok {
				if name == "." {
					w.buf.WriteString(t.Obj().Name())
				} else {
					w.buf.WriteString(fmt.Sprintf("%s.%s", name, t.Obj().Name()))
				}
			} else {
				w.buf.WriteString(fmt.Sprintf("%s.%s", pkg.Name(), t.Obj().Name()))
			}
		} else {
			w.buf.WriteString(t.Obj().Name())
		}

	default:
		// For externally defined implementations of Type.
		w.buf.WriteString(t.String())
	}
}

func (w *typeWriter) writeTuple(tup *types.Tuple, variadic bool, visited []types.Type) {
	w.buf.WriteByte('(')
	if tup != nil {
		for i := 0; i < tup.Len(); i++ {
			v := tup.At(i)
			if i > 0 {
				w.buf.WriteString(", ")
			}
			if v.Name() != "" {
				w.buf.WriteString(v.Name())
				w.buf.WriteByte(' ')
			}
			typ := v.Type()
			if variadic && i == tup.Len()-1 {
				if s, ok := typ.(*types.Slice); ok {
					w.buf.WriteString("...")
					typ = s.Elem()
				} else {
					// special case:
					// append(s, "foo"...) leads to signature func([]byte, string...)
					if t, ok := typ.Underlying().(*types.Basic); !ok || t.Kind() != types.String {
						panic("internal error: string type expected")
					}
					w.writeType(typ, visited)
					w.buf.WriteString("...")
					continue
				}
			}
			w.writeType(typ, visited)
		}
	}
	w.buf.WriteByte(')')
}

func (w *typeWriter) writeSignature(sig *types.Signature, visited []types.Type) {
	w.writeTuple(sig.Params(), sig.Variadic(), visited)

	n := sig.Results().Len()
	if n == 0 {
		return // no result
	}

	w.buf.WriteByte(' ')
	if n == 1 && sig.Results().At(0).Name() == "" {
		// single unnamed result
		w.writeType(sig.Results().At(0).Type(), visited)
		return
	}

	// multiple or named result(s)
	w.writeTuple(sig.Results(), false, visited)
}
