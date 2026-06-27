# gomoddirectives

A linter that handle directives into `go.mod`.

[![Sponsor](https://img.shields.io/badge/Sponsor%20me-%E2%9D%A4%EF%B8%8F-pink)](https://github.com/sponsors/ldez)
[![Build Status](https://github.com/ldez/gomoddirectives/workflows/Main/badge.svg?branch=master)](https://github.com/ldez/gomoddirectives/actions)

## Usage

### Inside golangci-lint

Recommended.

```yml
linters:
  enable:
    - gomoddirectives

  settings:
    gomoddirectives:
      # Allow local `replace` directives.
      # Default: false
      replace-local: true
      
      # List of allowed `replace` directives.
      # Default: []
      replace-allow-list:
        - launchpad.net/gocheck
      
      # Allow to not explain why the version has been retracted in the `retract` directives.
      # Default: false
      retract-allow-no-explanation: true
      
      # Forbid the use of the `exclude` directives.
      # Default: false
      exclude-forbidden: true
  
      # Forbid the use of the `ignore` directives (go >= 1.25).
      # Default: false
      ignore-forbidden: true
  
      # Forbid the use of the `toolchain` directive.
      # Default: false
      toolchain-forbidden: true
  
      # Defines a pattern to validate `toolchain` directive.
      # Default: '' (no match)
      toolchain-pattern: 'go1\.22\.\d+$'
  
      # Forbid the use of the `tool` directives.
      # Default: false
      tool-forbidden: true
  
      # Forbid the use of the `godebug` directive.
      # Default: false
      go-debug-forbidden: true
  
      # Defines a pattern to validate `go` minimum version directive.
      # Default: '' (no match)
      go-version-pattern: '1\.\d+(\.0)?$'

      # Check the validity of the module path.
      # Default: false
      check-module-path: true
```

### As a CLI

```
gomoddirectives [flags]

Flags:
  -check-module-path
        Check module path validity
  -exclude
        Forbid the use of exclude directives
  -godebug
        Forbid the use of godebug directives
  -goversion string
        Pattern to validate go min version directive
  -h    Show this help.
  -ignore
        Forbid the use of ignore directives
  -list value
        List of allowed replace directives
  -local
        Allow local replace directives
  -retract-no-explanation
        Allow to use retract directives without explanation
  -tool
        Forbid the use of tool directives
  -toolchain
        Forbid the use of toolchain directive
  -toolchain-pattern string
        Pattern to validate toolchain directive
```

## Details

### [`retract`](https://golang.org/ref/mod#go-mod-file-retract) directives

- Force explanation for `retract` directives.

```go
module example.com/foo

go 1.22

require (
	github.com/ldez/grignotin v0.4.1
)

retract (
    v1.0.0 // Explanation
)
```

### [`replace`](https://golang.org/ref/mod#go-mod-file-replace) directives

- Ban all `replace` directives.
- Allow only local `replace` directives.
- Allow only some `replace` directives.
- Detect duplicated `replace` directives.
- Detect identical `replace` directives.

```go
module example.com/foo

go 1.22

require (
	github.com/ldez/grignotin v0.4.1
)

replace github.com/ldez/grignotin => ../grignotin/
```

### [`exclude`](https://golang.org/ref/mod#go-mod-file-exclude) directives

- Ban all `exclude` directives.

```go
module example.com/foo

go 1.22

require (
	github.com/ldez/grignotin v0.4.1
)

exclude (
    golang.org/x/crypto v1.4.5
    golang.org/x/text v1.6.7
)
```

### [`ignore`](TODO) directives

- Ban all `ignore` directives.

```go
module example.com/foo

go 1.25

require (
	github.com/ldez/grignotin v0.4.1
)

ignore (
    ./foo/bar/path
    foo/bar
)
```

### [`tool`](https://golang.org/ref/mod#go-mod-file-tool) directives

- Ban all `tool` directives.

```go
module example.com/foo

go 1.24

tool (
    example.com/module/cmd/a
    example.com/module/cmd/b
)
```

### [`toolchain`](https://golang.org/ref/mod#go-mod-file-toolchain) directive

- Ban `toolchain` directive.
- Use a regular expression to constraint the Go minimum version.

```go
module example.com/foo

go 1.22

toolchain go1.23.3
```

### [`godebug`](https://go.dev/ref/mod#go-mod-file-godebug) directives

- Ban `godebug` directive.

```go
module example.com/foo

go 1.22

godebug default=go1.21
godebug (
    panicnil=1
    asynctimerchan=0
)
```

### [`go`](https://go.dev/ref/mod#go-mod-file-go) directive

- Use a regular expression to constraint the Go minimum version.

```go
module example.com/foo

go 1.22.0
```

### [`module`](https://go.dev/ref/mod#module-path) path

- Check the validity of the module path.

```go
module example.com/foo

go 1.22
```
