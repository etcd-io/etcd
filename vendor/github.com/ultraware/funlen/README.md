# Funlen linter

Funlen is a linter that checks for long functions. It can check both on the number of lines and the number of statements.

The default limits are 60 lines and 40 statements. You can configure these.

## Description

The intent for the funlen linter is to fit a function within one screen. If you need to scroll through a long function, tracing variables back to their definition or even just finding matching brackets can become difficult.

Besides checking lines there's also a separate check for the number of statements, which gives a clearer idea of how much is actually being done in a function.

The default values are used internally, but might to be adjusted for your specific environment.

## Installation

Funlen is included in [golangci-lint](https://github.com/golangci/golangci-lint/). Install it and enable funlen.

## Configuration

Available configuration options:

```yaml
linters-settings:
  funlen:
    # Checks the number of lines in a function.
    # If lower than 0, disable the check.
    # Default: 60
    lines: 60
    # Checks the number of statements in a function.
    # If lower than 0, disable the check.
    # Default: 40
    statements: 60
    # Ignore comments when counting lines.
    # Default false
    ignore-comments: false
```

# Exclude for tests

golangci-lint offers a way to exclude linters in certain cases. More info can be found here: https://golangci-lint.run/usage/configuration/#issues-configuration.

## Disable funlen for \_test.go files

You can utilize the issues configuration in `.golangci.yml` to exclude the funlen linter for all test files:

```yaml
issues:
  exclude-rules:
    # disable funlen for all _test.go files
    - path: _test.go
      linters:
        - funlen
```

## Disable funlen only for Test funcs

If you want to keep funlen enabled for example in helper functions in test files but disable it specifically for Test funcs, you can use the following configuration:

```yaml
issues:
  exclude-rules:
    # disable funlen for test funcs
    - source: "^func Test"
      linters:
        - funlen
```
