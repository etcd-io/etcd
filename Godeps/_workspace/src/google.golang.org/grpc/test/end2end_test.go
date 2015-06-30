/*
 *
 * Copyright 2014, Google Inc.
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are
 * met:
 *
 *     * Redistributions of source code must retain the above copyright
 * notice, this list of conditions and the following disclaimer.
 *     * Redistributions in binary form must reproduce the above
 * copyright notice, this list of conditions and the following disclaimer
 * in the documentation and/or other materials provided with the
 * distribution.
 *     * Neither the name of Google Inc. nor the names of its
 * contributors may be used to endorse or promote products derived from
 * this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 * LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 * A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 * OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 * SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 * LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 * THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 *
 */

package grpc_test

import (
	"fmt"
	"io"
	"math"
	"net"
	"reflect"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/golang/protobuf/proto"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc/codes"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc/credentials"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc/grpclog"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc/metadata"
	testpb "github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc/test/grpc_testing"
)

var (
	testMetadata = metadata.MD{
		"key1": "value1",
		"key2": "value2",
	}
)

type testServer struct {
}

func (s *testServer) EmptyCall(ctx context.Context, in *testpb.Empty) (*testpb.Empty, error) {
	if _, ok := metadata.FromContext(ctx); ok {
		// For testing purpose, returns an error if there is attached metadata.
		return nil, grpc.Errorf(codes.DataLoss, "got extra metadata")
	}
	return new(testpb.Empty), nil
}

func newPayload(t testpb.PayloadType, size int32) *testpb.Payload {
	if size < 0 {
		grpclog.Fatalf("Requested a response with invalid length %d", size)
	}
	body := make([]byte, size)
	switch t {
	case testpb.PayloadType_COMPRESSABLE:
	case testpb.PayloadType_UNCOMPRESSABLE:
		grpclog.Fatalf("PayloadType UNCOMPRESSABLE is not supported")
	default:
		grpclog.Fatalf("Unsupported payload type: %d", t)
	}
	return &testpb.Payload{
		Type: t.Enum(),
		Body: body,
	}
}

func (s *testServer) UnaryCall(ctx context.Context, in *testpb.SimpleRequest) (*testpb.SimpleResponse, error) {
	md, ok := metadata.FromContext(ctx)
	if ok {
		if err := grpc.SendHeader(ctx, md); err != nil {
			grpclog.Fatalf("grpc.SendHeader(%v, %v) = %v, want %v", ctx, md, err, nil)
		}
		grpc.SetTrailer(ctx, md)
	}
	// Simulate some service delay.
	time.Sleep(2 * time.Millisecond)
	return &testpb.SimpleResponse{
		Payload: newPayload(in.GetResponseType(), in.GetResponseSize()),
	}, nil
}

func (s *testServer) StreamingOutputCall(args *testpb.StreamingOutputCallRequest, stream testpb.TestService_StreamingOutputCallServer) error {
	if _, ok := metadata.FromContext(stream.Context()); ok {
		// For testing purpose, returns an error if there is attached metadata.
		return grpc.Errorf(codes.DataLoss, "got extra metadata")
	}
	cs := args.GetResponseParameters()
	for _, c := range cs {
		if us := c.GetIntervalUs(); us > 0 {
			time.Sleep(time.Duration(us) * time.Microsecond)
		}
		if err := stream.Send(&testpb.StreamingOutputCallResponse{
			Payload: newPayload(args.GetResponseType(), c.GetSize()),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *testServer) StreamingInputCall(stream testpb.TestService_StreamingInputCallServer) error {
	var sum int
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&testpb.StreamingInputCallResponse{
				AggregatedPayloadSize: proto.Int32(int32(sum)),
			})
		}
		if err != nil {
			return err
		}
		p := in.GetPayload().GetBody()
		sum += len(p)
	}
}

func (s *testServer) FullDuplexCall(stream testpb.TestService_FullDuplexCallServer) error {
	md, ok := metadata.FromContext(stream.Context())
	if ok {
		if err := stream.SendHeader(md); err != nil {
			grpclog.Fatalf("%v.SendHeader(%v) = %v, want %v", stream, md, err, nil)
		}
		stream.SetTrailer(md)
	}
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			// read done.
			return nil
		}
		if err != nil {
			return err
		}
		cs := in.GetResponseParameters()
		for _, c := range cs {
			if us := c.GetIntervalUs(); us > 0 {
				time.Sleep(time.Duration(us) * time.Microsecond)
			}
			if err := stream.Send(&testpb.StreamingOutputCallResponse{
				Payload: newPayload(in.GetResponseType(), c.GetSize()),
			}); err != nil {
				return err
			}
		}
	}
}

func (s *testServer) HalfDuplexCall(stream testpb.TestService_HalfDuplexCallServer) error {
	msgBuf := make([]*testpb.StreamingOutputCallRequest, 0)
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			// read done.
			break
		}
		if err != nil {
			return err
		}
		msgBuf = append(msgBuf, in)
	}
	for _, m := range msgBuf {
		cs := m.GetResponseParameters()
		for _, c := range cs {
			if us := c.GetIntervalUs(); us > 0 {
				time.Sleep(time.Duration(us) * time.Microsecond)
			}
			if err := stream.Send(&testpb.StreamingOutputCallResponse{
				Payload: newPayload(m.GetResponseType(), c.GetSize()),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

const tlsDir = "testdata/"

func TestDialTimeout(t *testing.T) {
	conn, err := grpc.Dial("Non-Existent.Server:80", grpc.WithTimeout(time.Millisecond))
	if err == nil {
		conn.Close()
	}
	if err != grpc.ErrClientConnTimeout {
		t.Fatalf("grpc.Dial(_, _) = %v, %v, want %v", conn, err, grpc.ErrClientConnTimeout)
	}
}

func TestTLSDialTimeout(t *testing.T) {
	creds, err := credentials.NewClientTLSFromFile(tlsDir+"ca.pem", "x.test.youtube.com")
	if err != nil {
		t.Fatalf("Failed to create credentials %v", err)
	}
	conn, err := grpc.Dial("Non-Existent.Server:80", grpc.WithTransportCredentials(creds), grpc.WithTimeout(time.Millisecond))
	if err == nil {
		conn.Close()
	}
	if err != grpc.ErrClientConnTimeout {
		t.Fatalf("grpc.Dial(_, _) = %v, %v, want %v", conn, err, grpc.ErrClientConnTimeout)
	}
}

func TestReconnectTimeout(t *testing.T) {
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	_, port, err := net.SplitHostPort(lis.Addr().String())
	if err != nil {
		t.Fatalf("Failed to parse listener address: %v", err)
	}
	addr := "localhost:" + port
	conn, err := grpc.Dial(addr, grpc.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Failed to dial to the server %q: %v", addr, err)
	}
	// Close unaccepted connection (i.e., conn).
	lis.Close()
	tc := testpb.NewTestServiceClient(conn)
	waitC := make(chan struct{})
	go func() {
		defer close(waitC)
		argSize := 271828
		respSize := 314159
		req := &testpb.SimpleRequest{
			ResponseType: testpb.PayloadType_COMPRESSABLE.Enum(),
			ResponseSize: proto.Int32(int32(respSize)),
			Payload:      newPayload(testpb.PayloadType_COMPRESSABLE, int32(argSize)),
		}
		if _, err := tc.UnaryCall(context.Background(), req); err == nil {
			t.Fatalf("TestService/UnaryCall(_, _) = _, <nil>, want _, non-nil")
		}
	}()
	// Block untill reconnect times out.
	<-waitC
	if err := conn.Close(); err != grpc.ErrClientConnClosing {
		t.Fatalf("%v.Close() = %v, want %v", conn, err, grpc.ErrClientConnClosing)
	}
}

func unixDialer(addr string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", addr, timeout)
}

type env struct {
	network  string // The type of network such as tcp, unix, etc.
	dialer   func(addr string, timeout time.Duration) (net.Conn, error)
	security string // The security protocol such as TLS, SSH, etc.
}

func listTestEnv() []env {
	if runtime.GOOS == "windows" {
		return []env{env{"tcp", nil, ""}, env{"tcp", nil, "tls"}}
	}
	return []env{env{"tcp", nil, ""}, env{"tcp", nil, "tls"}, env{"unix", unixDialer, ""}, env{"unix", unixDialer, "tls"}}
}

func setUp(maxStream uint32, e env) (s *grpc.Server, cc *grpc.ClientConn) {
	sopts := []grpc.ServerOption{grpc.MaxConcurrentStreams(maxStream)}
	la := ":0"
	switch e.network {
	case "unix":
		la = "/tmp/testsock" + fmt.Sprintf("%d", time.Now())
		syscall.Unlink(la)
	}
	lis, err := net.Listen(e.network, la)
	if err != nil {
		grpclog.Fatalf("Failed to listen: %v", err)
	}
	if e.security == "tls" {
		creds, err := credentials.NewServerTLSFromFile(tlsDir+"server1.pem", tlsDir+"server1.key")
		if err != nil {
			grpclog.Fatalf("Failed to generate credentials %v", err)
		}
		sopts = append(sopts, grpc.Creds(creds))
	}
	s = grpc.NewServer(sopts...)
	testpb.RegisterTestServiceServer(s, &testServer{})
	go s.Serve(lis)
	addr := la
	switch e.network {
	case "unix":
	default:
		_, port, err := net.SplitHostPort(lis.Addr().String())
		if err != nil {
			grpclog.Fatalf("Failed to parse listener address: %v", err)
		}
		addr = "localhost:" + port
	}
	if e.security == "tls" {
		creds, err := credentials.NewClientTLSFromFile(tlsDir+"ca.pem", "x.test.youtube.com")
		if err != nil {
			grpclog.Fatalf("Failed to create credentials %v", err)
		}
		cc, err = grpc.Dial(addr, grpc.WithTransportCredentials(creds), grpc.WithDialer(e.dialer))
	} else {
		cc, err = grpc.Dial(addr, grpc.WithDialer(e.dialer))
	}
	if err != nil {
		grpclog.Fatalf("Dial(%q) = %v", addr, err)
	}
	return
}

func tearDown(s *grpc.Server, cc *grpc.ClientConn) {
	cc.Close()
	s.Stop()
}

func TestTimeoutOnDeadServer(t *testing.T) {
	for _, e := range listTestEnv() {
		testTimeoutOnDeadServer(t, e)
	}
}

func testTimeoutOnDeadServer(t *testing.T, e env) {
	s, cc := setUp(math.MaxUint32, e)
	tc := testpb.NewTestServiceClient(cc)
	s.Stop()
	// Set -1 as the timeout to make sure if transportMonitor gets error
	// notification in time the failure path of the 1st invoke of
	// ClientConn.wait hits the deadline exceeded error.
	ctx, _ := context.WithTimeout(context.Background(), -1)
	if _, err := tc.EmptyCall(ctx, &testpb.Empty{}); grpc.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("TestService/EmptyCall(%v, _) = _, error %v, want _, error code: %d", ctx, err, codes.DeadlineExceeded)
	}
	cc.Close()
}

func TestEmptyUnary(t *testing.T) {
	for _, e := range listTestEnv() {
		testEmptyUnary(t, e)
	}
}

func testEmptyUnary(t *testing.T, e env) {
	s, cc := setUp(math.MaxUint32, e)
	tc := testpb.NewTestServiceClient(cc)
	defer tearDown(s, cc)
	reply, err := tc.EmptyCall(context.Background(), &testpb.Empty{})
	if err != nil || !proto.Equal(&testpb.Empty{}, reply) {
		t.Fatalf("TestService/EmptyCall(_, _) = %v, %v, want %v, <nil>", reply, err, &testpb.Empty{})
	}
}

func TestFailedEmptyUnary(t *testing.T) {
	for _, e := range listTestEnv() {
		testFailedEmptyUnary(t, e)
	}
}

func testFailedEmptyUnary(t *testing.T, e env) {
	s, cc := setUp(math.MaxUint32, e)
	tc := testpb.NewTestServiceClient(cc)
	defer tearDown(s, cc)
	ctx := metadata.NewContext(context.Background(), testMetadata)
	if _, err := tc.EmptyCall(ctx, &testpb.Empty{}); err != grpc.Errorf(codes.DataLoss, "got extra metadata") {
		t.Fatalf("TestService/EmptyCall(_, _) = _, %v, want _, %v", err, grpc.Errorf(codes.DataLoss, "got extra metadata"))
	}
}

func TestLargeUnary(t *testing.T) {
	for _, e := range listTestEnv() {
		testLargeUnary(t, e)
	}
}

func testLargeUnary(t *testing.T, e env) {
	s, cc := setUp(math.MaxUint32, e)
	tc := testpb.NewTestServiceClient(cc)
	defer tearDown(s, cc)
	argSize := 271828
	respSize := 314159
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseSize: proto.Int32(int32(respSize)),
		Payload:      newPayload(testpb.PayloadType_COMPRESSABLE, int32(argSize)),
	}
	reply, err := tc.UnaryCall(context.Background(), req)
	if err != nil {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, <nil>", err)
	}
	pt := reply.GetPayload().GetType()
	ps := len(reply.GetPayload().GetBody())
	if pt != testpb.PayloadType_COMPRESSABLE || ps != respSize {
		t.Fatalf("Got the reply with type %d len %d; want %d, %d", pt, ps, testpb.PayloadType_COMPRESSABLE, respSize)
	}
}

func TestMetadataUnaryRPC(t *testing.T) {
	for _, e := range listTestEnv() {
		testMetadataUnaryRPC(t, e)
	}
}

func testMetadataUnaryRPC(t *testing.T, e env) {
	s, cc := setUp(math.MaxUint32, e)
	tc := testpb.NewTestServiceClient(cc)
	defer tearDown(s, cc)
	argSize := 2718
	respSize := 314
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseSize: proto.Int32(int32(respSize)),
		Payload:      newPayload(testpb.PayloadType_COMPRESSABLE, int32(argSize)),
	}
	var header, trailer metadata.MD
	ctx := metadata.NewContext(context.Background(), testMetadata)
	_, err := tc.UnaryCall(ctx, req, grpc.Header(&header), grpc.Trailer(&trailer))
	if err != nil {
		t.Fatalf("TestService.UnaryCall(%v, _, _, _) = _, %v; want _, <nil>", ctx, err)
	}
	if !reflect.DeepEqual(testMetadata, header) {
		t.Fatalf("Received header metadata %v, want %v", header, testMetadata)
	}
	if !reflect.DeepEqual(testMetadata, trailer) {
		t.Fatalf("Received trailer metadata %v, want %v", trailer, testMetadata)
	}
}

func performOneRPC(t *testing.T, tc testpb.TestServiceClient, wg *sync.WaitGroup) {
	argSize := 2718
	respSize := 314
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseSize: proto.Int32(int32(respSize)),
		Payload:      newPayload(testpb.PayloadType_COMPRESSABLE, int32(argSize)),
	}
	reply, err := tc.UnaryCall(context.Background(), req)
	if err != nil {
		t.Fatalf("TestService/UnaryCall(_, _) = _, %v, want _, <nil>", err)
	}
	pt := reply.GetPayload().GetType()
	ps := len(reply.GetPayload().GetBody())
	if pt != testpb.PayloadType_COMPRESSABLE || ps != respSize {
		t.Fatalf("Got the reply with type %d len %d; want %d, %d", pt, ps, testpb.PayloadType_COMPRESSABLE, respSize)
	}
	wg.Done()
}

func TestRetry(t *testing.T) {
	for _, e := range listTestEnv() {
		testRetry(t, e)
	}
}

// This test mimics a user who sends 1000 RPCs concurrently on a faulty transport.
// TODO(zhaoq): Refactor to make this clearer and add more cases to test racy
// and error-prone paths.
func testRetry(t *testing.T, e env) {
	s, cc := setUp(math.MaxUint32, e)
	tc := testpb.NewTestServiceClient(cc)
	defer tearDown(s, cc)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		time.Sleep(1 * time.Second)
		// The server shuts down the network connection to make a
		// transport error which will be detected by the client side
		// code.
		s.TestingCloseConns()
		wg.Done()
	}()
	// All these RPCs should succeed eventually.
	for i := 0; i < 1000; i++ {
		time.Sleep(2 * time.Millisecond)
		wg.Add(1)
		go performOneRPC(t, tc, &wg)
	}
	wg.Wait()
}

func TestRPCTimeout(t *testing.T) {
	for _, e := range listTestEnv() {
		testRPCTimeout(t, e)
	}
}

// TODO(zhaoq): Have a better test coverage of timeout and cancellation mechanism.
func testRPCTimeout(t *testing.T, e env) {
	s, cc := setUp(math.MaxUint32, e)
	tc := testpb.NewTestServiceClient(cc)
	defer tearDown(s, cc)
	argSize := 2718
	respSize := 314
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseSize: proto.Int32(int32(respSize)),
		Payload:      newPayload(testpb.PayloadType_COMPRESSABLE, int32(argSize)),
	}
	// Performs 100 RPCs with various timeout values so that
	// the RPCs could timeout on different stages of their lifetime. This
	// is the best-effort to cover various cases when an rpc gets cancelled.
	for i := 1; i <= 100; i++ {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(i)*time.Microsecond)
		reply, err := tc.UnaryCall(ctx, req)
		if grpc.Code(err) != codes.DeadlineExceeded {
			t.Fatalf(`TestService/UnaryCallv(_, _) = %v, %v; want <nil>, error code: %d`, reply, err, codes.DeadlineExceeded)
		}
	}
}

func TestCancel(t *testing.T) {
	for _, e := range listTestEnv() {
		testCancel(t, e)
	}
}

func testCancel(t *testing.T, e env) {
	s, cc := setUp(math.MaxUint32, e)
	tc := testpb.NewTestServiceClient(cc)
	defer tearDown(s, cc)
	argSize := 2718
	respSize := 314
	req := &testpb.SimpleRequest{
		ResponseType: testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseSize: proto.Int32(int32(respSize)),
		Payload:      newPayload(testpb.PayloadType_COMPRESSABLE, int32(argSize)),
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(1*time.Millisecond, cancel)
	reply, err := tc.UnaryCall(ctx, req)
	if grpc.Code(err) != codes.Canceled {
		t.Fatalf(`TestService/UnaryCall(_, _) = %v, %v; want <nil>, error code: %d`, reply, err, codes.Canceled)
	}
}

// The following tests the gRPC streaming RPC implementations.
// TODO(zhaoq): Have better coverage on error cases.
var (
	reqSizes  = []int{27182, 8, 1828, 45904}
	respSizes = []int{31415, 9, 2653, 58979}
)

func TestPingPong(t *testing.T) {
	for _, e := range listTestEnv() {
		testPingPong(t, e)
	}
}

func testPingPong(t *testing.T, e env) {
	s, cc := setUp(math.MaxUint32, e)
	tc := testpb.NewTestServiceClient(cc)
	defer tearDown(s, cc)
	stream, err := tc.FullDuplexCall(context.Background())
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	var index int
	for index < len(reqSizes) {
		respParam := []*testpb.ResponseParameters{
			{
				Size: proto.Int32(int32(respSizes[index])),
			},
		}
		req := &testpb.StreamingOutputCallRequest{
			ResponseType:       testpb.PayloadType_COMPRESSABLE.Enum(),
			ResponseParameters: respParam,
			Payload:            newPayload(testpb.PayloadType_COMPRESSABLE, int32(reqSizes[index])),
		}
		if err := stream.Send(req); err != nil {
			t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, req, err)
		}
		reply, err := stream.Recv()
		if err != nil {
			t.Fatalf("%v.Recv() = %v, want <nil>", stream, err)
		}
		pt := reply.GetPayload().GetType()
		if pt != testpb.PayloadType_COMPRESSABLE {
			t.Fatalf("Got the reply of type %d, want %d", pt, testpb.PayloadType_COMPRESSABLE)
		}
		size := len(reply.GetPayload().GetBody())
		if size != int(respSizes[index]) {
			t.Fatalf("Got reply body of length %d, want %d", size, respSizes[index])
		}
		index++
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("%v.CloseSend() got %v, want %v", stream, err, nil)
	}
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("%v failed to complele the ping pong test: %v", stream, err)
	}
}

func TestMetadataStreamingRPC(t *testing.T) {
	for _, e := range listTestEnv() {
		testMetadataStreamingRPC(t, e)
	}
}

func testMetadataStreamingRPC(t *testing.T, e env) {
	s, cc := setUp(math.MaxUint32, e)
	tc := testpb.NewTestServiceClient(cc)
	defer tearDown(s, cc)
	ctx := metadata.NewContext(context.Background(), testMetadata)
	stream, err := tc.FullDuplexCall(ctx)
	if err != nil {
		t.Fatalf("%v.FullDuplexCall(_) = _, %v, want <nil>", tc, err)
	}
	go func() {
		headerMD, err := stream.Header()
		if err != nil || !reflect.DeepEqual(testMetadata, headerMD) {
			t.Errorf("#1 %v.Header() = %v, %v, want %v, <nil>", stream, headerMD, err, testMetadata)
		}
		// test the cached value.
		headerMD, err = stream.Header()
		if err != nil || !reflect.DeepEqual(testMetadata, headerMD) {
			t.Errorf("#2 %v.Header() = %v, %v, want %v, <nil>", stream, headerMD, err, testMetadata)
		}
		var index int
		for index < len(reqSizes) {
			respParam := []*testpb.ResponseParameters{
				{
					Size: proto.Int32(int32(respSizes[index])),
				},
			}
			req := &testpb.StreamingOutputCallRequest{
				ResponseType:       testpb.PayloadType_COMPRESSABLE.Enum(),
				ResponseParameters: respParam,
				Payload:            newPayload(testpb.PayloadType_COMPRESSABLE, int32(reqSizes[index])),
			}
			if err := stream.Send(req); err != nil {
				t.Errorf("%v.Send(%v) = %v, want <nil>", stream, req, err)
				return
			}
			index++
		}
		// Tell the server we're done sending args.
		stream.CloseSend()
	}()
	for {
		if _, err := stream.Recv(); err != nil {
			break
		}
	}
	trailerMD := stream.Trailer()
	if !reflect.DeepEqual(testMetadata, trailerMD) {
		t.Fatalf("%v.Trailer() = %v, want %v", stream, trailerMD, testMetadata)
	}
}

func TestServerStreaming(t *testing.T) {
	for _, e := range listTestEnv() {
		testServerStreaming(t, e)
	}
}

func testServerStreaming(t *testing.T, e env) {
	s, cc := setUp(math.MaxUint32, e)
	tc := testpb.NewTestServiceClient(cc)
	defer tearDown(s, cc)
	respParam := make([]*testpb.ResponseParameters, len(respSizes))
	for i, s := range respSizes {
		respParam[i] = &testpb.ResponseParameters{
			Size: proto.Int32(int32(s)),
		}
	}
	req := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseParameters: respParam,
	}
	stream, err := tc.StreamingOutputCall(context.Background(), req)
	if err != nil {
		t.Fatalf("%v.StreamingOutputCall(_) = _, %v, want <nil>", tc, err)
	}
	var rpcStatus error
	var respCnt int
	var index int
	for {
		reply, err := stream.Recv()
		if err != nil {
			rpcStatus = err
			break
		}
		pt := reply.GetPayload().GetType()
		if pt != testpb.PayloadType_COMPRESSABLE {
			t.Fatalf("Got the reply of type %d, want %d", pt, testpb.PayloadType_COMPRESSABLE)
		}
		size := len(reply.GetPayload().GetBody())
		if size != int(respSizes[index]) {
			t.Fatalf("Got reply body of length %d, want %d", size, respSizes[index])
		}
		index++
		respCnt++
	}
	if rpcStatus != io.EOF {
		t.Fatalf("Failed to finish the server streaming rpc: %v, want <EOF>", err)
	}
	if respCnt != len(respSizes) {
		t.Fatalf("Got %d reply, want %d", len(respSizes), respCnt)
	}
}

func TestFailedServerStreaming(t *testing.T) {
	for _, e := range listTestEnv() {
		testFailedServerStreaming(t, e)
	}
}

func testFailedServerStreaming(t *testing.T, e env) {
	s, cc := setUp(math.MaxUint32, e)
	tc := testpb.NewTestServiceClient(cc)
	defer tearDown(s, cc)
	respParam := make([]*testpb.ResponseParameters, len(respSizes))
	for i, s := range respSizes {
		respParam[i] = &testpb.ResponseParameters{
			Size: proto.Int32(int32(s)),
		}
	}
	req := &testpb.StreamingOutputCallRequest{
		ResponseType:       testpb.PayloadType_COMPRESSABLE.Enum(),
		ResponseParameters: respParam,
	}
	ctx := metadata.NewContext(context.Background(), testMetadata)
	stream, err := tc.StreamingOutputCall(ctx, req)
	if err != nil {
		t.Fatalf("%v.StreamingOutputCall(_) = _, %v, want <nil>", tc, err)
	}
	if _, err := stream.Recv(); err != grpc.Errorf(codes.DataLoss, "got extra metadata") {
		t.Fatalf("%v.Recv() = _, %v, want _, %v", stream, err, grpc.Errorf(codes.DataLoss, "got extra metadata"))
	}
}

func TestClientStreaming(t *testing.T) {
	for _, e := range listTestEnv() {
		testClientStreaming(t, e)
	}
}

func testClientStreaming(t *testing.T, e env) {
	s, cc := setUp(math.MaxUint32, e)
	tc := testpb.NewTestServiceClient(cc)
	defer tearDown(s, cc)
	stream, err := tc.StreamingInputCall(context.Background())
	if err != nil {
		t.Fatalf("%v.StreamingInputCall(_) = _, %v, want <nil>", tc, err)
	}
	var sum int
	for _, s := range reqSizes {
		pl := newPayload(testpb.PayloadType_COMPRESSABLE, int32(s))
		req := &testpb.StreamingInputCallRequest{
			Payload: pl,
		}
		if err := stream.Send(req); err != nil {
			t.Fatalf("%v.Send(%v) = %v, want <nil>", stream, req, err)
		}
		sum += s
	}
	reply, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("%v.CloseAndRecv() got error %v, want %v", stream, err, nil)
	}
	if reply.GetAggregatedPayloadSize() != int32(sum) {
		t.Fatalf("%v.CloseAndRecv().GetAggregatePayloadSize() = %v; want %v", stream, reply.GetAggregatedPayloadSize(), sum)
	}
}

func TestExceedMaxStreamsLimit(t *testing.T) {
	for _, e := range listTestEnv() {
		testExceedMaxStreamsLimit(t, e)
	}
}

func testExceedMaxStreamsLimit(t *testing.T, e env) {
	// Only allows 1 live stream per server transport.
	s, cc := setUp(1, e)
	tc := testpb.NewTestServiceClient(cc)
	defer tearDown(s, cc)
	var err error
	for {
		time.Sleep(2 * time.Millisecond)
		_, err = tc.StreamingInputCall(context.Background())
		// Loop until the settings of max concurrent streams is
		// received by the client.
		if err != nil {
			break
		}
	}
	if grpc.Code(err) != codes.Unavailable {
		t.Fatalf("got %v, want error code %d", err, codes.Unavailable)
	}
}
