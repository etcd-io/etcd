// Package sqlbuilders provides SQL builder library-specific checkers for SELECT * detection.
package sqlbuilders

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/MirrexOne/unqueryvet/pkg/config"
)

// SelectStarViolation represents a detected SELECT * usage in SQL builder code.
type SelectStarViolation struct {
	// Pos is the position in source code where the violation was detected
	Pos token.Pos
	// End is the end position of the violation
	End token.Pos
	// Message is the human-readable description of the violation
	Message string
	// Builder is the name of the SQL builder library
	Builder string
	// Context provides additional context about the violation type
	Context string
}

// SQLBuilderChecker is the interface for SQL builder library-specific checkers.
// Each SQL builder library (GORM, sqlx, etc.) implements this interface.
type SQLBuilderChecker interface {
	// Name returns the name of the SQL builder library
	Name() string

	// IsApplicable checks if the call expression is from this SQL builder.
	// It uses type information to verify the receiver type belongs to the correct package.
	IsApplicable(info *types.Info, call *ast.CallExpr) bool

	// CheckSelectStar checks a single call expression for SELECT * usage
	CheckSelectStar(call *ast.CallExpr) *SelectStarViolation

	// CheckChainedCalls analyzes method chains for SELECT * patterns
	CheckChainedCalls(call *ast.CallExpr) []*SelectStarViolation
}

// Registry holds all registered SQL builder checkers and provides a unified interface.
type Registry struct {
	checkers []SQLBuilderChecker
}

// NewRegistry creates a new Registry with checkers based on the configuration.
func NewRegistry(cfg *config.SQLBuildersConfig) *Registry {
	r := &Registry{
		checkers: make([]SQLBuilderChecker, 0),
	}

	// Register enabled checkers
	if cfg.Squirrel {
		r.checkers = append(r.checkers, NewSquirrelChecker())
	}
	if cfg.GORM {
		r.checkers = append(r.checkers, NewGORMChecker())
	}
	if cfg.SQLx {
		r.checkers = append(r.checkers, NewSQLxChecker())
	}
	if cfg.Ent {
		r.checkers = append(r.checkers, NewEntChecker())
	}
	if cfg.PGX {
		r.checkers = append(r.checkers, NewPGXChecker())
	}
	if cfg.Bun {
		r.checkers = append(r.checkers, NewBunChecker())
	}
	if cfg.SQLBoiler {
		r.checkers = append(r.checkers, NewSQLBoilerChecker())
	}
	if cfg.Jet {
		r.checkers = append(r.checkers, NewJetChecker())
	}
	if cfg.Sqlc {
		r.checkers = append(r.checkers, NewSQLCChecker())
	}
	if cfg.Goqu {
		r.checkers = append(r.checkers, NewGoquChecker())
	}
	if cfg.Rel {
		r.checkers = append(r.checkers, NewRelChecker())
	}
	if cfg.Reform {
		r.checkers = append(r.checkers, NewReformChecker())
	}

	return r
}

// Check analyzes a call expression against all registered checkers.
// Returns all violations found across all applicable checkers.
// The info parameter provides type information for accurate type checking.
func (r *Registry) Check(info *types.Info, call *ast.CallExpr) []*SelectStarViolation {
	var violations []*SelectStarViolation

	for _, checker := range r.checkers {
		if !checker.IsApplicable(info, call) {
			continue
		}

		// Check for direct SELECT * usage
		if v := checker.CheckSelectStar(call); v != nil {
			violations = append(violations, v)
		}

		// Check for SELECT * in method chains
		chainViolations := checker.CheckChainedCalls(call)
		violations = append(violations, chainViolations...)
	}

	return violations
}

// HasCheckers returns true if at least one checker is registered.
func (r *Registry) HasCheckers() bool {
	return len(r.checkers) > 0
}

// IsTypeFromPackage checks if the type of an expression belongs to a package
// with the given path prefix. This is used to verify that a method call
// is actually from the expected SQL builder library.
func IsTypeFromPackage(info *types.Info, expr ast.Expr, pkgPathPrefix string) bool {
	if info == nil {
		return false
	}

	typ := info.TypeOf(expr)
	if typ == nil {
		return false
	}

	return isTypeFromPackageRecursive(typ, pkgPathPrefix)
}

// isTypeFromPackageRecursive recursively checks if a type belongs to a package.
func isTypeFromPackageRecursive(typ types.Type, pkgPathPrefix string) bool {
	switch t := typ.(type) {
	case *types.Named:
		if obj := t.Obj(); obj != nil {
			if pkg := obj.Pkg(); pkg != nil {
				pkgPath := pkg.Path()
				if len(pkgPath) >= len(pkgPathPrefix) && pkgPath[:len(pkgPathPrefix)] == pkgPathPrefix {
					return true
				}
			}
		}
	case *types.Pointer:
		return isTypeFromPackageRecursive(t.Elem(), pkgPathPrefix)
	}
	return false
}
