# jsonschema v6.0.2

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GoDoc](https://godoc.org/github.com/santhosh-tekuri/jsonschema?status.svg)](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v6)
[![Go Report Card](https://goreportcard.com/badge/github.com/santhosh-tekuri/jsonschema/v6)](https://goreportcard.com/report/github.com/santhosh-tekuri/jsonschema/v6)
[![Build Status](https://github.com/santhosh-tekuri/jsonschema/actions/workflows/go.yaml/badge.svg?branch=boon)](https://github.com/santhosh-tekuri/jsonschema/actions/workflows/go.yaml)
[![codecov](https://codecov.io/gh/santhosh-tekuri/jsonschema/branch/boon/graph/badge.svg?token=JMVj1pFT2l)](https://codecov.io/gh/santhosh-tekuri/jsonschema/tree/boon)

see [godoc](https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v6) for examples

## Library Features

- [x] pass [JSON-Schema-Test-Suite](https://github.com/json-schema-org/JSON-Schema-Test-Suite) excluding optional(compare with other impls at [bowtie](https://bowtie-json-schema.github.io/bowtie/#))
  - [x] [![draft-04](https://img.shields.io/endpoint?url=https://bowtie.report/badges/go-jsonschema/compliance/draft4.json)](https://bowtie.report/#/dialects/draft4)
  - [x] [![draft-06](https://img.shields.io/endpoint?url=https://bowtie.report/badges/go-jsonschema/compliance/draft6.json)](https://bowtie.report/#/dialects/draft6)
  - [x] [![draft-07](https://img.shields.io/endpoint?url=https://bowtie.report/badges/go-jsonschema/compliance/draft7.json)](https://bowtie.report/#/dialects/draft7)
  - [x] [![draft/2019-09](https://img.shields.io/endpoint?url=https://bowtie.report/badges/go-jsonschema/compliance/draft2019-09.json)](https://bowtie.report/#/dialects/draft2019-09)
  - [x] [![draft/2020-12](https://img.shields.io/endpoint?url=https://bowtie.report/badges/go-jsonschema/compliance/draft2020-12.json)](https://bowtie.report/#/dialects/draft2020-12)
- [x] detect infinite loop traps
  - [x] `$schema` cycle
  - [x] validation cycle
- [x] custom `$schema` url
- [x] vocabulary based validation
- [x] custom regex engine
- [x] format assertions
  - [x] flag to enable in draft >= 2019-09
  - [x] custom format registration
  - [x] built-in formats
    - [x] regex, uuid
    - [x] ipv4, ipv6
    - [x] hostname, email
    - [x] date, time, date-time, duration
    - [x] json-pointer, relative-json-pointer
    - [x] uri, uri-reference, uri-template
    - [x] iri, iri-reference
    - [x] period, semver
- [x] content assertions
  - [x] flag to enable in draft >= 7
  - [x] contentEncoding
    - [x] base64
    - [x] custom
  - [x] contentMediaType
    - [x] application/json
    - [x] custom
  - [x] contentSchema
- [x] errors
  - [x] introspectable
  - [x] hierarchy
    - [x] alternative display with `#`
  - [x] output
    - [x] flag
    - [x] basic
    - [x] detailed
- [x] custom vocabulary
    - enable via `$vocabulary` for draft >=2019-19
    - enable via flag for draft <= 7
- [x] mixed dialect support

## CLI v0.7.0

to install: `go install github.com/santhosh-tekuri/jsonschema/cmd/jv@latest`

Note that the cli is versioned independently. you can see it in git tags `cmd/jv/v0.7.0`

```
Usage: jv [OPTIONS] SCHEMA [INSTANCE...]

Options:
  -c, --assert-content    Enable content assertions with draft >= 7
  -f, --assert-format     Enable format assertions with draft >= 2019
      --cacert pem-file   Use the specified pem-file to verify the peer. The file may contain multiple CA certificates
  -d, --draft version     Draft version used when '$schema' is missing. Valid values 4, 6, 7, 2019, 2020 (default 2020)
  -h, --help              Print help information
  -k, --insecure          Use insecure TLS connection
  -o, --output format     Output format. Valid values simple, alt, flag, basic, detailed (default "simple")
  -q, --quiet             Do not print errors
  -v, --version           Print build information
```

- [x] exit code `1` for validation errors, `2` for usage errors
- [x] validate both schema and multiple instances
- [x] support both json and yaml files
- [x] support standard input, use `-`
- [x] quite mode with parsable output
- [x] http(s) url support
  - [x] custom certs for validation, use `--cacert`
  - [x] flag to skip certificate verification, use `--insecure`

