# Contributing to unqueryvet

Thank you for your interest in contributing to unqueryvet! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Running Tests](#running-tests)
- [Code Style](#code-style)
- [Pull Request Process](#pull-request-process)
- [Adding New SQL Builders](#adding-new-sql-builders)
- [Reporting Issues](#reporting-issues)

---

## Getting Started

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/unqueryvet.git
   cd unqueryvet
   ```
3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/MirrexOne/unqueryvet.git
   ```
4. Create a new branch for your feature:
   ```bash
   git checkout -b feature/your-feature-name
   ```

---

## Development Setup

### Prerequisites

- Go 1.24 or later
- [Task](https://taskfile.dev/) (optional, but recommended)
- golangci-lint (for linting)

### Install Dependencies

```bash
go mod download
```

### Build

Using Task:
```bash
task build        # Build CLI
task build:lsp    # Build LSP server
task build:all    # Build both
```

Using Go directly:
```bash
go build ./cmd/unqueryvet
go build ./cmd/unqueryvet-lsp
```

### Install Locally

```bash
task install:all
# or
go install ./cmd/unqueryvet
go install ./cmd/unqueryvet-lsp
```

---

## Project Structure

```
unqueryvet/
├── cmd/
│   ├── unqueryvet/          # CLI entry point
│   └── unqueryvet-lsp/      # LSP server entry point
├── internal/
│   ├── analyzer/            # Core analysis engine
│   │   ├── sqlbuilders/     # SQL builder support (12 builders)
│   │   ├── n1detector.go    # N+1 query detection
│   │   └── sqli_scanner.go  # SQL injection scanner
│   ├── cli/                 # CLI output utilities
│   ├── configloader/        # Configuration loading
│   ├── dsl/                 # Custom Rules DSL engine
│   ├── lsp/                 # LSP server implementation
│   ├── messages/            # Error messages
│   ├── runner/              # Analysis runner
│   ├── tui/                 # Interactive TUI (Bubble Tea)
│   └── version/             # Version information
├── pkg/
│   └── config/              # Public configuration API
├── extensions/
│   ├── goland/              # GoLand/IntelliJ plugin (Kotlin)
│   └── vscode/              # VS Code extension (TypeScript)
├── docs/                    # Documentation
├── _examples/               # Example configurations
└── testdata/                # Test fixtures
```

### Key Packages

| Package | Description |
|---------|-------------|
| `internal/analyzer` | Core detection logic (SELECT *, N+1, SQL Injection) |
| `internal/analyzer/sqlbuilders` | Support for 12 SQL builder libraries |
| `internal/dsl` | Custom rules DSL using expr-lang |
| `internal/lsp` | Language Server Protocol implementation |
| `internal/tui` | Interactive terminal UI with Bubble Tea |
| `pkg/config` | Configuration with default rules |

> **Note**: All three detection rules (SELECT *, N+1, SQL Injection) are enabled by default.

---

## Running Tests

Using Task:
```bash
task test           # Run all tests
task test:race      # Run with race detection (requires CGO)
task test:short     # Run short tests only
task test:unit      # Run unit tests only
task test:integration # Run integration tests
task coverage       # Generate coverage report
```

Using Go directly:
```bash
go test ./...
go test -v -race ./...
go test -coverprofile=coverage.out ./...
```

### Running Benchmarks

```bash
task bench
# or
go test -bench=. -benchmem ./internal/analyzer
```

---

## Code Style

### Formatting

```bash
task fmt        # Format code
task fmt:check  # Check formatting
```

### Linting

```bash
task lint       # Run golangci-lint
task lint:fix   # Run with auto-fix
task vet        # Run go vet
```

### All Checks

```bash
task check:all  # fmt:check + vet + lint + test
```

### Guidelines

1. **Follow Go conventions** - Use `gofmt`, follow [Effective Go](https://go.dev/doc/effective_go)
2. **Write tests** - All new features should have tests
3. **Document exports** - All exported functions/types need documentation comments
4. **Keep it simple** - Prefer simple, readable code over clever solutions
5. **No breaking changes** - Maintain backward compatibility for public APIs

---

## Pull Request Process

1. **Sync with upstream**
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Ensure tests pass**
   ```bash
   task check:all
   ```

3. **Write meaningful commit messages**
   ```
   feat: add support for new SQL builder XYZ
   
   - Add XYZ builder detection in internal/analyzer/sqlbuilders/
   - Add tests for common XYZ patterns
   - Update documentation
   ```

4. **Push and create PR**
   ```bash
   git push origin feature/your-feature-name
   ```

5. **PR Requirements**
   - Clear description of changes
   - Tests for new functionality
   - Documentation updates if needed
   - Passing CI checks

---

## Adding New SQL Builders

To add support for a new SQL builder library:

### 1. Create Builder File

Create `internal/analyzer/sqlbuilders/yourbuilder.go`:

```go
package sqlbuilders

import (
    "go/ast"
)

// YourBuilderChecker checks for SELECT * in YourBuilder queries.
type YourBuilderChecker struct {
    BaseChecker
}

// NewYourBuilderChecker creates a new checker for YourBuilder.
func NewYourBuilderChecker() *YourBuilderChecker {
    return &YourBuilderChecker{
        BaseChecker: BaseChecker{
            name:        "yourbuilder",
            packagePath: "github.com/example/yourbuilder",
        },
    }
}

func (c *YourBuilderChecker) Name() string {
    return c.name
}

func (c *YourBuilderChecker) IsApplicable(call *ast.CallExpr) bool {
    // Check if this call is from yourbuilder package
    return c.isPackageCall(call, "yourbuilder")
}

func (c *YourBuilderChecker) CheckSelectStar(call *ast.CallExpr) *Violation {
    // Implement SELECT * detection logic
    return nil
}

func (c *YourBuilderChecker) CheckChainedCalls(call *ast.CallExpr) []*Violation {
    // Check method chains for SELECT *
    return nil
}
```

### 2. Register in Registry

Update `internal/analyzer/sqlbuilders/interface.go`:

```go
func NewRegistry(cfg *config.SQLBuildersConfig) *Registry {
    r := &Registry{checkers: make([]SQLBuilderChecker, 0)}
    
    // ... existing builders ...
    
    if cfg.YourBuilder {
        r.checkers = append(r.checkers, NewYourBuilderChecker())
    }
    
    return r
}
```

### 3. Add Configuration

Update `pkg/config/config.go`:

```go
type SQLBuildersConfig struct {
    // ... existing fields ...
    YourBuilder bool `yaml:"yourbuilder"`
}

func DefaultSQLBuildersConfig() SQLBuildersConfig {
    return SQLBuildersConfig{
        // ... existing defaults ...
        YourBuilder: true,
    }
}
```

### 4. Write Tests

Create `internal/analyzer/sqlbuilders/yourbuilder_test.go` with test cases.

### 5. Update Documentation

- Add to README.md SQL builders table
- Add example in `<details>` block
- Update `.unqueryvet.example.yaml`

---

## Reporting Issues

### Bug Reports

Please include:
- Go version (`go version`)
- unqueryvet version (`unqueryvet -version`)
- Operating system
- Minimal reproducible example
- Expected vs actual behavior

### Feature Requests

Please include:
- Use case description
- Proposed solution (if any)
- Examples of desired behavior

---

## Questions?

- Open a [GitHub Issue](https://github.com/MirrexOne/unqueryvet/issues)

Thank you for contributing!
