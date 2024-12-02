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

	"go.etcd.io/etcd/pkg/v3/expect"
	epb "go.etcd.io/etcd/server/v3/etcdserver/api/v3election/v3electionpb"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestCurlV3CampaignNoTLS(t *testing.T) {
	testCtl(t, testCurlV3Campaign, withCfg(*e2e.NewConfigNoTLS()))
}

func testCurlV3Campaign(cx ctlCtx) {
	// campaign
	cdata, err := json.Marshal(&epb.CampaignRequest{
		Name:  []byte("/election-prefix"),
		Value: []byte("v1"),
	})
	require.NoError(cx.t, err)
	cargs := e2e.CURLPrefixArgsCluster(cx.epc.Cfg, cx.epc.Procs[rand.Intn(cx.epc.Cfg.ClusterSize)], "POST", e2e.CURLReq{
		Endpoint: "/v3/election/campaign",
		Value:    string(cdata),
	})
	lines, err := e2e.SpawnWithExpectLines(context.TODO(), cargs, cx.envMap, expect.ExpectedResponse{Value: `"leader":{"name":"`})
	require.NoErrorf(cx.t, err, "failed post campaign request")
	if len(lines) != 1 {
		cx.t.Fatalf("len(lines) expected 1, got %+v", lines)
	}

	var cresp campaignResponse
	require.NoErrorf(cx.t, json.Unmarshal([]byte(lines[0]), &cresp), "failed to unmarshal campaign response")
	ndata, err := base64.StdEncoding.DecodeString(cresp.Leader.Name)
	require.NoErrorf(cx.t, err, "failed to decode leader key")
	kdata, err := base64.StdEncoding.DecodeString(cresp.Leader.Key)
	require.NoErrorf(cx.t, err, "failed to decode leader key")

	// observe
	observeReq, err := json.Marshal(&epb.LeaderRequest{
		Name: []byte("/election-prefix"),
	})
	require.NoError(cx.t, err)

	clus := cx.epc
	args := e2e.CURLPrefixArgsCluster(clus.Cfg, clus.Procs[0], "POST", e2e.CURLReq{
		Endpoint: "/v3/election/observe",
		Value:    string(observeReq),
	})
	proc, err := e2e.SpawnCmd(args, nil)
	require.NoError(cx.t, err)

	proc.ExpectWithContext(context.TODO(), expect.ExpectedResponse{
		Value: fmt.Sprintf(`"key":"%s"`, cresp.Leader.Key),
	})
	require.NoError(cx.t, proc.Stop())

	// proclaim
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
	require.NoError(cx.t, err)
	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/election/proclaim",
		Value:    string(pdata),
		Expected: expect.ExpectedResponse{Value: `"revision":`},
	}), "failed post proclaim request")
}

func TestCurlV3ProclaimMissiongLeaderKeyNoTLS(t *testing.T) {
	testCtl(t, testCurlV3ProclaimMissiongLeaderKey, withCfg(*e2e.NewConfigNoTLS()))
}

func testCurlV3ProclaimMissiongLeaderKey(cx ctlCtx) {
	pdata, err := json.Marshal(&epb.ProclaimRequest{Value: []byte("v2")})
	require.NoError(cx.t, err)
	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/election/proclaim",
		Value:    string(pdata),
		Expected: expect.ExpectedResponse{Value: `"message":"\"leader\" field must be provided"`},
	}), "failed post proclaim request")
}

func TestCurlV3ResignMissiongLeaderKeyNoTLS(t *testing.T) {
	testCtl(t, testCurlV3ResignMissiongLeaderKey, withCfg(*e2e.NewConfigNoTLS()))
}

func testCurlV3ResignMissiongLeaderKey(cx ctlCtx) {
	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/election/resign",
		Value:    `{}`,
		Expected: expect.ExpectedResponse{Value: `"message":"\"leader\" field must be provided"`},
	}), "failed post resign request")
}

func TestCurlV3ElectionLeader(t *testing.T) {
	testCtl(t, testCurlV3ElectionLeader, withCfg(*e2e.NewConfigNoTLS()))
}

func testCurlV3ElectionLeader(cx ctlCtx) {
	require.NoErrorf(cx.t, e2e.CURLPost(cx.epc, e2e.CURLReq{
		Endpoint: "/v3/election/leader",
		Value:    `{"name": "aGVsbG8="}`, // base64 encoded string "hello"
		Expected: expect.ExpectedResponse{Value: `election: no leader`},
	}), "testCurlV3ElectionLeader failed to get leader")
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
			return fmt.Errorf("endpoint (%s): error (%w), wanted %v", t.endpoint, err, t.expected)
		}
	}
	return nil
}
