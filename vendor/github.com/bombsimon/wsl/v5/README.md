# wsl - whitespace linter

[![GitHub Actions](https://github.com/bombsimon/wsl/actions/workflows/go.yml/badge.svg)](https://github.com/bombsimon/wsl/actions/workflows/go.yml)
[![Coverage Status](https://coveralls.io/repos/github/bombsimon/wsl/badge.svg?branch=main)](https://coveralls.io/github/bombsimon/wsl?branch=main)

`wsl` (**w**hite**s**pace **l**inter) is a linter that wants you to use empty
lines to separate grouping of different types to increase readability. There are
also a few places where it encourages you to _remove_ whitespaces which is at
the start and the end of blocks or between assigning and error checking.

## Checks and configuration

Each check can be disabled or enabled individually to the point where no checks
can be run. The idea with this is to attract more users. Some checks have
configuration that affect how they work but most of them can only be turned on
or off.

### Checks

This is an exhaustive list of all the checks that can be enabled or disabled and
their default value. The names are the same as the Go
[AST](https://pkg.go.dev/go/ast) type name for built-ins.

The base rule is that statements that has a block (e.g. `for`, `range`,
`switch`, `if` etc) should only be cuddled with variables that are used in the
block. By default only a single cuddled statement is allowed, but this can be
changed with [`cuddle-max-statements`](#configuration).

For more details and examples, see [CHECKS](CHECKS.md).

✅ = enabled by default, ❌ = disabled by default

#### Built-ins and keywords

- ✅ **assign** - Assignments should only be cuddled with other assignments,
  or increment/decrement
- ✅ **branch** - Branch statement (`break`, `continue`, `fallthrough`, `goto`)
  should only be cuddled if the block is less than `n` lines where `n` is the
  value of [`branch-max-lines`](#configuration)
- ✅ **decl** - Declarations should never be cuddled
- ✅ **defer** - Defer should only be cuddled with other `defer`, after error
  checking or with a single variable used on the line above
- ✅ **expr** - Expressions are e.g. function calls or index expressions, they
  should only be cuddled with variables used on the line above
- ✅ **for** - For loops should only be cuddled with a single variable used on
  the line above
- ✅ **go** - Go should only be cuddled with other `go` or a single variable
  used on the line above
- ✅ **if** - If should only be cuddled with a single variable used on the line
  above
- ✅ **inc-dec** - Increment/decrement (`++/--`) has the same rules as `assign`
- ✅ **label** - Labels should never be cuddled
- ✅ **range** - Range should only be cuddled with a single variable used on the
  line above
- ✅ **return** - Return should only be cuddled if the block is less than `n`
  lines where `n` is the value of [`branch-max-lines`](#configuration)
- ✅ **select** - Select should only be cuddled with a single variable used on
  the line above
- ✅ **send** - Send should only be cuddled with a single variable used on the
  line above
- ✅ **switch** - Switch should only be cuddled with a single variable used on
  the line above
- ✅ **type-switch** - Type switch should only be cuddled with a single variable
  used on the line above

#### Specific `wsl` cases

- ❌ **after-block** - Require empty line after block statements
- ❌ **after-decl** - Require empty line after declaration (`var`, `const`,
  `type`) statements
- ❌ **after-defer** - Require empty line after `defer` statements
- ❌ **after-expr** - Require empty line after expression statements
- ❌ **after-go** - Require empty line after `go` statements
- ✅ **append** - Only allow re-assigning with `append` if the value being
  appended exist on the line above
- ❌ **assign-exclusive** - Only allow cuddling either new variables or
  re-assigning of existing ones
- ❌ **assign-expr** - Don't allow assignments to be cuddled with expressions,
  e.g. function calls
- ❌ **cuddle-group** - Treat the cuddled chain as a unit; separate the whole
  group from the block instead of splitting between cuddled variables
- ✅ **err** - Error checking must follow immediately after the error variable
  is assigned
- ✅ **leading-whitespace** - Disallow leading empty lines in blocks
- ✅ **trailing-whitespace** - Disallow trailing empty lines in blocks

### Configuration

Other than enabling or disabling specific checks some checks can be configured
in more details. See [CHECKS.md](CHECKS.md#configuration) for details and
examples.

- ✅ **allow-first-in-block** - Allow cuddling a variable if it's used first in
  the immediate following block, even if the statement with the block doesn't
  use the variable
- ❌ **allow-whole-block** - Same as above, but allows cuddling if the variable
  is used _anywhere_ in the following (or nested) block
- **branch-max-lines** - If a block contains more than this number of lines the
  branch statement (e.g. `return`, `break`, `continue`) need to be separated by
  a whitespace (default 2)
- **case-max-lines** - If set to a non negative number, `case` blocks needs to
  end with a whitespace if exceeding this number (default 0, 0 = off, 1 =
  always)
- **cuddle-max-statements** - Max number of cuddled statements allowed above
  block statements, `go`, `defer` and `send`. Every cuddled statement must have
  at least one variable used in the block. Respects `allow-first-in-block` and
  `allow-whole-block`. With `0` no cuddling is allowed at all — every
  cuddle-checked trigger requires a blank line above it (default 1)
- ❌ **include-generated** - Include generated files when checking

## Installation

```sh
# Latest release
go install github.com/bombsimon/wsl/v5/cmd/wsl@latest

# Main branch
go install github.com/bombsimon/wsl/v5/cmd/wsl@main
```

## Usage

> **Note**: This linter provides a fixer that can fix most issues with the
> `--fix` flag.

`wsl` uses the [analysis] package meaning it will operate on package level with
the default analysis flags and way of working.

```sh
wsl --help
wsl [flags] </path/to/package/...>

wsl --default none --enable branch,return --fix ./...
```

`wsl` is also integrated in [`golangci-lint`][golangci-lint] but since v5 which
had a bunch of breaking changes it's renamed to `wsl_v5`. The previous version
of `wsl` is deprecated and will be removed from `golangci-lint` eventually.

```sh
golangci-lint run --no-config --enable-only wsl_v5 --fix
```

This is an exhaustive, default, configuration for `wsl_v5` in `golangci-lint`.

```yaml
linters:
  default: none
  enable:
    - wsl_v5

  settings:
    wsl_v5:
      allow-first-in-block: true
      allow-whole-block: false
      branch-max-lines: 2
      case-max-lines: 0
      cuddle-max-statements: 1
      default: ~ # Can be `all`, `none`, `default` or empty
      enable:
        - append
        - assign
        - branch
        - decl
        - defer
        - err
        - expr
        - for
        - go
        - if
        - inc-dec
        - label
        - range
        - return
        - select
        - send
        - switch
        - type-switch
        - leading-whitespace
        - trailing-whitespace
      disable:
        - after-block
        - after-decl
        - after-defer
        - after-expr
        - after-go
        - assign-exclusive
        - assign-expr
        - cuddle-group
```

## See also

- [`newline-after-block`][newline-after-block] - Require newline after blocks
- [`nlreturn`][nlreturn] - Use empty lines before `return`
- [`whitespace`][whitespace] - Don't use a blank newline at the start or end of
  a block.
- [`gofumpt`][gofumpt] - Stricter formatter than `gofmt`.

  [analysis]: https://pkg.go.dev/golang.org/x/tools/go/analysis
  [gofumpt]: https://github.com/mvdan/gofumpt
  [golangci-lint]: https://golangci-lint.run
  [newline-after-block]: https://github.com/breml/newline-after-block
  [nlreturn]: https://github.com/ssgreg/nlreturn
  [whitespace]: https://github.com/ultraware/whitespace
