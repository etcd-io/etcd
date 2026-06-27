# Decorder

A declaration order linter for Go. In case of this tool declarations are `type`, `const`, `var` and `func`.

## Rules

This linter applies multiple rules where each can be disabled via cli parameter.

| rule               | description                                                                                                                                                                                                        | cli-options                                                                                                                                                                                                                                                                                |
|--------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| declaration order  | Enforces the order of global declarations (e.g. all global constants are always defined before variables). <br>You can also define a subset of declarations if you don't want to enforce the order of all of them. | * disable all checks: `-disable-dec-order-check` <br> * disable type checks: `-disable-type-dec-order-check` <br> * disable const checks: `-disable-const-dec-order-check` <br> * disable var checks: `-disable-var-dec-order-check` <br> * custom order: `-dec-order var,const,func,type` |
| declaration number | Enforces that the statements const, var and type are only used once per file. You have to use parenthesis <br>to declare e.g multiple global types inside a file.                                                  | disable check: `-disable-dec-num-check`                                                                                                                                                                                                                                                    |
| init func first    | Enforces the init func to be the first function in file.                                                                                                                                                           | disable check: `-disable-init-func-first-check`                                                                                                                                                                                                                                            |

You may find the implementation of the rules inside `analyzer.go`.

Underscore var declarations can be ignored via `-ignore-underscore-vars`.

## Installation

```shell
go install gitlab.com/bosi/decorder/cmd/decorder
```

You can use the linter via golangci-lint as well: https://golangci-lint.run/usage/linters/#decorder.

## Usage

```shell
# with default options
decorder ./...

# custom declaration order
decorder -dec-order var,const,func,type ./...

# disable declaration order check
decorder -disable-dec-order-check ./...

# disable check for multiple declarations statements
decorder -disable-dec-num-check ./...

# disable check for multiple declarations (var only) statements
decorder -disable-var-dec-num-check ./...

# disable check that init func is always first function
decorder -disable-init-func-first-check ./...

# ignore underscore variables for all checks
decorder -ignore-underscore-vars ./...
```