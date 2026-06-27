# go-spancheck

![Latest release](https://img.shields.io/github/v/release/jjti/go-spancheck)
[![ci](https://github.com/jjti/go-spancheck/actions/workflows/ci.yaml/badge.svg)](https://github.com/jjti/go-spancheck/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/jjti/go-spancheck)](https://goreportcard.com/report/github.com/jjti/go-spancheck)
[![MIT License](http://img.shields.io/badge/license-MIT-blue.svg?style=flat)](LICENSE)

Checks usage of:

- [OpenTelemetry spans](https://opentelemetry.io/docs/instrumentation/go/manual/) from [go.opentelemetry.io/otel/trace](go.opentelemetry.io/otel/trace)
- [OpenCensus spans](https://opencensus.io/quickstart/go/tracing/) from [go.opencensus.io/trace](https://pkg.go.dev/go.opencensus.io/trace#Span)

## Example

```bash
spancheck -checks 'end,set-status,record-error' ./...
```

```go
func _() error {
    // span.End is not called on all paths, possible memory leak
    // span.SetStatus is not called on all paths
    // span.RecordError is not called on all paths
    _, span := otel.Tracer("foo").Start(context.Background(), "bar")

    if true {
        // return can be reached without calling span.End
        // return can be reached without calling span.SetStatus
        // return can be reached without calling span.RecordError
        return errors.New("err")
    }

    return nil // return can be reached without calling span.End
}
```

## Configuration

### golangci-lint

Docs on configuring the linter are also available at [https://golangci-lint.run/usage/linters/#spancheck](https://golangci-lint.run/usage/linters/#spancheck):

```yaml
linters:
  enable:
    - spancheck

linters-settings:
  spancheck:
    # Checks to enable.
    # Options include:
    # - `end`: check that `span.End()` is called
    # - `record-error`: check that `span.RecordError(err)` is called when an error is returned
    # - `set-status`: check that `span.SetStatus(codes.Error, msg)` is called when an error is returned
    # Default: ["end"]
    checks:
      - end
      - record-error
      - set-status
    # A list of regexes for function signatures that silence `record-error` and `set-status` reports
    # if found in the call path to a returned error.
    # https://github.com/jjti/go-spancheck#ignore-check-signatures
    # Default: []
    ignore-check-signatures:
      - "telemetry.RecordError"
    # A list of regexes for additional function signatures that create spans. This is useful if you have a utility
    # method to create spans. Each entry should be of the form <regex>:<telemetry-type>, where `telemetry-type`
    # can be `opentelemetry` or `opencensus`.
    # https://github.com/jjti/go-spancheck#extra-start-span-signatures
    # Default: []
    extra-start-span-signatures:
      - "github.com/user/repo/telemetry/trace.Start:opentelemetry"
```

### CLI

To install the linter as a CLI:

```bash
go install github.com/jjti/go-spancheck/cmd/spancheck@latest
spancheck ./...
```

Only the `span.End()` check is enabled by default. The others can be enabled with `-checks 'end,set-status,record-error'`.

```txt
$ spancheck -h
...
Flags:
  -checks string
        comma-separated list of checks to enable (options: end, set-status, record-error) (default "end")
  -extra-start-span-signatures string
        comma-separated list of regex:telemetry-type for function signatures that indicate the start of a span
  -ignore-check-signatures string
        comma-separated list of regex for function signatures that disable checks on errors
```

### Ignore Check Signatures

This setting avoids false positives from utility functions that return spans (which are handled gracefully by callers of the function).

The `span.SetStatus()` and `span.RecordError()` checks warn when there is:

1. a path to return statement
1. that returns an error
1. without a call (to `SetStatus` or `RecordError`, respectively)

But it's convenient to call `SetStatus` and `RecordError` from utility methods [[1](https://andydote.co.uk/2023/09/19/tracing-is-better/#step-2-wrap-the-errors)]. To support that, the `ignore-*-check-signatures` settings will suppress warnings if the configured function is present in the path.

For example, by default, the code below would have warnings as shown:

```go
func task(ctx context.Context) error {
    ctx, span := otel.Tracer("foo").Start(ctx, "bar") // span.SetStatus is not called on all paths
    defer span.End()

    if err := subTask(ctx); err != nil {
        return recordErr(span, err) // return can be reached without calling span.SetStatus
    }

    return nil
}

func recordErr(span trace.Span, err error) error {
    span.SetStatus(codes.Error, err.Error())
    span.RecordError(err)
    return err
}
```

The warnings are can be ignored by setting `-ignore-check-signatures` flag to `recordErr`:

```bash
spancheck -checks 'end,set-status,record-error' -ignore-check-signatures 'recordErr' ./...
```

### Extra Start Span Signatures

This setting informs spancheck of additional Span creation functions that should be linted (besides the library defaults).

By default, Span creation will be tracked from calls to [(go.opentelemetry.io/otel/trace.Tracer).Start](https://github.com/open-telemetry/opentelemetry-go/blob/98b32a6c3a87fbee5d34c063b9096f416b250897/trace/trace.go#L523), [go.opencensus.io/trace.StartSpan](https://pkg.go.dev/go.opencensus.io/trace#StartSpan), or [go.opencensus.io/trace.StartSpanWithRemoteParent](https://github.com/census-instrumentation/opencensus-go/blob/v0.24.0/trace/trace_api.go#L66).

You can use the `-extra-start-span-signatures` flag to list additional Span creation functions. For all such functions:

1. their Spans will be linted (for all enable checks)
1. checks will be disabled (i.e. there is no linting of Spans within the creation functions)

You must pass a comma-separated list of regex patterns and the telemetry library corresponding to the returned Span. Each entry should be of the form `<regex>:<telemetry-type>`, where `telemetry-type` can be `opentelemetry` or `opencensus`. For example, if you have created a function named `StartTrace` in a `telemetry` package, using the `go.opentelemetry.io/otel` library, you can include this function for analysis like so:

```bash
spancheck -extra-start-span-signatures 'github.com/user/repo/telemetry/StartTrace:opentelemetry' ./...
```

## Problem Statement

Tracing is a celebrated [[1](https://andydote.co.uk/2023/09/19/tracing-is-better/),[2](https://charity.wtf/2022/08/15/live-your-best-life-with-structured-events/)] and well marketed [[3](https://docs.datadoghq.com/tracing/),[4](https://www.honeycomb.io/distributed-tracing)] pillar of observability. But self-instrumented tracing requires a lot of easy-to-forget boilerplate:

```go
import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

func task(ctx context.Context) error {
    ctx, span := otel.Tracer("foo").Start(ctx, "bar")
    defer span.End() // call `.End()`

    if err := subTask(ctx); err != nil {
        span.SetStatus(codes.Error, err.Error()) // call SetStatus(codes.Error, msg) to set status:error
        span.RecordError(err) // call RecordError(err) to record an error event
        return err
    }

    return nil
}
```

For spans to be _really_ useful, developers need to:

1. call `span.End()` always
1. call `span.SetStatus(codes.Error, msg)` on error
1. call `span.RecordError(err)` on error
1. call `span.SetAttributes()` liberally

- OpenTelemetry: [Creating spans](https://opentelemetry.io/docs/instrumentation/go/manual/#creating-spans)
- Uptrace: [OpenTelemetry Go Tracing API](https://uptrace.dev/opentelemetry/go-tracing.html#quickstart)

This linter helps developers with steps 1-3.

## Checks

This linter supports three checks, each documented below. Only the check for `span.End()` is enabled by default. See [Configuration](#configuration) for instructions on enabling the others.

### `span.End()`

Enabled by default.

Not calling `End` can cause memory leaks and prevents spans from being closed.

> Any Span that is created MUST also be ended. This is the responsibility of the user. Implementations of this API may leak memory or other resources if Spans are not ended.

[source: trace.go](https://github.com/open-telemetry/opentelemetry-go/blob/98b32a6c3a87fbee5d34c063b9096f416b250897/trace/trace.go#L523)

```go
func task(ctx context.Context) error {
    otel.Tracer("app").Start(ctx, "foo") // span is unassigned, probable memory leak
    _, span := otel.Tracer().Start(ctx, "foo") // span.End is not called on all paths, possible memory leak
    return nil // return can be reached without calling span.End
}
```

### `span.SetStatus(codes.Error, "msg")`

Disabled by default. Enable with `-checks 'set-status'`.

Developers should call `SetStatus` on spans. The status attribute is an important, first-class attribute:

1. observability platforms and APMs differentiate "success" vs "failure" using [span's status codes](https://docs.datadoghq.com/tracing/metrics/).
1. telemetry collector agents, like the [Open Telemetry Collector's Tail Sampling Processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/tailsamplingprocessor/README.md#:~:text=Sampling%20Processor.-,status_code,-%3A%20Sample%20based%20upon), are configurable to sample `Error` spans at a higher rate than `OK` spans.
1. observability platforms, like [DataDog, have trace retention filters that use spans' status](https://docs.datadoghq.com/tracing/trace_pipeline/trace_retention/). In other words, `status:error` spans often receive special treatment with the assumption they are more useful for debugging. And forgetting to set the status can lead to spans, with useful debugging information, being dropped.

```go
func _() error {
    _, span := otel.Tracer("foo").Start(context.Background(), "bar") // span.SetStatus is not called on all paths
    defer span.End()

    if err := subTask(); err != nil {
        span.RecordError(err)
        return errors.New(err) // return can be reached without calling span.SetStatus
    }

    return nil
}
```

OpenTelemetry docs: [Set span status](https://opentelemetry.io/docs/instrumentation/go/manual/#set-span-status).

### `span.RecordError(err)`

Disabled by default. Enable with `-checks 'record-error'`.

Calling `RecordError` creates a new exception-type [event (structured log message)](https://opentelemetry.io/docs/concepts/signals/traces/#span-events) on the span. This is recommended to capture the error's stack trace.

```go
func _() error {
    _, span := otel.Tracer("foo").Start(context.Background(), "bar") // span.RecordError is not called on all paths
    defer span.End()

    if err := subTask(); err != nil {
        span.SetStatus(codes.Error, err.Error())
        return errors.New(err) // return can be reached without calling span.RecordError
    }

    return nil
}
```

OpenTelemetry docs: [Record errors](https://opentelemetry.io/docs/instrumentation/go/manual/#record-errors).

Note: this check is not applied to [OpenCensus spans](https://pkg.go.dev/go.opencensus.io/trace#SpanInterface) because they have no `RecordError` method.

## Attribution

This linter is the product of liberal copying of:

- [github.com/golang/tools/go/analysis/passes/lostcancel](https://github.com/golang/tools/tree/master/go/analysis/passes/lostcancel) (half the linter)
- [github.com/tomarrell/wrapcheck](https://github.com/tomarrell/wrapcheck) (error type checking and config)
- [github.com/Antonboom/testifylint](https://github.com/Antonboom/testifylint) (README)
- [github.com/ghostiam/protogetter](https://github.com/ghostiam/protogetter/blob/main/testdata/Makefile) (test setup)

And the contributions of:

- [@trixnz](https://github.com/trixnz) who [added support for custom span start functions](https://github.com/jjti/go-spancheck/pull/16)
- [@parsaaes](https://github.com/parsaaes) who [fixed a false negative bug in deferred methods that reference spans](https://github.com/jjti/go-spancheck/pull/31)
