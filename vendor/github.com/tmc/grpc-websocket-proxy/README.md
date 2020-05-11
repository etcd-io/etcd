# grpc-websocket-proxy

[![GoDoc](https://godoc.org/github.com/tmc/grpc-websocket-proxy/wsproxy?status.svg)](http://godoc.org/github.com/tmc/grpc-websocket-proxy/wsproxy)

Wrap your grpc-gateway mux with this helper to expose streaming endpoints over websockets.

On the wire this uses newline-delimited json encoding of the messages.

Usage:
```diff
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithInsecure()}
	if err := echoserver.RegisterEchoServiceHandlerFromEndpoint(ctx, mux, *grpcAddr, opts); err != nil {
		return err
	}
-	http.ListenAndServe(*httpAddr, mux)
+	http.ListenAndServe(*httpAddr, wsproxy.WebsocketProxy(mux))
```


# wsproxy
    import "github.com/tmc/grpc-websocket-proxy/wsproxy"

Package wsproxy implements a websocket proxy for grpc-gateway backed services

## Usage

```go
var (
	MethodOverrideParam = "method"
	TokenCookieName     = "token"
)
```

#### func  WebsocketProxy

```go
func WebsocketProxy(h http.Handler) http.HandlerFunc
```
WebsocketProxy attempts to expose the underlying handler as a bidi websocket
stream with newline-delimited JSON as the content encoding.

The HTTP Authorization header is either populated from the
Sec-Websocket-Protocol field or by a cookie. The cookie name is specified by the
TokenCookieName value.

example:

    Sec-Websocket-Protocol: Bearer, foobar

is converted to:

    Authorization: Bearer foobar

Method can be overwritten with the MethodOverrideParam get parameter in the
requested URL
