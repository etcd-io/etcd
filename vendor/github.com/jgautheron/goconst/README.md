# goconst

Find repeated strings that could be replaced by a constant.

### Motivation

There are obvious benefits to using constants instead of repeating strings, mostly to ease maintenance. Cannot argue against changing a single constant versus many strings.

While this could be considered a beginner mistake, across time, multiple packages and large codebases, some repetition could have slipped in.

### How it works

goconst detects string (and optionally number) literals that appear multiple times and could be replaced by a constant.

A few things to keep in mind:

- **Exact literal matching** — goconst compares complete, unquoted literal values. Repeated substrings inside larger strings are not detected (e.g., a shared prefix across two different string literals will not be reported).
- **`const` declarations are skipped by default** — constant values are only analyzed when `-match-constant` (match strings against existing constants) or `-find-duplicates` (find constants sharing the same value) is enabled.
- **String length is measured in runes**, not bytes, so multi-byte Unicode characters are counted correctly against `-min-length`.

### Get Started

    $ go install github.com/jgautheron/goconst/cmd/goconst@latest
    $ goconst ./...

### Usage

```
Usage:

  goconst ARGS <directory> [<directory>...]

Flags:

  -ignore            exclude files matching the given regular expression
  -ignore-strings    exclude strings matching the given regular expression
  -ignore-tests      exclude tests from the search (default: true)
  -min-occurrences   report from how many occurrences (default: 2)
  -min-length        only report strings with the minimum given length (default: 3)
  -match-constant    look for existing constants matching the strings
  -find-duplicates   look for constants with identical values
  -eval-const-expr   enable evaluation of constant expressions (e.g., Prefix + "suffix")
  -ignore-calls      ignore string literals in calls to these functions (comma separated)
  -numbers           search also for duplicated numbers
  -min               minimum value, only works with -numbers
  -max               maximum value, only works with -numbers
  -output            output formatting (text or json)
  -set-exit-status   Set exit status to 2 if any issues are found
  -grouped           print single line per match, only works with -output text

Examples:

  goconst ./...
  goconst -ignore "yacc|\.pb\." $GOPATH/src/github.com/cockroachdb/cockroach/...
  goconst -min-occurrences 3 -output json $GOPATH/src/github.com/cockroachdb/cockroach
  goconst -numbers -min 60 -max 512 .
  goconst -min-occurrences 5 $(go list -m -f '{{.Dir}}')
  goconst -eval-const-expr -match-constant . # Matches constant expressions like Prefix + "suffix"
  goconst -ignore-calls slog.Info,slog.Warn,fmt.Errorf ./... # Ignore strings in logging/error calls
```

### Development

#### Running Tests

The project includes a comprehensive test suite. To run the tests:

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with race detector
go test -race ./...

# Run benchmarks
go test -bench=. ./...

# Check test coverage
go test -cover ./...
```

#### Contributing

Contributions are welcome! Before submitting a PR:

1. Make sure all tests pass
2. Add tests for new functionality
3. Ensure your code passes linting checks
4. Update documentation as needed

### Other static analysis tools

- [gogetimports](https://github.com/jgautheron/gogetimports): Get a JSON-formatted list of imports.
- [usedexports](https://github.com/jgautheron/usedexports): Find exported variables that could be unexported.

### License

MIT
