# Depguard

A Go linter that checks package imports are in a list of acceptable packages.
This allows you to allow imports from a whole organization or only
allow specific packages within a repository. 

## Install

```bash
go install github.com/OpenPeeDeeP/depguard/cmd/depguard@latest
```

## Config

The Depguard binary looks for a file named `^\.?depguard\.(yaml|yml|json|toml)$` in the current working directory. Examples include (`.depguard.yml` or `depguard.toml`).

The following is an example configuration file.

```json
{
  "main": {
    "files": [
      "$all",
      "!$test"
    ],
    "listMode": "Strict",
    "allow": [
      "$gostd",
      "github.com/OpenPeeDeeP"
    ],
    "deny": {
      "reflect": "Who needs reflection",
    }
  },
  "tests": {
    "files": [
      "$test"
    ],
    "listMode": "Lax",
    "deny": {
      "github.com/stretchr/testify": "Please use standard library for tests"
    }
  }
}
```

- The top level is a map of lists. The key of the map is a name that shows up in 
the linter's output.
- `files` - list of file globs that will match this list of settings to compare against
- `allow` - list of allowed packages
- `deny` - map of packages that are not allowed where the value is a suggestion
- `listMode` - the mode to use for package matching

Files are matched using [Globs](https://github.com/gobwas/glob). If the files 
list is empty, then all files will match that list. Prefixing a file
with an exclamation mark `!` will put that glob in a "don't match" list. A file
will match a list if it is allowed and not denied.

> Should always prefix a file glob with `**/` as files are matched against absolute paths.

Allow is a prefix of packages to allow. A dollar sign `$` can be used at the end
of a package to specify it must be exact match only.

Deny is a map where the key is a prefix of the package to deny, and the value
is a suggestion on what to use instead. A dollar sign `$` can be used at the end
of a package to specify it must be exact match only.

A Prefix List just means that a package will match a value, if the value is a 
prefix of the package. Example `github.com/OpenPeeDeeP/depguard` package will match
a value of `github.com/OpenPeeDeeP` but won't match `github.com/OpenPeeDeeP/depguard/v2`.

ListMode is used to determine the package matching priority. There are three
different modes; Original, Strict, and Lax.

Original is the original way that the package was written to use. It is not recommended
to stay with this and is only here for backwards compatibility.

Strict, at its roots, is everything is denied unless in allowed.

Lax, at its roots, is everything is allowed unless it is denied.

There are cases where a package can be matched in both the allow and denied lists.
You may allow a subpackage but deny the root or vice versa. The `settings_tests.go` file
has many scenarios listed out under `TestListImportAllowed`. These tests will stay
up to date as features are added.

### Variables

There are variable replacements for each type of list (file or package). This is
to reduce repetition and tedious behaviors.

#### File Variables

> you can still use an exclamation mark `!` in front of a variable to say not to 
use it. Example `!$test` will match any file that is not a go test file.

- `$all` - matches all go files
- `$test` - matches all go test files

#### Package Variables

- `$gostd` - matches all of go's standard library (Pulled from GOROOT)

### Example Configs

Below:

- non-test go files will match `Main` and test go files will match `Test`.
- both allow all of go standard library except for the `reflect` package which will
tell the user "Please don't use reflect package".
- go test files are also allowed to use https://github.com/stretchr/testify package
and any sub-package of it.

```yaml
Main:
  files:
  - $all
  - "!$test"
  allow:
  - $gostd
  deny:
    reflect: Please don't use reflect package
Test:
  files:
  - $test
  allow:
  - $gostd
  - github.com/stretchr/testify
  deny:
    reflect: Please don't use reflect package
```

Below:

- All go files will match `Main`
- Go files in internal will match both `Main` and `Internal`

```yaml
Main:
  files:
  - $all
Internal:
  files:
  - "**/internal/**/*.go"
```

Below:

- All packages are allowed except for `github.com/OpenPeeDeeP/depguard`. Though
`github.com/OpenPeeDeeP/depguard/v2` and `github.com/OpenPeeDeeP/depguard/somepackage`
would be allowed.

```yaml
Main:
  deny:
    github.com/OpenPeeDeeP/depguard$: Please use v2
```

## golangci-lint

This linter was built with
[golangci-lint](https://github.com/golangci/golangci-lint) in mind, read the [linters docs](https://golangci-lint.run/usage/linters/#depguard) to see how to configure all their linters, including this one.

The config is similar to the YAML depguard config documented above, however due to [golangci-lint limitation](https://github.com/golangci/golangci-lint/pull/4227) the `deny` value must be provided as a list, with `pkg` and `desc` keys (otherwise a [panic](https://github.com/OpenPeeDeeP/depguard/issues/74) may occur):

```yaml
# golangci-lint config
linters-settings:
  depguard:
    rules:
      prevent_unmaintained_packages:
        list-mode: lax # allow unless explicitely denied
        files:
          - $all
          - "!$test"
        allow:
          - $gostd
        deny:
          - pkg: io/ioutil
            desc: "replaced by io and os packages since Go 1.16: https://tip.golang.org/doc/go1.16#ioutil"
```
