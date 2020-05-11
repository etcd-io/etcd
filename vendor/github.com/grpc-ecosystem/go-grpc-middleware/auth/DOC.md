# grpc_auth
`import "github.com/grpc-ecosystem/go-grpc-middleware/auth"`

* [Overview](#pkg-overview)
* [Imported Packages](#pkg-imports)
* [Index](#pkg-index)
* [Examples](#pkg-examples)

## <a name="pkg-overview">Overview</a>
`grpc_auth` a generic server-side auth middleware for gRPC.

### Server Side Auth Middleware
It allows for easy assertion of `:authorization` headers in gRPC calls, be it HTTP Basic auth, or
OAuth2 Bearer tokens.

The middleware takes a user-customizable `AuthFunc`, which can be customized to verify and extract
auth information from the request. The extracted information can be put in the `context.Context` of
handlers downstream for retrieval.

It also allows for per-service implementation overrides of `AuthFunc`. See `ServiceAuthFuncOverride`.

Please see examples for simple examples of use.

#### Example:

<details>
<summary>Click to expand code.</summary>

```go
package grpc_auth_test

import (
    "github.com/grpc-ecosystem/go-grpc-middleware/auth"
    "github.com/grpc-ecosystem/go-grpc-middleware/tags"
    "golang.org/x/net/context"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
)

var (
    cc *grpc.ClientConn
)

func parseToken(token string) (struct{}, error) {
    return struct{}{}, nil
}

func userClaimFromToken(struct{}) string {
    return "foobar"
}

// Simple example of server initialization code.
func Example_serverConfig() {
    exampleAuthFunc := func(ctx context.Context) (context.Context, error) {
        token, err := grpc_auth.AuthFromMD(ctx, "bearer")
        if err != nil {
            return nil, err
        }
        tokenInfo, err := parseToken(token)
        if err != nil {
            return nil, grpc.Errorf(codes.Unauthenticated, "invalid auth token: %v", err)
        }
        grpc_ctxtags.Extract(ctx).Set("auth.sub", userClaimFromToken(tokenInfo))
        newCtx := context.WithValue(ctx, "tokenInfo", tokenInfo)
        return newCtx, nil
    }

    _ = grpc.NewServer(
        grpc.StreamInterceptor(grpc_auth.StreamServerInterceptor(exampleAuthFunc)),
        grpc.UnaryInterceptor(grpc_auth.UnaryServerInterceptor(exampleAuthFunc)),
    )
}
```

</details>

## <a name="pkg-imports">Imported Packages</a>

- [github.com/grpc-ecosystem/go-grpc-middleware](./..)
- [github.com/grpc-ecosystem/go-grpc-middleware/util/metautils](./../util/metautils)
- [golang.org/x/net/context](https://godoc.org/golang.org/x/net/context)
- [google.golang.org/grpc](https://godoc.org/google.golang.org/grpc)
- [google.golang.org/grpc/codes](https://godoc.org/google.golang.org/grpc/codes)

## <a name="pkg-index">Index</a>
* [func AuthFromMD(ctx context.Context, expectedScheme string) (string, error)](#AuthFromMD)
* [func StreamServerInterceptor(authFunc AuthFunc) grpc.StreamServerInterceptor](#StreamServerInterceptor)
* [func UnaryServerInterceptor(authFunc AuthFunc) grpc.UnaryServerInterceptor](#UnaryServerInterceptor)
* [type AuthFunc](#AuthFunc)
* [type ServiceAuthFuncOverride](#ServiceAuthFuncOverride)

#### <a name="pkg-examples">Examples</a>
* [Package (ServerConfig)](#example__serverConfig)

#### <a name="pkg-files">Package files</a>
[auth.go](./auth.go) [doc.go](./doc.go) [metadata.go](./metadata.go) 

## <a name="AuthFromMD">func</a> [AuthFromMD](./metadata.go#L24)
``` go
func AuthFromMD(ctx context.Context, expectedScheme string) (string, error)
```
AuthFromMD is a helper function for extracting the :authorization header from the gRPC metadata of the request.

It expects the `:authorization` header to be of a certain scheme (e.g. `basic`, `bearer`), in a
case-insensitive format (see rfc2617, sec 1.2). If no such authorization is found, or the token
is of wrong scheme, an error with gRPC status `Unauthenticated` is returned.

## <a name="StreamServerInterceptor">func</a> [StreamServerInterceptor](./auth.go#L51)
``` go
func StreamServerInterceptor(authFunc AuthFunc) grpc.StreamServerInterceptor
```
StreamServerInterceptor returns a new unary server interceptors that performs per-request auth.

## <a name="UnaryServerInterceptor">func</a> [UnaryServerInterceptor](./auth.go#L34)
``` go
func UnaryServerInterceptor(authFunc AuthFunc) grpc.UnaryServerInterceptor
```
UnaryServerInterceptor returns a new unary server interceptors that performs per-request auth.

## <a name="AuthFunc">type</a> [AuthFunc](./auth.go#L23)
``` go
type AuthFunc func(ctx context.Context) (context.Context, error)
```
AuthFunc is the pluggable function that performs authentication.

The passed in `Context` will contain the gRPC metadata.MD object (for header-based authentication) and
the peer.Peer information that can contain transport-based credentials (e.g. `credentials.AuthInfo`).

The returned context will be propagated to handlers, allowing user changes to `Context`. However,
please make sure that the `Context` returned is a child `Context` of the one passed in.

If error is returned, its `grpc.Code()` will be returned to the user as well as the verbatim message.
Please make sure you use `codes.Unauthenticated` (lacking auth) and `codes.PermissionDenied`
(authed, but lacking perms) appropriately.

## <a name="ServiceAuthFuncOverride">type</a> [ServiceAuthFuncOverride](./auth.go#L29-L31)
``` go
type ServiceAuthFuncOverride interface {
    AuthFuncOverride(ctx context.Context, fullMethodName string) (context.Context, error)
}
```
ServiceAuthFuncOverride allows a given gRPC service implementation to override the global `AuthFunc`.

If a service implements the AuthFuncOverride method, it takes precedence over the `AuthFunc` method,
and will be called instead of AuthFunc for all method invocations within that service.

- - -
Generated by [godoc2ghmd](https://github.com/GandalfUK/godoc2ghmd)