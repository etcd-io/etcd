// Copyright 2023 The etcd Authors
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

package etcdserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/lease"
	"go.etcd.io/etcd/mvcc"
	betesting "go.etcd.io/etcd/mvcc/backend"
	"go.etcd.io/etcd/pkg/types"
)

func TestHashKVHandler(t *testing.T) {
	var remoteClusterID = 111195
	var localClusterID = 111196
	var revision = 1

	etcdSrv := &EtcdServer{}
	etcdSrv.cluster = newTestCluster(nil)
	etcdSrv.cluster.SetID(types.ID(localClusterID), types.ID(localClusterID))
	be, _ := betesting.NewDefaultTmpBackend()
	defer func() {
		assert.NoError(t, be.Close())
	}()
	etcdSrv.kv = mvcc.New(zap.NewNop(), be, &lease.FakeLessor{}, nil, nil, mvcc.StoreConfig{})
	defer func() {
		assert.NoError(t, etcdSrv.kv.Close())
	}()
	ph := &hashKVHandler{
		lg:     zap.NewNop(),
		server: etcdSrv,
	}
	srv := httptest.NewServer(ph)
	defer srv.Close()

	tests := []struct {
		name            string
		remoteClusterID int
		wcode           int
		wKeyWords       string
	}{
		{
			name:            "HashKV returns 200 if cluster hash matches",
			remoteClusterID: localClusterID,
			wcode:           http.StatusOK,
			wKeyWords:       "",
		},
		{
			name:            "HashKV returns 400 if cluster hash doesn't matche",
			remoteClusterID: remoteClusterID,
			wcode:           http.StatusPreconditionFailed,
			wKeyWords:       "cluster ID mismatch",
		},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hashReq := &pb.HashKVRequest{Revision: int64(revision)}
			hashReqBytes, err := json.Marshal(hashReq)
			if err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}
			req, err := http.NewRequest(http.MethodGet, srv.URL+PeerHashKVPath, bytes.NewReader(hashReqBytes))
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			req.Header.Set("X-Etcd-Cluster-ID", strconv.FormatUint(uint64(tt.remoteClusterID), 16))

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("failed to get http response: %v", err)
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				t.Fatalf("unexpected io.ReadAll error: %v", err)
			}
			if resp.StatusCode != tt.wcode {
				t.Fatalf("#%d: code = %d, want %d", i, resp.StatusCode, tt.wcode)
			}
			if resp.StatusCode != http.StatusOK {
				if !strings.Contains(string(body), tt.wKeyWords) {
					t.Errorf("#%d: body: %s, want body to contain keywords: %s", i, string(body), tt.wKeyWords)
				}
				return
			}

			hashKVResponse := pb.HashKVResponse{}
			err = json.Unmarshal(body, &hashKVResponse)
			if err != nil {
				t.Fatalf("unmarshal response error: %v", err)
			}
			hashValue, _, _, err := etcdSrv.KV().HashByRev(int64(revision))
			if err != nil {
				t.Fatalf("etcd server hash failed: %v", err)
			}
			if hashKVResponse.Hash != hashValue {
				t.Fatalf("hash value inconsistent: %d != %d", hashKVResponse.Hash, hashValue)
			}
		})
	}
}
