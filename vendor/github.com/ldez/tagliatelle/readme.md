# Tagliatelle

[![Sponsor](https://img.shields.io/badge/Sponsor%20me-%E2%9D%A4%EF%B8%8F-pink)](https://github.com/sponsors/ldez)
[![Build Status](https://github.com/ldez/tagliatelle/workflows/Main/badge.svg?branch=master)](https://github.com/ldez/tagliatelle/actions)

A linter that handles struct tags.

Supported string casing:

- `camel`
- `pascal`
- `kebab`
- `snake`
- `upperSnake`
- `goCamel` Respects [Go's common initialisms](https://github.com/golang/lint/blob/83fdc39ff7b56453e3793356bcff3070b9b96445/lint.go#L770-L809) (e.g. HttpResponse -> HTTPResponse).
- `goPascal` Respects [Go's common initialisms](https://github.com/golang/lint/blob/83fdc39ff7b56453e3793356bcff3070b9b96445/lint.go#L770-L809) (e.g. HttpResponse -> HTTPResponse).
- `goKebab` Respects [Go's common initialisms](https://github.com/golang/lint/blob/83fdc39ff7b56453e3793356bcff3070b9b96445/lint.go#L770-L809) (e.g. HttpResponse -> HTTPResponse).
- `goSnake` Respects [Go's common initialisms](https://github.com/golang/lint/blob/83fdc39ff7b56453e3793356bcff3070b9b96445/lint.go#L770-L809) (e.g. HttpResponse -> HTTPResponse).
- `header`
- `upper`
- `lower`

| Source         | Camel Case     | Go Camel Case  |
|----------------|----------------|----------------|
| GooID          | gooId          | gooID          |
| HTTPStatusCode | httpStatusCode | httpStatusCode |
| FooBAR         | fooBar         | fooBar         |
| URL            | url            | url            |
| ID             | id             | id             |
| hostIP         | hostIp         | hostIP         |
| JSON           | json           | json           |
| JSONName       | jsonName       | jsonName       |
| NameJSON       | nameJson       | nameJSON       |
| UneTête        | uneTête        | uneTête        |

| Source         | Pascal Case    | Go Pascal Case |
|----------------|----------------|----------------|
| GooID          | GooId          | GooID          |
| HTTPStatusCode | HttpStatusCode | HTTPStatusCode |
| FooBAR         | FooBar         | FooBar         |
| URL            | Url            | URL            |
| ID             | Id             | ID             |
| hostIP         | HostIp         | HostIP         |
| JSON           | Json           | JSON           |
| JSONName       | JsonName       | JSONName       |
| NameJSON       | NameJson       | NameJSON       |
| UneTête        | UneTête        | UneTête        |

| Source         | Snake Case       | Upper Snake Case | Go Snake Case    |
|----------------|------------------|------------------|------------------|
| GooID          | goo_id           | GOO_ID           | goo_ID           |
| HTTPStatusCode | http_status_code | HTTP_STATUS_CODE | HTTP_status_code |
| FooBAR         | foo_bar          | FOO_BAR          | foo_bar          |
| URL            | url              | URL              | URL              |
| ID             | id               | ID               | ID               |
| hostIP         | host_ip          | HOST_IP          | host_IP          |
| JSON           | json             | JSON             | JSON             |
| JSONName       | json_name        | JSON_NAME        | JSON_name        |
| NameJSON       | name_json        | NAME_JSON        | name_JSON        |
| UneTête        | une_tête         | UNE_TÊTE         | une_tête         |

| Source         | Kebab Case       | Go KebabCase     |
|----------------|------------------|------------------|
| GooID          | goo-id           | goo-ID           |
| HTTPStatusCode | http-status-code | HTTP-status-code |
| FooBAR         | foo-bar          | foo-bar          |
| URL            | url              | URL              |
| ID             | id               | ID               |
| hostIP         | host-ip          | host-IP          |
| JSON           | json             | JSON             |
| JSONName       | json-name        | JSON-name        |
| NameJSON       | name-json        | name-JSON        |
| UneTête        | une-tête         | une-tête         |

| Source         | Header Case      |
|----------------|------------------|
| GooID          | Goo-Id           |
| HTTPStatusCode | Http-Status-Code |
| FooBAR         | Foo-Bar          |
| URL            | Url              |
| ID             | Id               |
| hostIP         | Host-Ip          |
| JSON           | Json             |
| JSONName       | Json-Name        |
| NameJSON       | Name-Json        |
| UneTête        | Une-Tête         |

## Examples

```go
// json and camel case
type Foo struct {
    ID     string `json:"ID"` // must be "id"
    UserID string `json:"UserID"`// must be "userId"
    Name   string `json:"name"`
    Value  string `json:"val,omitempty"`// must be "value"
}
```

## What this linter is about

This linter is about validating tags according to the rules you define.
The linter also allows you to fix tags according to the rules you defined.

This linter is not intended to validate the fact a tag is valid or not.

## How to use the linter

### As a golangci-lint linter

Define the rules, you want via your [golangci-lint](https://golangci-lint.run) configuration file:

```yaml
linters:
  enable:
    - tagliatelle

  settings:
    tagliatelle:
      # Checks the struct tag name case.
      case:
        # Defines the association between tag name and case.
        # Any struct tag name can be used.
        # Supported string cases:
        # - `camel`
        # - `pascal`
        # - `kebab`
        # - `snake`
        # - `upperSnake`
        # - `goCamel`
        # - `goPascal`
        # - `goKebab`
        # - `goSnake`
        # - `upper`
        # - `lower`
        # - `header`
        rules:
          json: camel
          yaml: camel
          xml: camel
          toml: camel
          bson: camel
          avro: snake
          mapstructure: kebab
          env: upperSnake
          envconfig: upperSnake
          whatever: snake
        # Defines the association between tag name and case.
        # Important: the `extended-rules` overrides `rules`.
        # Default: empty
        extended-rules:
          json:
            # Supported string cases:
            # - `camel`
            # - `pascal`
            # - `kebab`
            # - `snake`
            # - `upperSnake`
            # - `goCamel`
            # - `goPascal`
            # - `goKebab`
            # - `goSnake`
            # - `header`
            # - `lower`
            # - `header`
            #
            # Required
            case: camel
            # Adds 'AMQP', 'DB', 'GID', 'RTP', 'SIP', 'TS' to initialisms,
            # and removes 'LHS', 'RHS' from initialisms.
            # Default: false
            extra-initialisms: true
            # Defines initialism additions and overrides.
            # Default: empty
            initialism-overrides:
              DB: true # add a new initialism
              LHS: false # disable a default initialism.
              # ...
        # Uses the struct field name to check the name of the struct tag.
        # Default: false
        use-field-name: true
        # The field names to ignore.
        # Default: []
        ignored-fields:
          - Bar
          - Foo
        # Overrides the default/root configuration.
        # Default: []
        overrides:
          -
            # The package path (uses `/` only as a separator).
            # Required
            pkg: foo/bar
            # Default: empty or the same as the default/root configuration.
            rules:
              json: snake
              xml: pascal
            # Default: empty or the same as the default/root configuration.
            extended-rules:
              # same options as the base `extended-rules`.
            # Default: false (WARNING: it doesn't follow the default/root configuration)
            use-field-name: true
            # The field names to ignore.
            # Default: [] or the same as the default/root configuration.
            ignored-fields:
              - Bar
              - Foo
            # Ignore the package (takes precedence over all other configurations).
            # Default: false
            ignore: true
```

#### Examples

Overrides case rules for the package `foo/bar`:

```yaml
linters:
  settings:
    tagliatelle:
      case:
        rules:
          json: camel
          yaml: camel
          xml: camel
        overrides:
          - pkg: foo/bar
            rules:
              json: snake
              xml: pascal
```

Ignore fields inside the package `foo/bar`:

```yaml
linters:
  settings:
    tagliatelle:
      case:
        rules:
          json: camel
          yaml: camel
          xml: camel
        overrides:
          - pkg: foo/bar
            ignored-fields:
              - Bar
              - Foo
```

Ignore the package `foo/bar`:

```yaml
linters:
  settings:
    tagliatelle:
      case:
        rules:
          json: camel
          yaml: camel
          xml: camel
        overrides:
          - pkg: foo/bar
            ignore: true
```

More information here https://golangci-lint.run/usage/linters/#tagliatelle

### Install and run it from the binary

Not recommended.

```shell
go install github.com/ldez/tagliatelle/cmd/tagliatelle@latest
```

then launch it manually.

## Rules

Here are the default rules for the well-known and used tags, when using tagliatelle as a binary or [golangci-lint linter](https://golangci-lint.run/usage/linters/#tagliatelle):

- `json`: `camel`
- `yaml`: `camel`
- `xml`: `camel`
- `bson`: `camel`
- `avro`: `snake`
- `header`: `header`
- `env`: `upperSnake`
- `envconfig`: `upperSnake`

### Custom Rules

The linter is not limited to the tags used in example, **you can use it to validate any tag**.

You can add your own tag, for example `whatever` and tells the linter you want to use `kebab`.

This option is only available via [golangci-lint](https://golangci-lint.run).

```yaml
linters:
  settings:
    tagliatelle:
      # Check the struck tag name case.
      case:
        rules:
          # Any struct tag type can be used.
          # Support string case: `camel`, `pascal`, `kebab`, `snake`, `goCamel`, `goPascal`, `goKebab`, `goSnake`, `upper`, `lower`
          json: camel
          yaml: camel
          xml: camel
          toml: camel
          whatever: kebab
        # Use the struct field name to check the name of the struct tag.
        # Default: false
        use-field-name: true
```
