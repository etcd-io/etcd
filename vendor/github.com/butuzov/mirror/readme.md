# `mirror` [![Stand with Ukraine](https://raw.githubusercontent.com/vshymanskyy/StandWithUkraine/main/badges/StandWithUkraine.svg)](https://u24.gov.ua/) [![Code Coverage](https://coveralls.io/repos/github/butuzov/mirror/badge.svg?branch=main)](https://coveralls.io/github/butuzov/mirror?branch=main) [![build status](https://github.com/butuzov/mirror/actions/workflows/main.yaml/badge.svg?branch=main)]()

`mirror` suggests use of alternative functions/methods in order to gain performance boosts by avoiding unnecessary `[]byte/string` conversion calls. See [MIRROR_FUNCS.md](MIRROR_FUNCS.md) list of mirror functions you can use in go's stdlib.

---

[![United 24](https://raw.githubusercontent.com/vshymanskyy/StandWithUkraine/main/banner-personal-page.svg)](https://u24.gov.ua/)
[![Help Oleg Butuzov](https://raw.githubusercontent.com/butuzov/butuzov/main/personal.svg)](https://github.com/butuzov)

---

## Linter Use Cases

### `github.com/argoproj/argo-cd`

```go
// Before
func IsValidHostname(hostname string, fqdn bool) bool {
  if !fqdn {
    return validHostNameRegexp.Match([]byte(hostname)) || validIPv6Regexp.Match([]byte(hostname))
  } else {
    return validFQDNRegexp.Match([]byte(hostname))
  }
}

// After: With alternative method (and lost `else` case)
func IsValidHostname(hostname string, fqdn bool) bool {
  if !fqdn {
    return validHostNameRegexp.MatchString(hostname) || validIPv6Regexp.MatchString(hostname)
  }

  return validFQDNRegexp.MatchString(hostname)
}
```

## Install

```
go install github.com/butuzov/mirror/cmd/mirror@latest
```

### `golangci-lint`
`golangci-lint` supports `mirror` since `v1.53.0`


## How to use

You run `mirror` with [`go vet`](https://pkg.go.dev/cmd/vet):

```
go vet -vettool=$(which mirror) ./...
# github.com/jcmoraisjr/haproxy-ingress/pkg/common/net/ssl
pkg/common/net/ssl/ssl.go:64:11: avoid allocations with (*os.File).WriteString
pkg/common/net/ssl/ssl.go:161:12: avoid allocations with (*os.File).WriteString
pkg/common/net/ssl/ssl.go:166:3: avoid allocations with (*os.File).WriteString
```

Can be called directly:
```
mirror ./...
# https://github.com/cosmtrek/air
/air/runner/util.go:149:6: avoid allocations with (*regexp.Regexp).MatchString
/air/runner/util.go:173:14: avoid allocations with (*os.File).WriteString
```

With [`golangci-lint`](https://github.com/golangci/golangci-lint)

```
golangci-lint run --no-config --disable-all -Emirror
# github.com/argoproj/argo-cd
test/e2e/fixture/app/actions.go:83:11: avoid allocations with (*os.File).WriteString (mirror)
	_, err = tmpFile.Write([]byte(data))
	         ^
server/server.go:1166:9: avoid allocations with (*regexp.Regexp).MatchString (mirror)
	return mainJsBundleRegex.Match([]byte(filename))
	       ^
server/account/account.go:91:6: avoid allocations with (*regexp.Regexp).MatchString (mirror)
	if !validPasswordRegexp.Match([]byte(q.NewPassword)) {
	    ^
server/badge/badge.go:52:20: avoid allocations with (*regexp.Regexp).FindAllStringSubmatchIndex (mirror)
	for _, v := range re.FindAllSubmatchIndex([]byte(str), -1) {
	                  ^
util/cert/cert.go:82:10: avoid allocations with (*regexp.Regexp).MatchString (mirror)
		return validHostNameRegexp.Match([]byte(hostname)) || validIPv6Regexp.Match([]byte(hostname))
```

## Command line

- You can add checks for `_test.go` files with cli option `--with-tests`

### `golangci-lint`
  With `golangci-lint` tests are checked by default and can be can be turned off by using the regular `golangci-lint` ways to do it:

  - flag `--tests` (e.g. `--tests=false`)
  - flag `--skip-files` (e.g. `--skip-files="_test.go"`)
  - yaml configuration `run.skip-files`:
    ```yaml
    run:
      skip-files:
        - '(.+)_test\.go'
    ```
  - yaml configuration `issues.exclude-rules`:
    ```yaml
      issues:
        exclude-rules:
          - path: '(.+)_test\.go'
            linters:
              - mirror
      ```


## Contributing

```shell
# Update Assets (testdata/(strings|bytes|os|utf8|maphash|regexp|bufio).go)
(task|make) generate
# Run Tests
(task|make) tests
# Lint Code
(task|make) lints
```
