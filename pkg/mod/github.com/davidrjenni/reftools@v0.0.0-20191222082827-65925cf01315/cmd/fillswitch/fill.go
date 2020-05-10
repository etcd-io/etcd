// Copyright (c) 2017 David R. Jenni. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go/ast"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/loader"
)

func fillSwitch(pkg *loader.PackageInfo, lprog *loader.Program, swtch ast.Stmt, typ types.Type) ast.Stmt {
	// Do not try to fill an empty switch statement (with no tag expression and therefore typ == nil).
	if typ == nil {
		return swtch
	}

	switch swtch := swtch.(type) {
	case *ast.SwitchStmt:
		existing := make(map[string]bool)
		// Don't add the identifier we switch over to the case statements.
		if id, ok := swtch.Tag.(*ast.Ident); ok {
			existing[id.Name] = true
		}
		for _, cc := range swtch.Body.List {
			for _, e := range cc.(*ast.CaseClause).List {
				existing[typeString(pkg.Pkg, pkg.Info.TypeOf(e))] = true
			}
		}
		for _, v := range findConstsAndVars(lprog, pkg.Pkg, typ) {
			name := ast.NewIdent(v.Name())
			if imported(pkg.Pkg, v) {
				name = ast.NewIdent(v.Pkg().Name() + "." + v.Name())
			}
			if !existing[v.Name()] {
				swtch.Body.List = append(swtch.Body.List, &ast.CaseClause{
					List: []ast.Expr{name},
				})
			}
		}
		return swtch

	case *ast.TypeSwitchStmt:
		iface, ok := typ.Underlying().(*types.Interface)
		if !ok {
			return swtch
		}
		existing := make(map[string]bool)
		for _, cc := range swtch.Body.List {
			for _, e := range cc.(*ast.CaseClause).List {
				name := typeString(pkg.Pkg, pkg.Info.TypeOf(e))
				existing[name] = true
			}
		}
		for _, t := range findTypes(lprog, pkg.Pkg, iface) {
			if ts := typeString(pkg.Pkg, t); !existing[ts] {
				swtch.Body.List = append(swtch.Body.List, &ast.CaseClause{
					List: []ast.Expr{ast.NewIdent(ts)},
				})
			}
		}
		return swtch

	default:
		panic("unreachable")
	}
}

func findConstsAndVars(lprog *loader.Program, pkg *types.Package, typ types.Type) []types.Object {
	var vars []types.Object
	for _, info := range lprog.AllPackages {
		for _, obj := range info.Defs {
			switch obj := obj.(type) {
			case *types.Const:
				if visible(pkg, obj) && types.AssignableTo(obj.Type(), typ) {
					vars = append(vars, obj)
				}
			case *types.Var:
				if visible(pkg, obj) && !obj.IsField() && types.AssignableTo(obj.Type(), typ) {
					vars = append(vars, obj)
				}
			}
		}
	}

	sort.Sort(objsByString(vars))
	return vars
}

func findTypes(lprog *loader.Program, pkg *types.Package, iface types.Type) []types.Type {
	var typs []types.Type

	err := types.Universe.Lookup("error").Type()
	if types.AssignableTo(err, iface) {
		typs = append(typs, err)
	}

	for _, info := range lprog.AllPackages {
		for _, obj := range info.Defs {
			obj, ok := obj.(*types.TypeName)
			if !ok || obj.IsAlias() || !visible(pkg, obj) {
				continue
			}

			t := obj.Type().(*types.Named)
			// Ignore iface itself and empty interfaces.
			if i, ok := t.Underlying().(*types.Interface); ok && (iface == i || i.NumMethods() == 0) {
				continue
			}

			if types.AssignableTo(t, iface) {
				typs = append(typs, t)
			} else if p := types.NewPointer(t); types.AssignableTo(p, iface) {
				typs = append(typs, p)
			}
		}
	}

	sort.Sort(typesByString(typs))
	return typs
}

func imported(pkg *types.Package, obj types.Object) bool {
	return obj.Pkg() != pkg
}

func visible(pkg *types.Package, obj types.Object) bool {
	if obj.Pkg() == pkg {
		return true
	}
	if !obj.Exported() {
		return false
	}

	// Rough approximation at the "internal" rules.

	path := obj.Pkg().Path()
	i := 0

	switch {
	case strings.HasSuffix(path, "/internal"):
		i = len(path) - len("/internal")
	case strings.Contains(path, "/internal/"):
		i = strings.LastIndex(path, "/internal/") + 1
	case path == "internal", strings.HasPrefix(path, "internal/"):
		i = 0
	default:
		return true
	}
	if i > 0 {
		i--
	}
	prefix := path[:i]
	return len(prefix) > 0 && strings.HasPrefix(pkg.Path(), prefix)
}

type typesByString []types.Type

func (t typesByString) Len() int           { return len(t) }
func (t typesByString) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }
func (t typesByString) Less(i, j int) bool { return t[i].String() < t[j].String() }

type objsByString []types.Object

func (o objsByString) Len() int           { return len(o) }
func (o objsByString) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o objsByString) Less(i, j int) bool { return o[i].String() < o[j].String() }
