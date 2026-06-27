[![Go Reference](https://pkg.go.dev/badge/github.com/uudashr/gocognit.svg)](https://pkg.go.dev/github.com/uudashr/gocognit)
[![go-recipes](https://raw.githubusercontent.com/nikolaydubina/go-recipes/main/badge.svg?raw=true)](https://github.com/nikolaydubina/go-recipes)

# Gocognit
Gocognit calculates cognitive complexities of functions (and methods) in Go source code. A measurement of how hard does the code is intuitively to understand.

## Understanding the complexity

Given code using `if` statement,
```go
func GetWords(number int) string {
    if number == 1 {            // +1
        return "one"
    } else if number == 2 {     // +1
        return "a couple"
    } else if number == 3 {     // +1
        return "a few"
    } else {                    // +1
        return "lots"
    }
} // Cognitive complexity = 4
```

Above code can be refactored using `switch` statement,
```go
func GetWords(number int) string {
    switch number {             // +1
        case 1:
            return "one"
        case 2:
            return "a couple"
        case 3:
            return "a few"
        default:
            return "lots"
    }
} // Cognitive complexity = 1
```

As you see above codes are the same, but the second code are easier to understand, that is why the cognitive complexity score are lower compare to the first one.

## Comparison with cyclomatic complexity

### Example 1
#### Cyclomatic complexity
```go
func GetWords(number int) string {      // +1
    switch number {
        case 1:                         // +1
            return "one"
        case 2:                         // +1
            return "a couple"
        case 3:                         // +1
             return "a few"
        default:
             return "lots"
    }
} // Cyclomatic complexity = 4
```

####  Cognitive complexity
```go
func GetWords(number int) string {
    switch number {                     // +1
        case 1:
            return "one"
        case 2:
            return "a couple"
        case 3:
            return "a few"
        default:
            return "lots"
    }
} // Cognitive complexity = 1
```

Cognitive complexity give lower score compare to cyclomatic complexity.

### Example 2
#### Cyclomatic complexity
```go
func SumOfPrimes(max int) int {         // +1
    var total int

OUT:
    for i := 1; i < max; i++ {          // +1
        for j := 2; j < i; j++ {        // +1
            if i%j == 0 {               // +1
                continue OUT
            }
        }
        total += i
    }

    return total
} // Cyclomatic complexity = 4
```

#### Cognitive complexity
```go
func SumOfPrimes(max int) int {
    var total int

OUT:
    for i := 1; i < max; i++ {          // +1
        for j := 2; j < i; j++ {        // +2 (nesting = 1)
            if i%j == 0 {               // +3 (nesting = 2)
                continue OUT            // +1
            }
        }
        total += i
    }

    return total
} // Cognitive complexity = 7
```

Cognitive complexity give higher score compare to cyclomatic complexity.

## Rules

The cognitive complexity of a function is calculated according to the
following rules:
> Note: these rules are specific for Go, please see the [original whitepaper](https://www.sonarsource.com/docs/CognitiveComplexity.pdf) for more complete reference.

### Increments
There is an increment for each of the following:
1. `if`, `else if`, `else`
2. `switch`, `select`
3. `for`
4. `goto` LABEL, `break` LABEL, `continue` LABEL
5. sequence of binary logical operators
6. each method in a recursion cycle

### Nesting level
The following structures increment the nesting level:
1. `if`, `else if`, `else`
2. `switch`, `select`
3. `for`
4. function literal or lambda

### Nesting increments
The following structures receive a nesting increment commensurate with their nested depth inside nesting structures:
1. `if`
2. `switch`, `select`
3. `for`

## Installation

```shell
go install github.com/uudashr/gocognit/cmd/gocognit@latest
```

or 

```shell
go get github.com/uudashr/gocognit/cmd/gocognit
```

## Usage

```
$ gocognit
Calculate cognitive complexities of Go functions.

Usage:

  gocognit [<flag> ...] <Go file or directory> ...

Flags:

  -over N       show functions with complexity > N only
                and return exit code 1 if the output is non-empty
  -top N        show the top N most complex functions only
  -avg          show the average complexity over all functions,
                not depending on whether -over or -top are set
  -test         indicates whether test files should be included
  -json         encode the output as JSON
  -d 	        enable diagnostic output
  -f format     string the format to use
                (default "{{.Complexity}} {{.PkgName}} {{.FuncName}} {{.Pos}}")
  -ignore expr  ignore files matching the given regexp

The (default) output fields for each line are:

  <complexity> <package> <function> <file:row:column>

The (default) output fields for each line are:

  {{.Complexity}} {{.PkgName}} {{.FuncName}} {{.Pos}}

or equal to <complexity> <package> <function> <file:row:column>

The struct being passed to the template is:

  type Stat struct {
    PkgName     string
    FuncName    string
    Complexity  int
    Pos         token.Position
    Diagnostics []Diagnostics
  }

  type Diagnostic struct {
    Inc     string
    Nesting int
    Text    string
    Pos     DiagnosticPosition
  }

  type DiagnosticPosition struct {
    Offset int
    Line   int
    Column int
  }
```

Examples:

```
$ gocognit .
$ gocognit main.go
$ gocognit -top 10 src/
$ gocognit -over 25 docker
$ gocognit -avg .
$ gocognit -ignore "_test|testdata" .
```

The output fields for each line are:
```
<complexity> <package> <function> <file:row:column>
```

## Ignore individual functions
Ignore individual functions by specifying `gocognit:ignore` directive.
```go
//gocognit:ignore
func IgnoreMe() {
    // ...
}
```

## Diagnostic
To understand how the complexity are calculated, we can enable the diagnostic by using `-d` flag.

Example:
```shell
$ gocognit -json -d .
```

It will show the diagnostic output in JSON format
<details>

<summary>JSON Output</summary>
    
```json
[
    {
        "PkgName": "prime",
        "FuncName": "SumOfPrimes",
        "Complexity": 7,
        "Pos": {
            "Filename": "prime.go",
            "Offset": 15,
            "Line": 3,
            "Column": 1
        },
        "Diagnostics": [
            {
                "Inc": 1,
                "Text": "for",
                "Pos": {
                    "Offset": 69,
                    "Line": 7,
                    "Column": 2
                }
            },
            {
                "Inc": 2,
                "Nesting": 1,
                "Text": "for",
                "Pos": {
                    "Offset": 104,
                    "Line": 8,
                    "Column": 3
                }
            },
            {
                "Inc": 3,
                "Nesting": 2,
                "Text": "if",
                "Pos": {
                    "Offset": 152,
                    "Line": 9,
                    "Column": 4
                }
            },
            {
                "Inc": 1,
                "Text": "continue",
                "Pos": {
                    "Offset": 190,
                    "Line": 10,
                    "Column": 5
                }
            }
        ]
    }
]
```
</details>

## Related project
- [Gocyclo](https://github.com/fzipp/gocyclo) where the code are based on.
- [Cognitive Complexity: A new way of measuring understandability](https://www.sonarsource.com/docs/CognitiveComplexity.pdf) white paper by G. Ann Campbell.
