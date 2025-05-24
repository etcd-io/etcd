// Copyright 2022 The etcd Authors
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

package integration

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	traceservice "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

// TestTracing ensures that distributed tracing is setup when the feature flag is enabled.
func TestTracing(t *testing.T) {
	testutil.SkipTestIfShortMode(t,
		"Wal creation tests are depending on embedded etcd server so are integration-level tests.")

	// Test Unary RPC tracing
	t.Run("UnaryRPC", func(t *testing.T) {
		testRPCTracing(t, "UnaryRPC", containsUnaryRPCSpan, func(cli *clientv3.Client) error {
			// make a request with the instrumented client
			resp, err := cli.Get(context.TODO(), "key")
			require.NoError(t, err)
			require.Empty(t, resp.Kvs)
			return nil
		})
	})

	// Test Stream RPC tracing
	t.Run("StreamRPC", func(t *testing.T) {
		testRPCTracing(t, "StreamRPC", containsStreamRPCSpan, func(cli *clientv3.Client) error {
			// Create a context with a reasonable timeout
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Create a watch channel
			watchChan := cli.Watch(ctx, "watch-key")

			// Put a value to trigger the watch
			_, err := cli.Put(context.TODO(), "watch-key", "watch-value")
			require.NoError(t, err)

			// Wait for watch event
			select {
			case watchResp := <-watchChan:
				require.NoError(t, watchResp.Err())
				require.Len(t, watchResp.Events, 1)
				t.Log("Received watch event successfully")
			case <-time.After(5 * time.Second):
				t.Fatal("Timed out waiting for watch event")
			}
			return nil
		})
	})
}

// testRPCTracing is a common test function for both Unary and Stream RPC tracing
func testRPCTracing(t *testing.T, testName string, filterFunc func(*traceservice.ExportTraceServiceRequest) bool, clientAction func(*clientv3.Client) error) {
	// set up trace collector
	listener, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)

	traceFound := make(chan struct{})
	defer close(traceFound)

	srv := grpc.NewServer()
	traceservice.RegisterTraceServiceServer(srv, &traceServer{
		traceFound: traceFound,
		filterFunc: filterFunc,
	})

	go srv.Serve(listener)
	defer srv.Stop()

	cfg := integration.NewEmbedConfig(t, "default")

	cfg.EnableDistributedTracing = true
	cfg.DistributedTracingAddress = listener.Addr().String()
	cfg.DistributedTracingServiceName = "integration-test-tracing"
	cfg.DistributedTracingSamplingRatePerMillion = 100

	// start an etcd instance with tracing enabled
	etcdSrv, err := embed.StartEtcd(cfg)
	require.NoError(t, err)
	defer etcdSrv.Close()

	select {
	case <-etcdSrv.Server.ReadyNotify():
	case <-time.After(5 * time.Second):
		// default randomized election timeout is 1 to 2s, single node will fast-forward 900ms
		// change the timeout from 1 to 5 seconds to ensure de-flaking this test
		t.Fatalf("failed to start embed.Etcd for test")
	}

	// create a client that has tracing enabled
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	defer tracer.Shutdown(context.TODO())
	tp := trace.TracerProvider(tracer)

	tracingOpts := []otelgrpc.Option{
		otelgrpc.WithTracerProvider(tp),
		otelgrpc.WithPropagators(
			propagation.NewCompositeTextMapPropagator(
				propagation.TraceContext{},
				propagation.Baggage{},
			)),
	}

	dialOptions := []grpc.DialOption{
		grpc.WithStatsHandler(otelgrpc.NewClientHandler(tracingOpts...)),
	}
	ccfg := clientv3.Config{DialOptions: dialOptions, Endpoints: []string{cfg.AdvertiseClientUrls[0].String()}}
	cli, err := integration.NewClient(t, ccfg)
	if err != nil {
		etcdSrv.Close()
		t.Fatal(err)
	}
	defer cli.Close()

	// Execute the client action (either Unary or Stream RPC)
	err = clientAction(cli)
	require.NoError(t, err)

	// Wait for a span to be recorded from our request
	select {
	case <-traceFound:
		t.Logf("%s trace found", testName)
		return
	case <-time.After(30 * time.Second):
		t.Fatal("Timed out waiting for trace")
	}
}

func containsUnaryRPCSpan(req *traceservice.ExportTraceServiceRequest) bool {
	for _, resourceSpans := range req.GetResourceSpans() {
		for _, attr := range resourceSpans.GetResource().GetAttributes() {
			if attr.GetKey() != "service.name" && attr.GetValue().GetStringValue() != "integration-test-tracing" {
				continue
			}
			for _, scoped := range resourceSpans.GetScopeSpans() {
				for _, span := range scoped.GetSpans() {
					if span.GetName() == "etcdserverpb.KV/Range" {
						return true
					}
				}
			}
		}
	}
	return false
}

// containsStreamRPCSpan checks for Watch/Watch spans in trace data
func containsStreamRPCSpan(req *traceservice.ExportTraceServiceRequest) bool {
	for _, resourceSpans := range req.GetResourceSpans() {
		for _, scoped := range resourceSpans.GetScopeSpans() {
			for _, span := range scoped.GetSpans() {
				if span.GetName() == "etcdserverpb.Watch/Watch" {
					return true
				}
			}
		}
	}
	return false
}

// traceServer implements TracesServiceServer
type traceServer struct {
	traceFound chan struct{}
	filterFunc func(req *traceservice.ExportTraceServiceRequest) bool
	traceservice.UnimplementedTraceServiceServer
}

func (t *traceServer) Export(ctx context.Context, req *traceservice.ExportTraceServiceRequest) (*traceservice.ExportTraceServiceResponse, error) {
	emptyValue := traceservice.ExportTraceServiceResponse{}
	if t.filterFunc(req) {
		select {
		case t.traceFound <- struct{}{}:
		default:
			// Channel already notified
		}
	}
	return &emptyValue, nil
}
