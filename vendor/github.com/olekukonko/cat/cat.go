// Package cat provides efficient and flexible string concatenation utilities.
// It includes optimized functions for concatenating various types, builders for fluent chaining,
// and configuration options for defaults, pooling, and unsafe optimizations.
// The package aims to minimize allocations and improve performance in string building scenarios.
package cat

import (
	"sync"
	"sync/atomic"
)

// Constants used throughout the package for separators, defaults, and configuration.
// These include common string literals for separators, empty strings, and special representations,
// as well as limits like recursion depth. Defining them as constants allows for compile-time
// optimizations, readability, and consistent usage in functions like Space, Path, CSV, and reflection handlers.
// cat.go (updated constants section)
const (
	empty   = ""   // Empty string constant, used for checks and defaults.
	space   = " "  // Single space, default separator.
	slash   = "/"  // Forward slash, for paths.
	dot     = "."  // Period, for extensions or decimals.
	comma   = ","  // Comma, for CSV or lists.
	equal   = "="  // Equals, for comparisons.
	newline = "\n" // Newline, for multi-line strings.

	// SQL-specific constants
	and        = "AND"      // AND operator, for SQL conditions.
	inOpen     = " IN ("    // Opening for SQL IN clause
	inClose    = ")"        // Closing for SQL IN clause
	asSQL      = " AS "     // SQL AS for aliasing
	count      = "COUNT("   // SQL COUNT function prefix
	sum        = "SUM("     // SQL SUM function prefix
	avg        = "AVG("     // SQL AVG function prefix
	maxOpen    = "MAX("     // SQL MAX function prefix
	minOpen    = "MIN("     // SQL MIN function prefix
	caseSQL    = "CASE "    // SQL CASE keyword
	when       = "WHEN "    // SQL WHEN clause
	then       = " THEN "   // SQL THEN clause
	elseSQL    = " ELSE "   // SQL ELSE clause
	end        = " END"     // SQL END for CASE
	countAll   = "COUNT(*)" // SQL COUNT(*) for all rows
	parenOpen  = "("        // Opening parenthesis
	parenClose = ")"        // Closing parenthesis

	maxRecursionDepth = 32      // Maximum recursion depth for nested structure handling.
	nilString         = "<nil>" // String representation for nil values.
	unexportedString  = "<?>"   // Placeholder for unexported fields.
)

// Numeric is a generic constraint interface for numeric types.
// It includes all signed/unsigned integers and floats.
// Used in generic functions like Number and NumberWith to constrain to numbers.
type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64
}

// poolEnabled controls whether New() reuses Builder instances from a pool.
// Atomic.Bool for thread-safe toggle.
// When true, Builders from New must be Released to avoid leaks.
var poolEnabled atomic.Bool

// builderPool stores reusable *Builder to reduce GC pressure on hot paths.
// Uses sync.Pool for efficient allocation/reuse.
// New func creates a fresh &Builder when pool is empty.
var builderPool = sync.Pool{
	New: func() any { return &Builder{} },
}

// Pool enables or disables Builder pooling for New()/Release().
// When enabled, you MUST call b.Release() after b.String() to return it.
// Thread-safe via atomic.Store.
// Enable for high-throughput scenarios to reduce allocations.
func Pool(enable bool) { poolEnabled.Store(enable) }

// unsafeBytesFlag controls zero-copy []byte -> string behavior via atomics.
// Int32 used for atomic operations: 1 = enabled, 0 = disabled.
// Affects bytesToString function for zero-copy conversions using unsafe.
var unsafeBytesFlag atomic.Int32 // 1 = true, 0 = false

// SetUnsafeBytes toggles zero-copy []byte -> string conversions globally.
// When enabled, bytesToString uses unsafe.String for zero-allocation conversion.
// Thread-safe via atomic.Store.
// Use with caution: assumes the byte slice is not modified after conversion.
// Compatible with Go 1.20+; fallback to string(bts) if disabled.
func SetUnsafeBytes(enable bool) {
	if enable {
		unsafeBytesFlag.Store(1)
	} else {
		unsafeBytesFlag.Store(0)
	}
}

// IsUnsafeBytes reports whether zero-copy []byte -> string is enabled.
// Thread-safe via atomic.Load.
// Returns true if flag is 1, false otherwise.
// Useful for checking current configuration.
func IsUnsafeBytes() bool { return unsafeBytesFlag.Load() == 1 }

// deterministicMaps controls whether map keys are sorted for deterministic output in string conversions.
// It uses atomic.Bool for thread-safe access.
var deterministicMaps atomic.Bool

// SetDeterministicMaps controls whether map keys are sorted for deterministic output
// in reflection-based handling (e.g., in writeReflect for maps).
// When enabled, keys are sorted using a string-based comparison for consistent string representations.
// Thread-safe via atomic.Store.
// Useful for reproducible outputs in testing or logging.
func SetDeterministicMaps(enable bool) {
	deterministicMaps.Store(enable)
}

// IsDeterministicMaps returns current map sorting setting.
// Thread-safe via atomic.Load.
// Returns true if deterministic sorting is enabled, false otherwise.
func IsDeterministicMaps() bool {
	return deterministicMaps.Load()
}
