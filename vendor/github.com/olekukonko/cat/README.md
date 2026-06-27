# 🐱 `cat` - The Fast & Fluent String Concatenation Library for Go

> **"Because building strings shouldn't feel like herding cats"** 😼

## Why `cat`?

Go's `strings.Builder` is great, but building complex strings often feels clunky. `cat` makes string concatenation:

- **Faster** - Optimized paths for common types, zero-allocation conversions
- **Fluent** - Chainable methods for beautiful, readable code
- **Flexible** - Handles any type, nested structures, and custom formatting
- **Smart** - Automatic pooling, size estimation, and separator handling

```go
// Without cat
var b strings.Builder
b.WriteString("Hello, ")
b.WriteString(user.Name)
b.WriteString("! You have ")
b.WriteString(strconv.Itoa(count))
b.WriteString(" new messages.")
result := b.String()

// With cat
result := cat.Concat("Hello, ", user.Name, "! You have ", count, " new messages.")
```

## 🔥 Hot Features

### 1. Fluent Builder API

Build strings like a boss with method chaining:

```go
s := cat.New(", ").
    Add("apple").
    If(user.IsVIP, "golden kiwi").
    Add("orange").
    Sep(" | ").  // Change separator mid-way
    Add("banana").
    String()
// "apple, golden kiwi, orange | banana"
```

### 2. Zero-Allocation Magic

- **Pooled builders** (optional) reduce GC pressure
- **Unsafe byte conversions** (opt-in) avoid `[]byte`→`string` copies
- **Stack buffers** for numbers instead of heap allocations

```go
// Enable performance features
cat.Pool(true)             // Builder pooling
cat.SetUnsafeBytes(true)   // Zero-copy []byte conversion
```

### 3. Handles Any Type - Even Nested Ones!

No more manual type conversions:

```go
data := map[string]any{
    "id": 12345,
    "tags": []string{"go", "fast", "efficient"},
}

fmt.Println(cat.JSONPretty(data))
// {
//   "id": 12345,
//   "tags": ["go", "fast", "efficient"]
// }
```

### 4. Concatenation for Every Use Case

```go
// Simple joins
cat.With(", ", "apple", "banana", "cherry")  // "apple, banana, cherry"

// File paths
cat.Path("dir", "sub", "file.txt")  // "dir/sub/file.txt"

// CSV
cat.CSV(1, 2, 3)  // "1,2,3"

// Conditional elements
cat.Start("Hello").If(user != nil, " ", user.Name)  // "Hello" or "Hello Alice"

// Repeated patterns
cat.RepeatWith("-+", "X", 3)  // "X-+X-+X"
```

### 5. Smarter Than Your Average String Lib

```go
// Automatic nesting handling
nested := []any{"a", []any{"b", "c"}, "d"}
cat.FlattenWith(",", nested)  // "a,b,c,d"

// Precise size estimation (minimizes allocations)
b := cat.New(", ").Grow(estimatedSize)  // Preallocate exactly what you need

// Reflection support for any type
cat.Reflect(anyComplexStruct)  // "{Field1:value Field2:[1 2 3]}"
```

## 🚀 Getting Started

```bash
go get github.com/your-repo/cat
```

```go
import "github.com/your-repo/cat"

func main() {
    // Simple concatenation
    msg := cat.Concat("User ", userID, " has ", count, " items")
    
    // Pooled builder (for high-performance loops)
    builder := cat.New(", ")
    defer builder.Release() // Return to pool
    result := builder.Add(items...).String()
}
```

## 🤔 Why Not Just Use...?

- `fmt.Sprintf` - Slow, many allocations
- `strings.Join` - Only works with strings
- `bytes.Buffer` - No separator support, manual type handling
- `string +` - Even worse performance, especially in loops

## 💡 Pro Tips

1. **Enable pooling** in high-throughput scenarios
2. **Preallocate** with `.Grow()` when you know the final size
3. Use **`If()`** for conditional elements in fluent chains
4. Try **`SetUnsafeBytes(true)`** if you can guarantee byte slices won't mutate
5. **Release builders** when pooling is enabled

## 🐱‍👤 Advanced Usage

```go
// Custom value formatting
type User struct {
    Name string
    Age  int
}

func (u User) String() string {
    return cat.With(" ", u.Name, cat.Wrap("(", u.Age, ")"))
}

// JSON-like output
func JSONPretty(v any) string {
    return cat.WrapWith(",\n  ", "{\n  ", "\n}", prettyFields(v))
}
```

```text
/\_/\
( o.o )  > Concatenate with purr-fection!
> ^ <

```

**`cat`** - Because life's too short for ugly string building code. 😻