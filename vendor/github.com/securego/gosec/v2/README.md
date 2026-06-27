
# gosec - Go Security Checker

Inspects source code for security problems by scanning the Go AST
and SSA code representation.

<img src="https://securego.io/img/gosec.png" width="320">

## Quick links

- [GitHub Action](#github-action)
- [Local installation](#local-installation)
- [Quick start](#quick-start)
- [Common usage patterns](#common-usage-patterns)
- [Selecting rules](#selecting-rules)
- [Output formats](#output-formats)

## Features

- **Pattern-based rules** for detecting common security issues
  in Go code
- **SSA-based analyzers** for type conversions, slice bounds,
  and crypto issues
- **Taint analysis** for tracking data flow from user input to
  dangerous functions (SQL injection, command injection, path
  traversal, SSRF, XSS, log injection, SMTP injection, SSTI,
  unsafe deserialization, open redirect)

## License

Licensed under the Apache License, Version 2.0 (the "License").
You may not use this file except in compliance with the License.
You may obtain a copy of the License
[here](http://www.apache.org/licenses/LICENSE-2.0).

## Project status

[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/3218/badge)](https://bestpractices.coreinfrastructure.org/projects/3218)
[![Build Status](https://github.com/securego/gosec/workflows/CI/badge.svg)](https://github.com/securego/gosec/actions?query=workflows%3ACI)
[![Coverage Status](https://codecov.io/gh/securego/gosec/branch/master/graph/badge.svg)](https://codecov.io/gh/securego/gosec)
[![GoReport](https://goreportcard.com/badge/github.com/securego/gosec)](https://goreportcard.com/report/github.com/securego/gosec)
[![GoDoc](https://pkg.go.dev/badge/github.com/securego/gosec/v2)](https://pkg.go.dev/github.com/securego/gosec/v2)
[![Docs](https://readthedocs.org/projects/docs/badge/?version=latest)](https://securego.io/)
[![Downloads](https://img.shields.io/github/downloads/securego/gosec/total.svg)](https://github.com/securego/gosec/releases)
[![GHCR](https://img.shields.io/badge/ghcr.io-securego%2Fgosec-blue)](https://github.com/orgs/securego/packages/container/package/gosec)
[![Slack](https://img.shields.io/badge/Slack-4A154B?style=for-the-badge&logo=slack&logoColor=white)](http://securego.slack.com)
[![go-recipes](https://raw.githubusercontent.com/nikolaydubina/go-recipes/main/badge.svg?raw=true)](https://github.com/nikolaydubina/go-recipes)

## Installation

### GitHub Action

You can run `gosec` as a GitHub action as follows:

Use the versioned tag with `@master` which is pinned to the
latest stable release. This will provide a stable behavior.

```yaml
name: Run Gosec
on:
  push:
    branches:
      - master
  pull_request:
    branches:
      - master
jobs:
  tests:
    runs-on: ubuntu-latest
    env:
      GO111MODULE: on
    steps:
      - name: Checkout Source
        uses: actions/checkout@v3
      - name: Run Gosec Security Scanner
        uses: securego/gosec@master
        with:
          args: ./...
```

#### Scanning Projects with Private Modules

If your project imports private Go modules, you need to
configure authentication so that `gosec` can fetch the
dependencies. Set the following environment variables in
your workflow:

- `GOPRIVATE`: A comma-separated list of module path prefixes
  that should be considered private
  (e.g., `github.com/your-org/*`).
- `GITHUB_AUTHENTICATION_TOKEN`: A GitHub token with read
  access to your private repositories.

```yaml
name: Run Gosec
on:
  push:
    branches:
      - master
  pull_request:
    branches:
      - master
jobs:
  tests:
    runs-on: ubuntu-latest
    env:
      GO111MODULE: on
      GOPRIVATE: github.com/your-org/*
      GITHUB_AUTHENTICATION_TOKEN: ${{ secrets.PRIVATE_REPO_TOKEN }}
    steps:
      - name: Checkout Source
        uses: actions/checkout@v3
      - name: Run Gosec Security Scanner
        uses: securego/gosec@v2
        with:
          args: ./...
```

### Integrating with code scanning

You can [integrate third-party code analysis tools](https://docs.github.com/en/github/finding-security-vulnerabilities-and-errors-in-your-code/integrating-with-code-scanning)
with GitHub code scanning by uploading data as SARIF files.

The workflow shows an example of running the `gosec` as a step
in a GitHub action workflow which outputs the `results.sarif`
file. The workflow then uploads the `results.sarif` file to
GitHub using the `upload-sarif` action.

```yaml
name: "Security Scan"

# Run workflow each time code is pushed to your repository and on a schedule.
# The scheduled workflow runs every at 00:00 on Sunday UTC time.
on:
  push:
  schedule:
  - cron: '0 0 * * 0'

jobs:
  tests:
    runs-on: ubuntu-latest
    env:
      GO111MODULE: on
    steps:
      - name: Checkout Source
        uses: actions/checkout@v3
      - name: Run Gosec Security Scanner
        uses: securego/gosec@v2
        with:
          # we let the report trigger content trigger a failure using the GitHub Security features.
          args: '-no-fail -fmt sarif -out results.sarif ./...'
      - name: Upload SARIF file
        uses: github/codeql-action/upload-sarif@v2
        with:
          # Path to SARIF file relative to the root of the repository
          sarif_file: results.sarif
```

### Go Analysis

The `goanalysis` package provides a
[`golang.org/x/tools/go/analysis.Analyzer`](https://pkg.go.dev/golang.org/x/tools/go/analysis)
for integration with tools that support the standard Go
analysis interface, such as Bazel's
[nogo](https://github.com/bazelbuild/rules_go/blob/master/go/nogo.rst)
framework:

```starlark
nogo(
    name = "nogo",
    deps = [
        "@com_github_securego_gosec_v2//goanalysis",
        # add more analyzers as needed
    ],
    visibility = ["//visibility:public"],
)
```

### Local Installation

gosec requires Go 1.25 or newer.

```bash
go install github.com/securego/gosec/v2/cmd/gosec@latest
```

## Quick start

```bash
# Scan all packages in current module
gosec ./...

# Write JSON report
gosec -fmt json -out results.json ./...

# Write SARIF report for code scanning
gosec -fmt sarif -out results.sarif ./...
```

### Exit codes

- `0`: scan finished without unsuppressed findings/errors
- `1`: at least one unsuppressed finding or processing error
- Use `-no-fail` to always return `0`

## Usage

Gosec can be configured to only run a subset of rules, to
exclude certain file paths, and produce reports in different
formats. By default all rules will be run against the supplied
input files. To recursively scan from the current directory you
can supply `./...` as the input argument.

### Available rules

gosec includes rules across these categories:

- `G1xx`: general secure coding issues (for example hardcoded
  credentials, unsafe usage, HTTP hardening, cookie security)
- `G2xx`: injection risks in query/template/command
  construction
- `G3xx`: file and path handling risks (permissions, traversal,
  temp files, archive extraction)
- `G4xx`: crypto and TLS weaknesses
- `G5xx`: blocklisted imports
- `G6xx`: Go-specific correctness/security checks (for example
  range aliasing and slice bounds)
- `G7xx`: taint analysis rules (SQL injection, command
  injection, path traversal, SSRF, XSS, log, SMTP injection,
  SSTI, unsafe deserialization, and open redirect)

For the full list, rule descriptions, and per-rule
configuration, see [RULES.md](RULES.md).

### Retired rules

- G105: Audit the use of math/big.Int.Exp -
  [CVE is fixed](https://github.com/golang/go/issues/15184)
- G307: Deferring a method which returns an error - causing
  more inconvenience than fixing a security issue, despite the
  details from this
  [blog post](https://www.joeshaw.org/dont-defer-close-on-writable-files/)

### Selecting rules

By default, gosec will run all rules against the supplied file
paths. It is however possible to select a subset of rules to
run via the `-include=` flag, or to specify a set of rules to
explicitly exclude using the `-exclude=` flag.

```bash
# Run a specific set of rules
$ gosec -include=G101,G203,G401 ./...

# Run everything except for rule G303
$ gosec -exclude=G303 ./...
```

### CWE Mapping

Every issue detected by `gosec` is mapped to a
[CWE (Common Weakness Enumeration)](http://cwe.mitre.org/data/index.html)
which describes in more generic terms the vulnerability. The
exact mapping can be found
[here](https://github.com/securego/gosec/blob/master/issue/issue.go#L50).

### Configuration

A number of global settings can be provided in a configuration
file as follows:

```JSON
{
    "global": {
        "nosec": "enabled",
        "audit": "enabled"
    }
}
```

- `nosec`: this setting will overwrite all `#nosec` directives
  defined throughout the code base
- `audit`: runs in audit mode which enables addition checks
  that for normal code analysis might be too nosy

```bash
# Run with a global configuration file
$ gosec -conf config.json .
```

### Path-Based Rule Exclusions

Large repositories with multiple components may need different
security rules for different paths. Use `exclude-rules` to
suppress specific rules for specific paths.

**Configuration File:**
```json
{
  "exclude-rules": [
    {
      "path": "cmd/.*",
      "rules": ["G204", "G304"]
    },
    {
      "path": "scripts/.*",
      "rules": ["*"]
    }
  ]
}
```

**CLI Flag:**
```bash
# Exclude G204 and G304 from cmd/ directory
gosec --exclude-rules="cmd/.*:G204,G304" ./...

# Exclude all rules from scripts/ directory  
gosec --exclude-rules="scripts/.*:*" ./...

# Multiple exclusions
gosec --exclude-rules="cmd/.*:G204,G304;test/.*:G101" ./...
```

| Field | Type | Description |
|-------|------|-------------|
| `path` | string (regex) | Regex matched against file paths |
| `rules` | []string | Rule IDs to exclude. `*` for all |

#### Rule Configuration

Some rules accept configuration flags as well; these flags are
documented in
[RULES.md](https://github.com/securego/gosec/blob/master/RULES.md).

#### Go version

Some rules require a specific Go version which is retrieved
from the Go module file present in the project. If this version
cannot be found, it will fallback to Go runtime version.

The Go module version is parsed using the `go list` command
which in some cases might lead to performance degradation. In
this situation, the go module version can be easily provided by
setting the environment variable
`GOSECGOVERSION=go1.21.1`.

### Dependencies

gosec loads packages using Go modules. In most projects,
dependencies are resolved automatically during scanning.

If dependencies are missing, run:

```bash
go mod tidy
go mod download
```

### Excluding test files and folders

gosec will ignore test files across all packages and any
dependencies in your vendor directory.

The scanning of test files can be enabled with the following
flag:

```bash
gosec -tests ./...
```

Also additional folders can be excluded as follows:

```bash
 gosec -exclude-dir=rules -exclude-dir=cmd ./...
```

### Excluding generated files

gosec can ignore generated go files with default generated
code comment.

```
// Code generated by some generator DO NOT EDIT.
```

```bash
gosec -exclude-generated ./...
```

### Auto fixing vulnerabilities

gosec can suggest fixes based on AI recommendation. It will
call an AI API to receive a suggestion for a security finding.

You can enable this feature by providing the following command
line arguments:

- `ai-api-provider`: the name of the AI API provider.
  Supported providers:
  - **Gemini**: `gemini-3-pro-preview` (default),
    `gemini-2.5-pro`, `gemini-2.5-flash`,
    `gemini-2.5-flash-lite`
  - **Claude**: `claude-sonnet-4-6` (default),
    `claude-opus-4-7`, `claude-opus-4-6`,
    `claude-sonnet-4-5`, `claude-opus-4-5`,
    `claude-haiku-4-5`
  - **OpenAI**: `gpt-5.4` (default), `gpt-5.4-mini`,
    `gpt-5.4-nano`
  - **Custom OpenAI-compatible**: Any custom model name
    (requires `ai-base-url`)
- `ai-api-key` or set the environment variable
  `GOSEC_AI_API_KEY`: the key to access the AI API
  - For Gemini, you can create an API key following
    [these instructions](https://ai.google.dev/gemini-api/docs/api-key)
  - For Claude, get your API key from
    [Anthropic Console](https://console.anthropic.com/)
  - For OpenAI, get your API key from
    [OpenAI Platform](https://platform.openai.com/api-keys)
- `ai-base-url`: (optional) custom base URL for
  OpenAI-compatible APIs (e.g., Azure OpenAI, LocalAI,
  Ollama)
- `ai-skip-ssl`: (optional) skip SSL certificate verification
  for AI API (useful for self-signed certificates)

**Examples:**

```bash
# Using Gemini
gosec -ai-api-provider="gemini-3-pro-preview" \
  -ai-api-key="your_key" ./...

# Using Claude
gosec -ai-api-provider="claude-sonnet-4-6" \
  -ai-api-key="your_key" ./...

# Using OpenAI
gosec -ai-api-provider="gpt-5.4" \
  -ai-api-key="your_key" ./...

# Using Azure OpenAI
gosec -ai-api-provider="gpt-5.4" \
  -ai-api-key="your_azure_key" \
  -ai-base-url="https://your-resource.openai.azure.com/openai/deployments/your-deployment" \
  ./...

# Using local Ollama with custom model
gosec -ai-api-provider="llama3.2" \
  -ai-base-url="http://localhost:11434/v1" \
  ./...

# Using self-signed certificate API
gosec -ai-api-provider="custom-model" \
  -ai-api-key="your_key" \
  -ai-base-url="https://internal-api.company.com/v1" \
  -ai-skip-ssl \
  ./...
```

### Annotating code

As with all automated detection tools, there will be cases of
false positives. In cases where gosec reports a failure that
has been manually verified as being safe, it is possible to
annotate the code with a comment that starts with `#nosec`.

The `#nosec` comment should have the format
`#nosec [RuleList] [- Justification]`.

The `#nosec` comment needs to be placed on the line where the
warning is reported.

```go
func main() {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // #nosec G402
		},
	}

	client := &http.Client{Transport: tr}
	_, err := client.Get("https://go.dev/")
	if err != nil {
		fmt.Println(err)
	}
}
```

When a specific false positive has been identified and verified
as safe, you may wish to suppress only that single rule (or a
specific set of rules) within a section of code, while
continuing to scan for other problems. To do this, you can list
the rule(s) to be suppressed within the `#nosec` annotation,
e.g: `/* #nosec G401 */` or `//#nosec G201 G202 G203`

You could put the description or justification text for the
annotation. The justification should be after the rule(s) to
suppress and start with two or more dashes,
e.g: `//#nosec G101 G102 -- This is a false positive`

Alternatively, gosec also supports the `//gosec:disable`
directive, which functions similar to `#nosec`:

```go
//gosec:disable G101 -- This is a false positive
```

In some cases you may also want to revisit places where
`#nosec` or `//gosec:disable` annotations have been used. To
run the scanner and ignore any `#nosec` annotations you can do
the following:

```bash
gosec -nosec=true ./...
```

### Tracking suppressions

As described above, we could suppress violations externally
(using `-include`/`-exclude`) or inline (using `#nosec`
annotations). Suppression metadata can be emitted for auditing.

Enable suppression tracking with `-track-suppressions`:

```bash
gosec -track-suppressions -exclude=G101 \
  -fmt=sarif -out=results.sarif ./...
```

- For external suppressions, gosec records suppression info
  where `kind` is `external` and `justification` is
  `Globally suppressed.`.
- For inline suppressions, gosec records suppression info
  where `kind` is `inSource` and `justification` is the text
  after two or more dashes in the comment.

**Note:** Only SARIF and JSON formats support tracking
suppressions.

### Build tags

gosec is able to pass your
[Go build tags](https://pkg.go.dev/go/build/) to the analyzer.
They can be provided as a comma separated list as follows:

```bash
gosec -tags debug,ignore ./...
```

### Output formats

gosec supports `text`, `json`, `yaml`, `csv`, `junit-xml`,
`html`, `sonarqube`, `golint`, and `sarif`. By default,
results will be reported to stdout, but can also be written to
an output file. The output format is controlled by the `-fmt`
flag, and the output file is controlled by the `-out` flag as
follows:

```bash
# Write output in json format to results.json
$ gosec -fmt=json -out=results.json *.go
```

Use `-stdout` to print results while also writing `-out`.
Use `-verbose` to override stdout format while preserving the
file format.
```bash
# Write output in json format to results.json as well as stdout
$ gosec -fmt=json -out=results.json -stdout *.go

# Overrides the output format to 'text' when stdout the results,
# while writing it to results.json
$ gosec -fmt=json -out=results.json -stdout -verbose=text *.go
```

**Note:** gosec generates the
[generic issue import format](https://docs.sonarqube.org/latest/analysis/generic-issue/)
for SonarQube, and a report has to be imported into SonarQube
using
`sonar.externalIssuesReportPaths=path/to/gosec-report.json`.

## Common usage patterns

```bash
# Fail only on medium+ severity findings
gosec -severity medium ./...

# Fail only on medium+ confidence findings
gosec -confidence medium ./...

# Exclude specific rules for specific paths
gosec --exclude-rules="cmd/.*:G204,G304;scripts/.*:*" ./...

# Exclude generated files in scan
gosec -exclude-generated ./...

# Include test files in scan
gosec -tests ./...
```

## Development

Development documentation was moved to
[DEVELOPMENT.md](DEVELOPMENT.md).

## Who is using gosec?

This is a [list](USERS.md) with some of the gosec's users.

## Sponsors

Support this project by becoming a sponsor. Your logo will
show up here with a link to your website

<a href="https://github.com/mercedes-benz" target="_blank"><img src="https://avatars.githubusercontent.com/u/34240465?s=80&v=4"></a>
