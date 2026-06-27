# wsl - Whitespace Linter

[![forthebadge](https://forthebadge.com/images/badges/made-with-go.svg)](https://forthebadge.com)
[![forthebadge](https://forthebadge.com/images/badges/built-with-love.svg)](https://forthebadge.com)

[![GitHub Actions](https://github.com/bombsimon/wsl/actions/workflows/go.yml/badge.svg)](https://github.com/bombsimon/wsl/actions/workflows/go.yml)
[![Coverage Status](https://coveralls.io/repos/github/bombsimon/wsl/badge.svg?branch=master)](https://coveralls.io/github/bombsimon/wsl?branch=master)

`wsl` is a linter that enforces a very **non scientific** vision of how to make
code more readable by enforcing empty lines at the right places.

**This linter is aggressive** and a lot of projects I've tested it on have
failed miserably. For this linter to be useful at all I want to be open to new
ideas, configurations and discussions! Also note that some of the warnings might
be bugs or unintentional false positives so I would love an
[issue](https://github.com/bombsimon/wsl/issues/new) to fix, discuss, change or
make something configurable!

## Installation

```sh
# Latest release
go install github.com/bombsimon/wsl/v4/cmd/wsl@latest

# Main branch
go install github.com/bombsimon/wsl/v4/cmd/wsl@master
```

## Usage

`wsl` uses the [analysis](https://pkg.go.dev/golang.org/x/tools/go/analysis)
package meaning it will operate on package level with the default analysis flags
and way of working.

```sh
wsl --help
wsl [flags] </path/to/package/...>

wsl --allow-cuddle-declarations --fix ./...
```

`wsl` is also integrated in [`golangci-lint`](https://golangci-lint.run)

```sh
golangci-lint run --no-config --disable-all --enable wsl --fix
```

> **Note**: If you're not sure what the diagnostic is trying to tell you, use
> any of the fix approaches to fix the code for you.

## Issues and configuration

The linter suppers a few ways to configure it to satisfy more than one kind of
code style. These settings could be set either with flags or with YAML
configuration if used via `golangci-lint`.

The supported configuration can be found [in the
documentation](doc/configuration.md).

Below are the available checklist for any hit from `wsl`. If you do not see any,
feel free to raise an [issue](https://github.com/bombsimon/wsl/issues/new).

* [Anonymous switch statements should never be cuddled](doc/rules.md#anonymous-switch-statements-should-never-be-cuddled)
* [Append only allowed to cuddle with appended value](doc/rules.md#append-only-allowed-to-cuddle-with-appended-value)
* [Assignments should only be cuddled with other assignments](doc/rules.md#assignments-should-only-be-cuddled-with-other-assignments)
* [Block should not end with a whitespace (or comment)](doc/rules.md#block-should-not-end-with-a-whitespace-or-comment)
* [Block should not start with a whitespace](doc/rules.md#block-should-not-start-with-a-whitespace)
* [Case block should end with newline at this size](doc/rules.md#case-block-should-end-with-newline-at-this-size)
* [Branch statements should not be cuddled if block has more than two lines](doc/rules.md#branch-statements-should-not-be-cuddled-if-block-has-more-than-two-lines)
* [Declarations should never be cuddled](doc/rules.md#declarations-should-never-be-cuddled)
* [Defer statements should only be cuddled with expressions on same variable](doc/rules.md#defer-statements-should-only-be-cuddled-with-expressions-on-same-variable)
* [Expressions should not be cuddled with blocks](doc/rules.md#expressions-should-not-be-cuddled-with-blocks)
* [Expressions should not be cuddled with declarations or returns](doc/rules.md#expressions-should-not-be-cuddled-with-declarations-or-returns)
* [For statement without condition should never be cuddled](doc/rules.md#for-statement-without-condition-should-never-be-cuddled)
* [For statements should only be cuddled with assignments used in the iteration](doc/rules.md#for-statements-should-only-be-cuddled-with-assignments-used-in-the-iteration)
* [Go statements can only invoke functions assigned on line above](doc/rules.md#go-statements-can-only-invoke-functions-assigned-on-line-above)
* [If statements should only be cuddled with assignments](doc/rules.md#if-statements-should-only-be-cuddled-with-assignments)
* [If statements should only be cuddled with assignments used in the if statement itself](doc/rules.md#if-statements-should-only-be-cuddled-with-assignments-used-in-the-if-statement-itself)
* [If statements that check an error must be cuddled with the statement that assigned the error](doc/rules.md#if-statements-that-check-an-error-must-be-cuddled-with-the-statement-that-assigned-the-error)
* [Only cuddled expressions if assigning variable or using from line above](doc/rules.md#only-cuddled-expressions-if-assigning-variable-or-using-from-line-above)
* [Only one cuddle assignment allowed before defer statement](doc/rules.md#only-one-cuddle-assignment-allowed-before-defer-statement)
* [Only one cuddle assignment allowed before for statement](doc/rules.md#only-one-cuddle-assignment-allowed-before-for-statement)
* [Only one cuddle assignment allowed before go statement](doc/rules.md#only-one-cuddle-assignment-allowed-before-go-statement)
* [Only one cuddle assignment allowed before if statement](doc/rules.md#only-one-cuddle-assignment-allowed-before-if-statement)
* [Only one cuddle assignment allowed before range statement](doc/rules.md#only-one-cuddle-assignment-allowed-before-range-statement)
* [Only one cuddle assignment allowed before switch statement](doc/rules.md#only-one-cuddle-assignment-allowed-before-switch-statement)
* [Only one cuddle assignment allowed before type switch statement](doc/rules.md#only-one-cuddle-assignment-allowed-before-type-switch-statement)
* [Ranges should only be cuddled with assignments used in the iteration](doc/rules.md#ranges-should-only-be-cuddled-with-assignments-used-in-the-iteration)
* [Return statements should not be cuddled if block has more than two lines](doc/rules.md#return-statements-should-not-be-cuddled-if-block-has-more-than-two-lines)
* [Short declarations should cuddle only with other short declarations](doc/rules.md#short-declaration-should-cuddle-only-with-other-short-declarations)
* [Switch statements should only be cuddled with variables switched](doc/rules.md#switch-statements-should-only-be-cuddled-with-variables-switched)
* [Type switch statements should only be cuddled with variables switched](doc/rules.md#type-switch-statements-should-only-be-cuddled-with-variables-switched)
