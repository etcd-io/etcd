// Copyright 2016 The etcd Authors
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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/pkg/v3/expect"
	epb "go.etcd.io/etcd/server/v3/etcdserver/api/v3election/v3electionpb"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCurlV3Watch(t *testing.T) {
	testCtl(t, testCurlV3Watch)
}

func testCurlV3Watch(cx ctlCtx) {
	// store "bar" into "foo"
	putreq, err := json.Marshal(&pb.PutRequest{Key: []byte("foo"), Value: []byte("bar")})
	if err != nil {
		cx.t.Fatal(err)
	}
	// watch for first update to "foo"
	wcr := &pb.WatchCreateRequest{Key: []byte("foo"), StartRevision: 1}
	wreq, err := json.Marshal(wcr)
	if err != nil {
		cx.t.Fatal(err)
	}
	// marshaling the grpc to json gives:
	// "{"RequestUnion":{"CreateRequest":{"key":"Zm9v","start_revision":1}}}"
	// but the gprc-gateway expects a different format..
	wstr := `{"create_request" : ` + string(wreq) + "}"

	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: "/v3/kv/put", Value: string(putreq), Expected: expect.ExpectedResponse{Value: "revision"}}); err != nil {
		cx.t.Fatalf("failed testCurlV3Watch put with curl (%v)", err)
	}
	// expects "bar", timeout after 2 seconds since stream waits forever
	err = e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: "/v3/watch", Value: wstr, Expected: expect.ExpectedResponse{Value: `"YmFy"`}, Timeout: 2})
	require.ErrorContains(cx.t, err, "unexpected exit code")
}

func TestCurlV3CampaignNoTLS(t *testing.T) {
	testCtl(t, testCurlV3Campaign, withCfg(*e2e.NewConfigNoTLS()))
}

func testCurlV3Campaign(cx ctlCtx) {
	cdata, err := json.Marshal(&epb.CampaignRequest{
		Name:  []byte("/election-prefix"),
		Value: []byte("v1"),
	})
	if err != nil {
		cx.t.Fatal(err)
	}
	cargs := e2e.CURLPrefixArgsCluster(cx.epc.Cfg, cx.epc.Procs[rand.Intn(cx.epc.Cfg.ClusterSize)], "POST", e2e.CURLReq{
		Endpoint: "/v3/election/campaign",
		Value:    string(cdata),
	})
	lines, err := e2e.SpawnWithExpectLines(context.TODO(), cargs, cx.envMap, expect.ExpectedResponse{Value: `"leader":{"name":"`})
	if err != nil {
		cx.t.Fatalf("failed post campaign request (%v)", err)
	}
	if len(lines) != 1 {
		cx.t.Fatalf("len(lines) expected 1, got %+v", lines)
	}

	var cresp campaignResponse
	if err = json.Unmarshal([]byte(lines[0]), &cresp); err != nil {
		cx.t.Fatalf("failed to unmarshal campaign response %v", err)
	}
	ndata, err := base64.StdEncoding.DecodeString(cresp.Leader.Name)
	if err != nil {
		cx.t.Fatalf("failed to decode leader key %v", err)
	}
	kdata, err := base64.StdEncoding.DecodeString(cresp.Leader.Key)
	if err != nil {
		cx.t.Fatalf("failed to decode leader key %v", err)
	}

	rev, _ := strconv.ParseInt(cresp.Leader.Rev, 10, 64)
	lease, _ := strconv.ParseInt(cresp.Leader.Lease, 10, 64)
	pdata, err := json.Marshal(&epb.ProclaimRequest{
		Leader: &epb.LeaderKey{
			Name:  ndata,
			Key:   kdata,
			Rev:   rev,
			Lease: lease,
		},
		Value: []byte("v2"),
	})
	if err != nil {
		cx.t.Fatal(err)
	}
	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/election/proclaim",
		Value:    string(pdata),
		Expected: expect.ExpectedResponse{Value: `"revision":`},
	}); err != nil {
		cx.t.Fatalf("failed post proclaim request (%v)", err)
	}
}

func TestCurlV3ProclaimMissiongLeaderKeyNoTLS(t *testing.T) {
	testCtl(t, testCurlV3ProclaimMissiongLeaderKey, withCfg(*e2e.NewConfigNoTLS()))
}

func testCurlV3ProclaimMissiongLeaderKey(cx ctlCtx) {
	pdata, err := json.Marshal(&epb.ProclaimRequest{Value: []byte("v2")})
	if err != nil {
		cx.t.Fatal(err)
	}
	if err = e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/election/proclaim",
		Value:    string(pdata),
		Expected: expect.ExpectedResponse{Value: `{"error":"\"leader\" field must be provided","code":2,"message":"\"leader\" field must be provided"}`},
	}); err != nil {
		cx.t.Fatalf("failed post proclaim request (%v)", err)
	}
}

func TestCurlV3ResignMissiongLeaderKeyNoTLS(t *testing.T) {
	testCtl(t, testCurlV3ResignMissiongLeaderKey, withCfg(*e2e.NewConfigNoTLS()))
}

func testCurlV3ResignMissiongLeaderKey(cx ctlCtx) {
	if err := e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/election/resign",
		Value:    `{}`,
		Expected: expect.ExpectedResponse{Value: `{"error":"\"leader\" field must be provided","code":2,"message":"\"leader\" field must be provided"}`},
	}); err != nil {
		cx.t.Fatalf("failed post resign request (%v)", err)
	}
}

// to manually decode; JSON marshals integer fields with
// string types, so can't unmarshal with epb.CampaignResponse
type campaignResponse struct {
	Leader struct {
		Name  string `json:"name,omitempty"`
		Key   string `json:"key,omitempty"`
		Rev   string `json:"rev,omitempty"`
		Lease string `json:"lease,omitempty"`
	} `json:"leader,omitempty"`
}

func CURLWithExpected(cx ctlCtx, tests []v3cURLTest) error {
	for _, t := range tests {
		value := fmt.Sprintf("%v", t.value)
		if err := e2e.CURLPost(cx.epc, e2e.CURLReq{Endpoint: t.endpoint, Value: value, Expected: expect.ExpectedResponse{Value: t.expected}}); err != nil {
			return fmt.Errorf("endpoint (%s): error (%v), wanted %v", t.endpoint, err, t.expected)
		}
	}
	return nil
}
