// Copyright 2016 The CMux Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package cmux_test

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strings"

	"google.golang.org/grpc"

	"golang.org/x/net/context"
	"golang.org/x/net/websocket"

	"github.com/soheilhy/cmux"
	grpchello "google.golang.org/grpc/examples/helloworld/helloworld"
)

type exampleHTTPHandler struct{}

func (h *exampleHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "example http response")
}

func serveHTTP(l net.Listener) {
	s := &http.Server{
		Handler: &exampleHTTPHandler{},
	}
	if err := s.Serve(l); err != cmux.ErrListenerClosed {
		panic(err)
	}
}

func EchoServer(ws *websocket.Conn) {
	if _, err := io.Copy(ws, ws); err != nil {
		panic(err)
	}
}

func serveWS(l net.Listener) {
	s := &http.Server{
		Handler: websocket.Handler(EchoServer),
	}
	if err := s.Serve(l); err != cmux.ErrListenerClosed {
		panic(err)
	}
}

type ExampleRPCRcvr struct{}

func (r *ExampleRPCRcvr) Cube(i int, j *int) error {
	*j = i * i
	return nil
}

func serveRPC(l net.Listener) {
	s := rpc.NewServer()
	if err := s.Register(&ExampleRPCRcvr{}); err != nil {
		panic(err)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			if err != cmux.ErrListenerClosed {
				panic(err)
			}
			return
		}
		go s.ServeConn(conn)
	}
}

type grpcServer struct{}

func (s *grpcServer) SayHello(ctx context.Context, in *grpchello.HelloRequest) (
	*grpchello.HelloReply, error) {

	return &grpchello.HelloReply{Message: "Hello " + in.Name + " from cmux"}, nil
}

func serveGRPC(l net.Listener) {
	grpcs := grpc.NewServer()
	grpchello.RegisterGreeterServer(grpcs, &grpcServer{})
	if err := grpcs.Serve(l); err != cmux.ErrListenerClosed {
		panic(err)
	}
}

func Example() {
	l, err := net.Listen("tcp", "127.0.0.1:50051")
	if err != nil {
		log.Panic(err)
	}

	m := cmux.New(l)

	// We first match the connection against HTTP2 fields. If matched, the
	// connection will be sent through the "grpcl" listener.
	grpcl := m.Match(cmux.HTTP2HeaderFieldPrefix("content-type", "application/grpc"))
	//Otherwise, we match it againts a websocket upgrade request.
	wsl := m.Match(cmux.HTTP1HeaderField("Upgrade", "websocket"))

	// Otherwise, we match it againts HTTP1 methods. If matched,
	// it is sent through the "httpl" listener.
	httpl := m.Match(cmux.HTTP1Fast())
	// If not matched by HTTP, we assume it is an RPC connection.
	rpcl := m.Match(cmux.Any())

	// Then we used the muxed listeners.
	go serveGRPC(grpcl)
	go serveWS(wsl)
	go serveHTTP(httpl)
	go serveRPC(rpcl)

	if err := m.Serve(); !strings.Contains(err.Error(), "use of closed network connection") {
		panic(err)
	}
}
