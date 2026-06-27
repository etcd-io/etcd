# unqueryvet

[![Go Report Card](https://goreportcard.com/badge/github.com/MirrexOne/unqueryvet)](https://goreportcard.com/report/github.com/MirrexOne/unqueryvet)
[![Go Reference](https://pkg.go.dev/badge/github.com/MirrexOne/unqueryvet.svg)](https://pkg.go.dev/github.com/MirrexOne/unqueryvet)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

[![JetBrains Plugins](https://img.shields.io/jetbrains/plugin/v/29733-unqueryvet.svg)](https://plugins.jetbrains.com/plugin/29733-unqueryvet)
[![JetBrains Plugins Downloads](https://img.shields.io/jetbrains/plugin/d/29733-unqueryvet.svg)](https://plugins.jetbrains.com/plugin/29733-unqueryvet)

[![VS Code Marketplace](https://vsmarketplacebadges.dev/version-short/mirrexdev.unqueryvet.svg)](https://marketplace.visualstudio.com/items?itemName=mirrexdev.unqueryvet)
[![VS Code Installs](https://vsmarketplacebadges.dev/installs-short/mirrexdev.unqueryvet.svg)](https://marketplace.visualstudio.com/items?itemName=mirrexdev.unqueryvet)

[![Get from Marketplace](https://img.shields.io/badge/JetBrains-Get%20Plugin-000000?style=for-the-badge&logo=jetbrains)](https://plugins.jetbrains.com/plugin/29733-unqueryvet)
[![Get from VS Code Marketplace](https://img.shields.io/badge/VS%20Code-Install%20Extension-007ACC?style=for-the-badge&logo=visualstudiocode)](https://marketplace.visualstudio.com/items?itemName=mirrexdev.unqueryvet)

**unqueryvet** is a comprehensive Go linter for SQL queries. It detects `SELECT *` usage, N+1 query problems, SQL injection vulnerabilities, and provides suggestions for query optimization.

## Key Features

| Feature                       | Description                                                                  |
| ----------------------------- | ---------------------------------------------------------------------------- |
| **SELECT \* Detection**       | Finds `SELECT *` in raw SQL, SQL builders, and templates                     |
| **N+1 Query Detection**       | Identifies queries inside loops                                              |
| **SQL Injection Scanner**     | Detects `fmt.Sprintf` and string concatenation vulnerabilities               |
| **Transaction Leak Detection**| Detects unclosed transactions and improper lifecycle management              |
| **12 SQL Builder Support**    | Squirrel, GORM, SQLx, Ent, PGX, Bun, SQLBoiler, Jet, sqlc, goqu, rel, reform |
| **Custom Rules DSL**          | Define your own analysis rules                                               |
| **LSP Server**                | Real-time IDE integration                                                    |
| **Interactive TUI**           | Fix issues interactively                                                     |

---

## Installation

### Standalone Tool

```bash
go install github.com/MirrexOne/unqueryvet/cmd/unqueryvet@latest
```

### LSP Server (for IDE integration)

```bash
go install github.com/MirrexOne/unqueryvet/cmd/unqueryvet-lsp@latest
```

### Docker

```bash
docker pull ghcr.io/mirrexone/unqueryvet:latest
docker run --rm -v $(pwd):/app ghcr.io/mirrexone/unqueryvet /app/...
```

### With golangci-lint

Add to your `.golangci.yml`:

```yaml
version: "2"

linters:
  enable:
    - unqueryvet

  settings:
    unqueryvet:
      check-sql-builders: true
```

---

## Quick Start

### Basic Usage

```bash
# Analyze all packages (all rules enabled by default)
unqueryvet ./...

# Verbose output with explanations
unqueryvet -verbose ./...

# Quiet mode (errors only) for CI/CD
unqueryvet -quiet ./...

# Show statistics
unqueryvet -stats ./...

# Interactive fix mode
unqueryvet -fix ./...

# Show version
unqueryvet -version
```

### Default Rules

All detection rules are **enabled by default**:

| Rule | Default Severity | Description |
|------|-----------------|-------------|
| `select-star` | warning | Detects `SELECT *` usage |
| `n1-queries` | warning | Detects N+1 query patterns (queries in loops) |
| `sql-injection` | error | Detects SQL injection vulnerabilities |
| `tx-leak` | warning | Detects unclosed SQL transactions |

To disable a rule, set its severity to `ignore` in your `.unqueryvet.yaml`:

```yaml
rules:
  n1-queries: ignore  # Disable N+1 detection
```

### CLI Flags

| Flag | Description |
|------|-------------|
| `-version` | Print version information |
| `-verbose` | Enable verbose output with detailed explanations |
| `-quiet` | Quiet mode (only errors) |
| `-stats` | Show analysis statistics |
| `-no-color` | Disable colored output |
| `-n1` | Force enable N+1 detection (overrides config) |
| `-sqli` | Force enable SQL injection detection (overrides config) |
| `-tx-leak` | Force enable transaction leak detection (overrides config) |
| `-fix` | Interactive fix mode - step through issues and apply fixes |

### With Configuration File

```bash
# Create config file
cat > .unqueryvet.yaml << 'EOF'
severity: warning
check-sql-builders: true

# Rules are enabled by default - configure severity if needed
rules:
  select-star: warning
  n1-queries: warning
  sql-injection: error

ignore:
  - "*_test.go"
  - "vendor/**"
EOF

# Run (auto-loads config)
unqueryvet ./...
```

---

## Detection Examples

### 1. SELECT \* Detection

**Bad code:**

```go
// Direct SELECT *
query := "SELECT * FROM users"

// Aliased wildcard
query := "SELECT t.* FROM users t"

// In subquery
query := "SELECT id FROM (SELECT * FROM users)"

// String concatenation
query := "SELECT * " + "FROM users"

// Format string
query := fmt.Sprintf("SELECT * FROM %s", table)

// SQL builders
squirrel.Select("*").From("users")
db.Model(&User{}).Select("*")
goqu.From("users").Select(goqu.Star())
```

**Good code:**

```go
// Explicit columns
query := "SELECT id, name, email FROM users"

// SQL builders
squirrel.Select("id", "name", "email").From("users")
db.Model(&User{}).Select("id", "name", "email")
goqu.From("users").Select("id", "name", "email")
```

### 2. N+1 Query Detection

**Bad code (triggers warning):**

```go
users, _ := db.Query("SELECT id, name FROM users")
for users.Next() {
    var user User
    users.Scan(&user.ID, &user.Name)

    // N+1 problem: query inside loop
    orders, _ := db.Query("SELECT * FROM orders WHERE user_id = ?", user.ID)
}
```

**Good code:**

```go
// Use JOIN
query := `
    SELECT u.id, u.name, o.id, o.total
    FROM users u
    LEFT JOIN orders o ON u.id = o.user_id
`

// Or use IN clause
userIDs := []int{1, 2, 3, 4, 5}
query := "SELECT * FROM orders WHERE user_id IN (?)"
db.Query(query, userIDs)
```

### 3. SQL Injection Detection

**Bad code (triggers warning):**

```go
// String concatenation with user input
query := "SELECT * FROM users WHERE name = '" + userName + "'"

// fmt.Sprintf with user input
query := fmt.Sprintf("SELECT * FROM users WHERE id = %s", userID)
```

**Good code:**

```go
// Parameterized query
query := "SELECT id, name FROM users WHERE name = ?"
db.Query(query, userName)

// Named parameters
query := "SELECT id, name FROM users WHERE id = :id"
db.NamedQuery(query, map[string]interface{}{"id": userID})
```

### 4. Transaction Leak Detection

Detects unclosed SQL transactions using 19-phase AST analysis. Supports multiple transaction begin methods across different libraries.

**Supported Begin Methods:**

| Library | Methods |
|---------|---------|
| database/sql | `Begin`, `BeginTx` |
| sqlx | `Beginx`, `BeginTxx`, `MustBegin`, `MustBeginTx` |
| pgx | `BeginFunc`, `BeginTxFunc` |
| bun | `RunInTx` |
| ent | `Tx`, `NewTx` |

**Detection Patterns:**

| Violation Type | Severity | Description |
|----------------|----------|-------------|
| `no_commit_rollback` | critical | Transaction has neither Commit() nor Rollback() |
| `no_rollback` | high | Transaction has Commit() but no Rollback() for error paths |
| `no_commit` | medium | Transaction has Rollback() but no Commit() |
| `early_return` | high | Early return paths bypass Commit() without defer |
| `defer_in_loop` | high | Defer inside loop - defers pile up until function returns |
| `shadowed_transaction` | high | Transaction variable shadowed in inner scope |
| `goroutine_capture` | high | Transaction captured by goroutine without defer |
| `variable_reassignment` | high | Transaction variable reassigned - previous tx may leak |
| `fatal_without_defer` | high | Transaction may leak if os.Exit/log.Fatal called |
| `panic_without_defer` | medium | Transaction may leak if panic() called |
| `conditional_commit` | medium | Commit() inside conditional - may not execute |
| `commit_in_switch` | medium | Commit() in switch/case - may not execute in all cases |
| `commit_in_select` | medium | Commit() in select/case - may not execute |
| `commit_in_loop` | medium | Commit() in loop - may not execute if loop doesn't iterate |
| `deferred_commit` | medium | Using defer Commit() is an antipattern |
| `commit_error_ignored` | low | Commit() error ignored with blank identifier |
| `rollback_error_ignored` | low | Rollback() error ignored with blank identifier |

**Bad code (triggers error):**

```go
func createUser(db *sql.DB, name string) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    // Missing defer tx.Rollback() and missing tx.Commit()!
    
    _, err = tx.Exec("INSERT INTO users (name) VALUES (?)", name)
    if err != nil {
        return err  // Transaction leaked!
    }
    return nil
}

func deferInLoop(db *sql.DB, items []string) error {
    for _, item := range items {
        tx, _ := db.Begin()
        defer tx.Rollback()  // Defers pile up until function returns!
        tx.Exec("INSERT...", item)
        tx.Commit()
    }
    return nil
}
```

**Good code:**

```go
func createUser(db *sql.DB, name string) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()  // Safe: called after Commit() is a no-op
    
    _, err = tx.Exec("INSERT INTO users (name) VALUES (?)", name)
    if err != nil {
        return err  // Rollback will be called via defer
    }
    
    return tx.Commit()
}

// Using callback pattern (automatically handled)
func createUserCallback(db *pgx.Conn, name string) error {
    return db.BeginFunc(ctx, func(tx pgx.Tx) error {
        _, err := tx.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", name)
        return err  // Commit/Rollback handled automatically
    })
}
```

---

## Configuration

### Full Configuration File (.unqueryvet.yaml)

```yaml
# Built-in rules severity (all enabled by default)
# Available values: error, warning, info, ignore
rules:
  select-star: warning    # SELECT * detection
  n1-queries: warning     # N+1 query detection
  sql-injection: error    # SQL injection scanning
  tx-leak: warning        # Transaction leak detection

# Diagnostic severity for legacy options: "error" or "warning"
severity: warning

# Core analysis options
check-sql-builders: true
check-aliased-wildcard: true
check-string-concat: true
check-format-strings: true
check-string-builder: true
check-subqueries: true

# SQL builder libraries to check
sql-builders:
  squirrel: true
  gorm: true
  sqlx: true
  ent: true
  pgx: true
  bun: true
  sqlboiler: true
  jet: true
  sqlc: true
  goqu: true
  rel: true
  reform: true

# File patterns to ignore (glob)
ignored-files:
  - "*_test.go"
  - "testdata/**"
  - "vendor/**"
  - "mock_*.go"

# Function patterns to ignore (regex)
ignored-functions:
  - "debug\\..*"
  - "test.*"

# Allowed SELECT * patterns (regex)
allowed-patterns:
  - "SELECT \\* FROM information_schema\\..*"
  - "SELECT \\* FROM pg_catalog\\..*"
  - "SELECT \\* FROM temp_.*"
```

---

## Custom Rules DSL

Define your own analysis rules using a powerful DSL with three levels of complexity.

### Level 1: Simple Configuration

```yaml
# .unqueryvet.yaml
rules:
  select-star: error      # Built-in rule severity
  n1-queries: warning
  sql-injection: error

ignore:
  - "*_test.go"
  - "testdata/**"

allow:
  - "COUNT(*)"
  - "information_schema.*"
```

### Level 2: Pattern Matching

```yaml
custom-rules:
  - id: allow-temp-tables
    pattern: SELECT * FROM $TABLE
    when: isTempTable(table)
    action: allow

  - id: dangerous-delete
    pattern: DELETE FROM $TABLE
    when: "!has_where"
    message: "DELETE without WHERE clause"
    severity: error

  - id: require-tx-timeout
    pattern: db.BeginTx($CTX, $OPTS)
    when: "!contains(opts, 'Timeout')"
    message: "Transaction should have timeout set"
    severity: warning
```

### Level 3: Advanced Conditions

```yaml
custom-rules:
  - id: n1-detection
    pattern: $DB.Query($QUERY)
    when: |
      in_loop && 
      !contains(function, "batch") &&
      !matches(file, "_test.go$")
    message: "N+1 query in loop"
    severity: warning
    fix: "Use batch query or preloading"
```

### DSL Reference

| **Metavariables** | **Description** |
|-------------------|-----------------|
| `$TABLE` | Table name (with optional schema) |
| `$VAR` | Identifier/variable |
| `$QUERY` | String literal |
| `$COLS` | Column list |
| `$DB` | Database object |

| **Variables** | **Description** |
|---------------|-----------------|
| `file`, `package`, `function` | Code context |
| `query`, `query_type`, `table` | SQL context |
| `has_where`, `has_join` | Query structure |
| `in_loop`, `loop_depth` | Loop context |
| `builder` | SQL builder type |

| **Functions** | **Description** |
|---------------|-----------------|
| `contains(s, sub)` | String contains |
| `matches(s, regex)` | Regex match |
| `isSystemTable(t)` | System table check |
| `isTempTable(t)` | Temp table check |
| `isAggregate(q)` | Aggregate function check |

| **Operators** | **Description** |
|---------------|-----------------|
| `=~`, `!~` | Regex match/not match |
| `&&`, `\|\|`, `!` | Logical operators |

Full documentation: [docs/DSL.md](docs/DSL.md)

---

## LSP Server (IDE Integration)

The LSP server provides real-time analysis in your IDE.

### Starting the Server

```bash
unqueryvet-lsp
```

### VS Code Setup

Install the extension from `extensions/vscode/` or configure manually:

```json
// .vscode/settings.json
{
  "unqueryvet.enable": true,
  "unqueryvet.path": "unqueryvet-lsp",
  // All rules enabled by default, args optional
  "unqueryvet.trace.server": "verbose"
}
```

### Features

- **Real-time diagnostics** - See issues as you type
- **Hover information** - Explanations on hover
- **Quick fixes** - One-click fixes for SELECT \*
- **Code completion** - Column name suggestions

### GoLand/IntelliJ Setup

1. Build the plugin: `cd extensions/goland && ./gradlew buildPlugin`
2. Install from disk: Settings → Plugins → Install from disk
3. Configure: Settings → Tools → unqueryvet

---

## Interactive TUI Mode

Fix issues interactively with a terminal UI.

```bash
unqueryvet -fix ./...
```

### Controls

| Category | Key | Action |
|----------|-----|--------|
| **Navigation** | `↑/k` | Previous issue |
| | `↓/j` | Next issue |
| | `g` | Go to first issue |
| | `G` | Go to last issue |
| **Actions** | `Enter/a` | Apply fix |
| | `s` | Skip issue |
| | `u` | Undo last action |
| | `p` | Toggle preview |
| **Batch** | `A` | Apply all remaining |
| | `S` | Skip all remaining |
| | `R` | Reset all actions |
| **Other** | `e` | Export results to JSON |
| | `?` | Toggle help |
| | `q/Esc` | Quit |

### Example Session

```
Found 15 issues. Review each one:

[1/15] internal/api/users.go:42:15
─────────────────────────────────────
  41 | func getUsers(db *sql.DB) {
  42 |     query := "SELECT * FROM users"
     |              ^^^^^^^^^^^^^^^^^^^^^ avoid SELECT *
  43 |     rows, _ := db.Query(query)

Suggestions:
  1. SELECT id, username, email, created_at (from struct User)
  2. SELECT id, username, email
  3. Skip this issue
  4. Edit manually

Your choice [1-4]: _
```

---

## Supported SQL Builders

### Full Support (12 builders)

| Builder       | Package                             | Patterns Detected                            |
| ------------- | ----------------------------------- | -------------------------------------------- |
| **Squirrel**  | `github.com/Masterminds/squirrel`   | `Select("*")`, `Columns("*")`                |
| **GORM**      | `gorm.io/gorm`                      | `Select("*")`, `Find(&users)` without Select |
| **SQLx**      | `github.com/jmoiron/sqlx`           | `Select()`, raw queries                      |
| **Ent**       | `entgo.io/ent`                      | Query builder patterns                       |
| **PGX**       | `github.com/jackc/pgx`              | `Query()`, `QueryRow()`                      |
| **Bun**       | `github.com/uptrace/bun`            | `NewSelect()`, raw queries                   |
| **SQLBoiler** | `github.com/volatiletech/sqlboiler` | Generated query methods                      |
| **Jet**       | `github.com/go-jet/jet`             | `SELECT()`, `STAR`                           |
| **sqlc**      | Generated code                      | SELECT \* in .sql files                      |
| **goqu**      | `github.com/doug-martin/goqu`       | `Select(goqu.Star())`, `SelectAll()`         |
| **rel**       | `github.com/go-rel/rel`             | `Find()`, `FindAll()` without Select         |
| **reform**    | `gopkg.in/reform.v1`                | `FindByPrimaryKeyFrom()`, `SelectAllFrom()`  |

### Examples by Builder

<details>
<summary>Squirrel</summary>

```go
// Bad
sq.Select("*").From("users")
sq.Select().Columns("*").From("users")

// Good
sq.Select("id", "name", "email").From("users")
```

</details>

<details>
<summary>GORM</summary>

```go
// Bad
db.Select("*").Find(&users)
db.Table("users").Find(&users) // implicit SELECT *

// Good
db.Select("id", "name", "email").Find(&users)
```

</details>

<details>
<summary>goqu</summary>

```go
// Bad
goqu.From("users").Select(goqu.Star())
goqu.From("users").SelectAll()

// Good
goqu.From("users").Select("id", "name", "email")
```

</details>

<details>
<summary>rel</summary>

```go
// Bad
repo.Find(ctx, &user) // loads all columns
repo.FindAll(ctx, &users)

// Good
repo.Find(ctx, &user, rel.Select("id", "name", "email"))
```

</details>

<details>
<summary>reform</summary>

```go
// Bad
db.FindByPrimaryKeyFrom(UserTable, id, &user)
db.FindAllFrom(UserTable, "status", "active")

// Good
db.SelectOneFrom(UserTable, "id, name, email WHERE id = ?", id)
```

</details>

<details>
<summary>SQLx</summary>

```go
// Bad
db.Select(&users, "SELECT * FROM users")
db.Get(&user, "SELECT * FROM users WHERE id = ?", id)

// Good
db.Select(&users, "SELECT id, name, email FROM users")
db.Get(&user, "SELECT id, name, email FROM users WHERE id = ?", id)
```

</details>

<details>
<summary>Ent</summary>

```go
// Bad - implicit SELECT *
users, err := client.User.Query().All(ctx)

// Good - explicit column selection
users, err := client.User.Query().
    Select(user.FieldID, user.FieldName, user.FieldEmail).
    All(ctx)
```

</details>

<details>
<summary>PGX</summary>

```go
// Bad
rows, err := conn.Query(ctx, "SELECT * FROM users")

// Good
rows, err := conn.Query(ctx, "SELECT id, name, email FROM users")
```

</details>

<details>
<summary>Bun</summary>

```go
// Bad
db.NewSelect().Model(&users).Scan(ctx)
db.NewSelect().TableExpr("users").Scan(ctx, &users)

// Good
db.NewSelect().Model(&users).Column("id", "name", "email").Scan(ctx)
```

</details>

<details>
<summary>SQLBoiler</summary>

```go
// Bad - loads all columns
users, err := models.Users().All(ctx, db)
user, err := models.FindUser(ctx, db, userID)

// Good - explicit column selection
users, err := models.Users(
    qm.Select("id", "name", "email"),
).All(ctx, db)
```

</details>

<details>
<summary>Jet</summary>

```go
// Bad
stmt := SELECT(User.AllColumns).FROM(User)

// Good
stmt := SELECT(User.ID, User.Name, User.Email).FROM(User)
```

</details>

<details>
<summary>sqlc</summary>

```sql
-- Bad (in .sql file)
-- name: GetUsers :many
SELECT * FROM users;

-- Good
-- name: GetUsers :many
SELECT id, name, email FROM users;
```

</details>

---

## Docker & CI/CD

### Dockerfile

```dockerfile
FROM golang:1.24-alpine AS builder
RUN go install github.com/MirrexOne/unqueryvet/cmd/unqueryvet@latest

FROM alpine:latest
COPY --from=builder /go/bin/unqueryvet /usr/local/bin/
ENTRYPOINT ["unqueryvet"]
```

### GitHub Actions

```yaml
name: SQL Lint

on: [push, pull_request]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v6
        with:
          go-version: '1.24'

      - name: Install unqueryvet
        run: go install github.com/MirrexOne/unqueryvet/cmd/unqueryvet@latest

      - name: Run unqueryvet
        run: unqueryvet -n1 -sqli -tx-leak ./...
```

### GitLab CI

```yaml
sql-lint:
  image: ghcr.io/mirrexone/unqueryvet:latest
  script:
    - unqueryvet -quiet -n1 -sqli -tx-leak ./...
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | No issues found |
| 1 | Warnings found |
| 2 | Errors found |
| 3 | Analysis failed |

---

## Documentation

- [CLI Features Guide](docs/CLI_FEATURES.md)
- [Custom Rules DSL](docs/DSL.md)
- [IDE Integration Guide](docs/IDE_INTEGRATION.md)

---

## Development

### Build

```bash
go build ./cmd/unqueryvet
go build ./cmd/unqueryvet-lsp
```

### Test

```bash
go test ./...
```

### Install locally

```bash
go install ./cmd/unqueryvet
go install ./cmd/unqueryvet-lsp
```

---

## Contributing

```bash
git clone https://github.com/MirrexOne/unqueryvet.git
cd unqueryvet
go mod download
go test ./...
go build ./...
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## License

MIT License - see [LICENSE](LICENSE) file for details.

---

## Acknowledgments

- Built on [golang.org/x/tools/go/analysis](https://pkg.go.dev/golang.org/x/tools/go/analysis)
- TUI powered by [Bubbletea](https://github.com/charmbracelet/bubbletea)

---

## Support

- **Bug Reports**: [GitHub Issues](https://github.com/MirrexOne/unqueryvet/issues)
