// Copyright 2025 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package etcdmain

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/testing/testpb"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
)

type loggingPayloadSuite struct {
	*testpb.InterceptorTestSuite
	logger *zap.Logger
	logs   *observer.ObservedLogs
}

func TestLoggingPayloadSuite(t *testing.T) {
	observer, logs := observer.New(zap.InfoLevel)
	logger := zap.New(observer)
	s := &loggingPayloadSuite{
		InterceptorTestSuite: &testpb.InterceptorTestSuite{
			TestService: &testpb.TestPingService{},
			ServerOpts: []grpc.ServerOption{
				grpc.UnaryInterceptor(interceptors.UnaryServerInterceptor(reportable(logger))),
				grpc.StreamInterceptor(interceptors.StreamServerInterceptor(reportable(logger))),
			},
		},
		logs:   logs,
		logger: zap.New(observer),
	}
	suite.Run(t, s)
}

func (s *loggingPayloadSuite) SetupTest() {
	s.logs.TakeAll() // clear logs
	s.Require().Empty(s.logs.TakeAll())
}

func (s *loggingPayloadSuite) TestPing_LogsBothRequestAndResponse() {
	_, err := s.Client.Ping(s.SimpleCtx(), testpb.GoodPing)
	s.Require().NoError(err)
	s.Require().Len(s.logs.All(), 2) // request and response
	s.assertField("grpc.request.content", `{"value":"something","sleepTimeMs":9999}`, 1)
	s.assertField("grpc.response.content", `{"value":"something"}`, 1)
}

func (s *loggingPayloadSuite) TestPingError_LogsError() {
	_, err := s.Client.PingError(s.SimpleCtx(), &testpb.PingErrorRequest{Value: "something", ErrorCodeReturned: uint32(4)})
	s.Require().Error(err)
	s.Require().Len(s.logs.All(), 2) // request and response
	s.assertField("grpc.request.content", `{"value":"something","errorCodeReturned":4}`, 1)
	s.assertField("grpc.response.content", ``, 1)
	s.assertField("grpc.response.error", "rpc error: code = DeadlineExceeded desc = Userspace error", 1)
}

func (s *loggingPayloadSuite) TestPingStream_LogsAllRequestsAndResponses() {
	messagesExpected := 10
	stream, err := s.Client.PingStream(s.SimpleCtx())
	s.Require().NoError(err)

	for range messagesExpected {
		s.Require().NoError(stream.Send(testpb.GoodPingStream))
		pong := &testpb.PingResponse{}
		err := stream.RecvMsg(pong)
		s.Require().NoError(err)
	}
	s.Require().NoError(stream.CloseSend())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s.Require().NoError(waitUntil(200*time.Millisecond, ctx.Done(), func() error {
		if len(s.logs.FilterFieldKey("grpc.request.error").All()) > 0 {
			return nil
		}
		return fmt.Errorf("no EOF log yet")
	}))
	eof := s.logs.FilterFieldKey("grpc.request.error").All()
	s.Len(eof, 1)
	s.Equal(io.EOF.Error(), eof[0].ContextMap()["grpc.request.error"])
	s.assertField("grpc.request.content", `{"value":"something","sleepTimeMs":9999}`, messagesExpected+1)
	s.assertField("grpc.response.content", `{"value":"something"}`, messagesExpected)
}

func (s *loggingPayloadSuite) assertField(key, expectedValue string, expectedLineCount int) {
	s.T().Helper()
	filtered := s.logs.FilterFieldKey(key).All()
	s.Require().Len(filtered, expectedLineCount)
	actualValue, ok := filtered[0].ContextMap()[key].(string)
	s.Require().True(ok)
	s.Equal(expectedValue, actualValue)
}

// waitUntil executes f every interval seconds until timeout or no error is returned from f.
func waitUntil(interval time.Duration, stopc <-chan struct{}, f func() error) error {
	tick := time.NewTicker(interval)
	defer tick.Stop()

	var err error
	for {
		if err = f(); err == nil {
			return nil
		}
		select {
		case <-stopc:
			return err
		case <-tick.C:
		}
	}
}
