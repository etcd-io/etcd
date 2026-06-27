# Development

## Table of Contents

- [Local workflow](#local-workflow)
- [Contributing: adding rules and analyzers](#contributing-adding-rules-and-analyzers)
	- [Add an AST rule](#add-an-ast-rule)
	- [Add an SSA analyzer](#add-an-ssa-analyzer)
	- [Creating taint analysis rules](#creating-taint-analysis-rules)
		- [Steps](#steps)
		- [Taint configuration reference](#taint-configuration-reference)
			- [Sources](#sources)
			- [Sinks](#sinks)
			- [Sanitizers](#sanitizers)
		- [Common taint sources](#common-taint-sources)
- [AI-generated rule workflow (Copilot)](#ai-generated-rule-workflow-copilot)
- [AI-generated bug fix workflow (Copilot)](#ai-generated-bug-fix-workflow-copilot)
- [AI-supported Go version update workflow (Copilot)](#ai-supported-go-version-update-workflow-copilot)
- [Rule development utilities](#rule-development-utilities)
- [SARIF types generation](#sarif-types-generation)
- [Performance regression guard](#performance-regression-guard)
- [Generate TLS rule data](#generate-tls-rule-data)
- [Release](#release)
- [Docker image](#docker-image)

## Local workflow

- Go version: `1.25+` (see `go.mod`)
- Build: `make`
- Run all checks used in CI (format, vet, security scan, vulnerability scan, tests): `make test`
- Run linter only: `make golangci`

## Contributing: adding rules and analyzers

gosec supports three implementation styles:

- **AST rules** (`gosec.Rule`) for node-level checks in `rules/`
- **SSA analyzers** (`analysis.Analyzer`) for whole-program context in `analyzers/`
- **Taint analyzers** for source-to-sink data-flow checks in `analyzers/` via `taint.NewGosecAnalyzer`

### Add an AST rule

1. Create a new file in `rules/` (for example, use `rules/unsafe.go` as a simple template).
2. Implement your rule constructor and `Match` logic.
3. Register the rule in `rules/rulelist.go`.
4. Add rule-to-CWE mapping in `issue/issue.go` (and add CWE data in `cwe/data.go` only if needed).
5. Add tests and samples:
   - sample code in `testutils/`
   - rule tests in `rules/` or integration tests in `analyzer_test.go`

### Add an SSA analyzer

1. Create a new file in `analyzers/`.
2. Define the analyzer and require `buildssa.Analyzer`.
3. Read SSA input using `ssautil.GetSSAResult(pass)`.
4. Return findings as `[]*issue.Issue`.
5. Register in `analyzers/analyzerslist.go`.
6. Add rule-to-CWE mapping in `issue/issue.go`.
7. Add tests and sample code in `analyzers/` and `testutils/`.

Minimal skeleton:

```go
package analyzers

import (
	"fmt"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"

	"github.com/securego/gosec/v2/internal/ssautil"
	"github.com/securego/gosec/v2/issue"
)

func newMyAnalyzer(id, description string) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     id,
		Doc:      description,
		Run:      runMyAnalyzer,
		Requires: []*analysis.Analyzer{buildssa.Analyzer},
	}
}

func runMyAnalyzer(pass *analysis.Pass) (interface{}, error) {
	ssaResult, err := ssautil.GetSSAResult(pass)
	if err != nil {
		return nil, fmt.Errorf("getting SSA result: %w", err)
	}
	_ = ssaResult

	var issues []*issue.Issue
	return issues, nil
}
```

### Creating taint analysis rules

gosec taint analyzers track data flow from untrusted sources to dangerous sinks.
Current taint rules include SQL injection, command injection, path traversal, SSRF, XSS, log injection, SMTP injection, server-side template injection, unsafe deserialization, and open redirect.

#### Steps

1. Create a new analyzer file in `analyzers/` (for example `analyzers/newvuln.go`) with both:
   - the taint `Config` (sources, sinks, optional sanitizers)
   - the analyzer constructor that returns `taint.NewGosecAnalyzer(...)`

```go
package analyzers

import (
	"golang.org/x/tools/go/analysis"

	"github.com/securego/gosec/v2/taint"
)

func NewVulnerability() taint.Config {
	return taint.Config{
		Sources: []taint.Source{
			{Package: "net/http", Name: "Request", Pointer: true},
			{Package: "os", Name: "Args", IsFunc: true},
		},
		Sinks: []taint.Sink{
			{Package: "dangerous/package", Method: "DangerousFunc"},
		},
	}
}

func newNewVulnAnalyzer(id string, description string) *analysis.Analyzer {
	config := NewVulnerability()
	rule := NewVulnerabilityRule
	rule.ID = id
	rule.Description = description
	return taint.NewGosecAnalyzer(&rule, &config)
}
```

2. Register the analyzer in `analyzers/analyzerslist.go`:

```go
var defaultAnalyzers = []AnalyzerDefinition{
	// ... existing analyzers ...
	{"G7XX", "Description of vulnerability", newNewVulnAnalyzer},
}
```

3. Add sample programs in `testutils/g7xx_samples.go`.

4. Add the analyzer test in `analyzers/analyzers_test.go`:

```go
It("should detect your new vulnerability", func() {
	runner("G7XX", testutils.SampleCodeG7XX)
})
```

Each taint analyzer keeps its configuration function in the same file as the analyzer.
Reference implementations:
- `analyzers/sqlinjection.go` (G701)
- `analyzers/commandinjection.go` (G702)
- `analyzers/pathtraversal.go` (G703)

#### Taint configuration reference

##### Sources

Sources define where untrusted data starts:
- `Package`: import path (for example `"net/http"`)
- `Name`: type or function name (for example `"Request"`, `"Getenv"`)
- `Pointer`: set `true` for pointer types (for example `*http.Request`)
- `IsFunc`: set `true` when the source is a function that returns tainted data

##### Sinks

Sinks define where tainted data must not reach:
- `Package`
- `Receiver`: method receiver type, empty for package functions
- `Method`
- `Pointer`: whether receiver is a pointer
- `CheckArgs`: optional argument indexes to inspect; if omitted, all args are inspected

Example:

```go
// For *sql.DB.Query, Args[1] is the query string.
{Package: "database/sql", Receiver: "DB", Method: "Query", Pointer: true, CheckArgs: []int{1}}

// Skip writer arg in fmt.Fprintf and check the rest.
{Package: "fmt", Method: "Fprintf", CheckArgs: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}}
```

##### Sanitizers

Sanitizers break taint flow after validation/escaping:
- `Package`
- `Receiver`
- `Method`
- `Pointer`

If data passes through a configured sanitizer, it is treated as safe for subsequent sinks.

#### Common taint sources

| Source Type | Package | Type/Method | Pointer | IsFunc |
|-------------|---------|-------------|---------|--------|
| HTTP Request | `net/http` | `Request` | `true` | `false` |
| Command Line Args | `os` | `Args` | `false` | `true` |
| Environment Variables | `os` | `Getenv` | `false` | `true` |
| File Content | `bufio` | `Reader` | `true` | `false` |

## AI-generated rule workflow (Copilot)

This repository includes a reusable Copilot skill and prompt for creating new gosec rules from an issue description.

- Skill file: `.github/skills/gosec-new-rule/SKILL.md`
- Prompt file: `.github/prompts/create-gosec-rule.prompt.md`

### Use via `/prompt` (recommended)

1. In VS Code Copilot Chat, run `/prompt` and select **Create Gosec Rule**.
2. Fill in the issue fields (`Summary`, repro steps, versions, environment, expected, actual).
3. Submit the prompt.
4. First response should only propose:
	- rule ID
	- implementation approach (SSA / taint / AST)
	- relevance for Go `1.25` and `1.26`
	- confirmation request
5. Reply with explicit confirmation (for example: `Confirmed. Proceed with implementation.`).

### Use the skill directly (without `/prompt`)

Send this in Copilot Chat:

```text
Use the skill "Create New Gosec Rule" from .github/skills/gosec-new-rule/SKILL.md.
```

Then paste the same issue template fields and confirm after the proposal step.

### If `/prompt` does not list the prompt

1. Ensure the workspace root is this repository.
2. Confirm the file exists at `.github/prompts/create-gosec-rule.prompt.md`.
3. Reload VS Code window and start a new chat session.
4. As fallback, open the prompt file and send its content directly in chat.

## AI-generated bug fix workflow (Copilot)

This repository also includes a Copilot skill and prompt for fixing bugs described in GitHub issues.

- Skill file: `.github/skills/gosec-fix-issue/SKILL.md`
- Prompt file: `.github/prompts/fix-gosec-bug-from-issue.prompt.md`

### Use via `/prompt` (recommended)

1. In VS Code Copilot Chat, run `/prompt` and select **Fix Gosec Bug From Issue**.
2. Fill in at least the `GitHub issue URL` field (other fields are optional but useful).
3. Submit the prompt.
4. First response should only include:
	- reproduction status on `master` (or clear blocker)
	- root cause analysis
	- detailed fix plan
	- confirmation request
5. Reply with explicit confirmation (for example: `Confirmed. Proceed with fix.`).

### Use the skill directly (without `/prompt`)

Send this in Copilot Chat:

```text
Use the skill "Fix Gosec Bug From Issue" from .github/skills/gosec-fix-issue/SKILL.md.
```

Then provide the GitHub issue URL and confirm after the analysis and plan step.

### Expected implementation guardrails

After confirmation, the workflow should:

- keep the fix small and isolated to the problem
- use idiomatic Go and good design
- add positive and negative tests
- add or update `testutils/` code samples when appropriate for reproducing/validating the issue
- validate with build, tests, `golangci-lint`, and a `gosec` CLI run against a sample

## AI-supported Go version update workflow (Copilot)

This repository includes a Copilot skill and prompt to update supported Go versions to the latest patch versions of the two newest major Go series.

- Skill file: `.github/skills/gosec-update-go-versions/SKILL.md`
- Prompt file: `.github/prompts/update-supported-go-versions.prompt.md`

### Use via `/prompt` (recommended)

1. In VS Code Copilot Chat, run `/prompt` and select **Update Supported Go Versions**.
2. Submit the prompt (no additional fields required).
3. The workflow should:
	- read `https://go.dev/doc/devel/release`
	- detect latest two supported Go series and latest patch for each
	- update all active repository locations where supported Go versions are configured or documented
	- run validation checks
	- create branch, commit, push, and open a PR

### Use the skill directly (without `/prompt`)

Send this in Copilot Chat:

```text
Use the skill "Update Supported Go Versions" from .github/skills/gosec-update-go-versions/SKILL.md.
```

### Expected outputs

The result should include:

- detected versions (`previous_patch`, `latest_patch`, `previous_minor`, `latest_minor`)
- grouped file update summary
- test command result
- branch, commit SHA, PR title, and PR URL

## Rule development utilities

Use these tools while building or debugging rules:

- Dump SSA with [`ssadump`](https://pkg.go.dev/golang.org/x/tools/cmd/ssadump):

```bash
ssadump -build F main.go
```

- Inspect AST/types/defs/imports with `gosecutil`:

```bash
gosecutil -tool ast main.go
```

Valid `-tool` values: `ast`, `callobj`, `uses`, `types`, `defs`, `comments`, `imports`.


## SARIF types generation

Install `schema-generate`:

```bash
go install github.com/a-h/generate/cmd/schema-generate@latest
```

Generate types:

```bash
schema-generate -i sarif-schema-2.1.0.json -o path/to/types.go
```

Most `MarshalJSON`/`UnmarshalJSON` helpers can be removed after generation, except `PropertyBag` where inlined additional properties are useful.

## Performance regression guard

CI includes a taint benchmark guard based on `BenchmarkTaintPackageAnalyzers_SharedCache`.

- Baseline and thresholds: `.github/benchmarks/taint_benchmark_baseline.env`
- Guard script: `tools/check_taint_benchmark.sh`

Run locally:

```bash
bash tools/check_taint_benchmark.sh
```

Update baseline after intentional changes:

```bash
BENCH_COUNT=10 bash tools/check_taint_benchmark.sh --update-baseline
```

If you update the baseline, commit both the benchmark-related code and the baseline file.

## Generate TLS rule data

The TLS rule data is generated from Mozilla recommendations.

From the repository root:

```bash
go generate ./...
```

If `go generate` fails with `exec: "tlsconfig": executable file not found in $PATH`, install the local generator and add `$(go env GOPATH)/bin` to `PATH`:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
go install ./cmd/tlsconfig
go generate ./...
```

This updates `rules/tls_config.go`.

If you need to install the generator binary outside this repository:

```bash
go install github.com/securego/gosec/v2/cmd/tlsconfig@latest
```

## Release

Tag and push:

```bash
git tag v1.0.0 -m "Release version v1.0.0"
git push origin v1.0.0
```

The release workflow builds binaries and Docker images, then signs artifacts.

Verify signatures:

```bash
cosign verify --key cosign.pub ghcr.io/securego/gosec:<TAG>
cosign verify-blob --key cosign.pub --signature gosec_<VERSION>_darwin_amd64.tar.gz.sig gosec_<VERSION>_darwin_amd64.tar.gz
```

## Docker image

Build locally:

```bash
make image
```

Run against a local project:

```bash
docker run --rm -it -w /<PROJECT>/ -v <YOUR_PROJECT_PATH>/<PROJECT>:/<PROJECT> ghcr.io/securego/gosec:latest /<PROJECT>/...
```

Set `-w` so module dependencies resolve from the mounted project root.