# noinlineerr

A Go linter that forbids inline error handling using `if err := ...; err != nil`.

---

## Why?
Inline error handling in Go can hurt readability by hiding the actual function call behind error plumbing.
We believe errors and functions deserve their own spotlight.

Instead of:
```go
if err := doSomething(); err != nil {
    return err
}
```
Prefer the more explicit and readable:
```go
err := doSomething()
if err != nil {
    return err
}
```

---

## Install
```bash
go install github.com/AlwxSin/noinlineerr/cmd/noinlineerr@latest
```

---

## Usage
### As a standalone tool
```bash
noinlineerr ./...
```

⚠️ Note: The linter detects inline error assignments only when the error variable is explicitly typed or deducible. It doesn't handle dynamically typed interfaces (e.g., `foo().Err()` where `Err()` returns an error via an interface).

---

## Development
Run tests:
```bash
go test ./...
```
Test data lives under `testdata/src/...`

---

## Contributing
PRs are welcome. Let's make Go code cleaner, one `err` at a time.
