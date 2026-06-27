package cat

import (
	"reflect"
	"strings"
)

// Append appends args to dst and returns the grown slice.
// Callers can reuse dst across calls to amortize allocs.
// It uses an internal Builder for efficient concatenation of the args (no separators),
// then appends the result to the dst byte slice.
// Preallocates based on a size estimate to minimize reallocations.
// Benefits from Builder pooling if enabled.
// Useful for building byte slices incrementally without separators.
func Append(dst []byte, args ...any) []byte {
	return AppendWith(empty, dst, args...)
}

// AppendWith appends args to dst and returns the grown slice.
// Callers can reuse dst across calls to amortize allocs.
// Similar to Append, but inserts the specified sep between each arg.
// Preallocates based on a size estimate including separators.
// Benefits from Builder pooling if enabled.
// Useful for building byte slices incrementally with custom separators.
func AppendWith(sep string, dst []byte, args ...any) []byte {
	if len(args) == 0 {
		return dst
	}
	b := New(sep)
	b.Grow(estimateWith(sep, args))
	b.Add(args...)
	out := b.Output()
	return append(dst, out...)
}

// AppendBytes joins byte slices without separators.
// Only for compatibility with low-level byte processing.
// Directly appends each []byte arg to dst without any conversion or separators.
// Efficient for pure byte concatenation; no allocations if dst has capacity.
// Returns the extended dst slice.
// Does not use Builder, as it's simple append operations.
func AppendBytes(dst []byte, args ...[]byte) []byte {
	if len(args) == 0 {
		return dst
	}
	for _, b := range args {
		dst = append(dst, b...)
	}
	return dst
}

// AppendTo writes arguments to an existing strings.Builder.
// More efficient than creating new builders.
// Appends each arg to the provided strings.Builder using the optimized write function.
// No separators are added; for direct concatenation.
// Useful when you already have a strings.Builder and want to add more values efficiently.
// Does not use cat.Builder, as it appends to an existing strings.Builder.
func AppendTo(b *strings.Builder, args ...any) {
	for _, arg := range args {
		write(b, arg)
	}
}

// AppendStrings writes strings to an existing strings.Builder.
// Directly writes each string arg to the provided strings.Builder.
// No type checks or conversions; assumes all args are strings.
// Efficient for appending known strings without separators.
// Does not use cat.Builder, as it appends to an existing strings.Builder.
func AppendStrings(b *strings.Builder, ss ...string) {
	for _, s := range ss {
		b.WriteString(s)
	}
}

// Between concatenates values wrapped between x and y (no separator between args).
// Equivalent to BetweenWith with an empty separator.
func Between(x, y any, args ...any) string {
	return BetweenWith(empty, x, y, args...)
}

// BetweenWith concatenates values wrapped between x and y, using sep between x, args, and y.
// Uses a pooled Builder if enabled; releases it after use.
// Equivalent to With(sep, x, args..., y).
func BetweenWith(sep string, x, y any, args ...any) string {
	b := New(sep)
	// Estimate size for all parts to avoid re-allocation.
	b.Grow(estimate([]any{x, y}) + estimateWith(sep, args))

	b.Add(x)
	b.Add(args...)
	b.Add(y)

	return b.Output()
}

// CSV joins arguments with "," separators (no space).
// Convenience wrapper for With using a comma as separator.
// Useful for simple CSV string generation without spaces.
func CSV(args ...any) string { return With(comma, args...) }

// Comma joins arguments with ", " separators.
// Convenience wrapper for With using ", " as separator.
// Useful for human-readable lists with comma and space.
func Comma(args ...any) string { return With(comma+space, args...) }

// Concat concatenates any values (no separators).
// Usage: cat.Concat("a", 1, true) → "a1true"
// Equivalent to With with an empty separator.
func Concat(args ...any) string {
	return With(empty, args...)
}

// ConcatWith concatenates any values with separator.
// Alias for With; joins args with the provided sep.
func ConcatWith(sep string, args ...any) string {
	return With(sep, args...)
}

// Flatten joins nested values into a single concatenation using empty.
// Convenience for FlattenWith using empty.
func Flatten(args ...any) string {
	return FlattenWith(empty, args...)
}

// FlattenWith joins nested values into a single concatenation with sep, avoiding
// intermediate slice allocations where possible.
// It recursively flattens any nested []any arguments, concatenating all leaf items
// with sep between them. Skips empty nested slices to avoid extra separators.
// Leaf items (non-slices) are converted using the optimized write function.
// Uses a pooled Builder if enabled; releases it after use.
// Preallocates based on a recursive estimate for efficiency.
// Example: FlattenWith(",", 1, []any{2, []any{3,4}}, 5) → "1,2,3,4,5"
func FlattenWith(sep string, args ...any) string {
	if len(args) == 0 {
		return empty
	}

	// Recursive estimate for preallocation.
	totalSize := recursiveEstimate(sep, args)

	b := New(sep)
	b.Grow(totalSize)
	recursiveAdd(b, args)
	return b.Output()
}

// Group joins multiple groups with empty between groups (no intra-group separators).
// Convenience for GroupWith using empty.
func Group(groups ...[]any) string {
	return GroupWith(empty, groups...)
}

// GroupWith joins multiple groups with a separator between groups (no intra-group separators).
// Concatenates each group internally without separators, then joins non-empty groups with sep.
// Preestimates total size for allocation; uses pooled Builder if enabled.
// Optimized for single group: direct Concat.
// Useful for grouping related items with inter-group separation.
func GroupWith(sep string, groups ...[]any) string {
	if len(groups) == 0 {
		return empty
	}
	if len(groups) == 1 {
		return Concat(groups[0]...)
	}

	total := 0
	nonEmpty := 0
	for _, g := range groups {
		if len(g) == 0 {
			continue
		}
		if nonEmpty > 0 {
			total += len(sep)
		}
		total += estimate(g)
		nonEmpty++
	}

	b := New(empty)
	b.Grow(total)
	first := true
	for _, g := range groups {
		if len(g) == 0 {
			continue
		}
		if !first && sep != empty {
			b.buf.WriteString(sep)
		}
		first = false
		for _, a := range g {
			write(&b.buf, a)
		}
	}
	return b.Output()
}

// Indent prefixes the concatenation of args with depth levels of two spaces per level.
// Example: Indent(2, "hello") => "    hello"
// If depth <= 0, equivalent to Concat(args...).
// Uses "  " repeated depth times as prefix, followed by concatenated args (no separators).
// Benefits from pooling via Concat.
func Indent(depth int, args ...any) string {
	if depth <= 0 {
		return Concat(args...)
	}
	prefix := strings.Repeat("  ", depth)
	return Prefix(prefix, args...)
}

// Join joins strings (matches stdlib strings.Join behavior).
// Usage: cat.Join("a", "b") → "a b" (using empty)
// Joins the variadic string args with the current empty.
// Useful for compatibility with stdlib but using package default sep.
func Join(elems ...string) string {
	return strings.Join(elems, empty)
}

// JoinWith joins strings with separator (variadic version).
// Directly uses strings.Join on the variadic string args with sep.
// Efficient for known strings; no conversions needed.
func JoinWith(sep string, elems ...string) string {
	return strings.Join(elems, sep)
}

// Lines joins arguments with newline separators.
// Convenience for With using "\n" as separator.
// Useful for building multi-line strings.
func Lines(args ...any) string { return With(newline, args...) }

// Number concatenates numeric values without separators.
// Generic over Numeric types.
// Equivalent to NumberWith with empty sep.
func Number[T Numeric](a ...T) string {
	return NumberWith(empty, a...)
}

// NumberWith concatenates numeric values with the provided separator.
// Generic over Numeric types.
// If no args, returns empty string.
// Uses pooled Builder if enabled, with rough growth estimate (8 bytes per item).
// Relies on valueToString for numeric conversion.
func NumberWith[T Numeric](sep string, a ...T) string {
	if len(a) == 0 {
		return empty
	}

	b := New(sep)
	b.Grow(len(a) * 8)
	for _, v := range a {
		b.Add(v)
	}
	return b.Output()
}

// Path joins arguments with "/" separators.
// Convenience for With using "/" as separator.
// Useful for building file paths or URLs.
func Path(args ...any) string { return With(slash, args...) }

// Prefix concatenates with a prefix (no separator).
// Equivalent to PrefixWith with empty sep.
func Prefix(p any, args ...any) string {
	return PrefixWith(empty, p, args...)
}

// PrefixWith concatenates with a prefix and separator.
// Adds p, then sep (if args present and sep not empty), then joins args with sep.
// Uses pooled Builder if enabled.
func PrefixWith(sep string, p any, args ...any) string {
	b := New(sep)
	b.Grow(estimateWith(sep, args) + estimate([]any{p}))
	b.Add(p)
	b.Add(args...)
	return b.Output()
}

// PrefixEach applies the same prefix to each argument and joins the pairs with sep.
// Example: PrefixEach("pre-", ",", "a","b") => "pre-a,pre-b"
// Preestimates size including prefixes and seps.
// Uses pooled Builder if enabled; manually adds sep between pairs, no sep between p and a.
// Returns empty if no args.
func PrefixEach(p any, sep string, args ...any) string {
	if len(args) == 0 {
		return empty
	}
	pSize := estimate([]any{p})
	total := len(sep)*(len(args)-1) + estimate(args) + pSize*len(args)

	b := New(empty)
	b.Grow(total)
	for i, a := range args {
		if i > 0 && sep != empty {
			b.buf.WriteString(sep)
		}
		write(&b.buf, p)
		write(&b.buf, a)
	}
	return b.Output()
}

// Pair joins exactly two values (no separator).
// Equivalent to PairWith with empty sep.
func Pair(a, b any) string {
	return PairWith(empty, a, b)
}

// PairWith joins exactly two values with a separator.
// Optimized for two args: uses With(sep, a, b).
func PairWith(sep string, a, b any) string {
	return With(sep, a, b)
}

// Quote wraps each argument in double quotes, separated by spaces.
// Equivalent to QuoteWith with '"' as quote.
func Quote(args ...any) string {
	return QuoteWith('"', args...)
}

// QuoteWith wraps each argument with the specified quote byte, separated by spaces.
// Wraps each arg with quote, writes arg, closes with quote; joins with space.
// Preestimates with quotes and spaces.
// Uses pooled Builder if enabled.
func QuoteWith(quote byte, args ...any) string {
	if len(args) == 0 {
		return empty
	}
	total := estimate(args) + 2*len(args) + len(space)*(len(args)-1)

	b := New(empty)
	b.Grow(total)
	need := false
	for _, a := range args {
		if need {
			b.buf.WriteString(space)
		}
		b.buf.WriteByte(quote)
		write(&b.buf, a)
		b.buf.WriteByte(quote)
		need = true
	}
	return b.Output()
}

// Repeat concatenates val n times (no sep between instances).
// Equivalent to RepeatWith with empty sep.
func Repeat(val any, n int) string {
	return RepeatWith(empty, val, n)
}

// RepeatWith concatenates val n times with sep between each instance.
// If n <= 0, returns an empty string.
// Optimized to make exactly one allocation; converts val once.
// Uses pooled Builder if enabled.
func RepeatWith(sep string, val any, n int) string {
	if n <= 0 {
		return empty
	}
	if n == 1 {
		return valueToString(val)
	}
	b := New(sep)
	b.Grow(n*estimate([]any{val}) + (n-1)*len(sep))
	for i := 0; i < n; i++ {
		b.Add(val)
	}
	return b.Output()
}

// Reflect converts a reflect.Value to its string representation.
// It handles all kinds of reflected values including primitives, structs, slices, maps, etc.
// For nil values, it returns the nilString constant ("<nil>").
// For unexported or inaccessible fields, it returns unexportedString ("<?>").
// The output follows Go's syntax conventions where applicable (e.g., slices as [a, b], maps as {k:v}).
func Reflect(r reflect.Value) string {
	if !r.IsValid() {
		return nilString
	}

	var b strings.Builder
	writeReflect(&b, r.Interface(), 0)
	return b.String()
}

// Space concatenates arguments with space separators.
// Convenience for With using " " as separator.
func Space(args ...any) string { return With(space, args...) }

// Dot concatenates arguments with dot separators.
// Convenience for With using " " as separator.
func Dot(args ...any) string { return With(dot, args...) }

// Suffix concatenates with a suffix (no separator).
// Equivalent to SuffixWith with empty sep.
func Suffix(s any, args ...any) string {
	return SuffixWith(empty, s, args...)
}

// SuffixWith concatenates with a suffix and separator.
// Joins args with sep, then adds sep (if args present and sep not empty), then s.
// Uses pooled Builder if enabled.
func SuffixWith(sep string, s any, args ...any) string {
	b := New(sep)
	b.Grow(estimateWith(sep, args) + estimate([]any{s}))
	b.Add(args...)
	b.Add(s)
	return b.Output()
}

// SuffixEach applies the same suffix to each argument and joins the pairs with sep.
// Example: SuffixEach("-suf", " | ", "a","b") => "a-suf | b-suf"
// Preestimates size including suffixes and seps.
// Uses pooled Builder if enabled; manually adds sep between pairs, no sep between a and s.
// Returns empty if no args.
func SuffixEach(s any, sep string, args ...any) string {
	if len(args) == 0 {
		return empty
	}
	sSize := estimate([]any{s})
	total := len(sep)*(len(args)-1) + estimate(args) + sSize*len(args)

	b := New(empty)
	b.Grow(total)
	for i, a := range args {
		if i > 0 && sep != empty {
			b.buf.WriteString(sep)
		}
		write(&b.buf, a)
		write(&b.buf, s)
	}
	return b.Output()
}

// Sprint concatenates any values (no separators).
// Usage: Sprint("a", 1, true) → "a1true"
// Equivalent to Concat or With with an empty separator.
func Sprint(args ...any) string {
	if len(args) == 0 {
		return empty
	}
	if len(args) == 1 {
		return valueToString(args[0])
	}

	// For multiple args, use the existing Concat functionality
	return Concat(args...)
}

// Trio joins exactly three values (no separator).
// Equivalent to TrioWith with empty sep
func Trio(a, b, c any) string {
	return TrioWith(empty, a, b, c)
}

// TrioWith joins exactly three values with a separator.
// Optimized for three args: uses With(sep, a, b, c).
func TrioWith(sep string, a, b, c any) string {
	return With(sep, a, b, c)
}

// With concatenates arguments with the specified separator.
// Core concatenation function with sep.
// Optimized for zero or one arg: empty or direct valueToString.
// Fast path for all strings: exact preallocation, direct writes via raw strings.Builder (minimal branches/allocs).
// Fallback: pooled Builder with estimateWith, adds args with sep.
// Benefits from pooling if enabled for mixed types.
func With(sep string, args ...any) string {
	switch len(args) {
	case 0:
		return empty
	case 1:
		return valueToString(args[0])
	}

	// Fast path for all strings: use raw strings.Builder for speed, no pooling needed.
	allStrings := true
	totalLen := len(sep) * (len(args) - 1)
	for _, a := range args {
		if s, ok := a.(string); ok {
			totalLen += len(s)
		} else {
			allStrings = false
			break
		}
	}

	if allStrings {
		var b strings.Builder
		b.Grow(totalLen)
		b.WriteString(args[0].(string))
		for i := 1; i < len(args); i++ {
			if sep != empty {
				b.WriteString(sep)
			}
			b.WriteString(args[i].(string))
		}
		return b.String()
	}

	// Fallback for mixed types: use pooled Builder.
	b := New(sep)
	b.Grow(estimateWith(sep, args))
	b.Add(args...)
	return b.Output()
}

// Wrap encloses concatenated args between before and after strings (no inner separator).
// Equivalent to Concat(before, args..., after).
func Wrap(before, after string, args ...any) string {
	b := Start()
	b.Grow(len(before) + len(after) + estimate(args))

	b.Add(before)
	b.Add(args...)
	b.Add(after)

	return b.Output()
}

// WrapEach wraps each argument individually with before/after, concatenated without separators.
// Applies before + arg + after to each arg.
// Preestimates size; uses pooled Builder if enabled.
// Returns empty if no args.
// Useful for wrapping multiple items identically without joins.
func WrapEach(before, after string, args ...any) string {
	if len(args) == 0 {
		return empty
	}
	total := (len(before)+len(after))*len(args) + estimate(args)

	b := Start() // Use pooled builder, but we will write manually.
	b.Grow(total)
	for _, a := range args {
		write(&b.buf, before)
		write(&b.buf, a)
		write(&b.buf, after)
	}
	// No separators were ever added, so this is safe.
	b.needsSep = true // Correctly set state in case of reuse.
	return b.Output()
}

// WrapWith encloses concatenated args between before and after strings,
// joining the arguments with the provided separator.
// If no args, returns before + after.
// Builds inner with With(sep, args...), then Concat(before, inner, after).
// Benefits from pooling via With and Concat.
func WrapWith(sep, before, after string, args ...any) string {
	if len(args) == 0 {
		return before + after
	}
	// First, efficiently build the inner part.
	inner := With(sep, args...)

	// Then, wrap it without allocating another slice.
	b := Start()
	b.Grow(len(before) + len(inner) + len(after))

	b.Add(before)
	b.Add(inner)
	b.Add(after)

	return b.Output()
}

// Pad surrounds a string with spaces on both sides.
// Ensures proper spacing for SQL operators like "=", "AND", etc.
// Example: Pad("=") returns " = " for cleaner formatting.
func Pad(s string) string {
	return Concat(space, s, space)
}

// PadWith adds a separator before the string and a space after it.
// Useful for formatting SQL parts with custom leading separators.
// Example: PadWith(",", "column") returns ",column ".
func PadWith(sep, s string) string {
	return Concat(sep, s, space)
}

// Parens wraps content in parentheses
// Useful for grouping SQL conditions or expressions
// Example: Parens("a = b AND c = d") → "(a = b AND c = d)"
func Parens(content string) string {
	return Concat(parenOpen, content, parenClose)
}

// ParensWith wraps multiple arguments in parentheses with a separator
// Example: ParensWith(" AND ", "a = b", "c = d") → "(a = b AND c = d)"
func ParensWith(sep string, args ...any) string {
	return Concat(parenOpen, With(sep, args...), parenClose)
}
