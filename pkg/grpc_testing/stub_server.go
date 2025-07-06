package grpc_testing

import (
	"context"
	"fmt"
	"net"

	"google.golang.org/grpc"
	testpb "google.golang.org/grpc/test/grpc_testing"
)

// StubServer is borrowed from the interal package of grpc-go.
// See https://github.com/grpc/grpc-go/blob/master/internal/stubserver/stubserver.go
// Since it cannot be imported directly, we have to copy and paste it here,
// and useless code for our testing is removed.

// StubServer is a server that is easy to customize within individual test
// cases.
type StubServer struct {
	testService testpb.TestServiceServer

	// Network and Address are parameters for Listen. Defaults will be used if these are empty before Start.
	Network string
	Address string

	s *grpc.Server

	cleanups []func() // Lambdas executed in Stop(); populated by Start().
}

func New(testService testpb.TestServiceServer) *StubServer {
	return &StubServer{testService: testService}
}

// Start starts the server and creates a client connected to it.
func (ss *StubServer) Start(sopts []grpc.ServerOption, dopts ...grpc.DialOption) error {
	if ss.Network == "" {
		ss.Network = "tcp"
	}
	if ss.Address == "" {
		ss.Address = "localhost:0"
	}

	lis, err := net.Listen(ss.Network, ss.Address)
	if err != nil {
		return fmt.Errorf("net.Listen(%q, %q) = %v", ss.Network, ss.Address, err)
	}
	ss.Address = lis.Addr().String()
	ss.cleanups = append(ss.cleanups, func() { lis.Close() })

	s := grpc.NewServer(sopts...)
	testpb.RegisterTestServiceServer(s, ss.testService)
	go s.Serve(lis)
	ss.cleanups = append(ss.cleanups, s.Stop)
	ss.s = s

	return nil
}

// Stop stops ss and cleans up all resources it consumed.
func (ss *StubServer) Stop() {
	for i := len(ss.cleanups) - 1; i >= 0; i-- {
		ss.cleanups[i]()
	}
}

// Addr gets the address the server listening on.
func (ss *StubServer) Addr() string {
	return ss.Address
}

type dummyStubServer struct {
	testpb.UnimplementedTestServiceServer
	body []byte
}

func (d dummyStubServer) UnaryCall(context.Context, *testpb.SimpleRequest) (*testpb.SimpleResponse, error) {
	return &testpb.SimpleResponse{
		Payload: &testpb.Payload{
			Type: testpb.PayloadType_COMPRESSABLE,
			Body: d.body,
		},
	}, nil
}

// NewDummyStubServer creates a simple test server that serves Unary calls with
// responses with the given payload.
func NewDummyStubServer(body []byte) *StubServer {
	return New(dummyStubServer{body: body})
}
