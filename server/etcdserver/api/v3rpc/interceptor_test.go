// Copyright 2026 The etcd Authors
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

package v3rpc

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
)

func newTestLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zap.InfoLevel)
	return zap.New(core), logs
}

// findField returns the zap.Field with the given key, or false if not found.
func findField(entry observer.LoggedEntry, key string) (zap.Field, bool) {
	for _, f := range entry.Context {
		if f.Key == key {
			return f, true
		}
	}
	return zap.Field{}, false
}

// stringVal extracts a string value from a String or ByteString field.
func stringVal(f zap.Field) string {
	switch f.Type {
	case zapcore.StringType:
		return f.String
	case zapcore.ByteStringType:
		return string(f.Interface.([]byte))
	default:
		return ""
	}
}

func TestLogLightweightRequestInfo_RangeRequest(t *testing.T) {
	lg, logs := newTestLogger()
	req := &pb.RangeRequest{Key: []byte("/registry/pods"), RangeEnd: []byte("/registry/pods0")}
	logLightweightRequestInfo(lg, "/etcdserverpb.KV/Range", "10.0.0.1:34567", 5*time.Millisecond, req)

	require.Equal(t, 1, logs.Len())
	entry := logs.All()[0]
	require.Equal(t, "request info", entry.Message)
	require.Equal(t, zap.InfoLevel, entry.Level)

	f, ok := findField(entry, "method")
	require.True(t, ok)
	require.Equal(t, "/etcdserverpb.KV/Range", stringVal(f))

	f, ok = findField(entry, "remote")
	require.True(t, ok)
	require.Equal(t, "10.0.0.1:34567", stringVal(f))

	f, ok = findField(entry, "key")
	require.True(t, ok)
	require.Equal(t, "/registry/pods", stringVal(f))

	f, ok = findField(entry, "range_end")
	require.True(t, ok)
	require.Equal(t, "/registry/pods0", stringVal(f))
}

func TestLogLightweightRequestInfo_RangeRequestNoRangeEnd(t *testing.T) {
	lg, logs := newTestLogger()
	req := &pb.RangeRequest{Key: []byte("/registry/pods")}
	logLightweightRequestInfo(lg, "/etcdserverpb.KV/Range", "10.0.0.1:34567", 5*time.Millisecond, req)

	entry := logs.All()[0]
	_, ok := findField(entry, "key")
	require.True(t, ok)
	_, ok = findField(entry, "range_end")
	require.False(t, ok, "range_end should not be logged when empty")
}

func TestLogLightweightRequestInfo_PutRequest(t *testing.T) {
	lg, logs := newTestLogger()
	req := &pb.PutRequest{Key: []byte("/registry/secrets/my-secret"), Value: []byte("sensitive-data")}
	logLightweightRequestInfo(lg, "/etcdserverpb.KV/Put", "10.0.0.2:34568", 10*time.Millisecond, req)

	entry := logs.All()[0]
	f, ok := findField(entry, "key")
	require.True(t, ok)
	require.Equal(t, "/registry/secrets/my-secret", stringVal(f))

	_, ok = findField(entry, "value")
	require.False(t, ok, "value should not be logged for PutRequest")
}

func TestLogLightweightRequestInfo_DeleteRangeRequest(t *testing.T) {
	lg, logs := newTestLogger()
	req := &pb.DeleteRangeRequest{Key: []byte("/registry/pods"), RangeEnd: []byte("/registry/pods0")}
	logLightweightRequestInfo(lg, "/etcdserverpb.KV/DeleteRange", "10.0.0.3:34569", 3*time.Millisecond, req)

	entry := logs.All()[0]
	f, ok := findField(entry, "key")
	require.True(t, ok)
	require.Equal(t, "/registry/pods", stringVal(f))

	f, ok = findField(entry, "range_end")
	require.True(t, ok)
	require.Equal(t, "/registry/pods0", stringVal(f))
}

func TestLogLightweightRequestInfo_TxnRequest(t *testing.T) {
	lg, logs := newTestLogger()
	req := &pb.TxnRequest{
		Compare: []*pb.Compare{
			{Key: []byte("/key1"), RangeEnd: []byte("/key10")},
			{Key: []byte("/key2")},
		},
	}
	logLightweightRequestInfo(lg, "/etcdserverpb.KV/Txn", "10.0.0.4:34570", 15*time.Millisecond, req)

	entry := logs.All()[0]
	_, ok := findField(entry, "compare_keys")
	require.True(t, ok)
	_, ok = findField(entry, "compare_range_ends")
	require.True(t, ok)
}

func TestLogLightweightRequestInfo_TxnRequestNoCompare(t *testing.T) {
	lg, logs := newTestLogger()
	req := &pb.TxnRequest{}
	logLightweightRequestInfo(lg, "/etcdserverpb.KV/Txn", "10.0.0.4:34570", 1*time.Millisecond, req)

	entry := logs.All()[0]
	_, ok := findField(entry, "compare_keys")
	require.False(t, ok, "compare_keys should not be logged when empty")
}

func TestLogLightweightRequestInfo_TxnRequestTruncated(t *testing.T) {
	lg, logs := newTestLogger()
	const totalCompares = 15
	compares := make([]*pb.Compare, totalCompares)
	for i := 0; i < totalCompares; i++ {
		compares[i] = &pb.Compare{Key: []byte(fmt.Sprintf("/key%d", i)), RangeEnd: []byte(fmt.Sprintf("/key%d-end", i))}
	}
	req := &pb.TxnRequest{Compare: compares}
	logLightweightRequestInfo(lg, "/etcdserverpb.KV/Txn", "10.0.0.4:34570", 15*time.Millisecond, req)

	entry := logs.All()[0]
	_, ok := findField(entry, "compare_keys")
	require.True(t, ok)
	_, ok = findField(entry, "compare_range_ends")
	require.True(t, ok)

	truncatedField, ok := findField(entry, "truncated")
	require.True(t, ok, "truncated field should be present when compares exceed maxLogKeys")
	require.Equal(t, int64(totalCompares-maxLogKeys), truncatedField.Integer)
}

func TestLogLightweightRequestInfo_LeaseGrantRequest(t *testing.T) {
	lg, logs := newTestLogger()
	req := &pb.LeaseGrantRequest{TTL: 60, ID: 12345}
	logLightweightRequestInfo(lg, "/etcdserverpb.Lease/LeaseGrant", "10.0.0.5:34571", 1*time.Millisecond, req)

	entry := logs.All()[0]
	f, ok := findField(entry, "ttl")
	require.True(t, ok)
	require.Equal(t, int64(60), f.Integer)

	f, ok = findField(entry, "lease_id")
	require.True(t, ok)
	require.Equal(t, int64(12345), f.Integer)
}

func TestLogLightweightRequestInfo_LeaseRevokeRequest(t *testing.T) {
	lg, logs := newTestLogger()
	req := &pb.LeaseRevokeRequest{ID: 12345}
	logLightweightRequestInfo(lg, "/etcdserverpb.Lease/LeaseRevoke", "10.0.0.5:34571", 1*time.Millisecond, req)

	entry := logs.All()[0]
	f, ok := findField(entry, "lease_id")
	require.True(t, ok)
	require.Equal(t, int64(12345), f.Integer)
}

func TestLogLightweightRequestInfo_UnknownRequest(t *testing.T) {
	lg, logs := newTestLogger()
	req := &pb.AuthUserAddRequest{Name: "test-user"}
	logLightweightRequestInfo(lg, "/etcdserverpb.Auth/UserAdd", "10.0.0.6:34572", 2*time.Millisecond, req)

	entry := logs.All()[0]
	f, ok := findField(entry, "method")
	require.True(t, ok)
	require.Equal(t, "/etcdserverpb.Auth/UserAdd", stringVal(f))

	f, ok = findField(entry, "remote")
	require.True(t, ok)
	require.Equal(t, "10.0.0.6:34572", stringVal(f))

	_, ok = findField(entry, "key")
	require.False(t, ok)
}
