package cat

import (
	"strings"
)

// Builder is a fluent concatenation helper. It is safe for concurrent use by
// multiple goroutines only if each goroutine uses a distinct *Builder.
// If pooling is enabled via Pool(true), call Release() when done.
// The Builder uses an internal strings.Builder for efficient string concatenation
// and manages a separator that is inserted between added values.
// It supports chaining methods for a fluent API style.
type Builder struct {
	buf      strings.Builder
	sep      string
	needsSep bool
}

// New begins a new Builder with a separator. If pooling is enabled,
// the Builder is reused and MUST be released with b.Release() when done.
// If sep is empty, uses DefaultSep().
// Optional initial arguments x are added immediately after creation.
// Pooling is controlled globally via Pool(true/false); when enabled, Builders
// are recycled to reduce allocations in high-throughput scenarios.
func New(sep string, x ...any) *Builder {
	var b *Builder
	if poolEnabled.Load() {
		b = builderPool.Get().(*Builder)
		b.buf.Reset()
		b.sep = sep
		b.needsSep = false
	} else {
		b = &Builder{sep: sep}
	}

	// Process initial arguments *after* the builder is prepared.
	if len(x) > 0 {
		b.Add(x...)
	}
	return b
}

// Start begins a new Builder with no separator (using an empty string as sep).
// It is a convenience function that wraps New(empty, x...), where empty is a constant empty string.
// This allows starting a concatenation without any separator between initial or subsequent additions.
// If pooling is enabled via Pool(true), the returned Builder MUST be released with b.Release() when done.
// Optional variadic arguments x are passed directly to New and added immediately after creation.
// Useful for fluent chains where no default separator is desired from the start.
func Start(x ...any) *Builder {
	return New(empty, x...)
}

// Grow pre-sizes the internal buffer.
// This can be used to preallocate capacity based on an estimated total size,
// reducing reallocations during subsequent Add calls.
// It chains, returning the Builder for fluent use.
func (b *Builder) Grow(n int) *Builder { b.buf.Grow(n); return b }

// Add appends values to the builder.
// It inserts the current separator before each new value if needed (i.e., after the first addition).
// Values are converted to strings using the optimized write function, which handles
// common types efficiently without allocations where possible.
// Supports any number of arguments of any type.
// Chains, returning the Builder for fluent use.
func (b *Builder) Add(args ...any) *Builder {
	for _, arg := range args {
		if b.needsSep && b.sep != empty {
			b.buf.WriteString(b.sep)
		}
		write(&b.buf, arg)
		b.needsSep = true
	}
	return b
}

// If appends values to the builder only if the condition is true.
// Behaves like Add when condition is true; does nothing otherwise.
// Useful for conditional concatenation in chains.
// Chains, returning the Builder for fluent use.
func (b *Builder) If(condition bool, args ...any) *Builder {
	if condition {
		b.Add(args...)
	}
	return b
}

// Sep changes the separator for subsequent additions.
// Future Add calls will use this new separator.
// Does not affect already added content.
// If sep is empty, no separator will be added between future values.
// Chains, returning the Builder for fluent use.
func (b *Builder) Sep(sep string) *Builder { b.sep = sep; return b }

// String returns the concatenated result.
// This does not release the Builder; if pooling is enabled, call Release separately
// if you are done with the Builder.
// Can be called multiple times; the internal buffer remains unchanged.
func (b *Builder) String() string { return b.buf.String() }

// Output returns the concatenated result and releases the Builder if pooling is enabled.
// This is a convenience method to get the string and clean up in one call.
// After Output, the Builder should not be used further if pooled, as it may be recycled.
// If pooling is disabled, it behaves like String without release.
func (b *Builder) Output() string {
	out := b.buf.String()
	b.Release() // Release takes care of the poolEnabled check
	return out
}

// Release returns the Builder to the pool if pooling is enabled.
// You should call this exactly once per New() when Pool(true) is active.
// Resets the internal state (buffer, separator, needsSep) before pooling to avoid
// retaining data or large allocations.
// If pooling is disabled, this is a no-op.
// Safe to call multiple times, but typically called once at the end of use.
func (b *Builder) Release() {
	if poolEnabled.Load() {
		// Avoid retaining large buffers.
		b.buf.Reset()
		b.sep = empty
		b.needsSep = false
		builderPool.Put(b)
	}
}
