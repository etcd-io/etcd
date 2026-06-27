package cat

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unsafe"
)

// write writes a value to the given strings.Builder using fast paths to avoid temporary allocations.
// It handles common types like strings, byte slices, integers, floats, and booleans directly for efficiency.
// For other types, it falls back to fmt.Fprint, which may involve allocations.
// This function is optimized for performance in string concatenation scenarios, prioritizing
// common cases like strings and numbers at the top of the type switch for compiler optimization.
// Note: For integers and floats, it uses stack-allocated buffers and strconv.Append* functions to
// convert numbers to strings without heap allocations.
func write(b *strings.Builder, arg any) {
	writeValue(b, arg, 0)
}

// writeValue appends the string representation of arg to b, handling recursion with a depth limit.
// It serves as a recursive helper for write, directly handling primitives and delegating complex
// types to writeReflect. The depth parameter prevents excessive recursion in deeply nested structures.
func writeValue(b *strings.Builder, arg any, depth int) {
	// Handle recursion depth limit
	if depth > maxRecursionDepth {
		b.WriteString("...")
		return
	}

	// Handle nil values
	if arg == nil {
		b.WriteString(nilString)
		return
	}

	// Fast path type switch for all primitive types
	switch v := arg.(type) {
	case string:
		b.WriteString(v)
	case []byte:
		b.WriteString(bytesToString(v))
	case int:
		var buf [20]byte
		b.Write(strconv.AppendInt(buf[:0], int64(v), 10))
	case int64:
		var buf [20]byte
		b.Write(strconv.AppendInt(buf[:0], v, 10))
	case int32:
		var buf [11]byte
		b.Write(strconv.AppendInt(buf[:0], int64(v), 10))
	case int16:
		var buf [6]byte
		b.Write(strconv.AppendInt(buf[:0], int64(v), 10))
	case int8:
		var buf [4]byte
		b.Write(strconv.AppendInt(buf[:0], int64(v), 10))
	case uint:
		var buf [20]byte
		b.Write(strconv.AppendUint(buf[:0], uint64(v), 10))
	case uint64:
		var buf [20]byte
		b.Write(strconv.AppendUint(buf[:0], v, 10))
	case uint32:
		var buf [10]byte
		b.Write(strconv.AppendUint(buf[:0], uint64(v), 10))
	case uint16:
		var buf [5]byte
		b.Write(strconv.AppendUint(buf[:0], uint64(v), 10))
	case uint8:
		var buf [3]byte
		b.Write(strconv.AppendUint(buf[:0], uint64(v), 10))
	case float64:
		var buf [24]byte
		b.Write(strconv.AppendFloat(buf[:0], v, 'f', -1, 64))
	case float32:
		var buf [24]byte
		b.Write(strconv.AppendFloat(buf[:0], float64(v), 'f', -1, 32))
	case bool:
		if v {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case fmt.Stringer:
		b.WriteString(v.String())
	case error:
		b.WriteString(v.Error())
	default:
		// Fallback to reflection-based handling
		writeReflect(b, arg, depth)
	}
}

// writeReflect handles all complex types safely.
func writeReflect(b *strings.Builder, arg any, depth int) {
	defer func() {
		if r := recover(); r != nil {
			b.WriteString("[!reflect panic!]")
		}
	}()

	val := reflect.ValueOf(arg)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			b.WriteString(nilString)
			return
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Slice, reflect.Array:
		b.WriteByte('[')
		for i := 0; i < val.Len(); i++ {
			if i > 0 {
				b.WriteString(", ") // Use comma-space for readability
			}
			writeValue(b, val.Index(i).Interface(), depth+1)
		}
		b.WriteByte(']')

	case reflect.Struct:
		typ := val.Type()
		b.WriteByte('{') // Use {} for structs to follow Go convention
		first := true
		for i := 0; i < val.NumField(); i++ {
			fieldValue := val.Field(i)
			if !fieldValue.CanInterface() {
				continue // Skip unexported fields
			}
			if !first {
				b.WriteByte(' ') // Use space as separator
			}
			first = false
			b.WriteString(typ.Field(i).Name)
			b.WriteByte(':')

			writeValue(b, fieldValue.Interface(), depth+1)
		}
		b.WriteByte('}')

	case reflect.Map:
		b.WriteByte('{')
		keys := val.MapKeys()
		sort.Slice(keys, func(i, j int) bool {
			// A simple string-based sort for keys
			return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
		})
		for i, key := range keys {
			if i > 0 {
				b.WriteByte(' ') // Use space as separator
			}
			writeValue(b, key.Interface(), depth+1)
			b.WriteByte(':')
			writeValue(b, val.MapIndex(key).Interface(), depth+1)
		}
		b.WriteByte('}')

	case reflect.Interface:
		if val.IsNil() {
			b.WriteString(nilString)
			return
		}
		writeValue(b, val.Elem().Interface(), depth+1)

	default:
		fmt.Fprint(b, arg)
	}
}

// valueToString converts any value to a string representation.
// It uses optimized paths for common types to avoid unnecessary allocations.
// For types like integers and floats, it directly uses strconv functions.
// This function is useful for single-argument conversions or as a helper in other parts of the package.
// Unlike write, it returns a string instead of appending to a builder.
func valueToString(arg any) string {
	switch v := arg.(type) {
	case string:
		return v
	case []byte:
		return bytesToString(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case fmt.Stringer:
		return v.String()
	case error:
		return v.Error()
	default:
		return fmt.Sprint(v)
	}
}

// estimateWith calculates a conservative estimate of the total string length when concatenating
// the given arguments with a separator. This is used for preallocating capacity in strings.Builder
// to minimize reallocations during building.
// It accounts for the length of separators and estimates the length of each argument based on its type.
// If no arguments are provided, it returns 0.
func estimateWith(sep string, args []any) int {
	if len(args) == 0 {
		return 0
	}
	size := len(sep) * (len(args) - 1)
	size += estimate(args)
	return size
}

// estimate calculates a conservative estimate of the combined string length of the given arguments.
// It iterates over each argument and adds an estimated length based on its type:
// - Strings and byte slices: exact length.
// - Numbers: calculated digit count using numLen or uNumLen.
// - Floats and others: fixed conservative estimates (e.g., 16 or 24 bytes).
// This helper is used internally by estimateWith and focuses solely on the arguments without separators.
func estimate(args []any) int {
	var size int
	for _, a := range args {
		switch v := a.(type) {
		case string:
			size += len(v)
		case []byte:
			size += len(v)
		case int:
			size += numLen(int64(v))
		case int8:
			size += numLen(int64(v))
		case int16:
			size += numLen(int64(v))
		case int32:
			size += numLen(int64(v))
		case int64:
			size += numLen(v)
		case uint:
			size += uNumLen(uint64(v))
		case uint8:
			size += uNumLen(uint64(v))
		case uint16:
			size += uNumLen(uint64(v))
		case uint32:
			size += uNumLen(uint64(v))
		case uint64:
			size += uNumLen(v)
		case float32:
			size += 16
		case float64:
			size += 24
		case bool:
			size += 5 // "false"
		case fmt.Stringer, error:
			size += 16 // conservative
		default:
			size += 16 // conservative
		}
	}
	return size
}

// numLen returns the number of characters required to represent the signed integer n as a string.
// It handles negative numbers by adding 1 for the '-' sign and uses a loop to count digits.
// Special handling for math.MinInt64 to avoid overflow when negating.
// Returns 1 for 0, and up to 20 for the largest values.
func numLen(n int64) int {
	if n == 0 {
		return 1
	}
	c := 0
	if n < 0 {
		c = 1 // for '-'
		// NOTE: math.MinInt64 negated overflows; handle by adding one digit and returning 20.
		if n == -1<<63 {
			return 20
		}
		n = -n
	}
	for n > 0 {
		n /= 10
		c++
	}
	return c
}

// uNumLen returns the number of characters required to represent the unsigned integer n as a string.
// It uses a loop to count digits.
// Returns 1 for 0, and up to 20 for the largest uint64 values.
func uNumLen(n uint64) int {
	if n == 0 {
		return 1
	}
	c := 0
	for n > 0 {
		n /= 10
		c++
	}
	return c
}

// bytesToString converts a byte slice to a string efficiently.
// If the package's UnsafeBytes flag is set (via IsUnsafeBytes()), it uses unsafe operations
// to create a string backed by the same memory as the byte slice, avoiding a copy.
// This is zero-allocation when unsafe is enabled.
// Falls back to standard string(bts) conversion otherwise.
// For empty slices, it returns a constant empty string.
// Compatible with Go 1.20+ unsafe functions like unsafe.String and unsafe.SliceData.
func bytesToString(bts []byte) string {
	if len(bts) == 0 {
		return empty
	}
	if IsUnsafeBytes() {
		// Go 1.20+: unsafe.String with SliceData (1.20 introduced, 1.22 added SliceData).
		return unsafe.String(unsafe.SliceData(bts), len(bts))
	}
	return string(bts)
}

// recursiveEstimate calculates the estimated string length for potentially nested arguments,
// including the lengths of separators between elements. It recurses on nested []any slices,
// flattening the structure while accounting for separators only between non-empty subparts.
// This function is useful for preallocating capacity in builders for nested concatenation operations.
func recursiveEstimate(sep string, args []any) int {
	if len(args) == 0 {
		return 0
	}
	size := 0
	needsSep := false
	for _, a := range args {
		switch v := a.(type) {
		case []any:
			subSize := recursiveEstimate(sep, v)
			if subSize > 0 {
				if needsSep {
					size += len(sep)
				}
				size += subSize
				needsSep = true
			}
		default:
			if needsSep {
				size += len(sep)
			}
			size += estimate([]any{a})
			needsSep = true
		}
	}
	return size
}

// recursiveAdd appends the string representations of potentially nested arguments to the builder.
// It recurses on nested []any slices, effectively flattening the structure by adding leaf values
// directly via b.Add without inserting separators (separators are handled externally if needed).
// This function is designed for efficient concatenation of nested argument lists.
func recursiveAdd(b *Builder, args []any) {
	for _, a := range args {
		switch v := a.(type) {
		case []any:
			recursiveAdd(b, v)
		default:
			b.Add(a)
		}
	}
}
