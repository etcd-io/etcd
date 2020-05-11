# grpc_zap
`import "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"`

* [Overview](#pkg-overview)
* [Imported Packages](#pkg-imports)
* [Index](#pkg-index)
* [Examples](#pkg-examples)

## <a name="pkg-overview">Overview</a>
`grpc_zap` is a gRPC logging middleware backed by ZAP loggers

It accepts a user-configured `zap.Logger` that will be used for logging completed gRPC calls. The same `zap.Logger` will
be used for logging completed gRPC calls, and be populated into the `context.Context` passed into gRPC handler code.

On calling `StreamServerInterceptor` or `UnaryServerInterceptor` this logging middleware will add gRPC call information
to the ctx so that it will be present on subsequent use of the `ctx_zap` logger.

If a deadline is present on the gRPC request the grpc.request.deadline tag is populated when the request begins. grpc.request.deadline
is a string representing the time (RFC3339) when the current call will expire.

This package also implements request and response *payload* logging, both for server-side and client-side. These will be
logged as structured `jsonpb` fields for every message received/sent (both unary and streaming). For that please use
`Payload*Interceptor` functions for that. Please note that the user-provided function that determines whetether to log
the full request/response payload needs to be written with care, this can significantly slow down gRPC.

ZAP can also be made as a backend for gRPC library internals. For that use `ReplaceGrpcLogger`.

*Server Interceptor*
Below is a JSON formatted example of a log that would be logged by the server interceptor:

	{
	  "level": "info",									// string  zap log levels
	  "msg": "finished unary call",						// string  log message
	
	  "grpc.code": "OK",								// string  grpc status code
	  "grpc.method": "Ping",							// string  method name
	  "grpc.service": "mwitkow.testproto.TestService",  // string  full name of the called service
	  "grpc.start_time": "2006-01-02T15:04:05Z07:00",   // string  RFC3339 representation of the start time
	  "grpc.request.deadline": "2006-01-02T15:04:05Z07:00",   // string  RFC3339 deadline of the current request if supplied
	  "grpc.request.value": "something",				// string  value on the request
	  "grpc.time_ms": 1.345,							// float32 run time of the call in ms
	
	  "peer.address": {
	    "IP": "127.0.0.1",								// string  IP address of calling party
	    "Port": 60216,									// int     port call is coming in on
	    "Zone": ""										// string  peer zone for caller
	  },
	  "span.kind": "server",							// string  client | server
	  "system": "grpc"									// string
	
	  "custom_field": "custom_value",					// string  user defined field
	  "custom_tags.int": 1337,							// int     user defined tag on the ctx
	  "custom_tags.string": "something",				// string  user defined tag on the ctx
	}

*Payload Interceptor*
Below is a JSON formatted example of a log that would be logged by the payload interceptor:

	{
	  "level": "info",													// string zap log levels
	  "msg": "client request payload logged as grpc.request.content",   // string log message
	
	  "grpc.request.content": {											// object content of RPC request
	    "msg" : {														// object ZAP specific inner object
		  "value": "something",											// string defined by caller
	      "sleepTimeMs": 9999											// int    defined by caller
	    }
	  },
	  "grpc.method": "Ping",											// string method being called
	  "grpc.service": "mwitkow.testproto.TestService",					// string service being called
	
	  "span.kind": "client",											// string client | server
	  "system": "grpc"													// string
	}

Note - due to implementation ZAP differs from Logrus in the "grpc.request.content" object by having an inner "msg" object.

Please see examples and tests for examples of use.

#### Example:

<details>
<summary>Click to expand code.</summary>

```go
// Shared options for the logger, with a custom gRPC code to log level function.
opts := []grpc_zap.Option{
    grpc_zap.WithLevels(customFunc),
}
// Make sure that log statements internal to gRPC library are logged using the zapLogger as well.
grpc_zap.ReplaceGrpcLogger(zapLogger)
// Create a server, make sure we put the grpc_ctxtags context before everything else.
_ = grpc.NewServer(
    grpc_middleware.WithUnaryServerChain(
        grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
        grpc_zap.UnaryServerInterceptor(zapLogger, opts...),
    ),
    grpc_middleware.WithStreamServerChain(
        grpc_ctxtags.StreamServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
        grpc_zap.StreamServerInterceptor(zapLogger, opts...),
    ),
)
```

</details>

#### Example:

<details>
<summary>Click to expand code.</summary>

```go
opts := []grpc_zap.Option{
    grpc_zap.WithDecider(func(fullMethodName string, err error) bool {
        // will not log gRPC calls if it was a call to healthcheck and no error was raised
        if err == nil && fullMethodName == "foo.bar.healthcheck" {
            return false
        }

        // by default everything will be logged
        return true
    }),
}

_ = []grpc.ServerOption{
    grpc_middleware.WithStreamServerChain(
        grpc_ctxtags.StreamServerInterceptor(),
        grpc_zap.StreamServerInterceptor(zap.NewNop(), opts...)),
    grpc_middleware.WithUnaryServerChain(
        grpc_ctxtags.UnaryServerInterceptor(),
        grpc_zap.UnaryServerInterceptor(zap.NewNop(), opts...)),
}
```

</details>

#### Example:

<details>
<summary>Click to expand code.</summary>

```go
opts := []grpc_zap.Option{
    grpc_zap.WithDurationField(func(duration time.Duration) zapcore.Field {
        return zap.Int64("grpc.time_ns", duration.Nanoseconds())
    }),
}

_ = grpc.NewServer(
    grpc_middleware.WithUnaryServerChain(
        grpc_ctxtags.UnaryServerInterceptor(),
        grpc_zap.UnaryServerInterceptor(zapLogger, opts...),
    ),
    grpc_middleware.WithStreamServerChain(
        grpc_ctxtags.StreamServerInterceptor(),
        grpc_zap.StreamServerInterceptor(zapLogger, opts...),
    ),
)
```

</details>

## <a name="pkg-imports">Imported Packages</a>

- [github.com/golang/protobuf/jsonpb](https://godoc.org/github.com/golang/protobuf/jsonpb)
- [github.com/golang/protobuf/proto](https://godoc.org/github.com/golang/protobuf/proto)
- [github.com/grpc-ecosystem/go-grpc-middleware](./../..)
- [github.com/grpc-ecosystem/go-grpc-middleware/logging](./..)
- [github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap](./ctxzap)
- [github.com/grpc-ecosystem/go-grpc-middleware/tags/zap](./../../tags/zap)
- [go.uber.org/zap](https://godoc.org/go.uber.org/zap)
- [go.uber.org/zap/zapcore](https://godoc.org/go.uber.org/zap/zapcore)
- [golang.org/x/net/context](https://godoc.org/golang.org/x/net/context)
- [google.golang.org/grpc](https://godoc.org/google.golang.org/grpc)
- [google.golang.org/grpc/codes](https://godoc.org/google.golang.org/grpc/codes)
- [google.golang.org/grpc/grpclog](https://godoc.org/google.golang.org/grpc/grpclog)

## <a name="pkg-index">Index</a>
* [Variables](#pkg-variables)
* [func AddFields(ctx context.Context, fields ...zapcore.Field)](#AddFields)
* [func DefaultClientCodeToLevel(code codes.Code) zapcore.Level](#DefaultClientCodeToLevel)
* [func DefaultCodeToLevel(code codes.Code) zapcore.Level](#DefaultCodeToLevel)
* [func DurationToDurationField(duration time.Duration) zapcore.Field](#DurationToDurationField)
* [func DurationToTimeMillisField(duration time.Duration) zapcore.Field](#DurationToTimeMillisField)
* [func Extract(ctx context.Context) \*zap.Logger](#Extract)
* [func PayloadStreamClientInterceptor(logger \*zap.Logger, decider grpc\_logging.ClientPayloadLoggingDecider) grpc.StreamClientInterceptor](#PayloadStreamClientInterceptor)
* [func PayloadStreamServerInterceptor(logger \*zap.Logger, decider grpc\_logging.ServerPayloadLoggingDecider) grpc.StreamServerInterceptor](#PayloadStreamServerInterceptor)
* [func PayloadUnaryClientInterceptor(logger \*zap.Logger, decider grpc\_logging.ClientPayloadLoggingDecider) grpc.UnaryClientInterceptor](#PayloadUnaryClientInterceptor)
* [func PayloadUnaryServerInterceptor(logger \*zap.Logger, decider grpc\_logging.ServerPayloadLoggingDecider) grpc.UnaryServerInterceptor](#PayloadUnaryServerInterceptor)
* [func ReplaceGrpcLogger(logger \*zap.Logger)](#ReplaceGrpcLogger)
* [func StreamClientInterceptor(logger \*zap.Logger, opts ...Option) grpc.StreamClientInterceptor](#StreamClientInterceptor)
* [func StreamServerInterceptor(logger \*zap.Logger, opts ...Option) grpc.StreamServerInterceptor](#StreamServerInterceptor)
* [func UnaryClientInterceptor(logger \*zap.Logger, opts ...Option) grpc.UnaryClientInterceptor](#UnaryClientInterceptor)
* [func UnaryServerInterceptor(logger \*zap.Logger, opts ...Option) grpc.UnaryServerInterceptor](#UnaryServerInterceptor)
* [type CodeToLevel](#CodeToLevel)
* [type DurationToField](#DurationToField)
* [type Option](#Option)
  * [func WithCodes(f grpc\_logging.ErrorToCode) Option](#WithCodes)
  * [func WithDecider(f grpc\_logging.Decider) Option](#WithDecider)
  * [func WithDurationField(f DurationToField) Option](#WithDurationField)
  * [func WithLevels(f CodeToLevel) Option](#WithLevels)

#### <a name="pkg-examples">Examples</a>
* [Extract (Unary)](#example_Extract_unary)
* [Package (Initialization)](#example__initialization)
* [Package (InitializationWithDecider)](#example__initializationWithDecider)
* [Package (InitializationWithDurationFieldOverride)](#example__initializationWithDurationFieldOverride)

#### <a name="pkg-files">Package files</a>
[client_interceptors.go](./client_interceptors.go) [context.go](./context.go) [doc.go](./doc.go) [grpclogger.go](./grpclogger.go) [options.go](./options.go) [payload_interceptors.go](./payload_interceptors.go) [server_interceptors.go](./server_interceptors.go) 

## <a name="pkg-variables">Variables</a>
``` go
var (
    // SystemField is used in every log statement made through grpc_zap. Can be overwritten before any initialization code.
    SystemField = zap.String("system", "grpc")

    // ServerField is used in every server-side log statement made through grpc_zap.Can be overwritten before initialization.
    ServerField = zap.String("span.kind", "server")
)
```
``` go
var (
    // ClientField is used in every client-side log statement made through grpc_zap. Can be overwritten before initialization.
    ClientField = zap.String("span.kind", "client")
)
```
``` go
var DefaultDurationToField = DurationToTimeMillisField
```
DefaultDurationToField is the default implementation of converting request duration to a Zap field.

``` go
var (
    // JsonPbMarshaller is the marshaller used for serializing protobuf messages.
    JsonPbMarshaller = &jsonpb.Marshaler{}
)
```

## <a name="AddFields">func</a> [AddFields](./context.go#L12)
``` go
func AddFields(ctx context.Context, fields ...zapcore.Field)
```
AddFields adds zap fields to the logger.
Deprecated: should use the ctxzap.AddFields instead

## <a name="DefaultClientCodeToLevel">func</a> [DefaultClientCodeToLevel](./options.go#L127)
``` go
func DefaultClientCodeToLevel(code codes.Code) zapcore.Level
```
DefaultClientCodeToLevel is the default implementation of gRPC return codes to log levels for client side.

## <a name="DefaultCodeToLevel">func</a> [DefaultCodeToLevel](./options.go#L85)
``` go
func DefaultCodeToLevel(code codes.Code) zapcore.Level
```
DefaultCodeToLevel is the default implementation of gRPC return codes and interceptor log level for server side.

## <a name="DurationToDurationField">func</a> [DurationToDurationField](./options.go#L178)
``` go
func DurationToDurationField(duration time.Duration) zapcore.Field
```
DurationToDurationField uses a Duration field to log the request duration
and leaves it up to Zap's encoder settings to determine how that is output.

## <a name="DurationToTimeMillisField">func</a> [DurationToTimeMillisField](./options.go#L172)
``` go
func DurationToTimeMillisField(duration time.Duration) zapcore.Field
```
DurationToTimeMillisField converts the duration to milliseconds and uses the key `grpc.time_ms`.

## <a name="Extract">func</a> [Extract](./context.go#L18)
``` go
func Extract(ctx context.Context) *zap.Logger
```
Extract takes the call-scoped Logger from grpc_zap middleware.
Deprecated: should use the ctxzap.Extract instead

#### Example:

<details>
<summary>Click to expand code.</summary>

```go
_ = func(ctx context.Context, ping *pb_testproto.PingRequest) (*pb_testproto.PingResponse, error) {
    // Add fields the ctxtags of the request which will be added to all extracted loggers.
    grpc_ctxtags.Extract(ctx).Set("custom_tags.string", "something").Set("custom_tags.int", 1337)

    // Extract a single request-scoped zap.Logger and log messages. (containing the grpc.xxx tags)
    l := ctx_zap.Extract(ctx)
    l.Info("some ping")
    l.Info("another ping")
    return &pb_testproto.PingResponse{Value: ping.Value}, nil
}
```

</details>

## <a name="PayloadStreamClientInterceptor">func</a> [PayloadStreamClientInterceptor](./payload_interceptors.go#L74)
``` go
func PayloadStreamClientInterceptor(logger *zap.Logger, decider grpc_logging.ClientPayloadLoggingDecider) grpc.StreamClientInterceptor
```
PayloadStreamClientInterceptor returns a new streaming client interceptor that logs the paylods of requests and responses.

## <a name="PayloadStreamServerInterceptor">func</a> [PayloadStreamServerInterceptor](./payload_interceptors.go#L46)
``` go
func PayloadStreamServerInterceptor(logger *zap.Logger, decider grpc_logging.ServerPayloadLoggingDecider) grpc.StreamServerInterceptor
```
PayloadStreamServerInterceptor returns a new server server interceptors that logs the payloads of requests.

This *only* works when placed *after* the `grpc_zap.StreamServerInterceptor`. However, the logging can be done to a
separate instance of the logger.

## <a name="PayloadUnaryClientInterceptor">func</a> [PayloadUnaryClientInterceptor](./payload_interceptors.go#L58)
``` go
func PayloadUnaryClientInterceptor(logger *zap.Logger, decider grpc_logging.ClientPayloadLoggingDecider) grpc.UnaryClientInterceptor
```
PayloadUnaryClientInterceptor returns a new unary client interceptor that logs the paylods of requests and responses.

## <a name="PayloadUnaryServerInterceptor">func</a> [PayloadUnaryServerInterceptor](./payload_interceptors.go#L26)
``` go
func PayloadUnaryServerInterceptor(logger *zap.Logger, decider grpc_logging.ServerPayloadLoggingDecider) grpc.UnaryServerInterceptor
```
PayloadUnaryServerInterceptor returns a new unary server interceptors that logs the payloads of requests.

This *only* works when placed *after* the `grpc_zap.UnaryServerInterceptor`. However, the logging can be done to a
separate instance of the logger.

## <a name="ReplaceGrpcLogger">func</a> [ReplaceGrpcLogger](./grpclogger.go#L15)
``` go
func ReplaceGrpcLogger(logger *zap.Logger)
```
ReplaceGrpcLogger sets the given zap.Logger as a gRPC-level logger.
This should be called *before* any other initialization, preferably from init() functions.

## <a name="StreamClientInterceptor">func</a> [StreamClientInterceptor](./client_interceptors.go#L34)
``` go
func StreamClientInterceptor(logger *zap.Logger, opts ...Option) grpc.StreamClientInterceptor
```
StreamClientInterceptor returns a new streaming client interceptor that optionally logs the execution of external gRPC calls.

## <a name="StreamServerInterceptor">func</a> [StreamServerInterceptor](./server_interceptors.go#L51)
``` go
func StreamServerInterceptor(logger *zap.Logger, opts ...Option) grpc.StreamServerInterceptor
```
StreamServerInterceptor returns a new streaming server interceptor that adds zap.Logger to the context.

## <a name="UnaryClientInterceptor">func</a> [UnaryClientInterceptor](./client_interceptors.go#L22)
``` go
func UnaryClientInterceptor(logger *zap.Logger, opts ...Option) grpc.UnaryClientInterceptor
```
UnaryClientInterceptor returns a new unary client interceptor that optionally logs the execution of external gRPC calls.

## <a name="UnaryServerInterceptor">func</a> [UnaryServerInterceptor](./server_interceptors.go#L25)
``` go
func UnaryServerInterceptor(logger *zap.Logger, opts ...Option) grpc.UnaryServerInterceptor
```
UnaryServerInterceptor returns a new unary server interceptors that adds zap.Logger to the context.

## <a name="CodeToLevel">type</a> [CodeToLevel](./options.go#L51)
``` go
type CodeToLevel func(code codes.Code) zapcore.Level
```
CodeToLevel function defines the mapping between gRPC return codes and interceptor log level.

## <a name="DurationToField">type</a> [DurationToField](./options.go#L54)
``` go
type DurationToField func(duration time.Duration) zapcore.Field
```
DurationToField function defines how to produce duration fields for logging

## <a name="Option">type</a> [Option](./options.go#L48)
``` go
type Option func(*options)
```

### <a name="WithCodes">func</a> [WithCodes](./options.go#L71)
``` go
func WithCodes(f grpc_logging.ErrorToCode) Option
```
WithCodes customizes the function for mapping errors to error codes.

### <a name="WithDecider">func</a> [WithDecider](./options.go#L57)
``` go
func WithDecider(f grpc_logging.Decider) Option
```
WithDecider customizes the function for deciding if the gRPC interceptor logs should log.

### <a name="WithDurationField">func</a> [WithDurationField](./options.go#L78)
``` go
func WithDurationField(f DurationToField) Option
```
WithDurationField customizes the function for mapping request durations to Zap fields.

### <a name="WithLevels">func</a> [WithLevels](./options.go#L64)
``` go
func WithLevels(f CodeToLevel) Option
```
WithLevels customizes the function for mapping gRPC return codes and interceptor log level statements.

- - -
Generated by [godoc2ghmd](https://github.com/GandalfUK/godoc2ghmd)