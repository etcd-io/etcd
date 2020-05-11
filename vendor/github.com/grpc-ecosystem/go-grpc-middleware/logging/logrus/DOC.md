# grpc_logrus
`import "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"`

* [Overview](#pkg-overview)
* [Imported Packages](#pkg-imports)
* [Index](#pkg-index)
* [Examples](#pkg-examples)

## <a name="pkg-overview">Overview</a>
`grpc_logrus` is a gRPC logging middleware backed by Logrus loggers

It accepts a user-configured `logrus.Entry` that will be used for logging completed gRPC calls. The same
`logrus.Entry` will be used for logging completed gRPC calls, and be populated into the `context.Context` passed into gRPC handler code.

On calling `StreamServerInterceptor` or `UnaryServerInterceptor` this logging middleware will add gRPC call information
to the ctx so that it will be present on subsequent use of the `ctxlogrus` logger.

This package also implements request and response *payload* logging, both for server-side and client-side. These will be
logged as structured `jsonpb` fields for every message received/sent (both unary and streaming). For that please use
`Payload*Interceptor` functions for that. Please note that the user-provided function that determines whetether to log
the full request/response payload needs to be written with care, this can significantly slow down gRPC.

If a deadline is present on the gRPC request the grpc.request.deadline tag is populated when the request begins. grpc.request.deadline
is a string representing the time (RFC3339) when the current call will expire.

Logrus can also be made as a backend for gRPC library internals. For that use `ReplaceGrpcLogger`.

*Server Interceptor*
Below is a JSON formatted example of a log that would be logged by the server interceptor:

	{
	  "level": "info",					// string  logrus log levels
	  "msg": "finished unary call",				// string  log message
	  "grpc.code": "OK",					// string  grpc status code
	  "grpc.method": "Ping",				// string  method name
	  "grpc.service": "mwitkow.testproto.TestService",      // string  full name of the called service
	  "grpc.start_time": "2006-01-02T15:04:05Z07:00",       // string  RFC3339 representation of the start time
	  "grpc.request.deadline": "2006-01-02T15:04:05Z07:00",   // string  RFC3339 deadline of the current request if supplied
	  "grpc.request.value": "something",			// string  value on the request
	  "grpc.time_ms": 1.234,				// float32 run time of the call in ms
	  "peer.address": {
	    "IP": "127.0.0.1",					// string  IP address of calling party
	    "Port": 60216,					// int     port call is coming in on
	    "Zone": ""						// string  peer zone for caller
	  },
	  "span.kind": "server",				// string  client | server
	  "system": "grpc"					// string
	
	  "custom_field": "custom_value",			// string  user defined field
	  "custom_tags.int": 1337,				// int     user defined tag on the ctx
	  "custom_tags.string": "something",			// string  user defined tag on the ctx
	}

*Payload Interceptor*
Below is a JSON formatted example of a log that would be logged by the payload interceptor:

	{
	  "level": "info",							// string logrus log levels
	  "msg": "client request payload logged as grpc.request.content",   	// string log message
	
	  "grpc.request.content": {						// object content of RPC request
	    "value": "something",						// string defined by caller
	    "sleepTimeMs": 9999							// int    defined by caller
	  },
	  "grpc.method": "Ping",						// string method being called
	  "grpc.service": "mwitkow.testproto.TestService",			// string service being called
	  "span.kind": "client",						// string client | server
	  "system": "grpc"							// string
	}

Note - due to implementation ZAP differs from Logrus in the "grpc.request.content" object by having an inner "msg" object.

Please see examples and tests for examples of use.

#### Example:

<details>
<summary>Click to expand code.</summary>

```go
// Logrus entry is used, allowing pre-definition of certain fields by the user.
logrusEntry := logrus.NewEntry(logrusLogger)
// Shared options for the logger, with a custom gRPC code to log level function.
opts := []grpc_logrus.Option{
    grpc_logrus.WithLevels(customFunc),
}
// Make sure that log statements internal to gRPC library are logged using the logrus Logger as well.
grpc_logrus.ReplaceGrpcLogger(logrusEntry)
// Create a server, make sure we put the grpc_ctxtags context before everything else.
_ = grpc.NewServer(
    grpc_middleware.WithUnaryServerChain(
        grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
        grpc_logrus.UnaryServerInterceptor(logrusEntry, opts...),
    ),
    grpc_middleware.WithStreamServerChain(
        grpc_ctxtags.StreamServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
        grpc_logrus.StreamServerInterceptor(logrusEntry, opts...),
    ),
)
```

</details>

#### Example:

<details>
<summary>Click to expand code.</summary>

```go
// Logrus entry is used, allowing pre-definition of certain fields by the user.
logrusEntry := logrus.NewEntry(logrusLogger)
// Shared options for the logger, with a custom duration to log field function.
opts := []grpc_logrus.Option{
    grpc_logrus.WithDurationField(func(duration time.Duration) (key string, value interface{}) {
        return "grpc.time_ns", duration.Nanoseconds()
    }),
}
_ = grpc.NewServer(
    grpc_middleware.WithUnaryServerChain(
        grpc_ctxtags.UnaryServerInterceptor(),
        grpc_logrus.UnaryServerInterceptor(logrusEntry, opts...),
    ),
    grpc_middleware.WithStreamServerChain(
        grpc_ctxtags.StreamServerInterceptor(),
        grpc_logrus.StreamServerInterceptor(logrusEntry, opts...),
    ),
)
```

</details>

## <a name="pkg-imports">Imported Packages</a>

- [github.com/golang/protobuf/jsonpb](https://godoc.org/github.com/golang/protobuf/jsonpb)
- [github.com/golang/protobuf/proto](https://godoc.org/github.com/golang/protobuf/proto)
- [github.com/grpc-ecosystem/go-grpc-middleware](./../..)
- [github.com/grpc-ecosystem/go-grpc-middleware/logging](./..)
- [github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus/ctxlogrus](./ctxlogrus)
- [github.com/grpc-ecosystem/go-grpc-middleware/tags/logrus](./../../tags/logrus)
- [github.com/sirupsen/logrus](https://godoc.org/github.com/sirupsen/logrus)
- [golang.org/x/net/context](https://godoc.org/golang.org/x/net/context)
- [google.golang.org/grpc](https://godoc.org/google.golang.org/grpc)
- [google.golang.org/grpc/codes](https://godoc.org/google.golang.org/grpc/codes)
- [google.golang.org/grpc/grpclog](https://godoc.org/google.golang.org/grpc/grpclog)

## <a name="pkg-index">Index</a>
* [Variables](#pkg-variables)
* [func AddFields(ctx context.Context, fields logrus.Fields)](#AddFields)
* [func DefaultClientCodeToLevel(code codes.Code) logrus.Level](#DefaultClientCodeToLevel)
* [func DefaultCodeToLevel(code codes.Code) logrus.Level](#DefaultCodeToLevel)
* [func DurationToDurationField(duration time.Duration) (key string, value interface{})](#DurationToDurationField)
* [func DurationToTimeMillisField(duration time.Duration) (key string, value interface{})](#DurationToTimeMillisField)
* [func Extract(ctx context.Context) \*logrus.Entry](#Extract)
* [func PayloadStreamClientInterceptor(entry \*logrus.Entry, decider grpc\_logging.ClientPayloadLoggingDecider) grpc.StreamClientInterceptor](#PayloadStreamClientInterceptor)
* [func PayloadStreamServerInterceptor(entry \*logrus.Entry, decider grpc\_logging.ServerPayloadLoggingDecider) grpc.StreamServerInterceptor](#PayloadStreamServerInterceptor)
* [func PayloadUnaryClientInterceptor(entry \*logrus.Entry, decider grpc\_logging.ClientPayloadLoggingDecider) grpc.UnaryClientInterceptor](#PayloadUnaryClientInterceptor)
* [func PayloadUnaryServerInterceptor(entry \*logrus.Entry, decider grpc\_logging.ServerPayloadLoggingDecider) grpc.UnaryServerInterceptor](#PayloadUnaryServerInterceptor)
* [func ReplaceGrpcLogger(logger \*logrus.Entry)](#ReplaceGrpcLogger)
* [func StreamClientInterceptor(entry \*logrus.Entry, opts ...Option) grpc.StreamClientInterceptor](#StreamClientInterceptor)
* [func StreamServerInterceptor(entry \*logrus.Entry, opts ...Option) grpc.StreamServerInterceptor](#StreamServerInterceptor)
* [func UnaryClientInterceptor(entry \*logrus.Entry, opts ...Option) grpc.UnaryClientInterceptor](#UnaryClientInterceptor)
* [func UnaryServerInterceptor(entry \*logrus.Entry, opts ...Option) grpc.UnaryServerInterceptor](#UnaryServerInterceptor)
* [type CodeToLevel](#CodeToLevel)
* [type DurationToField](#DurationToField)
* [type Option](#Option)
  * [func WithCodes(f grpc\_logging.ErrorToCode) Option](#WithCodes)
  * [func WithDecider(f grpc\_logging.Decider) Option](#WithDecider)
  * [func WithDurationField(f DurationToField) Option](#WithDurationField)
  * [func WithLevels(f CodeToLevel) Option](#WithLevels)

#### <a name="pkg-examples">Examples</a>
* [Extract (Unary)](#example_Extract_unary)
* [WithDecider](#example_WithDecider)
* [Package (Initialization)](#example__initialization)
* [Package (InitializationWithDurationFieldOverride)](#example__initializationWithDurationFieldOverride)

#### <a name="pkg-files">Package files</a>
[client_interceptors.go](./client_interceptors.go) [context.go](./context.go) [doc.go](./doc.go) [grpclogger.go](./grpclogger.go) [options.go](./options.go) [payload_interceptors.go](./payload_interceptors.go) [server_interceptors.go](./server_interceptors.go) 

## <a name="pkg-variables">Variables</a>
``` go
var (
    // SystemField is used in every log statement made through grpc_logrus. Can be overwritten before any initialization code.
    SystemField = "system"

    // KindField describes the log gield used to incicate whether this is a server or a client log statment.
    KindField = "span.kind"
)
```
``` go
var DefaultDurationToField = DurationToTimeMillisField
```
DefaultDurationToField is the default implementation of converting request duration to a log field (key and value).

``` go
var (
    // JsonPbMarshaller is the marshaller used for serializing protobuf messages.
    JsonPbMarshaller = &jsonpb.Marshaler{}
)
```

## <a name="AddFields">func</a> [AddFields](./context.go#L11)
``` go
func AddFields(ctx context.Context, fields logrus.Fields)
```
AddFields adds logrus fields to the logger.
Deprecated: should use the ctxlogrus.Extract instead

## <a name="DefaultClientCodeToLevel">func</a> [DefaultClientCodeToLevel](./options.go#L129)
``` go
func DefaultClientCodeToLevel(code codes.Code) logrus.Level
```
DefaultClientCodeToLevel is the default implementation of gRPC return codes to log levels for client side.

## <a name="DefaultCodeToLevel">func</a> [DefaultCodeToLevel](./options.go#L87)
``` go
func DefaultCodeToLevel(code codes.Code) logrus.Level
```
DefaultCodeToLevel is the default implementation of gRPC return codes to log levels for server side.

## <a name="DurationToDurationField">func</a> [DurationToDurationField](./options.go#L179)
``` go
func DurationToDurationField(duration time.Duration) (key string, value interface{})
```
DurationToDurationField uses the duration value to log the request duration.

## <a name="DurationToTimeMillisField">func</a> [DurationToTimeMillisField](./options.go#L174)
``` go
func DurationToTimeMillisField(duration time.Duration) (key string, value interface{})
```
DurationToTimeMillisField converts the duration to milliseconds and uses the key `grpc.time_ms`.

## <a name="Extract">func</a> [Extract](./context.go#L17)
``` go
func Extract(ctx context.Context) *logrus.Entry
```
Extract takes the call-scoped logrus.Entry from grpc_logrus middleware.
Deprecated: should use the ctxlogrus.Extract instead

#### Example:

<details>
<summary>Click to expand code.</summary>

```go
_ = func(ctx context.Context, ping *pb_testproto.PingRequest) (*pb_testproto.PingResponse, error) {
    // Add fields the ctxtags of the request which will be added to all extracted loggers.
    grpc_ctxtags.Extract(ctx).Set("custom_tags.string", "something").Set("custom_tags.int", 1337)
    // Extract a single request-scoped logrus.Logger and log messages.
    l := ctx_logrus.Extract(ctx)
    l.Info("some ping")
    l.Info("another ping")
    return &pb_testproto.PingResponse{Value: ping.Value}, nil
}
```

</details>

## <a name="PayloadStreamClientInterceptor">func</a> [PayloadStreamClientInterceptor](./payload_interceptors.go#L74)
``` go
func PayloadStreamClientInterceptor(entry *logrus.Entry, decider grpc_logging.ClientPayloadLoggingDecider) grpc.StreamClientInterceptor
```
PayloadStreamClientInterceptor returns a new streaming client interceptor that logs the paylods of requests and responses.

## <a name="PayloadStreamServerInterceptor">func</a> [PayloadStreamServerInterceptor](./payload_interceptors.go#L45)
``` go
func PayloadStreamServerInterceptor(entry *logrus.Entry, decider grpc_logging.ServerPayloadLoggingDecider) grpc.StreamServerInterceptor
```
PayloadStreamServerInterceptor returns a new server server interceptors that logs the payloads of requests.

This *only* works when placed *after* the `grpc_logrus.StreamServerInterceptor`. However, the logging can be done to a
separate instance of the logger.

## <a name="PayloadUnaryClientInterceptor">func</a> [PayloadUnaryClientInterceptor](./payload_interceptors.go#L58)
``` go
func PayloadUnaryClientInterceptor(entry *logrus.Entry, decider grpc_logging.ClientPayloadLoggingDecider) grpc.UnaryClientInterceptor
```
PayloadUnaryClientInterceptor returns a new unary client interceptor that logs the paylods of requests and responses.

## <a name="PayloadUnaryServerInterceptor">func</a> [PayloadUnaryServerInterceptor](./payload_interceptors.go#L25)
``` go
func PayloadUnaryServerInterceptor(entry *logrus.Entry, decider grpc_logging.ServerPayloadLoggingDecider) grpc.UnaryServerInterceptor
```
PayloadUnaryServerInterceptor returns a new unary server interceptors that logs the payloads of requests.

This *only* works when placed *after* the `grpc_logrus.UnaryServerInterceptor`. However, the logging can be done to a
separate instance of the logger.

## <a name="ReplaceGrpcLogger">func</a> [ReplaceGrpcLogger](./grpclogger.go#L13)
``` go
func ReplaceGrpcLogger(logger *logrus.Entry)
```
ReplaceGrpcLogger sets the given logrus.Logger as a gRPC-level logger.
This should be called *before* any other initialization, preferably from init() functions.

## <a name="StreamClientInterceptor">func</a> [StreamClientInterceptor](./client_interceptors.go#L28)
``` go
func StreamClientInterceptor(entry *logrus.Entry, opts ...Option) grpc.StreamClientInterceptor
```
StreamServerInterceptor returns a new streaming client interceptor that optionally logs the execution of external gRPC calls.

## <a name="StreamServerInterceptor">func</a> [StreamServerInterceptor](./server_interceptors.go#L58)
``` go
func StreamServerInterceptor(entry *logrus.Entry, opts ...Option) grpc.StreamServerInterceptor
```
StreamServerInterceptor returns a new streaming server interceptor that adds logrus.Entry to the context.

## <a name="UnaryClientInterceptor">func</a> [UnaryClientInterceptor](./client_interceptors.go#L16)
``` go
func UnaryClientInterceptor(entry *logrus.Entry, opts ...Option) grpc.UnaryClientInterceptor
```
UnaryClientInterceptor returns a new unary client interceptor that optionally logs the execution of external gRPC calls.

## <a name="UnaryServerInterceptor">func</a> [UnaryServerInterceptor](./server_interceptors.go#L26)
``` go
func UnaryServerInterceptor(entry *logrus.Entry, opts ...Option) grpc.UnaryServerInterceptor
```
UnaryServerInterceptor returns a new unary server interceptors that adds logrus.Entry to the context.

## <a name="CodeToLevel">type</a> [CodeToLevel](./options.go#L53)
``` go
type CodeToLevel func(code codes.Code) logrus.Level
```
CodeToLevel function defines the mapping between gRPC return codes and interceptor log level.

## <a name="DurationToField">type</a> [DurationToField](./options.go#L56)
``` go
type DurationToField func(duration time.Duration) (key string, value interface{})
```
DurationToField function defines how to produce duration fields for logging

## <a name="Option">type</a> [Option](./options.go#L50)
``` go
type Option func(*options)
```

### <a name="WithCodes">func</a> [WithCodes](./options.go#L73)
``` go
func WithCodes(f grpc_logging.ErrorToCode) Option
```
WithCodes customizes the function for mapping errors to error codes.

### <a name="WithDecider">func</a> [WithDecider](./options.go#L59)
``` go
func WithDecider(f grpc_logging.Decider) Option
```
WithDecider customizes the function for deciding if the gRPC interceptor logs should log.

#### Example:

<details>
<summary>Click to expand code.</summary>

```go
opts := []grpc_logrus.Option{
    grpc_logrus.WithDecider(func(methodFullName string, err error) bool {
        // will not log gRPC calls if it was a call to healthcheck and no error was raised
        if err == nil && methodFullName == "blah.foo.healthcheck" {
            return false
        }

        // by default you will log all calls
        return true
    }),
}

_ = []grpc.ServerOption{
    grpc_middleware.WithStreamServerChain(
        grpc_ctxtags.StreamServerInterceptor(),
        grpc_logrus.StreamServerInterceptor(logrus.NewEntry(logrus.New()), opts...)),
    grpc_middleware.WithUnaryServerChain(
        grpc_ctxtags.UnaryServerInterceptor(),
        grpc_logrus.UnaryServerInterceptor(logrus.NewEntry(logrus.New()), opts...)),
}
```

</details>
### <a name="WithDurationField">func</a> [WithDurationField](./options.go#L80)
``` go
func WithDurationField(f DurationToField) Option
```
WithDurationField customizes the function for mapping request durations to log fields.

### <a name="WithLevels">func</a> [WithLevels](./options.go#L66)
``` go
func WithLevels(f CodeToLevel) Option
```
WithLevels customizes the function for mapping gRPC return codes and interceptor log level statements.

- - -
Generated by [godoc2ghmd](https://github.com/GandalfUK/godoc2ghmd)