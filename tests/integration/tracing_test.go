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
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	traceservice "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/testing/protocmp"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

// TestTracing ensures that distributed tracing is setup when the feature flag is enabled.
func TestTracing(t *testing.T) {
	testutil.SkipTestIfShortMode(t,
		"Wal creation tests are depending on embedded etcd server so are integration-level tests.")

	for _, tc := range []struct {
		name     string
		rpc      func(context.Context, *clientv3.Client) error
		wantSpan *v1.Span
	}{
		{
			name: "UnaryGet",
			rpc: func(ctx context.Context, cli *clientv3.Client) error {
				_, err := cli.Get(ctx, "key")
				return err
			},
			wantSpan: &v1.Span{
				Name: "etcdserverpb.KV/Range",
				// Attributes are set outside Etcd in otelgrpc, so they are ignored here.
			},
		},
		{
			name: "UnaryGetWithCountOnly",
			rpc: func(ctx context.Context, cli *clientv3.Client) error {
				_, err := cli.Get(ctx, "key", clientv3.WithCountOnly())
				return err
			},
			wantSpan: &v1.Span{
				Name: "range",
				Attributes: []*commonv1.KeyValue{
					{
						Key:   "range_begin",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: "key"}},
					},
					{
						Key:   "range_end",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: ""}},
					},
					{
						Key:   "rev",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: 0}},
					},
					{
						Key:   "limit",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: 0}},
					},
					{
						Key:   "count_only",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_BoolValue{BoolValue: true}},
					},
					{
						Key:   "keys_only",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_BoolValue{BoolValue: false}},
					},
				},
			},
		},
		{
			name: "UnaryTxn",
			rpc: func(ctx context.Context, cli *clientv3.Client) error {
				_, err := cli.Txn(ctx).
					If(clientv3.Compare(clientv3.ModRevision("cmp_key"), "=", 1)).
					Then(clientv3.OpPut("op_key", "val", clientv3.WithLease(1234)), clientv3.OpGet("other_key")).
					Commit()
				return err
			},
			wantSpan: &v1.Span{
				Name: "txn",
				Attributes: []*commonv1.KeyValue{
					{
						Key:   "compare_first_key",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: "cmp_key"}},
					},
					{
						Key:   "success_first_key",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: "op_key"}},
					},
					{
						Key:   "success_first_type",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: "put"}},
					},
					{
						Key:   "success_first_lease",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: 1234}},
					},
					{
						Key:   "compare_len",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: 1}},
					},
					{
						Key:   "success_len",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: 2}},
					},
					{
						Key:   "failure_len",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: 0}},
					},
					{
						Key:   "read_only",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_BoolValue{BoolValue: false}},
					},
				},
			},
		},
		{
			name: "UnaryLeaseGrant",
			rpc: func(ctx context.Context, cli *clientv3.Client) error {
				_, err := cli.Grant(ctx, 1_000_123)
				return err
			},
			wantSpan: &v1.Span{
				Name: "lease_grant",
				Attributes: []*commonv1.KeyValue{
					{
						Key:   "id",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: 0}},
					},
					{
						Key:   "ttl",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: 1_000_123}},
					},
				},
			},
		},
		{
			name: "UnaryLeaseRenew",
			rpc: func(ctx context.Context, cli *clientv3.Client) error {
				_, err := cli.KeepAliveOnce(ctx, 2345)
				if err != nil && strings.Contains(err.Error(), "requested lease not found") {
					// errors.Is does not work accross gRPC bounduaries.
					return nil
				}
				return err
			},
			wantSpan: &v1.Span{
				Name: "lease_renew",
				Attributes: []*commonv1.KeyValue{
					{
						Key:   "id",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: 2345}},
					},
				},
			},
		},
		{
			name: "UnaryLeaseRevoke",
			rpc: func(ctx context.Context, cli *clientv3.Client) error {
				_, err := cli.Revoke(ctx, 1234)
				if err != nil && strings.Contains(err.Error(), "requested lease not found") {
					// errors.Is does not work accross gRPC bounduaries.
					return nil
				}
				return err
			},
			wantSpan: &v1.Span{
				Name: "lease_revoke",
				Attributes: []*commonv1.KeyValue{
					{
						Key:   "id",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: 1234}},
					},
				},
			},
		},
		{
			name: "StreamWatch",
			rpc: func(ctx context.Context, cli *clientv3.Client) error {
				// Create a context with a reasonable timeout
				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				// Create a watch channel
				watchChan := cli.Watch(ctx, "watch-key", clientv3.WithProgressNotify(), clientv3.WithRev(1))

				// Put a value to trigger the watch
				_, err := cli.Put(ctx, "watch-key", "watch-value")
				if err != nil {
					return err
				}

				// Wait for watch event
				select {
				case watchResp := <-watchChan:
					return watchResp.Err()
				case <-time.After(5 * time.Second):
					return fmt.Errorf("Timed out waiting for watch event")
				}
			},
			wantSpan: &v1.Span{
				Name: "watch",
				Attributes: []*commonv1.KeyValue{
					{
						Key:   "key",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: "watch-key"}},
					},
					{
						Key:   "range_end",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: ""}},
					},
					{
						Key:   "start_rev",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: 1}},
					},
					{
						Key:   "progress_notify",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_BoolValue{BoolValue: true}},
					},
					{
						Key:   "prev_kv",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_BoolValue{BoolValue: false}},
					},
					{
						Key:   "fragment",
						Value: &commonv1.AnyValue{Value: &commonv1.AnyValue_BoolValue{BoolValue: false}},
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testRPCTracing(t, tc.wantSpan, tc.rpc)
		})
	}
}

// testRPCTracing is a common test function for both Unary and Stream RPC tracing
func testRPCTracing(t *testing.T, wantSpan *v1.Span, clientAction func(context.Context, *clientv3.Client) error) {
	// set up trace collector
	listener, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err)

	traceFound := make(chan struct{})
	defer close(traceFound)

	srv := grpc.NewServer()
	traceservice.RegisterTraceServiceServer(srv, &traceServer{
		traceFound: traceFound,
		filterFunc: func(req *traceservice.ExportTraceServiceRequest) bool {
			for _, resourceSpans := range req.GetResourceSpans() {
				// Skip spans which weren't produced by test's gRPC client.
				matched := false
				for _, attr := range resourceSpans.GetResource().GetAttributes() {
					if attr.GetKey() == "service.name" && attr.GetValue().GetStringValue() == "integration-test-tracing" {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}

				for _, scoped := range resourceSpans.GetScopeSpans() {
					for _, gotSpan := range scoped.GetSpans() {
						if gotSpan.GetName() != wantSpan.GetName() {
							continue
						}
						if len(wantSpan.GetAttributes()) == 0 {
							// Diff will compare only attributes and events when needed
							return true
						}
						if gotSpan.GetName() == "lease_grant" {
							// Ignore ID in lease grant which is not controlled by the client.
							for _, attr := range gotSpan.GetAttributes() {
								if attr.GetKey() == "id" {
									attr.Value.Value.(*commonv1.AnyValue_IntValue).IntValue = 0
								}
							}
						}
						if diff := cmp.Diff(wantSpan, gotSpan,
							protocmp.Transform(),
							protocmp.IgnoreFields(&v1.Span{}, "end_time_unix_nano", "flags", "kind", "parent_span_id", "span_id", "start_time_unix_nano", "status", "trace_id", "events"),
						); diff != "" {
							t.Errorf("Span mismatch (-want +got):\n%s", diff)
						}
						return true
					}
				}
			}
			return false
		},
	})

	go srv.Serve(listener)
	defer srv.Stop()

	cfg := integration.NewEmbedConfig(t, "default")

	cfg.EnableDistributedTracing = true
	cfg.DistributedTracingAddress = listener.Addr().String()
	cfg.DistributedTracingServiceName = "integration-test-tracing"
	cfg.DistributedTracingSamplingRatePerMillion = 100 // overriden later in the test

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
	tp := sdktrace.NewTracerProvider()
	defer tp.Shutdown(t.Context())

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
	err = clientAction(t.Context(), cli)
	require.NoError(t, err)

	// Wait for a span to be recorded from our request
	select {
	case <-traceFound:
		t.Logf("Trace found")
		return
	case <-time.After(30 * time.Second):
		// default exporter has 5s scheduling delay
		t.Fatal("Timed out waiting for trace")
	}
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
