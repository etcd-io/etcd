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

	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
)

func TestV3CurlLeaseGrantNoTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlLeaseGrant, withApiPrefix(p), withCfg(configNoTLS))
	}
}
func TestV3CurlLeaseRevokeNoTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlLeaseRevoke, withApiPrefix(p), withCfg(configNoTLS))
	}
}
func TestV3CurlLeaseLeasesNoTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlLeaseLeases, withApiPrefix(p), withCfg(configNoTLS))
	}
}
func TestV3CurlLeaseKeepAliveNoTLS(t *testing.T) {
	for _, p := range apiPrefix {
		testCtl(t, testV3CurlLeaseKeepAlive, withApiPrefix(p), withCfg(configNoTLS))
	}
}

type v3cURLTest struct {
	endpoint string
	value    string
	expected string
}

// TODO remove /kv/lease/timetolive, /kv/lease/revoke, /kv/lease/leases tests in 3.5 release

func testV3CurlLeaseGrant(cx ctlCtx) {
	leaseID := randomLeaseID()

	tests := []v3cURLTest{
		{
			endpoint: "/lease/grant",
			value:    gwLeaseGrant(cx, leaseID, 0),
			expected: gwLeaseIDExpected(leaseID),
		},
		{
			endpoint: "/lease/grant",
			value:    gwLeaseGrant(cx, 0, 20),
			expected: `"TTL":"20"`,
		},
		{
			endpoint: "/kv/put",
			value:    gwKVPutLease(cx, "foo", "bar", leaseID),
			expected: `"revision":"`,
		},
		{
			endpoint: "/lease/timetolive",
			value:    gwLeaseTTLWithKeys(cx, leaseID),
			expected: `"grantedTTL"`,
		},
		{
			endpoint: "/kv/lease/timetolive",
			value:    gwLeaseTTLWithKeys(cx, leaseID),
			expected: `"grantedTTL"`,
		},
	}
	if err := cURLWithExpected(cx, tests); err != nil {
		cx.t.Fatalf("testV3CurlLeaseGrant: %v", err)
	}
}

func testV3CurlLeaseRevoke(cx ctlCtx) {
	leaseID := randomLeaseID()

	tests := []v3cURLTest{
		{
			endpoint: "/lease/grant",
			value:    gwLeaseGrant(cx, leaseID, 0),
			expected: gwLeaseIDExpected(leaseID),
		},
		{
			endpoint: "/lease/revoke",
			value:    gwLeaseRevoke(cx, leaseID),
			expected: `"revision":"`,
		},
		{
			endpoint: "/kv/lease/revoke",
			value:    gwLeaseRevoke(cx, leaseID),
			expected: `etcdserver: requested lease not found`,
		},
	}
	if err := cURLWithExpected(cx, tests); err != nil {
		cx.t.Fatalf("testV3CurlLeaseRevoke: %v", err)
	}
}

func testV3CurlLeaseLeases(cx ctlCtx) {
	leaseID := randomLeaseID()

	tests := []v3cURLTest{
		{
			endpoint: "/lease/grant",
			value:    gwLeaseGrant(cx, leaseID, 0),
			expected: gwLeaseIDExpected(leaseID),
		},
		{
			endpoint: "/lease/leases",
			value:    "{}",
			expected: gwLeaseIDExpected(leaseID),
		},
		{
			endpoint: "/kv/lease/leases",
			value:    "{}",
			expected: gwLeaseIDExpected(leaseID),
		},
	}
	if err := cURLWithExpected(cx, tests); err != nil {
		cx.t.Fatalf("testV3CurlLeaseGrant: %v", err)
	}
}

func testV3CurlLeaseKeepAlive(cx ctlCtx) {
	leaseID := randomLeaseID()

	tests := []v3cURLTest{
		{
			endpoint: "/lease/grant",
			value:    gwLeaseGrant(cx, leaseID, 0),
			expected: gwLeaseIDExpected(leaseID),
		},
		{
			endpoint: "/lease/keepalive",
			value:    gwLeaseKeepAlive(cx, leaseID),
			expected: gwLeaseIDExpected(leaseID),
		},
	}
	if err := cURLWithExpected(cx, tests); err != nil {
		cx.t.Fatalf("testV3CurlLeaseGrant: %v", err)
	}
}

func gwLeaseIDExpected(leaseID int64) string {
	return fmt.Sprintf(`"ID":"%d"`, leaseID)
}

func gwLeaseTTLWithKeys(cx ctlCtx, leaseID int64) string {
	d := &pb.LeaseTimeToLiveRequest{ID: leaseID, Keys: true}
	s, err := dataMarshal(d)
	if err != nil {
		cx.t.Fatalf("gwLeaseTTLWithKeys: error (%v)", err)
	}
	return s
}

func gwLeaseKeepAlive(cx ctlCtx, leaseID int64) string {
	d := &pb.LeaseKeepAliveRequest{ID: leaseID}
	s, err := dataMarshal(d)
	if err != nil {
		cx.t.Fatalf("gwLeaseKeepAlive: Marshal error (%v)", err)
	}
	return s
}

func gwLeaseGrant(cx ctlCtx, leaseID int64, ttl int64) string {
	d := &pb.LeaseGrantRequest{ID: leaseID, TTL: ttl}
	s, err := dataMarshal(d)
	if err != nil {
		cx.t.Fatalf("gwLeaseGrant: Marshal error (%v)", err)
	}
	return s
}

func gwLeaseRevoke(cx ctlCtx, leaseID int64) string {
	d := &pb.LeaseRevokeRequest{ID: leaseID}
	s, err := dataMarshal(d)
	if err != nil {
		cx.t.Fatalf("gwLeaseRevoke: Marshal error (%v)", err)
	}
	return s
}

func gwKVPutLease(cx ctlCtx, k string, v string, leaseID int64) string {
	d := pb.PutRequest{Key: []byte(k), Value: []byte(v), Lease: leaseID}
	s, err := dataMarshal(d)
	if err != nil {
		cx.t.Fatalf("gwKVPutLease: Marshal error (%v)", err)
	}
	return s
}
