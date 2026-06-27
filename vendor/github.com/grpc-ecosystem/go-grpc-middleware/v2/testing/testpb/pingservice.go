// Copyright (c) The go-grpc-middleware Authors.
// Licensed under the Apache License 2.0.

/*
Package `grpc_testing` provides helper functions for testing validators in this package.
*/

package testpb

import (
	"context"
	"errors"
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// ListResponseCount is the expected number of responses to PingList
	ListResponseCount = 100
)

var TestServiceFullName = TestService_ServiceDesc.ServiceName

// Interface implementation assert.
var _ TestServiceServer = &TestPingService{}

type TestPingService struct {
	UnimplementedTestServiceServer
	PingFunc func(ctx context.Context)
}

func (s *TestPingService) PingEmpty(_ context.Context, _ *PingEmptyRequest) (*PingEmptyResponse, error) {
	return &PingEmptyResponse{}, nil
}

func (s *TestPingService) Ping(ctx context.Context, ping *PingRequest) (*PingResponse, error) {
	if s.PingFunc != nil {
		s.PingFunc(ctx)
	}
	// Modify the ctx value to verify the logger sees the value updated from the initial value
	n := ExtractCtxTestNumber(ctx)
	if n != nil {
		*n = 42
	}
	// Send user trailers and headers.
	return &PingResponse{Value: ping.Value, Counter: 0}, nil
}

func (s *TestPingService) PingError(_ context.Context, ping *PingErrorRequest) (*PingErrorResponse, error) {
	code := codes.Code(ping.ErrorCodeReturned)
	return nil, WrapFieldsInError(status.Error(code, "Userspace error"), []any{"error-field", "plop"})
}

func (s *TestPingService) PingList(ping *PingListRequest, stream TestService_PingListServer) error {
	if ping.ErrorCodeReturned != 0 {
		return status.Error(codes.Code(ping.ErrorCodeReturned), "foobar")
	}

	// Send user trailers and headers.
	for i := 0; i < ListResponseCount; i++ {
		if err := stream.Send(&PingListResponse{Value: ping.Value, Counter: int32(i)}); err != nil {
			return err
		}
	}
	return nil
}

func (s *TestPingService) PingStream(stream TestService_PingStreamServer) error {
	count := 0
	for {
		ping, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if err := stream.Send(&PingStreamResponse{Value: ping.Value, Counter: int32(count)}); err != nil {
			return err
		}

		count += 1
	}
	return nil
}

func (s *TestPingService) PingClientStream(stream TestService_PingClientStreamServer) error {
	count := 0
	for {
		_, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		count += 1
	}
	return stream.SendAndClose(&PingClientStreamResponse{Counter: int32(count)})
}
