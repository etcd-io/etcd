# grpc_retry
`import "github.com/grpc-ecosystem/go-grpc-middleware/retry"`

* [Overview](#pkg-overview)
* [Imported Packages](#pkg-imports)
* [Index](#pkg-index)
* [Examples](#pkg-examples)

## <a name="pkg-overview">Overview</a>
`grpc_retry` provides client-side request retry logic for gRPC.

### Client-Side Request Retry Interceptor
It allows for automatic retry, inside the generated gRPC code of requests based on the gRPC status
of the reply. It supports unary (1:1), and server stream (1:n) requests.

By default the interceptors *are disabled*, preventing accidental use of retries. You can easily
override the number of retries (setting them to more than 0) with a `grpc.ClientOption`, e.g.:

	myclient.Ping(ctx, goodPing, grpc_retry.WithMax(5))

Other default options are: retry on `ResourceExhausted` and `Unavailable` gRPC codes, use a 50ms
linear backoff with 10% jitter.

For chained interceptors, the retry interceptor will call every interceptor that follows it
whenever when a retry happens.

Please see examples for more advanced use.

#### Example:

<details>
<summary>Click to expand code.</summary>

```go
grpc.Dial("myservice.example.com",
    grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor()),
    grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor()),
)
```

</details>

#### Example:

<details>
<summary>Click to expand code.</summary>

```go
opts := []grpc_retry.CallOption{
    grpc_retry.WithBackoff(grpc_retry.BackoffLinear(100 * time.Millisecond)),
    grpc_retry.WithCodes(codes.NotFound, codes.Aborted),
}
grpc.Dial("myservice.example.com",
    grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(opts...)),
    grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(opts...)),
)
```

</details>

#### Example:

<details>
<summary>Click to expand code.</summary>

```go
client := pb_testproto.NewTestServiceClient(cc)
stream, _ := client.PingList(newCtx(1*time.Second), &pb_testproto.PingRequest{}, grpc_retry.WithMax(3))

for {
    pong, err := stream.Recv() // retries happen here
    if err == io.EOF {
        break
    } else if err != nil {
        return
    }
    fmt.Printf("got pong: %v", pong)
}
```

</details>

## <a name="pkg-imports">Imported Packages</a>

- [github.com/grpc-ecosystem/go-grpc-middleware/util/backoffutils](./../util/backoffutils)
- [github.com/grpc-ecosystem/go-grpc-middleware/util/metautils](./../util/metautils)
- [golang.org/x/net/context](https://godoc.org/golang.org/x/net/context)
- [golang.org/x/net/trace](https://godoc.org/golang.org/x/net/trace)
- [google.golang.org/grpc](https://godoc.org/google.golang.org/grpc)
- [google.golang.org/grpc/codes](https://godoc.org/google.golang.org/grpc/codes)
- [google.golang.org/grpc/metadata](https://godoc.org/google.golang.org/grpc/metadata)

## <a name="pkg-index">Index</a>
* [Constants](#pkg-constants)
* [Variables](#pkg-variables)
* [func StreamClientInterceptor(optFuncs ...CallOption) grpc.StreamClientInterceptor](#StreamClientInterceptor)
* [func UnaryClientInterceptor(optFuncs ...CallOption) grpc.UnaryClientInterceptor](#UnaryClientInterceptor)
* [type BackoffFunc](#BackoffFunc)
  * [func BackoffLinear(waitBetween time.Duration) BackoffFunc](#BackoffLinear)
  * [func BackoffLinearWithJitter(waitBetween time.Duration, jitterFraction float64) BackoffFunc](#BackoffLinearWithJitter)
* [type CallOption](#CallOption)
  * [func Disable() CallOption](#Disable)
  * [func WithBackoff(bf BackoffFunc) CallOption](#WithBackoff)
  * [func WithCodes(retryCodes ...codes.Code) CallOption](#WithCodes)
  * [func WithMax(maxRetries uint) CallOption](#WithMax)
  * [func WithPerRetryTimeout(timeout time.Duration) CallOption](#WithPerRetryTimeout)

#### <a name="pkg-examples">Examples</a>
* [WithPerRetryTimeout](#example_WithPerRetryTimeout)
* [Package (Initialization)](#example__initialization)
* [Package (InitializationWithOptions)](#example__initializationWithOptions)
* [Package (SimpleCall)](#example__simpleCall)

#### <a name="pkg-files">Package files</a>
[backoff.go](./backoff.go) [doc.go](./doc.go) [options.go](./options.go) [retry.go](./retry.go) 

## <a name="pkg-constants">Constants</a>
``` go
const (
    AttemptMetadataKey = "x-retry-attempty"
)
```

## <a name="pkg-variables">Variables</a>
``` go
var (
    // DefaultRetriableCodes is a set of well known types gRPC codes that should be retri-able.
    //
    // `ResourceExhausted` means that the user quota, e.g. per-RPC limits, have been reached.
    // `Unavailable` means that system is currently unavailable and the client should retry again.
    DefaultRetriableCodes = []codes.Code{codes.ResourceExhausted, codes.Unavailable}
)
```

## <a name="StreamClientInterceptor">func</a> [StreamClientInterceptor](./retry.go#L76)
``` go
func StreamClientInterceptor(optFuncs ...CallOption) grpc.StreamClientInterceptor
```
StreamClientInterceptor returns a new retrying stream client interceptor for server side streaming calls.

The default configuration of the interceptor is to not retry *at all*. This behaviour can be
changed through options (e.g. WithMax) on creation of the interceptor or on call (through grpc.CallOptions).

Retry logic is available *only for ServerStreams*, i.e. 1:n streams, as the internal logic needs
to buffer the messages sent by the client. If retry is enabled on any other streams (ClientStreams,
BidiStreams), the retry interceptor will fail the call.

## <a name="UnaryClientInterceptor">func</a> [UnaryClientInterceptor](./retry.go#L28)
``` go
func UnaryClientInterceptor(optFuncs ...CallOption) grpc.UnaryClientInterceptor
```
UnaryClientInterceptor returns a new retrying unary client interceptor.

The default configuration of the interceptor is to not retry *at all*. This behaviour can be
changed through options (e.g. WithMax) on creation of the interceptor or on call (through grpc.CallOptions).

## <a name="BackoffFunc">type</a> [BackoffFunc](./options.go#L35)
``` go
type BackoffFunc func(attempt uint) time.Duration
```
BackoffFunc denotes a family of functions that control the backoff duration between call retries.

They are called with an identifier of the attempt, and should return a time the system client should
hold off for. If the time returned is longer than the `context.Context.Deadline` of the request
the deadline of the request takes precedence and the wait will be interrupted before proceeding
with the next iteration.

### <a name="BackoffLinear">func</a> [BackoffLinear](./backoff.go#L13)
``` go
func BackoffLinear(waitBetween time.Duration) BackoffFunc
```
BackoffLinear is very simple: it waits for a fixed period of time between calls.

### <a name="BackoffLinearWithJitter">func</a> [BackoffLinearWithJitter](./backoff.go#L22)
``` go
func BackoffLinearWithJitter(waitBetween time.Duration, jitterFraction float64) BackoffFunc
```
BackoffLinearWithJitter waits a set period of time, allowing for jitter (fractional adjustment).

For example waitBetween=1s and jitter=0.10 can generate waits between 900ms and 1100ms.

## <a name="CallOption">type</a> [CallOption](./options.go#L94-L97)
``` go
type CallOption struct {
    grpc.EmptyCallOption // make sure we implement private after() and before() fields so we don't panic.
    // contains filtered or unexported fields
}
```
CallOption is a grpc.CallOption that is local to grpc_retry.

### <a name="Disable">func</a> [Disable](./options.go#L40)
``` go
func Disable() CallOption
```
Disable disables the retry behaviour on this call, or this interceptor.

Its semantically the same to `WithMax`

### <a name="WithBackoff">func</a> [WithBackoff](./options.go#L52)
``` go
func WithBackoff(bf BackoffFunc) CallOption
```
WithBackoff sets the `BackoffFunc `used to control time between retries.

### <a name="WithCodes">func</a> [WithCodes](./options.go#L63)
``` go
func WithCodes(retryCodes ...codes.Code) CallOption
```
WithCodes sets which codes should be retried.

Please *use with care*, as you may be retrying non-idempotend calls.

You cannot automatically retry on Cancelled and Deadline, please use `WithPerRetryTimeout` for these.

### <a name="WithMax">func</a> [WithMax](./options.go#L45)
``` go
func WithMax(maxRetries uint) CallOption
```
WithMax sets the maximum number of retries on this call, or this interceptor.

### <a name="WithPerRetryTimeout">func</a> [WithPerRetryTimeout](./options.go#L79)
``` go
func WithPerRetryTimeout(timeout time.Duration) CallOption
```
WithPerRetryTimeout sets the RPC timeout per call (including initial call) on this call, or this interceptor.

The context.Deadline of the call takes precedence and sets the maximum time the whole invocation
will take, but WithPerRetryTimeout can be used to limit the RPC time per each call.

For example, with context.Deadline = now + 10s, and WithPerRetryTimeout(3 * time.Seconds), each
of the retry calls (including the initial one) will have a deadline of now + 3s.

A value of 0 disables the timeout overrides completely and returns to each retry call using the
parent `context.Deadline`.

#### Example:

<details>
<summary>Click to expand code.</summary>

```go
client := pb_testproto.NewTestServiceClient(cc)
pong, _ := client.Ping(
    newCtx(5*time.Second),
    &pb_testproto.PingRequest{},
    grpc_retry.WithMax(3),
    grpc_retry.WithPerRetryTimeout(1*time.Second))

fmt.Printf("got pong: %v", pong)
```

</details>

- - -
Generated by [godoc2ghmd](https://github.com/GandalfUK/godoc2ghmd)