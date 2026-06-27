# gosec - Go Security Checker

gosec is a Go static analysis tool that inspects Go source code for security vulnerabilities by scanning the Go AST and SSA form.

## Build & Test

```bash
# Build
go build ./cmd/gosec/

# Run all tests
go test ./...

# Run a specific test
go test -run TestName ./path/to/package/

# Lint
golangci-lint run

# Run gosec against a sample file
go run ./cmd/gosec/ ./path/to/sample.go
```

## Code Style

- Idiomatic Go; follow existing patterns in the codebase.
- Prefer SSA-based analyzers over AST-based rules when feasible.
- Optimize for performance — avoid unnecessary repeated AST or SSA traversals.

## Project Structure

- `rules/` — AST-based rule implementations
- `analyzers/` — SSA-based analyzer implementations
- `cmd/gosec/` — CLI entry point
- `testutils/` — sample files used in tests (positive and negative cases)
- `issue/` — issue and CWE type definitions
- `report/` — output formatters

## Adding Rules

- Select an appropriate CWE aligned with current repository mappings.
- Integrate the rule in all required registration points.
- Add sample files in `testutils/` with at least 2 positive and 2 negative cases.
- Update rule documentation in `README.md` in the same style as other rules.

## Custom Commands

- `/create-gosec-rule` — Design and implement a new gosec rule from an issue description
- `/fix-gosec-bug` — Investigate and fix a bug from a GitHub issue URL
- `/update-go-versions` — Bump supported Go versions across the repo
- `/update-action-version` — Update the gosec GHCR image version in action.yml
