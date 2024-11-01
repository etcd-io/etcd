// Copyright 2018 The etcd Authors
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

package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCurlV3LeaseGrantNoTLS(t *testing.T) {
	testCtl(t, testCurlV3LeaseGrant, withCfg(*e2e.NewConfigNoTLS()))
}

func TestCurlV3LeaseRevokeNoTLS(t *testing.T) {
	testCtl(t, testCurlV3LeaseRevoke, withCfg(*e2e.NewConfigNoTLS()))
}

func TestCurlV3LeaseLeasesNoTLS(t *testing.T) {
	testCtl(t, testCurlV3LeaseLeases, withCfg(*e2e.NewConfigNoTLS()))
}

func TestCurlV3LeaseKeepAliveNoTLS(t *testing.T) {
	testCtl(t, testCurlV3LeaseKeepAlive, withCfg(*e2e.NewConfigNoTLS()))
}

type v3cURLTest struct {
	endpoint string
	value    string
	expected string
}

func testCurlV3LeaseGrant(cx ctlCtx) {
	leaseID := e2e.RandomLeaseID()

	tests := []v3cURLTest{
		{
			endpoint: "/v3/lease/grant",
			value:    gwLeaseGrant(cx, leaseID, 0),
			expected: gwLeaseIDExpected(leaseID),
		},
		{
			endpoint: "/v3/lease/grant",
			value:    gwLeaseGrant(cx, 0, 20),
			expected: `"TTL":"20"`,
		},
		{
			endpoint: "/v3/kv/put",
			value:    gwKVPutLease(cx, "foo", "bar", leaseID),
			expected: `"revision":"`,
		},
		{
			endpoint: "/v3/lease/timetolive",
			value:    gwLeaseTTLWithKeys(cx, leaseID),
			expected: `"grantedTTL"`,
		},
	}
	require.NoErrorf(cx.t, CURLWithExpected(cx, tests), "testCurlV3LeaseGrant")
}

func testCurlV3LeaseRevoke(cx ctlCtx) {
	leaseID := e2e.RandomLeaseID()

	tests := []v3cURLTest{
		{
			endpoint: "/v3/lease/grant",
			value:    gwLeaseGrant(cx, leaseID, 0),
			expected: gwLeaseIDExpected(leaseID),
		},
		{
			endpoint: "/v3/lease/revoke",
			value:    gwLeaseRevoke(cx, leaseID),
			expected: `"revision":"`,
		},
	}
	require.NoErrorf(cx.t, CURLWithExpected(cx, tests), "testCurlV3LeaseRevoke")
}

func testCurlV3LeaseLeases(cx ctlCtx) {
	leaseID := e2e.RandomLeaseID()

	tests := []v3cURLTest{
		{
			endpoint: "/v3/lease/grant",
			value:    gwLeaseGrant(cx, leaseID, 0),
			expected: gwLeaseIDExpected(leaseID),
		},
		{
			endpoint: "/v3/lease/leases",
			value:    "{}",
			expected: gwLeaseIDExpected(leaseID),
		},
	}
	require.NoErrorf(cx.t, CURLWithExpected(cx, tests), "testCurlV3LeaseGrant")
}

func testCurlV3LeaseKeepAlive(cx ctlCtx) {
	leaseID := e2e.RandomLeaseID()

	tests := []v3cURLTest{
		{
			endpoint: "/v3/lease/grant",
			value:    gwLeaseGrant(cx, leaseID, 0),
			expected: gwLeaseIDExpected(leaseID),
		},
		{
			endpoint: "/v3/lease/keepalive",
			value:    gwLeaseKeepAlive(cx, leaseID),
			expected: gwLeaseIDExpected(leaseID),
		},
	}
	require.NoErrorf(cx.t, CURLWithExpected(cx, tests), "testCurlV3LeaseGrant")
}

func gwLeaseIDExpected(leaseID int64) string {
	return fmt.Sprintf(`"ID":"%d"`, leaseID)
}

func gwLeaseTTLWithKeys(cx ctlCtx, leaseID int64) string {
	d := &pb.LeaseTimeToLiveRequest{ID: leaseID, Keys: true}
	s, err := e2e.DataMarshal(d)
	require.NoErrorf(cx.t, err, "gwLeaseTTLWithKeys: error")
	return s
}

func gwLeaseKeepAlive(cx ctlCtx, leaseID int64) string {
	d := &pb.LeaseKeepAliveRequest{ID: leaseID}
	s, err := e2e.DataMarshal(d)
	require.NoErrorf(cx.t, err, "gwLeaseKeepAlive: Marshal error")
	return s
}

func gwLeaseGrant(cx ctlCtx, leaseID int64, ttl int64) string {
	d := &pb.LeaseGrantRequest{ID: leaseID, TTL: ttl}
	s, err := e2e.DataMarshal(d)
	require.NoErrorf(cx.t, err, "gwLeaseGrant: Marshal error")
	return s
}

func gwLeaseRevoke(cx ctlCtx, leaseID int64) string {
	d := &pb.LeaseRevokeRequest{ID: leaseID}
	s, err := e2e.DataMarshal(d)
	require.NoErrorf(cx.t, err, "gwLeaseRevoke: Marshal error")
	return s
}

func gwKVPutLease(cx ctlCtx, k string, v string, leaseID int64) string {
	d := pb.PutRequest{Key: []byte(k), Value: []byte(v), Lease: leaseID}
	s, err := e2e.DataMarshal(d)
	require.NoErrorf(cx.t, err, "gwKVPutLease: Marshal error")
	return s
}
