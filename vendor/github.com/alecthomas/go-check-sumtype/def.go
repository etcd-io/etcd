package gochecksumtype

import (
	"flag"
	"fmt"
	"go/token"
	"go/types"
	"log"
)

var debug = flag.Bool("debug", false, "enable debug logging")

func debugf(format string, args ...interface{}) {
	if *debug {
		log.Printf(format, args...)
	}
}

// Error as returned by Run()
type Error interface {
	error
	Pos() token.Position
}

// unsealedError corresponds to a declared sum type whose interface is not
// sealed. A sealed interface requires at least one unexported method.
type unsealedError struct {
	Decl sumTypeDecl
}

func (e unsealedError) Pos() token.Position { return e.Decl.Pos }
func (e unsealedError) Error() string {
	return fmt.Sprintf(
		"%s: interface '%s' is not sealed "+
			"(sealing requires at least one unexported method)",
		e.Decl.Location(), e.Decl.TypeName)
}

// notFoundError corresponds to a declared sum type whose type definition
// could not be found in the same Go package.
type notFoundError struct {
	Decl sumTypeDecl
}

func (e notFoundError) Pos() token.Position { return e.Decl.Pos }
func (e notFoundError) Error() string {
	return fmt.Sprintf("%s: type '%s' is not defined", e.Decl.Location(), e.Decl.TypeName)
}

// notInterfaceError corresponds to a declared sum type that does not
// correspond to an interface.
type notInterfaceError struct {
	Decl sumTypeDecl
}

func (e notInterfaceError) Pos() token.Position { return e.Decl.Pos }
func (e notInterfaceError) Error() string {
	return fmt.Sprintf("%s: type '%s' is not an interface", e.Decl.Location(), e.Decl.TypeName)
}

// sumTypeDef corresponds to the definition of a Go interface that is
// interpreted as a sum type. Its variants are determined by finding all types
// that implement said interface in the same package.
type sumTypeDef struct {
	Decl     sumTypeDecl
	Ty       *types.Interface
	Variants []types.Object
}

// findSumTypeDefs attempts to find a Go type definition for each of the given
// sum type declarations. If no such sum type definition could be found for
// any of the given declarations, then an error is returned.
func findSumTypeDefs(decls []sumTypeDecl) ([]sumTypeDef, []error) {
	defs := make([]sumTypeDef, 0, len(decls))
	var errs []error
	for _, decl := range decls {
		def, err := newSumTypeDef(decl.Package.Types, decl)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if def == nil {
			errs = append(errs, notFoundError{decl})
			continue
		}
		defs = append(defs, *def)
	}
	return defs, errs
}

// newSumTypeDef attempts to extract a sum type definition from a single
// package. If no such type corresponds to the given decl, then this function
// returns a nil def and a nil error.
//
// If the decl corresponds to a type that isn't an interface containing at
// least one unexported method, then this returns an error.
func newSumTypeDef(pkg *types.Package, decl sumTypeDecl) (*sumTypeDef, error) {
	obj := pkg.Scope().Lookup(decl.TypeName)
	if obj == nil {
		return nil, nil
	}
	iface, ok := obj.Type().Underlying().(*types.Interface)
	if !ok {
		return nil, notInterfaceError{decl}
	}
	hasUnexported := false
	for i := range iface.NumMethods() {
		if !iface.Method(i).Exported() {
			hasUnexported = true
			break
		}
	}
	if !hasUnexported {
		return nil, unsealedError{decl}
	}
	def := &sumTypeDef{
		Decl: decl,
		Ty:   iface,
	}
	debugf("searching for variants of %s.%s\n", pkg.Path(), decl.TypeName)
	for _, name := range pkg.Scope().Names() {
		obj, ok := pkg.Scope().Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		ty := obj.Type()
		if types.Identical(ty.Underlying(), iface) {
			continue
		}
		// Skip generic types.
		if named, ok := ty.(*types.Named); ok && named.TypeParams() != nil {
			continue
		}
		if types.Implements(ty, iface) || types.Implements(types.NewPointer(ty), iface) {
			debugf("  found variant: %s.%s\n", pkg.Path(), obj.Name())
			def.Variants = append(def.Variants, obj)
		}
	}
	return def, nil
}

func (def *sumTypeDef) String() string {
	return def.Decl.TypeName
}

// missing returns a list of variants in this sum type that are not in the
// given list of types.
func (def *sumTypeDef) missing(tys []types.Type, includeSharedInterfaces bool) []types.Object {
	// TODO(ag): This is O(n^2). Fix that. /shrug
	var missing []types.Object
	for _, v := range def.Variants {
		found := false
		varty := indirect(v.Type())
		for _, ty := range tys {
			ty = indirect(ty)
			if types.Identical(varty, ty) {
				found = true
				break
			}
			if includeSharedInterfaces && implements(varty, ty) {
				found = true
				break
			}
		}
		if !found && !isInterface(varty) {
			// we do not include interfaces extending the sumtype, as the
			// all implementations of those interfaces are already covered
			// by the sumtype.
			missing = append(missing, v)
		}
	}
	return missing
}

func isInterface(ty types.Type) bool {
	underlying := indirect(ty).Underlying()
	_, ok := underlying.(*types.Interface)
	return ok
}

// indirect dereferences through an arbitrary number of pointer types.
func indirect(ty types.Type) types.Type {
	if ty, ok := ty.(*types.Pointer); ok {
		return indirect(ty.Elem())
	}
	return ty
}

func implements(varty, interfaceType types.Type) bool {
	underlying := interfaceType.Underlying()
	if interf, ok := underlying.(*types.Interface); ok {
		return types.Implements(varty, interf) || types.Implements(types.NewPointer(varty), interf)
	}
	return false
}
