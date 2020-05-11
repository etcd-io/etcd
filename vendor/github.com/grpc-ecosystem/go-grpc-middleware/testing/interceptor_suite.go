// Copyright 2016 Michal Witkowski. All Rights Reserved.
// See LICENSE for licensing terms.

package grpc_testing

import (
	"net"
	"time"

	"flag"
	"path"
	"runtime"

	pb_testproto "github.com/grpc-ecosystem/go-grpc-middleware/testing/testproto"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	flagTls = flag.Bool("use_tls", true, "whether all gRPC middleware tests should use tls")
)

func getTestingCertsPath() string {
	_, callerPath, _, _ := runtime.Caller(0)
	return path.Join(path.Dir(callerPath), "certs")
}

// InterceptorTestSuite is a testify/Suite that starts a gRPC PingService server and a client.
type InterceptorTestSuite struct {
	suite.Suite

	TestService pb_testproto.TestServiceServer
	ServerOpts  []grpc.ServerOption
	ClientOpts  []grpc.DialOption

	ServerListener net.Listener
	Server         *grpc.Server
	clientConn     *grpc.ClientConn
	Client         pb_testproto.TestServiceClient
}

func (s *InterceptorTestSuite) SetupSuite() {
	var err error
	s.ServerListener, err = net.Listen("tcp", "127.0.0.1:0")
	require.NoError(s.T(), err, "must be able to allocate a port for serverListener")
	if *flagTls {
		creds, err := credentials.NewServerTLSFromFile(
			path.Join(getTestingCertsPath(), "localhost.crt"),
			path.Join(getTestingCertsPath(), "localhost.key"),
		)
		require.NoError(s.T(), err, "failed reading server credentials for localhost.crt")
		s.ServerOpts = append(s.ServerOpts, grpc.Creds(creds))
	}
	// This is the point where we hook up the interceptor
	s.Server = grpc.NewServer(s.ServerOpts...)
	// Crete a service of the instantiator hasn't provided one.
	if s.TestService == nil {
		s.TestService = &TestPingService{T: s.T()}
	}
	pb_testproto.RegisterTestServiceServer(s.Server, s.TestService)

	go func() {
		s.Server.Serve(s.ServerListener)
	}()
	s.Client = s.NewClient(s.ClientOpts...)
}

func (s *InterceptorTestSuite) NewClient(dialOpts ...grpc.DialOption) pb_testproto.TestServiceClient {
	newDialOpts := append(dialOpts, grpc.WithBlock(), grpc.WithTimeout(2*time.Second))
	if *flagTls {
		creds, err := credentials.NewClientTLSFromFile(
			path.Join(getTestingCertsPath(), "localhost.crt"), "localhost")
		require.NoError(s.T(), err, "failed reading client credentials for localhost.crt")
		newDialOpts = append(newDialOpts, grpc.WithTransportCredentials(creds))
	} else {
		newDialOpts = append(newDialOpts, grpc.WithInsecure())
	}
	clientConn, err := grpc.Dial(s.ServerAddr(), newDialOpts...)
	require.NoError(s.T(), err, "must not error on client Dial")
	return pb_testproto.NewTestServiceClient(clientConn)
}

func (s *InterceptorTestSuite) ServerAddr() string {
	return s.ServerListener.Addr().String()
}

func (s *InterceptorTestSuite) SimpleCtx() context.Context {
	ctx, _ := context.WithTimeout(context.TODO(), 2*time.Second)
	return ctx
}

func (s *InterceptorTestSuite) DeadlineCtx(deadline time.Time) context.Context {
	ctx, _ := context.WithDeadline(context.TODO(), deadline)
	return ctx
}

func (s *InterceptorTestSuite) TearDownSuite() {
	time.Sleep(10 * time.Millisecond)
	if s.ServerListener != nil {
		s.Server.GracefulStop()
		s.T().Logf("stopped grpc.Server at: %v", s.ServerAddr())
		s.ServerListener.Close()

	}
	if s.clientConn != nil {
		s.clientConn.Close()
	}
}
