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
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/raft/raftpb"
)

func mustMarshalMsg(t *testing.T, m raftpb.Message) []byte {
	json, err := m.Marshal()
	if err != nil {
		t.Fatalf("error marshalling raft Message: %#v", err)
	}
	return json
}

// errReader implements io.Reader to facilitate a broken request.
type errReader struct{}

func (er *errReader) Read(_ []byte) (int, error) { return 0, errors.New("some error") }

func TestServeRaft(t *testing.T) {
	testCases := []struct {
		method    string
		body      io.Reader
		serverErr error
		clusterID string

		wcode int
	}{
		{
			// bad method
			"GET",
			bytes.NewReader(
				mustMarshalMsg(
					t,
					raftpb.Message{},
				),
			),
			nil,
			"0",
			http.StatusMethodNotAllowed,
		},
		{
			// bad method
			"PUT",
			bytes.NewReader(
				mustMarshalMsg(
					t,
					raftpb.Message{},
				),
			),
			nil,
			"0",
			http.StatusMethodNotAllowed,
		},
		{
			// bad method
			"DELETE",
			bytes.NewReader(
				mustMarshalMsg(
					t,
					raftpb.Message{},
				),
			),
			nil,
			"0",
			http.StatusMethodNotAllowed,
		},
		{
			// bad request body
			"POST",
			&errReader{},
			nil,
			"0",
			http.StatusBadRequest,
		},
		{
			// bad request protobuf
			"POST",
			strings.NewReader("malformed garbage"),
			nil,
			"0",
			http.StatusBadRequest,
		},
		{
			// good request, etcdserver.Server internal error
			"POST",
			bytes.NewReader(
				mustMarshalMsg(
					t,
					raftpb.Message{},
				),
			),
			errors.New("some error"),
			"0",
			http.StatusInternalServerError,
		},
		{
			// good request from removed member
			"POST",
			bytes.NewReader(
				mustMarshalMsg(
					t,
					raftpb.Message{},
				),
			),
			etcdserver.ErrRemoved,
			"0",
			http.StatusForbidden,
		},
		{
			// good request
			"POST",
			bytes.NewReader(
				mustMarshalMsg(
					t,
					raftpb.Message{},
				),
			),
			nil,
			"1",
			http.StatusPreconditionFailed,
		},
		{
			// good request
			"POST",
			bytes.NewReader(
				mustMarshalMsg(
					t,
					raftpb.Message{},
				),
			),
			nil,
			"0",
			http.StatusNoContent,
		},
	}
	for i, tt := range testCases {
		req, err := http.NewRequest(tt.method, "foo", tt.body)
		if err != nil {
			t.Fatalf("#%d: could not create request: %#v", i, err)
		}
		req.Header.Set("X-Etcd-Cluster-ID", tt.clusterID)
		rw := httptest.NewRecorder()
		h := &raftHandler{stats: nil, server: &errServer{tt.serverErr}, clusterInfo: &fakeCluster{id: 0}}
		h.ServeHTTP(rw, req)
		if rw.Code != tt.wcode {
			t.Errorf("#%d: got code=%d, want %d", i, rw.Code, tt.wcode)
		}
	}
}

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
