/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package etcdhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/coreos/etcd/etcdserver"
)

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
		h := &peerMembersHandler{clusterInfo: nil}
		h.ServeHTTP(rw, &http.Request{Method: tt.method})
		if rw.Code != tt.wcode {
			t.Errorf("#%d: code=%d, want %d", i, rw.Code, tt.wcode)
		}
	}
}

func TestServeMembersGet(t *testing.T) {
	memb1 := etcdserver.Member{ID: 1, Attributes: etcdserver.Attributes{ClientURLs: []string{"http://localhost:8080"}}}
	memb2 := etcdserver.Member{ID: 2, Attributes: etcdserver.Attributes{ClientURLs: []string{"http://localhost:8081"}}}
	cluster := &fakeCluster{
		id:      1,
		members: map[uint64]*etcdserver.Member{1: &memb1, 2: &memb2},
	}
	h := &peerMembersHandler{clusterInfo: cluster}
	msb, err := json.Marshal([]etcdserver.Member{memb1, memb2})
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
		{peerMembersPrefix, http.StatusOK, "application/json", wms},
		{path.Join(peerMembersPrefix, "bad"), http.StatusBadRequest, "text/plain; charset=utf-8", "bad path\n"},
	}

	for i, tt := range tests {
		req, err := http.NewRequest("GET", mustNewURL(t, tt.path).String(), nil)
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
