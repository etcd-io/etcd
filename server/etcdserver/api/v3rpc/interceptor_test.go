package v3rpc

import (
	"context"
	"github.com/stretchr/testify/assert"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"net"
	"testing"
	"time"
)

func buildMockUnaryHandler(t *testing.T, mockResp interface{}, handlerLatency time.Duration) grpc.UnaryHandler {
	t.Helper()
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		// Add latency to mock handler.
		time.Sleep(handlerLatency)
		return mockResp, nil
	}
}

func TestLogUnaryInterceptor(t *testing.T) {
	// Warn on request latency if > 5ms.
	handlerWarnLatencyThreshold := time.Millisecond * 5

	unaryServerInfo := &grpc.UnaryServerInfo{
		FullMethod: "/foo/bar",
	}

	address := "10.0.0.1:37928"
	addr, err := net.ResolveTCPAddr("tcp", address)
	assert.NoError(t, err)
	p := &peer.Peer{
		Addr: addr,
	}

	testcases := []struct {
		name                 string
		req                  interface{}
		resp                 interface{}
		reqLatency           time.Duration
		debugLogLevel        bool // debugLogLevel indicates whether log level is debug.
		expectedRequestStats *requestStats
	}{
		{"sample transaction with successful compare",
			&pb.TxnRequest{
				Compare: []*pb.Compare{{
					Key:         []byte("/users/12345/email"),
					Result:      pb.Compare_EQUAL,
					Target:      pb.Compare_VALUE,
					TargetUnion: &pb.Compare_Value{Value: []byte("old.address@johndoe.com")},
				}},
				Success: []*pb.RequestOp{{
					Request: &pb.RequestOp_RequestPut{
						RequestPut: &pb.PutRequest{
							Key:   []byte("/users/12345/email"),
							Value: []byte("new.address@johndoe.com"),
						},
					},
				}},
			},
			&pb.TxnResponse{
				Succeeded: true,
				Responses: []*pb.ResponseOp{{Response: &pb.ResponseOp_ResponsePut{}}},
			},
			0, true,
			&requestStats{
				reqCount:   1,
				reqSize:    47,
				respCount:  0,
				respSize:   4,
				reqContent: "compare:<target:VALUE key:\"/users/12345/email\" value_size:23 > success:<request_put:<key:\"/users/12345/email\" value_size:23 >> failure:<>",
			},
		},
		{"sample transaction with failed compare",
			&pb.TxnRequest{
				Compare: []*pb.Compare{{
					Key:         []byte("/users/12345/email"),
					Result:      pb.Compare_EQUAL,
					Target:      pb.Compare_VALUE,
					TargetUnion: &pb.Compare_Value{Value: []byte("old.address@johndoe.com")},
				}},
				Failure: []*pb.RequestOp{{
					Request: &pb.RequestOp_RequestRange{
						RequestRange: &pb.RangeRequest{
							Key: []byte("/users/12345/email"),
						},
					}},
				},
			},
			&pb.TxnResponse{
				Succeeded: false,
				Responses: []*pb.ResponseOp{{Response: &pb.ResponseOp_ResponsePut{}}},
			},
			0, true,
			&requestStats{
				reqCount:   1,
				reqSize:    22,
				respCount:  0,
				respSize:   2,
				reqContent: "compare:<target:VALUE key:\"/users/12345/email\" value_size:23 > success:<> failure:<request_range:<key:\"/users/12345/email\" > >",
			},
		},
		{"expensive request with debug logs disabled", &pb.RangeRequest{Key: []byte("fooKey")}, &pb.RangeResponse{Count: 2},
			time.Millisecond * 10, false,
			&requestStats{
				reqCount:   0,
				reqSize:    8,
				respCount:  2,
				respSize:   2,
				reqContent: "key:\"fooKey\" ",
			},
		},
		// Unrecognized response types result in -1 values.
		{"default request stats -1", "fooRequest", "fooResponse", 0, true,
			&requestStats{
				reqCount:   -1,
				reqSize:    -1,
				respCount:  -1,
				respSize:   -1,
				reqContent: "",
			}},
		// Low-latency handler without debug-level logging enabled generates no request stat logs.
		{"no debug or warn level logs", "fooRequest", "fooResponse", 0, false, nil},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			expensiveUnaryHandler := buildMockUnaryHandler(t, tc.resp, tc.reqLatency)
			logLevel := zapcore.InfoLevel
			if tc.debugLogLevel {
				logLevel = zapcore.DebugLevel
			}

			observedZapCore, observedLogs := observer.New(logLevel)
			observedLogger := zap.New(observedZapCore)

			interceptor := newLogUnaryInterceptor(observedLogger, handlerWarnLatencyThreshold)

			ctx := peer.NewContext(context.TODO(), p)
			_, err := interceptor(ctx, tc.req, unaryServerInfo, expensiveUnaryHandler)
			assert.NoError(t, err)

			// Filter for request stats log messages.
			rsLogs := observedLogs.FilterMessage("request stats")

			// No request stats if log-level is not debug or warn latency threshold is not exceeded.
			if !(tc.debugLogLevel || tc.reqLatency > handlerWarnLatencyThreshold) {
				assert.Equal(t, 0, rsLogs.Len())
			} else {
				assert.Equal(t, 1, rsLogs.Len())
				le := rsLogs.All()[0]
				assert.Equal(t, 1, len(le.Context))
				fld := le.Context[0]
				rs, ok := fld.Interface.(requestStats)
				assert.True(t, ok)
				assert.Equal(t, tc.expectedRequestStats.reqCount, rs.reqCount)
				assert.Equal(t, tc.expectedRequestStats.reqSize, rs.reqSize)
				assert.Equal(t, tc.expectedRequestStats.respCount, rs.respCount)
				assert.Equal(t, tc.expectedRequestStats.respSize, rs.respSize)
				assert.Equal(t, tc.expectedRequestStats.reqContent, rs.reqContent)
				assert.Equal(t, unaryServerInfo.FullMethod, rs.responseType)
				// Check peer info read from context.
				assert.Equal(t, address, rs.remote)
				if tc.debugLogLevel {
					assert.Equal(t, zapcore.DebugLevel, le.Entry.Level)
				} else {
					// Expensive request produce warn-level log if debug-level logs are disabled.
					assert.Equal(t, zapcore.WarnLevel, le.Entry.Level)
				}
			}
		})
	}
}
