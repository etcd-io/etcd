package gochecksumtype

import "golang.org/x/tools/go/packages"

// Run sumtype checking on the given packages.
func Run(pkgs []*packages.Package, config Config) []error {
	var errs []error

	decls, err := findSumTypeDecls(pkgs)
	if err != nil {
		return []error{err}
	}

	defs, defErrs := findSumTypeDefs(decls)
	errs = append(errs, defErrs...)
	if len(defs) == 0 {
		return errs
	}

	for _, pkg := range pkgs {
		if pkgErrs := check(pkg, defs, config); pkgErrs != nil {
			errs = append(errs, pkgErrs...)
		}
	}
	return errs
}
