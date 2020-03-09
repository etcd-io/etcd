// Copyright 2015 The etcd Authors
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

package etcdhttp

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path"
	"sort"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/coreos/go-semver/semver"
	"go.etcd.io/etcd/etcdserver/api"
	"go.etcd.io/etcd/etcdserver/api/membership"
	"go.etcd.io/etcd/etcdserver/api/rafthttp"
	pb "go.etcd.io/etcd/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/pkg/testutil"
	"go.etcd.io/etcd/pkg/types"
)

type fakeCluster struct {
	id         uint64
	clientURLs []string
	members    map[uint64]*membership.Member
}

func (c *fakeCluster) ID() types.ID         { return types.ID(c.id) }
func (c *fakeCluster) ClientURLs() []string { return c.clientURLs }
func (c *fakeCluster) Members() []*membership.Member {
	var ms membership.MembersByID
	for _, m := range c.members {
		ms = append(ms, m)
	}
	sort.Sort(ms)
	return []*membership.Member(ms)
}
func (c *fakeCluster) Member(id types.ID) *membership.Member { return c.members[uint64(id)] }
func (c *fakeCluster) Version() *semver.Version              { return nil }

type fakeServer struct {
	cluster api.Cluster
}

func (s *fakeServer) AddMember(ctx context.Context, memb membership.Member) ([]*membership.Member, error) {
	return nil, fmt.Errorf("AddMember not implemented in fakeServer")
}
func (s *fakeServer) RemoveMember(ctx context.Context, id uint64) ([]*membership.Member, error) {
	return nil, fmt.Errorf("RemoveMember not implemented in fakeServer")
}
func (s *fakeServer) UpdateMember(ctx context.Context, updateMemb membership.Member) ([]*membership.Member, error) {
	return nil, fmt.Errorf("UpdateMember not implemented in fakeServer")
}
func (s *fakeServer) PromoteMember(ctx context.Context, id uint64) ([]*membership.Member, error) {
	return nil, fmt.Errorf("PromoteMember not implemented in fakeServer")
}
func (s *fakeServer) ClusterVersion() *semver.Version { return nil }
func (s *fakeServer) Cluster() api.Cluster            { return s.cluster }
func (s *fakeServer) Alarms() []*pb.AlarmMember       { return nil }

var fakeRaftHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("test data"))
})

// TestNewPeerHandlerOnRaftPrefix tests that NewPeerHandler returns a handler that
// handles raft-prefix requests well.
func TestNewPeerHandlerOnRaftPrefix(t *testing.T) {
	ph := newPeerHandler(zap.NewExample(), &fakeServer{cluster: &fakeCluster{}}, fakeRaftHandler, nil, nil)
	srv := httptest.NewServer(ph)
	defer srv.Close()

	tests := []string{
		rafthttp.RaftPrefix,
		rafthttp.RaftPrefix + "/hello",
	}
	for i, tt := range tests {
		resp, err := http.Get(srv.URL + tt)
		if err != nil {
			t.Fatalf("unexpected http.Get error: %v", err)
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("unexpected ioutil.ReadAll error: %v", err)
		}
		if w := "test data"; string(body) != w {
			t.Errorf("#%d: body = %s, want %s", i, body, w)
		}
	}
}

// TestServeMembersFails ensures peerMembersHandler only accepts GET request
func TestServeMembersFails(t *testing.T) {
	tests := []struct {
		method string
		wcode  int
	}{
		{
			"POST",
			http.StatusMethodNotAllowed,
		},
		{
			"PUT",
			http.StatusMethodNotAllowed,
		},
		{
			"DELETE",
			http.StatusMethodNotAllowed,
		},
		{
			"BAD",
			http.StatusMethodNotAllowed,
		},
	}
	for i, tt := range tests {
		rw := httptest.NewRecorder()
		h := newPeerMembersHandler(nil, &fakeCluster{})
		req, err := http.NewRequest(tt.method, "", nil)
		if err != nil {
			t.Fatalf("#%d: failed to create http request: %v", i, err)
		}
		h.ServeHTTP(rw, req)
		if rw.Code != tt.wcode {
			t.Errorf("#%d: code=%d, want %d", i, rw.Code, tt.wcode)
		}
	}
}

func TestServeMembersGet(t *testing.T) {
	memb1 := membership.Member{ID: 1, Attributes: membership.Attributes{ClientURLs: []string{"http://localhost:8080"}}}
	memb2 := membership.Member{ID: 2, Attributes: membership.Attributes{ClientURLs: []string{"http://localhost:8081"}}}
	cluster := &fakeCluster{
		id:      1,
		members: map[uint64]*membership.Member{1: &memb1, 2: &memb2},
	}
	h := newPeerMembersHandler(nil, cluster)
	msb, err := json.Marshal([]membership.Member{memb1, memb2})
	if err != nil {
		t.Fatal(err)
	}
	wms := string(msb) + "\n"

	tests := []struct {
		path  string
		wcode int
		wct   string
		wbody string
	}{
		{peerMembersPath, http.StatusOK, "application/json", wms},
		{path.Join(peerMembersPath, "bad"), http.StatusBadRequest, "text/plain; charset=utf-8", "bad path\n"},
	}

	for i, tt := range tests {
		req, err := http.NewRequest("GET", testutil.MustNewURL(t, tt.path).String(), nil)
		if err != nil {
			t.Fatal(err)
		}
		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, req)

		if rw.Code != tt.wcode {
			t.Errorf("#%d: code=%d, want %d", i, rw.Code, tt.wcode)
		}
		if gct := rw.Header().Get("Content-Type"); gct != tt.wct {
			t.Errorf("#%d: content-type = %s, want %s", i, gct, tt.wct)
		}
		if rw.Body.String() != tt.wbody {
			t.Errorf("#%d: body = %s, want %s", i, rw.Body.String(), tt.wbody)
		}
		gcid := rw.Header().Get("X-Etcd-Cluster-ID")
		wcid := cluster.ID().String()
		if gcid != wcid {
			t.Errorf("#%d: cid = %s, want %s", i, gcid, wcid)
		}
	}
}

// TestServeMemberPromoteFails ensures peerMemberPromoteHandler only accepts POST request
func TestServeMemberPromoteFails(t *testing.T) {
	tests := []struct {
		method string
		wcode  int
	}{
		{
			"GET",
			http.StatusMethodNotAllowed,
		},
		{
			"PUT",
			http.StatusMethodNotAllowed,
		},
		{
			"DELETE",
			http.StatusMethodNotAllowed,
		},
		{
			"BAD",
			http.StatusMethodNotAllowed,
		},
	}
	for i, tt := range tests {
		rw := httptest.NewRecorder()
		h := newPeerMemberPromoteHandler(nil, &fakeServer{cluster: &fakeCluster{}})
		req, err := http.NewRequest(tt.method, "", nil)
		if err != nil {
			t.Fatalf("#%d: failed to create http request: %v", i, err)
		}
		h.ServeHTTP(rw, req)
		if rw.Code != tt.wcode {
			t.Errorf("#%d: code=%d, want %d", i, rw.Code, tt.wcode)
		}
	}
}

// TestNewPeerHandlerOnMembersPromotePrefix verifies the request with members promote prefix is routed correctly
func TestNewPeerHandlerOnMembersPromotePrefix(t *testing.T) {
	ph := newPeerHandler(zap.NewExample(), &fakeServer{cluster: &fakeCluster{}}, fakeRaftHandler, nil, nil)
	srv := httptest.NewServer(ph)
	defer srv.Close()

	tests := []struct {
		path      string
		wcode     int
		checkBody bool
		wKeyWords string
	}{
		{
			// does not contain member id in path
			peerMemberPromotePrefix,
			http.StatusNotFound,
			false,
			"",
		},
		{
			// try to promote member id = 1
			peerMemberPromotePrefix + "1",
			http.StatusInternalServerError,
			true,
			"PromoteMember not implemented in fakeServer",
		},
	}
	for i, tt := range tests {
		req, err := http.NewRequest("POST", srv.URL+tt.path, nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to get http response: %v", err)
		}
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("unexpected ioutil.ReadAll error: %v", err)
		}
		if resp.StatusCode != tt.wcode {
			t.Fatalf("#%d: code = %d, want %d", i, resp.StatusCode, tt.wcode)
		}
		if tt.checkBody && strings.Contains(string(body), tt.wKeyWords) {
			t.Errorf("#%d: body: %s, want body to contain keywords: %s", i, string(body), tt.wKeyWords)
		}
	}
}
