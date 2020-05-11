// Copyright 2016 Michal Witkowski. All Rights Reserved.
// See LICENSE for licensing terms.

package grpc_validator_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	"io"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	"github.com/grpc-ecosystem/go-grpc-middleware/testing"
	pb_testproto "github.com/grpc-ecosystem/go-grpc-middleware/testing/testproto"
	"github.com/grpc-ecosystem/go-grpc-middleware/validator"
)

var (
	// See test.manual_validator.pb.go for the validator check of SleepTimeMs.
	goodPing = &pb_testproto.PingRequest{Value: "something", SleepTimeMs: 9999}
	badPing  = &pb_testproto.PingRequest{Value: "something", SleepTimeMs: 10001}
)

func TestValidatorTestSuite(t *testing.T) {
	s := &ValidatorTestSuite{
		InterceptorTestSuite: &grpc_testing.InterceptorTestSuite{
			ServerOpts: []grpc.ServerOption{
				grpc.StreamInterceptor(grpc_validator.StreamServerInterceptor()),
				grpc.UnaryInterceptor(grpc_validator.UnaryServerInterceptor()),
			},
		},
	}
	suite.Run(t, s)
}

type ValidatorTestSuite struct {
	*grpc_testing.InterceptorTestSuite
}

func (s *ValidatorTestSuite) TestValidPasses_Unary() {
	_, err := s.Client.Ping(s.SimpleCtx(), goodPing)
	assert.NoError(s.T(), err, "no error expected")
}

func (s *ValidatorTestSuite) TestInvalidErrors_Unary() {
	_, err := s.Client.Ping(s.SimpleCtx(), badPing)
	assert.Error(s.T(), err, "no error expected")
	assert.Equal(s.T(), codes.InvalidArgument, grpc.Code(err), "gRPC status must be InvalidArgument")
}

func (s *ValidatorTestSuite) TestValidPasses_ServerStream() {
	stream, err := s.Client.PingList(s.SimpleCtx(), goodPing)
	require.NoError(s.T(), err, "no error on stream establishment expected")
	for true {
		_, err := stream.Recv()
		if err == io.EOF {
			break
		}
		assert.NoError(s.T(), err, "no error on messages sent occured")
	}
}

func (s *ValidatorTestSuite) TestInvalidErrors_ServerStream() {
	stream, err := s.Client.PingList(s.SimpleCtx(), badPing)
	require.NoError(s.T(), err, "no error on stream establishment expected")
	_, err = stream.Recv()
	assert.Error(s.T(), err, "error should be received on first message")
	assert.Equal(s.T(), codes.InvalidArgument, grpc.Code(err), "gRPC status must be InvalidArgument")
}

func (s *ValidatorTestSuite) TestInvalidErrors_BidiStream() {
	stream, err := s.Client.PingStream(s.SimpleCtx())
	require.NoError(s.T(), err, "no error on stream establishment expected")

	stream.Send(goodPing)
	_, err = stream.Recv()
	assert.NoError(s.T(), err, "receving a good ping should return a good pong")
	stream.Send(goodPing)
	_, err = stream.Recv()
	assert.NoError(s.T(), err, "receving a good ping should return a good pong")

	stream.Send(badPing)
	_, err = stream.Recv()
	assert.Error(s.T(), err, "receving a good ping should return a good pong")
	assert.Equal(s.T(), codes.InvalidArgument, grpc.Code(err), "gRPC status must be InvalidArgument")

	err = stream.CloseSend()
	assert.NoError(s.T(), err, "there should be no error closing the stream on send")
}
