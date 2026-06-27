# UseTesting

Detects when some calls can be replaced by methods from the testing package.

[![Sponsor](https://img.shields.io/badge/Sponsor%20me-%E2%9D%A4%EF%B8%8F-pink)](https://github.com/sponsors/ldez)

## Usages

### Inside golangci-lint

Recommended.

```yml
linters:
  enable:
    - usetesting

  settings:
      usetesting:
        # Enable/disable `os.CreateTemp("", ...)` detections.
        # Default: true
        os-create-temp: false
    
        # Enable/disable `os.MkdirTemp()` detections.
        # Default: true
        os-mkdir-temp: false
    
        # Enable/disable `os.Setenv()` detections.
        # Default: true
        os-setenv: false
    
        # Enable/disable `os.TempDir()` detections.
        # Default: false
        os-temp-dir: true
        
        # Enable/disable `os.Chdir()` detections.
        # Disabled if Go < 1.24.
        # Default: true
        os-chdir: false
    
        # Enable/disable `context.Background()` detections.
        # Disabled if Go < 1.24.
        # Default: false
        context-background: true
    
        # Enable/disable `context.TODO()` detections.
        # Disabled if Go < 1.24.
        # Default: false
        context-todo: true
```

### As a CLI

```shell
go install github.com/ldez/usetesting/cmd/usetesting@latest
```

```
usetesting: Reports uses of functions with replacement inside the testing package.

Usage: usetesting [-flag] [package]

Flags:
  -contextbackground
        Enable/disable context.Background() detections (default true)
  -contexttodo
        Enable/disable context.TODO() detections (default true)
  -oschdir
        Enable/disable os.Chdir() detections (default true)
  -osmkdirtemp
        Enable/disable os.MkdirTemp() detections (default true)
  -ossetenv
        Enable/disable os.Setenv() detections (default false)
  -ostempdir
        Enable/disable os.TempDir() detections (default false)
  -oscreatetemp
        Enable/disable os.CreateTemp("", ...) detections (default true)
...
```

## Examples

### `os.MkdirTemp`

```go
func TestExample(t *testing.T) {
	os.MkdirTemp("a", "b")
	// ...
}
```

It can be replaced by:

```go
func TestExample(t *testing.T) {
	t.TempDir()
    // ...
}
```

### `os.TempDir`

```go
func TestExample(t *testing.T) {
	os.TempDir()
	// ...
}
```

It can be replaced by:

```go
func TestExample(t *testing.T) {
	t.TempDir()
    // ...
}
```

### `os.CreateTemp`

```go
func TestExample(t *testing.T) {
	os.CreateTemp("", "x")
	// ...
}
```

It can be replaced by:

```go
func TestExample(t *testing.T) {
    os.CreateTemp(t.TempDir(), "x")
    // ...
}
```

### `os.Setenv`

```go
func TestExample(t *testing.T) {
	os.Setenv("A", "b")
	// ...
}
```

It can be replaced by:

```go
func TestExample(t *testing.T) {
	t.Setenv("A", "b")
    // ...
}
```

### `os.Chdir` (Go >= 1.24)

```go
func TestExample(t *testing.T) {
	os.Chdir("x")
	// ...
}
```

It can be replaced by:

```go
func TestExample(t *testing.T) {
	t.Chdir("x")
    // ...
}
```

### `context.Background` (Go >= 1.24)

```go
func TestExample(t *testing.T) {
    ctx := context.Background()
	// ...
}
```

It can be replaced by:

```go
func TestExample(t *testing.T) {
    ctx := t.Context()
    // ...
}
```

### `context.TODO` (Go >= 1.24)

```go
func TestExample(t *testing.T) {
    ctx := context.TODO()
	// ...
}
```

It can be replaced by:

```go
func TestExample(t *testing.T) {
    ctx := t.Context()
    // ...
}
```

## References

- https://tip.golang.org/doc/go1.15#testingpkgtesting (`TempDir`)
- https://tip.golang.org/doc/go1.17#testingpkgtesting (`SetEnv`)
- https://tip.golang.org/doc/go1.24#testingpkgtesting (`Chdir`, `Context`)
