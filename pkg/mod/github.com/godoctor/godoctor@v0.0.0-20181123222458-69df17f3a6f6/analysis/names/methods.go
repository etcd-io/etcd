// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package names

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/loader"
)

// FindDeclarationsAcrossInterfaces finds all objects that might need to be
// renamed if the given identifier is renamed.  In the case of a method, there
// may be indirect relationships such as the following:
//
//      Interface1  Interface2
//         /  \      /  \
//        /  implements  \
//       /      \   /     \
//     Type1    Type2    Type3
//
// where renaming a method in Type1 could force a method with the same
// signature to be renamed in Interface1, Interface2, Type2, and Type3.  This
// method returns a set containing the reflexive, transitive closure of objects
// that must be renamed if the given identifier is renamed.
func FindDeclarationsAcrossInterfaces(obj types.Object, program *loader.Program) map[types.Object]bool {
	// XXX: This only searches for matches within the declaring package.  There
	// may be matches in other packages as well.
	if isMethod(obj) {
		// If obj is a method, search across interfaces: there may be
		// many other methods that need to change to ensure that all
		// types continue to implement the same interfaces
		return reachableMethods(obj.Name(), obj.(*types.Func), program.AllPackages[obj.Pkg()])
	} else {
		// If obj is not a method, then only one object needs to
		// change.  When this is called from inside the analysis/names
		// package, this will never occur, but it may when this method
		// is invoked as API.
		return map[types.Object]bool{obj: true}
	}
}

// isMethod reports whether obj is a method.
func isMethod(obj types.Object) bool {
	return methodReceiver(obj) != nil
}

// methodReceiver returns the receiver if obj is a method and nil otherwise.
func methodReceiver(obj types.Object) *types.Var {
	if obj, ok := obj.(*types.Func); ok {
		return obj.Type().(*types.Signature).Recv()
	}

	return nil
}

// reachableMethods receives an object for a method (i.e., a types.Func with
// a non-nil receiver) and the PackageInfo in which it was declared and returns
// a set of objects that must be renamed if that method is renamed.
func reachableMethods(name string, obj *types.Func, pkgInfo *loader.PackageInfo) map[types.Object]bool {
	// Find methods and interfaces defined in the given package that have
	// the same signature as the argument method (obj)
	sig := obj.Type().(*types.Signature)
	methods, interfaces := methodDeclsMatchingSig(name, sig, pkgInfo)

	// Map methods to interfaces their receivers implement and vice versa
	methodInterfaces := map[types.Object]map[*types.Interface]bool{}
	interfaceMethods := map[*types.Interface]map[types.Object]bool{}
	for iface := range interfaces {
		interfaceMethods[iface] = map[types.Object]bool{}
	}
	for method := range methods {
		methodInterfaces[method] = map[*types.Interface]bool{}
		recv := methodReceiver(method).Type()
		for iface := range interfaces {
			if types.Implements(recv, iface) {
				methodInterfaces[method][iface] = true
				interfaceMethods[iface][method] = true
			}
		}
	}

	// The two maps above define a bipartite graph with edges between
	// methods and the interfaces implemented by their receivers.  Perform
	// a breadth-first search of this graph, starting from obj, to find the
	// reflexive, transitive closure of methods affected by renaming obj.
	affectedMethods := map[types.Object]bool{obj: true}
	affectedInterfaces := map[*types.Interface]bool{}
	queue := []interface{}{obj}
	for i := 0; i < len(queue); i++ {
		switch elt := queue[i].(type) {
		case *types.Func:
			for iface := range methodInterfaces[elt] {
				if !affectedInterfaces[iface] {
					affectedInterfaces[iface] = true
					queue = append(queue, iface)
				}
			}
		case *types.Interface:
			for method := range interfaceMethods[elt] {
				if !affectedMethods[method] {
					affectedMethods[method] = true
					queue = append(queue, method)
				}
			}
		}
	}

	return affectedMethods
}

// methodDeclsMatchingSig walks all of the ASTs in the given package and
// returns methods with the given signature and interfaces that explicitly
// define a method with the given signature.
func methodDeclsMatchingSig(name string, sig *types.Signature, pkgInfo *loader.PackageInfo) (methods map[types.Object]bool, interfaces map[*types.Interface]bool) {
	// XXX(review D7): This looks quite expensive to do in a relatively low-level
	// function. Consider doing an initial pass over the ASTs to gather this
	// information if performance becomes an issue.

	// XXX(review D7): Two identifiers are identical iff (a) they are spelled the
	// same and (b) they are exported or they appear within the same package. So
	// really you need to know name's package too, construct a types.Id instance
	// for each side, and compare those.
	// I doubt it's a major practical problem in this case, but it's something
	// important corner case to bear in mind if you're building Go tools. It means
	// you can have a legal struct or interface with two fields/methods both named
	// "f", if they come from different packages.

	methods = map[types.Object]bool{}
	interfaces = map[*types.Interface]bool{}
	for _, file := range pkgInfo.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.InterfaceType:
				iface := pkgInfo.TypeOf(n).Underlying().(*types.Interface)
				interfaces[iface] = true
				for i := 0; i < iface.NumExplicitMethods(); i++ {
					method := iface.ExplicitMethod(i)
					methodSig := method.Type().(*types.Signature)
					if method.Name() == name && types.Identical(sig, methodSig) {
						methods[method] = true
					}
				}
			case *ast.FuncDecl:
				obj := pkgInfo.ObjectOf(n.Name)
				fnSig := obj.Type().Underlying().(*types.Signature)
				if fnSig.Recv() != nil && n.Name.Name == name && types.Identical(sig, fnSig) {
					methods[obj] = true
				}
			}
			return true
		})
	}
	return methods, interfaces
}
