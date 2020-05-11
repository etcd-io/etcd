// Copyright 2017 Michal Witkowski. All Rights Reserved.
// See LICENSE for licensing terms.

package grpc_opentracing_test

import (
	"encoding/json"
	"testing"

	"fmt"
	"net/http"

	"io"

	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_testing "github.com/grpc-ecosystem/go-grpc-middleware/testing"
	pb_testproto "github.com/grpc-ecosystem/go-grpc-middleware/testing/testproto"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

var (
	goodPing           = &pb_testproto.PingRequest{Value: "something", SleepTimeMs: 9999}
	fakeInboundTraceId = 1337
	fakeInboundSpanId  = 999
)

func tagsToJson(value map[string]interface{}) string {
	str, _ := json.Marshal(value)
	return string(str)
}

func tagsFromJson(t *testing.T, jstring string) map[string]interface{} {
	var msgMapTemplate interface{}
	err := json.Unmarshal([]byte(jstring), &msgMapTemplate)
	if err != nil {
		t.Fatalf("failed unmarshaling tags from response %v", err)
	}
	return msgMapTemplate.(map[string]interface{})
}

type tracingAssertService struct {
	pb_testproto.TestServiceServer
	T *testing.T
}

func (s *tracingAssertService) Ping(ctx context.Context, ping *pb_testproto.PingRequest) (*pb_testproto.PingResponse, error) {
	assert.NotNil(s.T, opentracing.SpanFromContext(ctx), "handlers must have the spancontext in their context, otherwise propagation will fail")
	tags := grpc_ctxtags.Extract(ctx)
	assert.True(s.T, tags.Has(grpc_opentracing.TagTraceId), "tags must contain traceid")
	assert.True(s.T, tags.Has(grpc_opentracing.TagSpanId), "tags must contain traceid")
	return s.TestServiceServer.Ping(ctx, ping)
}

func (s *tracingAssertService) PingError(ctx context.Context, ping *pb_testproto.PingRequest) (*pb_testproto.Empty, error) {
	assert.NotNil(s.T, opentracing.SpanFromContext(ctx), "handlers must have the spancontext in their context, otherwise propagation will fail")
	return s.TestServiceServer.PingError(ctx, ping)
}

func (s *tracingAssertService) PingList(ping *pb_testproto.PingRequest, stream pb_testproto.TestService_PingListServer) error {
	assert.NotNil(s.T, opentracing.SpanFromContext(stream.Context()), "handlers must have the spancontext in their context, otherwise propagation will fail")
	tags := grpc_ctxtags.Extract(stream.Context())
	assert.True(s.T, tags.Has(grpc_opentracing.TagTraceId), "tags must contain traceid")
	assert.True(s.T, tags.Has(grpc_opentracing.TagSpanId), "tags must contain traceid")
	return s.TestServiceServer.PingList(ping, stream)
}

func (s *tracingAssertService) PingEmpty(ctx context.Context, empty *pb_testproto.Empty) (*pb_testproto.PingResponse, error) {
	assert.NotNil(s.T, opentracing.SpanFromContext(ctx), "handlers must have the spancontext in their context, otherwise propagation will fail")
	return s.TestServiceServer.PingEmpty(ctx, empty)
}

func TestTaggingSuite(t *testing.T) {
	mockTracer := mocktracer.New()
	opts := []grpc_opentracing.Option{
		grpc_opentracing.WithTracer(mockTracer),
	}
	s := &OpentracingSuite{
		mockTracer: mockTracer,
		InterceptorTestSuite: &grpc_testing.InterceptorTestSuite{
			TestService: &tracingAssertService{TestServiceServer: &grpc_testing.TestPingService{T: t}, T: t},
			ClientOpts: []grpc.DialOption{
				grpc.WithUnaryInterceptor(grpc_opentracing.UnaryClientInterceptor(opts...)),
				grpc.WithStreamInterceptor(grpc_opentracing.StreamClientInterceptor(opts...)),
			},
			ServerOpts: []grpc.ServerOption{
				grpc_middleware.WithStreamServerChain(
					grpc_ctxtags.StreamServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
					grpc_opentracing.StreamServerInterceptor(opts...)),
				grpc_middleware.WithUnaryServerChain(
					grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
					grpc_opentracing.UnaryServerInterceptor(opts...)),
			},
		},
	}
	suite.Run(t, s)
}

type OpentracingSuite struct {
	*grpc_testing.InterceptorTestSuite
	mockTracer *mocktracer.MockTracer
}

func (s *OpentracingSuite) SetupTest() {
	s.mockTracer.Reset()
}

func (s *OpentracingSuite) createContextFromFakeHttpRequestParent(ctx context.Context) context.Context {
	hdr := http.Header{}
	hdr.Set("mockpfx-ids-traceid", fmt.Sprint(fakeInboundTraceId))
	hdr.Set("mockpfx-ids-spanid", fmt.Sprint(fakeInboundSpanId))
	hdr.Set("mockpfx-ids-sampled", fmt.Sprint(true))
	parentSpanContext, err := s.mockTracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(hdr))
	require.NoError(s.T(), err, "parsing a fake HTTP request headers shouldn't fail, ever")
	fakeSpan := s.mockTracer.StartSpan(
		"/fake/parent/http/request",
		// this is magical, it attaches the new span to the parent parentSpanContext, and creates an unparented one if empty.
		opentracing.ChildOf(parentSpanContext),
	)
	fakeSpan.Finish()
	return opentracing.ContextWithSpan(ctx, fakeSpan)
}

func (s *OpentracingSuite) assertTracesCreated(methodName string) (clientSpan *mocktracer.MockSpan, serverSpan *mocktracer.MockSpan) {
	spans := s.mockTracer.FinishedSpans()
	for _, span := range spans {
		s.T().Logf("span: %v, tags: %v", span, span.Tags())
	}
	require.Len(s.T(), spans, 3, "should record 3 spans: one fake inbound, one client, one server")
	traceIdAssert := fmt.Sprintf("traceId=%d", fakeInboundTraceId)
	for _, span := range spans {
		assert.Contains(s.T(), span.String(), traceIdAssert, "not part of the fake parent trace: %v", span)
		if span.OperationName == methodName {
			kind := fmt.Sprintf("%v", span.Tag("span.kind"))
			if kind == "client" {
				clientSpan = span
			} else if kind == "server" {
				serverSpan = span
			}
			assert.EqualValues(s.T(), span.Tag("component"), "gRPC", "span must be tagged with gRPC component")
		}
	}
	require.NotNil(s.T(), clientSpan, "client span must be there")
	require.NotNil(s.T(), serverSpan, "server span must be there")
	assert.EqualValues(s.T(), serverSpan.Tag("grpc.request.value"), "something", "grpc_ctxtags must be propagated, in this case ones from request fields")
	return clientSpan, serverSpan
}

func (s *OpentracingSuite) TestPing_PropagatesTraces() {
	ctx := s.createContextFromFakeHttpRequestParent(s.SimpleCtx())
	_, err := s.Client.Ping(ctx, goodPing)
	require.NoError(s.T(), err, "there must be not be an on a successful call")
	s.assertTracesCreated("/mwitkow.testproto.TestService/Ping")
}

func (s *OpentracingSuite) TestPing_ClientContextTags() {
	const name = "opentracing.custom"
	ctx := grpc_opentracing.ClientAddContextTags(
		s.createContextFromFakeHttpRequestParent(s.SimpleCtx()),
		opentracing.Tags{name: ""},
	)

	_, err := s.Client.Ping(ctx, goodPing)
	require.NoError(s.T(), err, "there must be not be an on a successful call")

	for _, span := range s.mockTracer.FinishedSpans() {
		if span.OperationName == "/mwitkow.testproto.TestService/Ping" {
			kind := fmt.Sprintf("%v", span.Tag("span.kind"))
			if kind == "client" {
				assert.Contains(s.T(), span.Tags(), name, "custom opentracing.Tags must be included in context")
			}
		}
	}
}

func (s *OpentracingSuite) TestPingList_PropagatesTraces() {
	ctx := s.createContextFromFakeHttpRequestParent(s.SimpleCtx())
	stream, err := s.Client.PingList(ctx, goodPing)
	require.NoError(s.T(), err, "should not fail on establishing the stream")
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(s.T(), err, "reading stream should not fail")
	}
	s.assertTracesCreated("/mwitkow.testproto.TestService/PingList")
}

func (s *OpentracingSuite) TestPingError_PropagatesTraces() {
	ctx := s.createContextFromFakeHttpRequestParent(s.SimpleCtx())
	erroringPing := &pb_testproto.PingRequest{Value: "something", ErrorCodeReturned: uint32(codes.OutOfRange)}
	_, err := s.Client.PingError(ctx, erroringPing)
	require.Error(s.T(), err, "there must be an error returned here")
	clientSpan, serverSpan := s.assertTracesCreated("/mwitkow.testproto.TestService/PingError")
	assert.Equal(s.T(), true, clientSpan.Tag("error"), "client span needs to be marked as an error")
	assert.Equal(s.T(), true, serverSpan.Tag("error"), "server span needs to be marked as an error")
}
