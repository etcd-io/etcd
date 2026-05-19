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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
)

func newTestLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zap.WarnLevel)
	return zap.New(core), logs
}

func TestLogLightweightRequestInfo_RangeRequest(t *testing.T) {
	lg, logs := newTestLogger()

	req := &pb.RangeRequest{Key: []byte("/registry/pods"), RangeEnd: []byte("/registry/pods0")}
	logLightweightRequestInfo(lg, "/etcdserverpb.KV/Range", "10.0.0.1:34567", 5*time.Millisecond, req)

	require.Equal(t, 1, logs.Len())
	entry := logs.All()[0]
	require.Equal(t, "request info", entry.Message)
	require.Equal(t, zap.WarnLevel, entry.Level)

	fields := fieldsMap(entry.Context)
	require.Equal(t, "/etcdserverpb.KV/Range", fields["method"])
	require.Equal(t, "10.0.0.1:34567", fields["remote"])
	require.Equal(t, []byte("/registry/pods"), fields["key"])
	require.Equal(t, []byte("/registry/pods0"), fields["range_end"])
}

func TestLogLightweightRequestInfo_RangeRequestNoRangeEnd(t *testing.T) {
	lg, logs := newTestLogger()

	req := &pb.RangeRequest{Key: []byte("/registry/pods")}
	logLightweightRequestInfo(lg, "/etcdserverpb.KV/Range", "10.0.0.1:34567", 5*time.Millisecond, req)

	require.Equal(t, 1, logs.Len())
	fields := fieldsMap(logs.All()[0].Context)
	require.Equal(t, []byte("/registry/pods"), fields["key"])
	_, hasRangeEnd := fields["range_end"]
	require.False(t, hasRangeEnd, "range_end should not be logged when empty")
}

func TestLogLightweightRequestInfo_PutRequest(t *testing.T) {
	lg, logs := newTestLogger()

	req := &pb.PutRequest{Key: []byte("/registry/secrets/my-secret"), Value: []byte("sensitive-data")}
	logLightweightRequestInfo(lg, "/etcdserverpb.KV/Put", "10.0.0.2:34568", 10*time.Millisecond, req)

	require.Equal(t, 1, logs.Len())
	fields := fieldsMap(logs.All()[0].Context)
	require.Equal(t, []byte("/registry/secrets/my-secret"), fields["key"])
	_, hasValue := fields["value"]
	require.False(t, hasValue, "value should not be logged for PutRequest")
}

func TestLogLightweightRequestInfo_DeleteRangeRequest(t *testing.T) {
	lg, logs := newTestLogger()

	req := &pb.DeleteRangeRequest{Key: []byte("/registry/pods"), RangeEnd: []byte("/registry/pods0")}
	logLightweightRequestInfo(lg, "/etcdserverpb.KV/DeleteRange", "10.0.0.3:34569", 3*time.Millisecond, req)

	require.Equal(t, 1, logs.Len())
	fields := fieldsMap(logs.All()[0].Context)
	require.Equal(t, []byte("/registry/pods"), fields["key"])
	require.Equal(t, []byte("/registry/pods0"), fields["range_end"])
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

	require.Equal(t, 1, logs.Len())
	fields := fieldsMap(logs.All()[0].Context)
	require.Equal(t, []string{"/key1", "/key2"}, fields["compare_keys"])
	require.Equal(t, []string{"/key10"}, fields["compare_range_ends"])
}

func TestLogLightweightRequestInfo_TxnRequestNoCompare(t *testing.T) {
	lg, logs := newTestLogger()

	req := &pb.TxnRequest{}
	logLightweightRequestInfo(lg, "/etcdserverpb.KV/Txn", "10.0.0.4:34570", 1*time.Millisecond, req)

	require.Equal(t, 1, logs.Len())
	fields := fieldsMap(logs.All()[0].Context)
	_, hasKeys := fields["compare_keys"]
	require.False(t, hasKeys, "compare_keys should not be logged when empty")
}

func TestLogLightweightRequestInfo_LeaseGrantRequest(t *testing.T) {
	lg, logs := newTestLogger()

	req := &pb.LeaseGrantRequest{TTL: 60, ID: 12345}
	logLightweightRequestInfo(lg, "/etcdserverpb.Lease/LeaseGrant", "10.0.0.5:34571", 1*time.Millisecond, req)

	require.Equal(t, 1, logs.Len())
	fields := fieldsMap(logs.All()[0].Context)
	require.Equal(t, int64(60), fields["ttl"])
	require.Equal(t, int64(12345), fields["lease_id"])
}

func TestLogLightweightRequestInfo_LeaseRevokeRequest(t *testing.T) {
	lg, logs := newTestLogger()

	req := &pb.LeaseRevokeRequest{ID: 12345}
	logLightweightRequestInfo(lg, "/etcdserverpb.Lease/LeaseRevoke", "10.0.0.5:34571", 1*time.Millisecond, req)

	require.Equal(t, 1, logs.Len())
	fields := fieldsMap(logs.All()[0].Context)
	require.Equal(t, int64(12345), fields["lease_id"])
}

func TestLogLightweightRequestInfo_UnknownRequest(t *testing.T) {
	lg, logs := newTestLogger()

	req := &pb.AuthUserAddRequest{Name: "test-user"}
	logLightweightRequestInfo(lg, "/etcdserverpb.Auth/UserAdd", "10.0.0.6:34572", 2*time.Millisecond, req)

	require.Equal(t, 1, logs.Len())
	fields := fieldsMap(logs.All()[0].Context)
	require.Equal(t, "/etcdserverpb.Auth/UserAdd", fields["method"])
	require.Equal(t, "10.0.0.6:34572", fields["remote"])
	_, hasKey := fields["key"]
	require.False(t, hasKey)
}

// fieldsMap converts zap fields to a map for easier assertions.
func fieldsMap(fields []zap.Field) map[string]any {
	m := make(map[string]any, len(fields))
	for _, f := range fields {
		m[f.Key] = f.Interface
	}
	return m
}
