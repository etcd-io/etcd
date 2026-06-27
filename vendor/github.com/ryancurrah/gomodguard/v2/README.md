# gomodguard
[![License](https://img.shields.io/github/license/ryancurrah/gomodguard?style=flat-square)](/LICENSE)
[![Codecov](https://img.shields.io/codecov/c/gh/ryancurrah/gomodguard?style=flat-square)](https://codecov.io/gh/ryancurrah/gomodguard)
[![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/ryancurrah/gomodguard/go.yml?branch=main&logo=Go&style=flat-square)](https://github.com/ryancurrah/gomodguard/actions?query=workflow%3AGo)
[![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/ryancurrah/gomodguard?style=flat-square)](https://github.com/ryancurrah/gomodguard/releases/latest)
[![Docker](https://img.shields.io/docker/pulls/ryancurrah/gomodguard?style=flat-square)](https://hub.docker.com/r/ryancurrah/gomodguard)
[![Github Releases Stats of golangci-lint](https://img.shields.io/github/downloads/ryancurrah/gomodguard/total.svg?logo=github&style=flat-square)](https://somsubhra.com/github-release-stats/?username=ryancurrah&repository=gomodguard)

<img src="https://storage.googleapis.com/gopherizeme.appspot.com/gophers/9afcc208898c763be95f046eb2f6080146607209.png" width="30%">

Allow and block list linter for direct Go module dependencies. This is useful for organizations where they want to standardize on the modules used and be able to recommend alternative modules.

## Description

Allowed and blocked modules are defined in a `./.gomodguard.yaml` or `~/.gomodguard.yaml` file. 

Modules can be allowed by module or prefix name. When allowed modules are specified any modules not in the allowed configuration are blocked.

If no allowed modules or module prefixes are specified then all modules are allowed except for blocked ones.

The linter looks for blocked modules in `go.mod` and searches for imported packages where the imported packages module is blocked. Indirect modules are not considered.

Alternative modules can be optionally recommended in the blocked modules list.

If the linted module imports a blocked module but the linted module is in the recommended modules list the blocked module is ignored. Usually, this means the linted module wraps that blocked module for use by other modules, therefore the import of the blocked module should not be blocked.

Version constraints can be specified for modules as well which lets you block new or old versions of modules or specific versions.

Results are printed to `stdout`.

Logging statements are printed to `stderr`.

Results can be exported to different report formats. Which can be imported into CI tools. See the help section for more information.

# Configuration

```yaml
# allowed defines the modules that are permitted as direct dependencies.
# When this section is non-empty, any module not matched by an entry is blocked.
# When omitted entirely, all modules are allowed except those in the blocked list.
allowed:
  # Exact match (default when match-type is omitted).
  - module: go.yaml.in/yaml/v4
  - module: github.com/go-xmlfmt/xmlfmt

  # version constrains which versions of the module are allowed.
  # Uses semver constraint syntax (e.g. ">= 1.0.0", "~1.2", "== 2.5.0").
  - module: github.com/confluentinc/confluent-kafka-go/v2
    version: "== 2.5.0"

  # match-type controls how the module is matched against module paths.
  # Options: exact (default), prefix, regex
  - module: github.com/kubernetes
    match-type: prefix
  - module: github.com/apache/arrow-go
    match-type: prefix
  - module: "github.com/somecompany/.*"
    match-type: regex

# blocked defines modules that are not permitted as direct dependencies.
blocked:
  - module: github.com/uudashr/go-module
    # match-type controls how the module is matched against module paths.
    # Options: exact (default), prefix, regex
    match-type: exact

    # recommendations lists alternative modules to suggest in the lint error.
    recommendations:
      - golang.org/x/mod

    # reason is a human-readable explanation appended to the lint error.
    reason: "`mod` is the official go.mod parser library."

  - module: github.com/mitchellh/go-homedir
    # version constrains which versions of the module are blocked.
    # Uses semver constraint syntax. When omitted, all versions are blocked.
    version: "<= 1.1.0"
    reason: "old versions have a known bug."

  - module: "github.com/badcompany/.*"
    match-type: regex
    reason: "No badcompany packages are permitted."

# Blocks 'replace' directives using local filesystem paths to prevent
# accidental commits of dev overrides. Sibling modules in multi-module
# repos are automatically detected and permitted.
local_replace_directives: true
```

### Field reference

#### Top-level fields

| Field | Type | Default | Description |
|---|---|---|---|
| `allowed` | list | *(none)* | Modules that are permitted. When non-empty, anything not matched is blocked. |
| `blocked` | list | *(none)* | Modules that are explicitly blocked. |
| `local_replace_directives` | bool | `false` | Block any module whose `replace` directive points to a local filesystem path. Multi-module repo aware: sibling modules whose replacement path contains a matching `go.mod` are not blocked. |

#### `allowed` / `blocked` entry fields

| Field | Type | Description |
|---|---|---|
| `module` | string | The module path to match against. |
| `match-type` | `exact` \| `prefix` \| `regex` | How `module` is matched against dependency paths. Defaults to `exact`. |
| `version` | semver constraint string | Restricts the rule to specific versions (e.g. `<= 1.2.0`, `>= 2.0.0`). When omitted, all versions match. |
| `recommendations` | list of module paths | *(blocked only)* Alternative modules to suggest in the lint error. If the module being linted is itself in this list, the block is skipped. |
| `reason` | string | *(blocked only)* Human-readable explanation appended to the lint error. |

#### Match type precedence

When multiple rules can match the same module the following precedence applies:

1. **Exact match** — highest priority; wins over prefix and regex.
2. **Prefix match** — next priority; longest matching prefix wins.
3. **Regex match** — lowest priority; evaluated in alphabetical key order; first match wins.

## Example .gomodguard.yaml Files

The following example configuration files are available:

- [examples/alloptions/.gomodguard.yaml](examples/alloptions/.gomodguard.yaml)
- [examples/allowedversion/.gomodguard.yaml](examples/allowedversion/.gomodguard.yaml)
- [examples/emptyallowlist/.gomodguard.yaml](examples/emptyallowlist/.gomodguard.yaml)
- [examples/indirectdep/.gomodguard.yaml](examples/indirectdep/.gomodguard.yaml)
- [examples/majorversion/.gomodguard.yaml](examples/majorversion/.gomodguard.yaml)
- [examples/regexversion/.gomodguard.yaml](examples/regexversion/.gomodguard.yaml)
- [examples/regextest/.gomodguard.yaml](examples/regextest/.gomodguard.yaml)

### Migrating from v1

If you have a v1 `.gomodguard.yaml` file, you can automatically migrate it to the new v2 schema by running:

```
gomodguard migrate > .gomodguard-v2.yaml
mv .gomodguard-v2.yaml .gomodguard.yaml
```

## Usage

```
╰─ gomodguard -help
Usage: gomodguard <file> [files...]
Also supports package syntax but will use it in relative path, i.e. ./pkg/...

Commands:
  (default)  Lint Go module dependencies using the configuration file
  migrate    Convert a v1 .gomodguard.yaml config file to v2 format and print to stdout

Flags:
  -f string
    	Report results to the specified file. A report type must also be specified
  -file string

  -h	Show this help text
  -help

  -i int
    	Exit code when issues were found (default 2)
  -issues-exit-code int
    	 (default 2)
  -n	Don't lint test files
  -no-test

  -r string
    	Report results to one of the following formats: checkstyle. A report file destination must also be specified
  -report string

  -version
    	Print the version
```

## Example

```
╰─ cd examples/alloptions
╰─ gomodguard -r checkstyle -f gomodguard-checkstyle.xml ./...

info: allowed modules, [github.com/Masterminds/semver/v3 github.com/go-xmlfmt/xmlfmt golang.org gopkg.in/yaml.v3]
info: blocked modules, [github.com/gofrs/uuid github.com/mitchellh/go-homedir github.com/uudashr/go-module]
blocked_example.go:6:1 import of package `github.com/gofrs/uuid` is blocked because the module is in the blocked modules list. `github.com/ryancurrah/gomodguard` is a recommended module. testing if module is not blocked when it is recommended.
blocked_example.go:7:1 import of package `github.com/mitchellh/go-homedir` is blocked because the module is in the blocked modules list. version `v1.1.0` is blocked because it does not meet the version constraint `<=1.1.0`. testing if blocked version constraint works.
blocked_example.go:8:1 import of package `github.com/uudashr/go-module` is blocked because the module is in the blocked modules list. `golang.org/x/mod` is a recommended module. `mod` is the official go.mod parser library.
```

Resulting checkstyle file

```
╰─ cat gomodguard-checkstyle.xml

<?xml version="1.0" encoding="UTF-8"?>
<checkstyle version="1.0.0">
  <file name="blocked_example.go">
    <error line="6" column="1" severity="error" message="import of package `github.com/gofrs/uuid` is blocked because the module is in the blocked modules list. `github.com/ryancurrah/gomodguard` is a recommended module. testing if module is not blocked when it is recommended." source="gomodguard"></error>
    <error line="7" column="1" severity="error" message="import of package `github.com/mitchellh/go-homedir` is blocked because the module is in the blocked modules list. version `v1.1.0` is blocked because it does not meet the version constraint `&lt;=1.1.0`. testing if blocked version constraint works." source="gomodguard"></error>
    <error line="8" column="1" severity="error" message="import of package `github.com/uudashr/go-module` is blocked because the module is in the blocked modules list. `golang.org/x/mod` is a recommended module. `mod` is the official go.mod parser library." source="gomodguard"></error>
  </file>
</checkstyle>
```

## Install

```
go install github.com/ryancurrah/gomodguard/cmd/gomodguard/v2@latest
```

## Develop

```
git clone https://github.com/ryancurrah/gomodguard.git && cd gomodguard/cmd/gomodguard

go build -o gomodguard main.go
```

The repository is a multi-module workspace: the library lives at the root and the CLI lives under `cmd/gomodguard`. A `go.work` file at the repo root wires them together so local CLI builds pick up local library changes without needing a `replace` directive in `cmd/gomodguard/go.mod`.

## License

**MIT**
