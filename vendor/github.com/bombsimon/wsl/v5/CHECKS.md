# Checks and configuration

## Table of content

- [Checks](#checks)
  - [`after-block`](#after-block)
  - [`after-decl`](#after-decl)
  - [`after-defer`](#after-defer)
  - [`after-expr`](#after-expr)
  - [`after-go`](#after-go)
  - [`append`](#append)
  - [`assign`](#assign)
  - [`assign-exclusive`](#assign-exclusive)
  - [`assign-expr`](#assign-expr)
  - [`branch`](#branch)
  - [`cuddle-group`](#cuddle-group)
  - [`decl`](#decl)
  - [`defer`](#defer)
  - [`err`](#err)
  - [`expr`](#expr)
  - [`for`](#for)
  - [`go`](#go)
  - [`if`](#if)
  - [`inc-dec`](#inc-dec)
  - [`label`](#label)
  - [`leading-whitespace`](#leading-whitespace)
  - [`range`](#range)
  - [`return`](#return)
  - [`select`](#select)
  - [`send`](#send)
  - [`switch`](#switch)
  - [`trailing-whitespace`](#trailing-whitespace)
  - [`type-switch`](#type-switch)
- [Configuration](#configuration)
  - [`allow-first-in-block`](#allow-first-in-block)
  - [`allow-whole-block`](#allow-whole-block)
  - [`branch-max-lines`](#branch-max-lines)
  - [`case-max-lines`](#case-max-lines)
  - [`cuddle-max-statements`](#cuddle-max-statements)

## Checks

This document describes all the checks done by `wsl` with examples of what's not
allowed and what's allowed.

### `after-block`

Block statements (`if`, `for`, `switch`, etc.) should be followed by a blank
line to visually separate them from subsequent code.

> [!IMPORTANT]
> An exception is made for `defer` statements that follow an `if err != nil`
> block when the defer references a variable assigned on the line above the if
> statement. This is a common pattern for resource cleanup.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
if true {
    fmt.Println("hello")
} // 1
fmt.Println("world")

for i := 0; i < 3; i++ {
    fmt.Println(i)
} // 2
x := 1
```

</td><td valign="top">

```go
if true {
    fmt.Println("hello")
}

fmt.Println("world")

for i := 0; i < 3; i++ {
    fmt.Println(i)
}

x := 1

// Exception: defer after error check
f, err := os.Open("file.txt")
if err != nil {
    return err
}
defer f.Close()
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> Missing whitespace after block

<sup>2</sup> Missing whitespace after block

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `after-decl`

Declaration statements (`var`, `const`, `type`) should be followed by a blank
line. Consecutive declarations of the same kind are allowed to cuddle, only the
last one in a run needs the blank line.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
var x int // 1
x = 1

var (
    a = 1
    b = 2
) // 2
fmt.Println(a, b)
```

</td><td valign="top">

```go
var x int

x = 1

var (
    a = 1
    b = 2
)

fmt.Println(a, b)

var a int
var b int

fmt.Println(a, b)
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> Missing whitespace after declaration

<sup>2</sup> Missing whitespace after declaration

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `after-defer`

`defer` statements should be followed by a blank line. Consecutive `defer`s
are allowed to cuddle, only the last one in a run needs the blank line.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
f, err := os.Open("x")
if err != nil {
    return err
}
defer f.Close() // 1
data := read(f)
```

</td><td valign="top">

```go
f, err := os.Open("x")
if err != nil {
    return err
}
defer f.Close()

data := read(f)

defer a()
defer b()

doWork()
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> Missing whitespace after defer

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `after-expr`

Expression statements (e.g. function calls used for their side effects) should
be followed by a blank line. Consecutive expression statements are allowed to
cuddle, only the last one in a run needs the blank line.

**Exception:** an expression statement that is immediately followed by a
`defer` referencing the same variable is exempt (e.g. `mu.Lock()` /
`defer mu.Unlock()`), since these two statements form a single logical unit.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
fmt.Println("hello") // 1
x := 5

fmt.Println(x)
```

</td><td valign="top">

```go
fmt.Println("hello")

x := 5

fmt.Println(x)

log.Info("a")
log.Info("b")

doWork()
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> Missing whitespace after expression

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `after-go`

`go` statements should be followed by a blank line. Consecutive `go`
statements are allowed to cuddle, only the last one in a run needs the blank
line.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
go work() // 1
fmt.Println("next")
```

</td><td valign="top">

```go
go work()

fmt.Println("next")

go a()
go b()

fmt.Println("next")
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> Missing whitespace after go

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `assign`

Assign (`foo := bar`) or re-assignments (`foo = bar`) should only be cuddled
with other assignments or increment/decrement.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
if true {
    fmt.Println("hello")
}
a := 1 // 1

defer func() {
    fmt.Println("hello")
}()
a := 1 // 2
```

</td><td valign="top">

```go
if true {
    fmt.Println("hello")
}

a := 1

defer func() {
    fmt.Println("hello")
}()

a := 1

a := 1
b := 2
c := 3
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> Not an assign statement above

<sup>2</sup> Not an assign statement above

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `branch`

> [!NOTE]
> Configurable via `branch-max-lines`. See [Configuration](#configuration) for
> details.

Branch statement (`break`, `continue`, `fallthrough`, `goto`) should only be
cuddled if the block is less than `n` lines where `n` is the value of
`branch-max-statements`.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
for {
    a, err : = SomeFn()
    if err != nil {
        return err
    }

    fmt.Println(a)
    break // 1
}
```

</td><td valign="top">

```go
for {
    a, err : = SomeFn()
    if err != nil {
        return err
    }

    fmt.Println(a)

    break
}

for {
    fmt.Println("hello")
    break
}
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> Block is more than 2 lines so should be a blank line above

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `decl`

Declarations should never be cuddled. When grouping multiple declarations
together they should be declared in the same group with parenthesis into a
single statement. The benefit of this is that it also aligns the declaration or
assignment increasing readability.

> [!IMPORTANT]
> The fixer can't do smart adjustments if there are comments on the same line
> as the declaration.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
var a string
var b int // 1

const a = 1
const b = 2 // 2

a := 1
var b string // 3

fmt.Println("hello")
var a string // 4
```

</td><td valign="top">

```go
var (
    a string
    b int
)

const (
    a = 1
    b = 2
)

a := 1

var b string

fmt.Println("hello")

var a string
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> Multiple declarations should be grouped to one

<sup>2</sup> Multiple declarations should be grouped to one

<sup>3</sup> Declaration should always have a whitespace above

<sup>4</sup> Declaration should always have a whitespace above

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `defer`

Deferring execution should only be used directly in the context of what's being
deferred and there should only be one statement above.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
val, closeFn := SomeFn()
val2 := fmt.Sprintf("v-%s", val)
fmt.Println(val)
defer closeFn() // 1

defer fn1()
a := 1
defer fn3() // 2

f, err := os.Open("/path/to/f.txt")
if err != nil {
   return err
}

lines := ReadFile(f)
trimLines(lines)
defer f.Close() // 3
```

</td><td valign="top">

```go
val, closeFn := SomeFn()
defer closeFn()

defer fn1()
defer fn2()
defer fn3()

f, err := os.Open("/path/to/f.txt")
if err != nil {
   return err
}
defer f.Close()

m.Lock()
defer m.Unlock()
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> More than a single statement between `defer` and `closeFn`

<sup>2</sup> `a` is not used in expression

<sup>3</sup> More than a single statement between `defer` and `f.Close`

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `expr`

Expressions can be multiple things and a big part of them are not handled by
`wsl`. However all function calls are expressions which can be verified.

> [!IMPORTANT]
> This is one of the few rules with non-configurable exceptions. Given the
> idiomatic way to acquire and release mutex locks and the fact that the `sync`
> mutex from the standard library is so widely used, any call to `Lock`,
> `RWLock`, or `TryLock` can be cuddled above any other statement(s) and
> similarly `Unlock` and `RWUnlock` can be cuddled below any other
> statement(s).

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
a := 1
b := 2
fmt.Println("not b") // 1

mu.Lock()
for _, item := range items {
    // Safely work with item
}
mu.Unlock()
```

</td><td valign="top">

```go
a := 1
b := 2

fmt.Println("not b")

a := 1
fmt.Println(a)
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> `b` is not used in expression

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `for`

> [!NOTE]
> Configurable via `allow-first-in-block` to allow cuddling if the variable is
> used _first_ in the block (enabled by default).
>
> Configurable via `allow-whole-block` to allow cuddling if the variable is used
> _anywhere_ in the following block (disabled by default).
>
> Configurable via `cuddle-max-statements` to change the maximum number of
> cuddled statements allowed (default 1).
>
> See [Configuration](#configuration) for details.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
i := 0
for j := 0; j < 3; j++ { // 1
    fmt.Println(j)
}

a := 0
i := 3
for j := 0; j < i; j++ { // 2
    fmt.Println(j)
}

x := 1
for { // 3
    fmt.Println("hello")
    break
}
```

</td><td valign="top">

```go
i := 0
for j := 0; j < i; j++ {
    fmt.Println(j)
}

a := 0

i := 3
for j := 0; j < i; j++ {
    fmt.Println(j)
}

// Allowed with `allow-first-in-block`
x := 1
for {
    x++
    break
}

// Allowed with `allow-whole-block`
x := 1
for {
    fmt.Println("hello")

    if shouldIncrement() {
        x++
    }
}
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> `i` is not used in expression

<sup>2</sup> More than one variable above statement

<sup>3</sup> No variable in expression

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `go`

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
someFunc := func() {}
go anotherFunc() // 1

x := 1
go func () // 2
    fmt.Println(y)
}()

someArg := 1
go Fn(notArg) // 3
```

</td><td valign="top">

```go
someFunc := func() {}
go someFunc()

x := 1
go func (s string) {
    fmt.Println(s)
}(x)

someArg := 1
go Fn(someArg)
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> `someFunc` is not used in expression

<sup>2</sup> `x` is not used in expression

<sup>3</sup> `someArg` is not used in expression

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `if`

> [!NOTE]
> Configurable via `allow-first-in-block` to allow cuddling if the variable is
> used _first_ in the block (enabled by default).
>
> Configurable via `allow-whole-block` to allow cuddling if the variable is used
> _anywhere_ in the following block (disabled by default).
>
> Configurable via `cuddle-max-statements` to change the maximum number of
> cuddled statements allowed (default 1).
>
> See [Configuration](#configuration) for details.

`if` statements are one of several block statements (a statement with a block)
that can have some form of expression or condition. To make block context more
readable, only one variable is allowed immediately above the `if` statement and
the variable must be used in the condition (unless configured otherwise).

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
x := 1
if y > 1 { // 1
    fmt.Println("y > 1")
}

a := 1
b := 2
if b > 1 { // 2
    fmt.Println("a > 1")
}

a := 1
b := 2
if a > 1 { // 3
    fmt.Println("a > 1")
}

a := 1
b := 2
if notEvenAOrB() { // 4
    fmt.Println("not a or b")
}

a := 1
x, err := SomeFn() // 5
if err != nil {
    return err
}
```

</td><td valign="top">

```go
x := 1

if y > 1 {
    fmt.Println("y > 1")
}

a := 1

b := 2
if b > 1 {
    fmt.Println("a > 1")
}

b := 2

a := 1
if a > 1 {
    fmt.Println("a > b")
}

a := 1
b := 2

if notEvenAOrB() {
    fmt.Println("not a or b")
}

a := 1

x, err := SomeFn()
if err != nil {
    return err
}

// Allowed with `allow-first-in-block`
x := 1
if xUsedFirstInBlock() {
    x = 2
}

// Allowed with `allow-whole-block`
x := 1
if xUsedLaterInBlock() {
    fmt.Println("will use x later")

    if orEvenNestedWouldWork() {
        x = 3
    }
}
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> `x` is not used in expression

<sup>2</sup> More than one variable above statement

<sup>3</sup> `b` is not used in expression and too many statements

<sup>4</sup> No variable in expression

<sup>5</sup> More than one variable above statement

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `inc-dec`

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
i := 1

if true {
    fmt.Println("hello")
}
i++ // 1

defer func() {
    fmt.Println("hello")
}()
i++ // 2
```

</td><td valign="top">

```go
i := 1
i++

i--
j := i
j++
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> Not an assign or inc/dec statement above

<sup>2</sup> Not an assign or inc/dec statement above

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `label`

Labels should never be cuddled. Labels in itself is often a symptom of big scope
and split context and because of that should always have an empty line above.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
L1:
    if true {
        _ = 1
    }
L2: // 1
    if true {
        _ = 1
    }
```

</td><td valign="top">

```go
L1:
    if true {
        _ = 1
    }

L2:
    if true {
        _ = 1
    }
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> Labels should always have a whitespabe above

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `range`

> [!NOTE]
> Configurable via `allow-first-in-block` to allow cuddling if the variable is
> used _first_ in the block (enabled by default).
>
> Configurable via `allow-whole-block` to allow cuddling if the variable is used
> _anywhere_ in the following block (disabled by default).
>
> Configurable via `cuddle-max-statements` to change the maximum number of
> cuddled statements allowed (default 1).
>
> See [Configuration](#configuration) for details.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
someRange := []int{1, 2, 3}
for _, i := range thisIsNotSomeRange { // 1
    fmt.Println(i)
}

x := 1
for i := range make([]int, 3) { // 2
    fmt.Println("hello")
    break
}

s1 := []int{1, 2, 3}
s2 := []int{3, 2, 1}
for _, v := range s2 { // 3
    fmt.Println(v)
}
```

</td><td valign="top">

```go
someRange := []int{1, 2, 3}

for _, i := range thisIsNotSomeRange {
    fmt.Println(i)
}

someRange := []int{1, 2, 3}
for _, i := range someRange {
    fmt.Println(i)
}

notARange := 1
for i := range returnsRange(notARange) {
    fmt.Println(i)
}

s1 := []int{1, 2, 3}

s2 := []int{3, 2, 1}
for _, v := range s2 {
    fmt.Println(v)
}
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> `someRange` is not used in expression

<sup>2</sup> `x` is not used in expression

<sup>3</sup> More than one variable above statement

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `return`

> [!NOTE]
> Configurable via `branch-max-lines`. See [Configuration](#configuration) for
> details.

Return statements is an important statement that is easiy to miss in larger code
blocks. To better visualize the `return` statement and that the method is
returning it should always be followed by a blank line unless the scope is as
small as `branch-max-lines`.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
func Fn() int {
    x, err := someFn()
    if err != nil {
        panic(err)
    }

    fmt.Println(x)
    return // 1
}
```

</td><td valign="top">

```go
func Fn() int {
    x, err := someFn()
    if err != nil {
        panic(err)
    }

    fmt.Println(x)

    return
}
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> Block is more than 2 lines so should be a blank line above

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `select`

Identifiers used in case arms of select statements are allowed to be cuddled.

> [!NOTE]
> Configurable via `allow-first-in-block` to allow cuddling if the variable is
> used _first_ in the block (enabled by default).
>
> Configurable via `allow-whole-block` to allow cuddling if the variable is used
> _anywhere_ in the following block (disabled by default).
>
> Configurable via `cuddle-max-statements` to change the maximum number of
> cuddled statements allowed (default 1).
>
> See [Configuration](#configuration) for details.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
x := 0
select { // 1
case <-time.After(time.Second):
    // ...
case <-stop:
    // ...
}
```

</td><td valign="top">

```go
stop := make(chan struct{})
select {
case <-time.After(time.Second):
    // ...
case <-stop:
    // ...
}

x := 0

select {
case <-time.After(time.Second):
    // ...
case <-stop:
    // ...
}

// Allowed with `allow-whole-block`
x := 1
select {
case <-time.After(time.Second):
    // ...
case <-stop:
    Fn(x)
}
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> `x` is not used in expression

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `send`

Send statements should only be cuddled with a single variable that is used on
the line above.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
a := 1
ch <- 1 // 1

b := 2
<-ch // 2
```

</td><td valign="top">

```go
a := 1
ch <- a

b := 1

<-ch
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> `a` is not used in expression

<sup>2</sup> `b` is not used in expression

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `switch`

In addition to checking the switch condition, switch statements also checks
identifiers in all case arms. If a variable is used in one or more of the case
arms it's allowed to be cuddled.

> [!NOTE]
> Configurable via `allow-first-in-block` to allow cuddling if the variable is
> used _first_ in the block (enabled by default).
>
> Configurable via `allow-whole-block` to allow cuddling if the variable is used
> _anywhere_ in the following block (disabled by default).
>
> Configurable via `cuddle-max-statements` to change the maximum number of
> cuddled statements allowed (default 1).
>
> See [Configuration](#configuration) for details.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
x := 0
switch y { // 1
case 1:
    // ...
case 2:
    // ...
}


x := 0
y := 1
switch y { // 2
case 1:
    // ...
case 2:
    // ...
}
```

</td><td valign="top">

```go
n := 1
switch n {
case 1:
    // ...
case 2:
    // ...
}

n := 1
switch {
case n < 1:
    // ...
case n > 1:
    // ...
}

x := 0

switch y {
case 1:
    // ...
case 2:
    // ...
}


x := 0

y := 1
switch y {
case 1:
    // ...
case 2:
    // ...
}

// Allowed with `allow-whole-block`
x := 1
switch y {
case 1:
    // ...
case 2:
    fmt.Println(x)
}
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> `x` is not used in expression

<sup>2</sup> More than one variable above statement

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `type-switch`

> [!NOTE]
> Configurable via `allow-first-in-block` to allow cuddling if the variable is
> used _first_ in the block (enabled by default).
>
> Configurable via `allow-whole-block` to allow cuddling if the variable is used
> _anywhere_ in the following block (disabled by default).
>
> Configurable via `cuddle-max-statements` to change the maximum number of
> cuddled statements allowed (default 1).
>
> See [Configuration](#configuration) for details.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
x := someType()
switch y.(type) { // 1
case int32:
    // ...
case int64:
    // ...
}


x := 0
y := someType()
switch y.(type) {
case int32:
    // ...
case int64:
    // ...
}
```

</td><td valign="top">

```go
x := someType()

switch y.(type) {
case int32:
    // ...
case int64:
    // ...
}


x := 0

y := someType()
switch y.(type) {
case int32:
    // ...
case int64:
    // ...
}

// Allowed with `allow-whole-block`
x := 1
switch y.(type) {
case int32:
    // ...
case int64:
    fmt.Println(x)
}
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> `x` is not used in expression

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `append`

Append enables strict `append` checking where assignments that are
re-assignments with `append` (e.g. `x = append(x, y)`) is only allowed to be
cuddled with other assignments if the `append` uses the variable on the line
above.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
s := []string{}

a := 1
s = append(s, 2) // 1
b := 3
s = append(s, a) // 2
```

</td><td valign="top">

```go
s := []string{}

a := 1
s = append(s, a)

b := 3

s = append(s, 2)
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> `a` is not used in append

<sup>2</sup> `b` is not used in append

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `assign-exclusive`

Assign exclusive does not allow mixing new assignments (`:=`) with
re-assignments (`=`).

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
a := 1
b = 2  // 1
c := 3 // 2
d = 4  // 3
```

</td><td valign="top">

```go
a := 1
c := 3

b = 2
d = 4
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> `a` is not a re-assignment

<sup>2</sup> `b` is not a new assignment

<sup>3</sup> `c` is not a re-assignment

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `assign-expr`

Assignments are allowed to be cuddled with expressions, primarily to support
mixing assignments and function calls which can often make sense in shorter
flows. By enabling this check `wsl` will ensure assignments are not cuddled with
expressions.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
t1.Fn1()
x := t1.Fn2() // 1
t1.Fn3()
```

</td><td valign="top">

```go
t1.Fn1()

x := t1.Fn2()
t1.Fn3()
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> Line above is not an assignment

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `err`

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
_, err := SomeFn()

if err != nil { // 1
    return fmt.Errorf("failed to fn: %w", err)
}
```

</td><td valign="top">

```go
_, err := SomeFn()
if err != nil {
    return fmt.Errorf("failed to fn: %w", err)
}
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> Whitespace between error assignment and error checking

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `cuddle-group`

Treats the cuddled chain above a trigger statement (`if`, `for`, `switch`,
etc., `go`, `defer`, `send`) as a single unit. The chain stays cuddled with
the trigger only when _every_ cuddled statement shares a variable with the
trigger _and_ the number of sharing statements is within
`cuddle-max-statements`.

Without this check, the same violations are reported on a cuddled statement
inside the chain, splitting the group between variables.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
// With the default cuddle-max-statements: 1
a := 1
b := 2
if a > b { // 1
    fmt.Println("ok")
}

// Mid-chain non-sharing
notUsed := 1
b := 2
if b > 0 { // 2
    fmt.Println("ok")
}
```

</td><td valign="top">

```go
// With the default cuddle-max-statements: 1
a := 1
b := 2

if a > b {
    fmt.Println("ok")
}

// Without cuddle-group, the same input is fixed by
// splitting between the cuddled variables instead:
a := 1

b := 2
if a > b {
    fmt.Println("ok")
}

// Mid-chain non-sharing — group still separated as
// a unit:
notUsed := 1
b := 2

if b > 0 {
    fmt.Println("ok")
}
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> Two cuddled statements share a variable with `if`; with
`cuddle-group` enabled the group stays cuddled and the blank line goes above
the `if` instead of between the variables

<sup>2</sup> Only one cuddled statement shares with `if`, but a non-sharing
stmt (`notUsed`) is in the chain so `cuddle-group` separates the entire group
from the `if` rather than splitting between `notUsed` and `b`

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `leading-whitespace`

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
if true {

    fmt.Println("hello")
}
```

</td><td valign="top">

```go
if true {
    fmt.Println("hello")
}
```

</td></tr>

</tbody></table>

[🔝](#table-of-content)

### `trailing-whitespace`

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
if true {
    fmt.Println("hello")

}
```

</td><td valign="top">

```go
if true {
    fmt.Println("hello")
}
```

</td></tr>

</tbody></table>

[🔝](#table-of-content)

## Configuration

One shared logic across different checks is the logic around statements
containing a block, i.e. a statement with a following `{}` (e.g. `if`, `for`,
`switch` etc).

`wsl` only allows one statement immediately above and that statement must also
be referenced in the expression in the statement with the block. E.g.

```go
someVariable := true
if someVariable {
    // Here `someVariable` used in the `if` expression is the only variable
    // immediately above the statement.
}
```

This can be configured to be more "laxed" by also allowing a single statement
immediately above if it's used either first in the following block or anywhere
inside the following block.

### `allow-first-in-block`

By setting this to true (default), the variable doesn't have to be used in the
expression itself but is also allowed if it's the first statement in the block
body.

```go
someVariable := 1
if anotherVariable {
    someVariable++
}
```

[🔝](#table-of-content)

### `allow-whole-block`

This is similar to `allow-first-in-block` but now allows the lack of whitespace
if it's used anywhere in the following block.

```go
someVariable := 1
if anotherVariable {
    someFn(yetAnotherVariable)

    if stillNotSomeVariable {
        someVariable++
    }
}
```

[🔝](#table-of-content)

### `branch-max-lines`

When set to a value greater than 0, `return`, `break`, `continue`, `fallthrough`
and `goto` statements that appear in a block with more than this many lines will
require a blank line above them. The default is 2, meaning blocks of 3 or more
lines require a blank line above the branch statement.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
// With branch-max-lines: 2 (default)
func Fn() {
    a, err := someFn()
    if err != nil {
        panic(err)
    }

    fmt.Println(a)
    return // 1
}
```

</td><td valign="top">

```go
// With branch-max-lines: 2 (default)
func Fn() {
    a, err := someFn()
    if err != nil {
        panic(err)
    }

    fmt.Println(a)

    return
}

// Short block: no blank line required
func ShortFn() int {
    x := compute()
    return x
}
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> Block has more than 2 lines, blank line required above

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `case-max-lines`

When set to a value greater than 0, case clauses in `switch` and `select`
statements that exceed this number of lines will require a blank line before the
next case. Setting this to 1 will make it always enabled.

Comments between case clauses are handled based on their indentation:

- **Indented comments** (deeper than `case`) are treated as trailing comments
  that belong to the current case body.
- **Left-aligned comments** (at the same level as `case`) are treated as leading
  comments that belong to the next case.

The blank line is placed at the transition point between trailing and leading
content. This means:

- If all comments are indented, the blank line goes before the next `case`.
- If all comments are left-aligned, the blank line goes after the last
  statement.
- If comments transition from indented to left-aligned, the blank line goes at
  the transition point.

Additionally, left-aligned comments must be flush against the next `case` - no
blank line is allowed between them. This ensures consistent formatting where
leading comments are visually attached to the case they describe.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
switch n {
case 1:
    fmt.Println("hello")
case 2: // 1
    fmt.Println("world")
}

switch n {
case 1:
    fmt.Println("hello")
    // Trailing comment
case 2: // 2
    fmt.Println("world")
}

switch n {
case 1:
    fmt.Println("hello")
    // Trailing comment
// Leading comment
case 2: // 3
    fmt.Println("world")
}

switch n {
case 1:
    fmt.Println("hello")
// Leading comment

case 2: // 4
    fmt.Println("world")
}
```

</td><td valign="top">

```go
switch n {
case 1:
    fmt.Println("hello")

case 2:
    fmt.Println("world")
}

switch n {
case 1:
    fmt.Println("hello")
    // Trailing comment

case 2:
    fmt.Println("world")
}

switch n {
case 1:
    fmt.Println("hello")
    // Trailing comment

// Leading comment
case 2:
    fmt.Println("world")
}

switch n {
case 1:
    fmt.Println("hello")

// Leading comment
case 2:
    fmt.Println("world")
}
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> Missing blank line after case body

<sup>2</sup> Missing blank line after trailing comment

<sup>3</sup> Missing blank line at transition (after trailing comment)

<sup>4</sup> Unnecessary blank line before case (after leading comment)

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)

### `cuddle-max-statements`

Controls the maximum number of consecutive statements that may be cuddled
(appear without a blank line) immediately above block statements (`if`, `for`,
`switch`, etc.), `go`, `defer`, and `send`. The default is 1. Every cuddled
statement must share at least one variable with the following block (respects
`allow-first-in-block` and `allow-whole-block`).

Setting it to `0` disallows any cuddling, the trigger always requires a blank
line above it, even when the variable on the line above is used by the block.
The recommended way to allow any number of statements is to set a really high
number such as `9999`.

<table>
<thead><tr><th>Bad</th><th>Good</th></tr></thead>
<tbody>
<tr><td valign="top">

```go
// With cuddle-max-statements: 1 (default)
a := 1
b := 2
if a < b { // 1
    fmt.Println("ok")
}

a := 1
b := 2
c := 3
if a+b+c > 0 { // 2
    fmt.Println("ok")
}

// With cuddle-max-statements: 0
a := 1
if a > 0 { // 3
    fmt.Println("ok")
}
```

</td><td valign="top">

```go
// With cuddle-max-statements: 1 (default)
a := 1

b := 2
if a < b {
    fmt.Println("ok")
}

// With cuddle-max-statements: 2
a := 1
b := 2
if a < b {
    fmt.Println("ok")
}

// With cuddle-max-statements: 0
a := 1

if a > 0 {
    fmt.Println("ok")
}
```

</td></tr>

<tr><td valign="top">

<sup>1</sup> Two statements cuddled above `if`, exceeds default limit of 1

<sup>2</sup> Three statements cuddled above `if`, exceeds limit of 2

<sup>3</sup> One statement cuddled above `if`; with `0` even a single shared
variable still requires a blank line above the trigger

</td><td valign="top">

</td></tr>
</tbody></table>

[🔝](#table-of-content)
