# grpc_opentracing
`import "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"`

* [Overview](#pkg-overview)
* [Imported Packages](#pkg-imports)
* [Index](#pkg-index)

## <a name="pkg-overview">Overview</a>
`grpc_opentracing` adds OpenTracing

### OpenTracing Interceptors
These are both client-side and server-side interceptors for OpenTracing. They are a provider-agnostic, with backends
such as Zipkin, or Google Stackdriver Trace.

For a service that sends out requests and receives requests, you *need* to use both, otherwise downstream requests will
not have the appropriate requests propagated.

All server-side spans are tagged with grpc_ctxtags information.

For more information see:
<a href="http://opentracing.io/documentation/">http://opentracing.io/documentation/</a>
<a href="https://github.com/opentracing/specification/blob/master/semantic_conventions.md">https://github.com/opentracing/specification/blob/master/semantic_conventions.md</a>

## <a name="pkg-imports">Imported Packages</a>

- [github.com/grpc-ecosystem/go-grpc-middleware](./../..)
- [github.com/grpc-ecosystem/go-grpc-middleware/tags](./../../tags)
- [github.com/grpc-ecosystem/go-grpc-middleware/util/metautils](./../../util/metautils)
- [github.com/opentracing/opentracing-go](https://godoc.org/github.com/opentracing/opentracing-go)
- [github.com/opentracing/opentracing-go/ext](https://godoc.org/github.com/opentracing/opentracing-go/ext)
- [github.com/opentracing/opentracing-go/log](https://godoc.org/github.com/opentracing/opentracing-go/log)
- [golang.org/x/net/context](https://godoc.org/golang.org/x/net/context)
- [google.golang.org/grpc](https://godoc.org/google.golang.org/grpc)
- [google.golang.org/grpc/grpclog](https://godoc.org/google.golang.org/grpc/grpclog)
- [google.golang.org/grpc/metadata](https://godoc.org/google.golang.org/grpc/metadata)

## <a name="pkg-index">Index</a>
* [Constants](#pkg-constants)
* [func ClientAddContextTags(ctx context.Context, tags opentracing.Tags) context.Context](#ClientAddContextTags)
* [func StreamClientInterceptor(opts ...Option) grpc.StreamClientInterceptor](#StreamClientInterceptor)
* [func StreamServerInterceptor(opts ...Option) grpc.StreamServerInterceptor](#StreamServerInterceptor)
* [func UnaryClientInterceptor(opts ...Option) grpc.UnaryClientInterceptor](#UnaryClientInterceptor)
* [func UnaryServerInterceptor(opts ...Option) grpc.UnaryServerInterceptor](#UnaryServerInterceptor)
* [type FilterFunc](#FilterFunc)
* [type Option](#Option)
  * [func WithFilterFunc(f FilterFunc) Option](#WithFilterFunc)
  * [func WithTracer(tracer opentracing.Tracer) Option](#WithTracer)

#### <a name="pkg-files">Package files</a>
[client_interceptors.go](./client_interceptors.go) [doc.go](./doc.go) [id_extract.go](./id_extract.go) [metadata.go](./metadata.go) [options.go](./options.go) [server_interceptors.go](./server_interceptors.go) 

## <a name="pkg-constants">Constants</a>
``` go
const (
    TagTraceId = "trace.traceid"
    TagSpanId  = "trace.spanid"
)
```

## <a name="ClientAddContextTags">func</a> [ClientAddContextTags](./client_interceptors.go#L105)
``` go
func ClientAddContextTags(ctx context.Context, tags opentracing.Tags) context.Context
```
ClientAddContextTags returns a context with specified opentracing tags, which
are used by UnaryClientInterceptor/StreamClientInterceptor when creating a
new span.

## <a name="StreamClientInterceptor">func</a> [StreamClientInterceptor](./client_interceptors.go#L35)
``` go
func StreamClientInterceptor(opts ...Option) grpc.StreamClientInterceptor
```
StreamClientInterceptor returns a new streaming client interceptor for OpenTracing.

## <a name="StreamServerInterceptor">func</a> [StreamServerInterceptor](./server_interceptors.go#L37)
``` go
func StreamServerInterceptor(opts ...Option) grpc.StreamServerInterceptor
```
StreamServerInterceptor returns a new streaming server interceptor for OpenTracing.

## <a name="UnaryClientInterceptor">func</a> [UnaryClientInterceptor](./client_interceptors.go#L21)
``` go
func UnaryClientInterceptor(opts ...Option) grpc.UnaryClientInterceptor
```
UnaryClientInterceptor returns a new unary client interceptor for OpenTracing.

## <a name="UnaryServerInterceptor">func</a> [UnaryServerInterceptor](./server_interceptors.go#L23)
``` go
func UnaryServerInterceptor(opts ...Option) grpc.UnaryServerInterceptor
```
UnaryServerInterceptor returns a new unary server interceptor for OpenTracing.

## <a name="FilterFunc">type</a> [FilterFunc](./options.go#L22)
``` go
type FilterFunc func(ctx context.Context, fullMethodName string) bool
```
FilterFunc allows users to provide a function that filters out certain methods from being traced.

If it returns false, the given request will not be traced.

## <a name="Option">type</a> [Option](./options.go#L41)
``` go
type Option func(*options)
```

### <a name="WithFilterFunc">func</a> [WithFilterFunc](./options.go#L44)
``` go
func WithFilterFunc(f FilterFunc) Option
```
WithFilterFunc customizes the function used for deciding whether a given call is traced or not.

### <a name="WithTracer">func</a> [WithTracer](./options.go#L51)
``` go
func WithTracer(tracer opentracing.Tracer) Option
```
WithTracer sets a custom tracer to be used for this middleware, otherwise the opentracing.GlobalTracer is used.

- - -
Generated by [godoc2ghmd](https://github.com/GandalfUK/godoc2ghmd)