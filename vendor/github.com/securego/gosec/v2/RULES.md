# Rule Documentation

## Table of Contents

- [Rules List](#rules-list)
  - [G1xx: General Secure Coding](#g1xx-general-secure-coding)
  - [G2xx: Injection Patterns](#g2xx-injection-patterns)
  - [G3xx: Filesystem and Permissions](#g3xx-filesystem-and-permissions)
  - [G4xx: Crypto and Protocol security](#g4xx-crypto-and-protocol-security)
  - [G5xx: Import Blocklist](#g5xx-import-blocklist)
  - [G6xx: Language/Runtime safety](#g6xx-languageruntime-safety)
  - [G7xx: Taint Analysis](#g7xx-taint-analysis)
  - [Retired and reassigned IDs](#retired-and-reassigned-ids)
- [Rules configuration](#rules-configuration)
  - [G101](#g101)
  - [G104](#g104)
  - [G111](#g111)
  - [G117](#g117)
  - [G118](#g118)
  - [G301, G302, G306, G307](#g301-g302-g306-g307)

## Rules List

### G1xx: General Secure Coding

- [G101](#g101) — Look for hardcoded credentials (**AST**)
- G102 — Bind to all interfaces (**AST**)
- G103 — Audit the use of unsafe block (**AST**)
- [G104](#g104) — Audit errors not checked (**AST**)
- G106 — Audit the use of `ssh.InsecureIgnoreHostKey` function (**AST**)
- G107 — URL provided to HTTP request as taint input (**AST**)
- G108 — Profiling endpoint is automatically exposed (**AST**)
- G109 — Converting `strconv.Atoi` result to `int32/int16` (**AST**)
- G110 — Detect `io.Copy` instead of `io.CopyN` when decompressing (**AST**)
- [G111](#g111) — Detect `http.Dir('/')` as a potential risk (**AST**)
- G112 — Detect `ReadHeaderTimeout` not configured as a potential risk (**AST**)
- G113 — HTTP request smuggling via conflicting headers or bare LF in body parsing (**SSA**)
- G114 — Use of `net/http` serve function that has no support for setting timeouts (**AST**)
- G115 — Type conversion which leads to integer overflow (**SSA**)
- G116 — Detect Trojan Source attacks using bidirectional Unicode characters (**AST**)
- [G117](#g117) — Potential exposure of secrets via JSON/YAML/XML/TOML marshaling (**AST**)
- [G118](#g118) — Context propagation failure leading to goroutine/resource leaks (**SSA**)
- G119 — Unsafe redirect policy may propagate sensitive headers (**SSA**)
- G120 — Unbounded `ParseMultipartForm` in HTTP handlers can cause memory exhaustion (**Taint**)
- G121 — Unsafe CrossOriginProtection bypass patterns (**SSA**)
- G122 — Filesystem TOCTOU race risk in `filepath.Walk/WalkDir` callbacks (**SSA**)
- G123 — TLS resumption may bypass `VerifyPeerCertificate` when `VerifyConnection` is unset (**SSA**)
- G124 — Insecure HTTP cookie configuration missing Secure, HttpOnly, or SameSite attributes (**SSA**)

### G2xx: Injection Patterns

- G201 — SQL query construction using format string (**AST**)
- G202 — SQL query construction using string concatenation (**AST**)
- G203 — Use of unescaped data in HTML templates (**AST**)
- G204 — Audit use of command execution (**AST**)

### G3xx: Filesystem and Permissions

- [G301](#g301-g302-g306-g307) — Poor file permissions used when creating a directory (**AST**)
- [G302](#g301-g302-g306-g307) — Poor file permissions used when creating file or using `chmod` (**AST**)
- G303 — Creating tempfile using a predictable path (**AST**)
- G304 — File path provided as taint input (**AST**)
- G305 — File path traversal when extracting zip archive (**AST**)
- [G306](#g301-g302-g306-g307) — Poor file permissions used when writing to a file (**AST**)
- [G307](#g301-g302-g306-g307) — Poor file permissions used when creating a file with `os.Create` (**AST**)

### G4xx: Crypto and Protocol security

- G401 — Detect the usage of MD5 or SHA1 (**AST**)
- G402 — Look for bad TLS connection settings (**AST**)
- G403 — Ensure minimum RSA key length of 2048 bits (**AST**)
- G404 — Insecure random number source (`rand`) (**AST**)
- G405 — Detect the usage of DES or RC4 (**AST**)
- G406 — Detect the usage of deprecated MD4 or RIPEMD160 (**AST**)
- G407 — Use of hardcoded IV/nonce for encryption (**SSA**)
- G408 — Stateful misuse of `ssh.PublicKeyCallback` leading to auth bypass (**SSA**)

### G5xx: Import Blocklist

- G501 — Import blocklist: `crypto/md5` (**AST**)
- G502 — Import blocklist: `crypto/des` (**AST**)
- G503 — Import blocklist: `crypto/rc4` (**AST**)
- G504 — Import blocklist: `net/http/cgi` (**AST**)
- G505 — Import blocklist: `crypto/sha1` (**AST**)
- G506 — Import blocklist: `golang.org/x/crypto/md4` (**AST**)
- G507 — Import blocklist: `golang.org/x/crypto/ripemd160` (**AST**)

### G6xx: Language/Runtime safety

- G601 — Implicit memory aliasing in `RangeStmt` (Go 1.21 or lower) (**AST**)
- G602 — Possible slice bounds out of range (**SSA**)

### G7xx: Taint Analysis

- G701 — SQL injection via taint analysis (**Taint**)
- G702 — Command injection via taint analysis (**Taint**)
- G703 — Path traversal via taint analysis (**Taint**)
- G704 — SSRF via taint analysis (**Taint**)
- G705 — XSS via taint analysis (**Taint**)
- G706 — Log injection via taint analysis (**Taint**)
- G707 — SMTP command/header injection via taint analysis (**Taint**)
- G708 — Server-side template injection via `text/template` (**Taint**)
- G709 — Unsafe deserialization of untrusted data (**Taint**)
- G710 — Open redirect via taint analysis (**Taint**)

_Note: Implementation types used in this document:_
- **AST**: rule implemented in `rules/` and evaluated on AST patterns
- **SSA**: analyzer implemented in `analyzers/` using the analyzer framework (SSA-backed execution path)
- **Taint**: taint analysis rule implemented via `taint.NewGosecAnalyzer`

### Retired and reassigned IDs

- G105 is retired.
- G307 (old meaning: deferred method error handling) is retired; the ID now refers to file creation permissions.
- G113 was previously used for a retired `math/big` check and is now used for HTTP request smuggling.

## Rules configuration

Some rules accept configuration in the gosec JSON config file.
Per-rule settings are top-level objects keyed by rule ID (`Gxxx`).

Configurable rules (alphabetical): [G101](#g101), [G104](#g104), [G111](#g111), [G117](#g117), [G301](#g301-g302-g306-g307), [G302](#g301-g302-g306-g307), [G306](#g301-g302-g306-g307), [G307](#g301-g302-g306-g307).

### G101

`G101` (hardcoded credentials) can be configured with custom patterns and entropy thresholds:

```json
{
  "G101": {
    "pattern": "(?i)passwd|pass|password|pwd|secret|private_key|token",
    "ignore_entropy": false,
    "entropy_threshold": "80.0",
    "per_char_threshold": "3.0",
    "truncate": "32",
    "min_entropy_length": "8"
  }
}
```

### G104

`G104` (unchecked errors) can be configured with function allowlists:

```json
{
  "G104": {
    "ioutil": ["WriteFile"]
  }
}
```

### G111

`G111` (HTTP directory serving) can be configured with a custom detection regex.
This replaces the default pattern.

```json
{
  "G111": {
    "pattern": "http\\.Dir\\(\"\\/\"\\)|http\\.Dir\\('\\/'\\)"
  }
}
```

### G117

`G117` (secret serialization) can be configured with a custom field-name pattern.

```json
{
  "G117": {
    "pattern": "(?i)secret|token|password"
  }
}
```

### G118

`G118` detects three classes of context-propagation failure using SSA-level analysis:

**1. Lost cancel function (CWE-400)**

Reports when a `context.WithCancel`, `context.WithTimeout`, or `context.WithDeadline` call
returns a cancel function that is never called, potentially leaking resources.

```go
// Flagged: cancel never called
func work(ctx context.Context) {
    child, _ := context.WithTimeout(ctx, time.Second)
    _ = child
}

// Safe: cancel deferred
func work(ctx context.Context) {
    child, cancel := context.WithTimeout(ctx, time.Second)
    defer cancel()
    _ = child
}
```

The following patterns are all recognised as *safe* (cancel is considered called):

| Pattern | Description |
|---|---|
| `defer cancel()` | Direct deferred call |
| `defer func() { cancel() }()` | Cancel in a deferred closure |
| `cancelCopy := cancel; defer cancelCopy()` | Alias via variable |
| `return ctx, cancel` | Cancel returned to caller (responsibility transferred) |
| `s.cancelFn = cancel` + method `s.cancelFn()` | Stored in struct field, called via receiver method |
| `s.cancel = cancel; defer s.cancel()` | Stored in struct field, deferred in same function |
| `s.cancel = cancel; defer func() { s.cancel() }()` | Stored in struct field, called in closure |
| Struct containing field is returned | Caller inherits cancel responsibility |
| `var cancel CancelFunc` in `init()` + `cancel()` in another function | Package-level variable assigned in init, called in any function (e.g., signal handlers) |

Example of package-level variable pattern:

```go
// Safe: cancel stored in package-level variable and called in signal handler
var cancel context.CancelFunc

func init() {
    ctx, c := context.WithCancel(context.Background())
    cancel = c
}

func handleShutdown() {
    cancel() // Called from signal handler
}
```

**2. Goroutine uses `context.Background`/`TODO` when request context is available (CWE-400)**

Reports when a goroutine spawned inside an HTTP handler or a function accepting a
`context.Context` / `*http.Request` uses `context.Background()` or `context.TODO()`
instead of the request-scoped context.

```go
// Flagged
func handler(w http.ResponseWriter, r *http.Request) {
    go func() {
        ctx := context.Background() // ignores request context
        doWork(ctx)
    }()
}
```

**3. Long-running loop without `ctx.Done()` guard (CWE-400)**

Reports an infinite loop that performs blocking I/O (e.g. `http.Get`, `db.Query`,
`time.Sleep`, interface methods such as `Read`/`Write`) but never checks `ctx.Done()`,
making the loop impossible to cancel.

```go
// Flagged
func poll(ctx context.Context) {
    for {
        http.Get("https://example.com") // blocks, no cancellation path
        time.Sleep(time.Second)
    }
}

// Safe
func poll(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case <-time.After(time.Second):
            http.Get("https://example.com")
        }
    }
}
```

Loops with an external exit path (e.g. a `break` or bounded `for i < n`) are not flagged.

### G301, G302, G306, G307

File and directory permission rules can be configured with stricter maximum permissions:

```json
{
  "G301": "0o600",
  "G302": "0o600",
  "G306": "0o750",
  "G307": "0o750"
}
```
