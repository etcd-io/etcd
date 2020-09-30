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

package v2http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/v3/etcdserver"
	"go.etcd.io/etcd/v3/etcdserver/api"
	"go.etcd.io/etcd/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/v3/etcdserver/api/v2error"
	"go.etcd.io/etcd/v3/etcdserver/api/v2http/httptypes"
	"go.etcd.io/etcd/v3/etcdserver/api/v2store"
	"go.etcd.io/etcd/v3/etcdserver/etcdserverpb"
	"go.etcd.io/etcd/v3/pkg/testutil"
	"go.etcd.io/etcd/v3/pkg/types"
	"go.etcd.io/etcd/v3/raft/raftpb"

	"github.com/coreos/go-semver/semver"
	"github.com/jonboulle/clockwork"
	"go.uber.org/zap"
)

func mustMarshalEvent(t *testing.T, ev *v2store.Event) string {
	b := new(bytes.Buffer)
	if err := json.NewEncoder(b).Encode(ev); err != nil {
		t.Fatalf("error marshalling event %#v: %v", ev, err)
	}
	return b.String()
}

// mustNewForm takes a set of Values and constructs a PUT *http.Request,
// with a URL constructed from appending the given path to the standard keysPrefix
func mustNewForm(t *testing.T, p string, vals url.Values) *http.Request {
	u := testutil.MustNewURL(t, path.Join(keysPrefix, p))
	req, err := http.NewRequest("PUT", u.String(), strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err != nil {
		t.Fatalf("error creating new request: %v", err)
	}
	return req
}

// mustNewPostForm takes a set of Values and constructs a POST *http.Request,
// with a URL constructed from appending the given path to the standard keysPrefix
func mustNewPostForm(t *testing.T, p string, vals url.Values) *http.Request {
	u := testutil.MustNewURL(t, path.Join(keysPrefix, p))
	req, err := http.NewRequest("POST", u.String(), strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err != nil {
		t.Fatalf("error creating new request: %v", err)
	}
	return req
}

// mustNewRequest takes a path, appends it to the standard keysPrefix, and constructs
// a GET *http.Request referencing the resulting URL
func mustNewRequest(t *testing.T, p string) *http.Request {
	return mustNewMethodRequest(t, "GET", p)
}

func mustNewMethodRequest(t *testing.T, m, p string) *http.Request {
	return &http.Request{
		Method: m,
		URL:    testutil.MustNewURL(t, path.Join(keysPrefix, p)),
	}
}

type fakeServer struct {
	dummyRaftTimer
	dummyStats
}

func (s *fakeServer) Leader() types.ID                    { return types.ID(1) }
func (s *fakeServer) Alarms() []*etcdserverpb.AlarmMember { return nil }
func (s *fakeServer) Cluster() api.Cluster                { return nil }
func (s *fakeServer) ClusterVersion() *semver.Version     { return nil }
func (s *fakeServer) RaftHandler() http.Handler           { return nil }
func (s *fakeServer) Do(ctx context.Context, r etcdserverpb.Request) (rr etcdserver.Response, err error) {
	return
}
func (s *fakeServer) ClientCertAuthEnabled() bool { return false }

type serverRecorder struct {
	fakeServer
	actions []action
}

func (s *serverRecorder) Do(_ context.Context, r etcdserverpb.Request) (etcdserver.Response, error) {
	s.actions = append(s.actions, action{name: "Do", params: []interface{}{r}})
	return etcdserver.Response{}, nil
}
func (s *serverRecorder) Process(_ context.Context, m raftpb.Message) error {
	s.actions = append(s.actions, action{name: "Process", params: []interface{}{m}})
	return nil
}
func (s *serverRecorder) AddMember(_ context.Context, m membership.Member) ([]*membership.Member, error) {
	s.actions = append(s.actions, action{name: "AddMember", params: []interface{}{m}})
	return nil, nil
}
func (s *serverRecorder) RemoveMember(_ context.Context, id uint64) ([]*membership.Member, error) {
	s.actions = append(s.actions, action{name: "RemoveMember", params: []interface{}{id}})
	return nil, nil
}

func (s *serverRecorder) UpdateMember(_ context.Context, m membership.Member) ([]*membership.Member, error) {
	s.actions = append(s.actions, action{name: "UpdateMember", params: []interface{}{m}})
	return nil, nil
}

func (s *serverRecorder) PromoteMember(_ context.Context, id uint64) ([]*membership.Member, error) {
	s.actions = append(s.actions, action{name: "PromoteMember", params: []interface{}{id}})
	return nil, nil
}

type action struct {
	name   string
	params []interface{}
}

// flushingRecorder provides a channel to allow users to block until the Recorder is Flushed()
type flushingRecorder struct {
	*httptest.ResponseRecorder
	ch chan struct{}
}

func (fr *flushingRecorder) Flush() {
	fr.ResponseRecorder.Flush()
	fr.ch <- struct{}{}
}

// resServer implements the etcd.Server interface for testing.
// It returns the given response from any Do calls, and nil error
type resServer struct {
	fakeServer
	res etcdserver.Response
}

func (rs *resServer) Do(_ context.Context, _ etcdserverpb.Request) (etcdserver.Response, error) {
	return rs.res, nil
}
func (rs *resServer) Process(_ context.Context, _ raftpb.Message) error { return nil }
func (rs *resServer) AddMember(_ context.Context, _ membership.Member) ([]*membership.Member, error) {
	return nil, nil
}
func (rs *resServer) RemoveMember(_ context.Context, _ uint64) ([]*membership.Member, error) {
	return nil, nil
}
func (rs *resServer) UpdateMember(_ context.Context, _ membership.Member) ([]*membership.Member, error) {
	return nil, nil
}
func (rs *resServer) PromoteMember(_ context.Context, _ uint64) ([]*membership.Member, error) {
	return nil, nil
}

func boolp(b bool) *bool { return &b }

type dummyRaftTimer struct{}

func (drt dummyRaftTimer) Index() uint64 { return uint64(100) }
func (drt dummyRaftTimer) Term() uint64  { return uint64(5) }

type dummyWatcher struct {
	echan chan *v2store.Event
	sidx  uint64
}

func (w *dummyWatcher) EventChan() chan *v2store.Event {
	return w.echan
}
func (w *dummyWatcher) StartIndex() uint64 { return w.sidx }
func (w *dummyWatcher) Remove()            {}

func TestBadRefreshRequest(t *testing.T) {
	tests := []struct {
		in    *http.Request
		wcode int
	}{
		{
			mustNewRequest(t, "foo?refresh=true&value=test"),
			v2error.EcodeRefreshValue,
		},
		{
			mustNewRequest(t, "foo?refresh=true&value=10"),
			v2error.EcodeRefreshValue,
		},
		{
			mustNewRequest(t, "foo?refresh=true"),
			v2error.EcodeRefreshTTLRequired,
		},
		{
			mustNewRequest(t, "foo?refresh=true&ttl="),
			v2error.EcodeRefreshTTLRequired,
		},
	}
	for i, tt := range tests {
		got, _, err := parseKeyRequest(tt.in, clockwork.NewFakeClock())
		if err == nil {
			t.Errorf("#%d: unexpected nil error!", i)
			continue
		}
		ee, ok := err.(*v2error.Error)
		if !ok {
			t.Errorf("#%d: err is not etcd.Error!", i)
			continue
		}
		if ee.ErrorCode != tt.wcode {
			t.Errorf("#%d: code=%d, want %v", i, ee.ErrorCode, tt.wcode)
			t.Logf("cause: %#v", ee.Cause)
		}
		if !reflect.DeepEqual(got, etcdserverpb.Request{}) {
			t.Errorf("#%d: unexpected non-empty Request: %#v", i, got)
		}
	}
}

func TestBadParseRequest(t *testing.T) {
	tests := []struct {
		in    *http.Request
		wcode int
	}{
		{
			// parseForm failure
			&http.Request{
				Body:   nil,
				Method: "PUT",
			},
			v2error.EcodeInvalidForm,
		},
		{
			// bad key prefix
			&http.Request{
				URL: testutil.MustNewURL(t, "/badprefix/"),
			},
			v2error.EcodeInvalidForm,
		},
		// bad values for prevIndex, waitIndex, ttl
		{
			mustNewForm(t, "foo", url.Values{"prevIndex": []string{"garbage"}}),
			v2error.EcodeIndexNaN,
		},
		{
			mustNewForm(t, "foo", url.Values{"prevIndex": []string{"1.5"}}),
			v2error.EcodeIndexNaN,
		},
		{
			mustNewForm(t, "foo", url.Values{"prevIndex": []string{"-1"}}),
			v2error.EcodeIndexNaN,
		},
		{
			mustNewForm(t, "foo", url.Values{"waitIndex": []string{"garbage"}}),
			v2error.EcodeIndexNaN,
		},
		{
			mustNewForm(t, "foo", url.Values{"waitIndex": []string{"??"}}),
			v2error.EcodeIndexNaN,
		},
		{
			mustNewForm(t, "foo", url.Values{"ttl": []string{"-1"}}),
			v2error.EcodeTTLNaN,
		},
		// bad values for recursive, sorted, wait, prevExist, dir, stream
		{
			mustNewForm(t, "foo", url.Values{"recursive": []string{"hahaha"}}),
			v2error.EcodeInvalidField,
		},
		{
			mustNewForm(t, "foo", url.Values{"recursive": []string{"1234"}}),
			v2error.EcodeInvalidField,
		},
		{
			mustNewForm(t, "foo", url.Values{"recursive": []string{"?"}}),
			v2error.EcodeInvalidField,
		},
		{
			mustNewForm(t, "foo", url.Values{"sorted": []string{"?"}}),
			v2error.EcodeInvalidField,
		},
		{
			mustNewForm(t, "foo", url.Values{"sorted": []string{"x"}}),
			v2error.EcodeInvalidField,
		},
		{
			mustNewForm(t, "foo", url.Values{"wait": []string{"?!"}}),
			v2error.EcodeInvalidField,
		},
		{
			mustNewForm(t, "foo", url.Values{"wait": []string{"yes"}}),
			v2error.EcodeInvalidField,
		},
		{
			mustNewForm(t, "foo", url.Values{"prevExist": []string{"yes"}}),
			v2error.EcodeInvalidField,
		},
		{
			mustNewForm(t, "foo", url.Values{"prevExist": []string{"#2"}}),
			v2error.EcodeInvalidField,
		},
		{
			mustNewForm(t, "foo", url.Values{"dir": []string{"no"}}),
			v2error.EcodeInvalidField,
		},
		{
			mustNewForm(t, "foo", url.Values{"dir": []string{"file"}}),
			v2error.EcodeInvalidField,
		},
		{
			mustNewForm(t, "foo", url.Values{"quorum": []string{"no"}}),
			v2error.EcodeInvalidField,
		},
		{
			mustNewForm(t, "foo", url.Values{"quorum": []string{"file"}}),
			v2error.EcodeInvalidField,
		},
		{
			mustNewForm(t, "foo", url.Values{"stream": []string{"zzz"}}),
			v2error.EcodeInvalidField,
		},
		{
			mustNewForm(t, "foo", url.Values{"stream": []string{"something"}}),
			v2error.EcodeInvalidField,
		},
		// prevValue cannot be empty
		{
			mustNewForm(t, "foo", url.Values{"prevValue": []string{""}}),
			v2error.EcodePrevValueRequired,
		},
		// wait is only valid with GET requests
		{
			mustNewMethodRequest(t, "HEAD", "foo?wait=true"),
			v2error.EcodeInvalidField,
		},
		// query values are considered
		{
			mustNewRequest(t, "foo?prevExist=wrong"),
			v2error.EcodeInvalidField,
		},
		{
			mustNewRequest(t, "foo?ttl=wrong"),
			v2error.EcodeTTLNaN,
		},
		// but body takes precedence if both are specified
		{
			mustNewForm(
				t,
				"foo?ttl=12",
				url.Values{"ttl": []string{"garbage"}},
			),
			v2error.EcodeTTLNaN,
		},
		{
			mustNewForm(
				t,
				"foo?prevExist=false",
				url.Values{"prevExist": []string{"yes"}},
			),
			v2error.EcodeInvalidField,
		},
	}
	for i, tt := range tests {
		got, _, err := parseKeyRequest(tt.in, clockwork.NewFakeClock())
		if err == nil {
			t.Errorf("#%d: unexpected nil error!", i)
			continue
		}
		ee, ok := err.(*v2error.Error)
		if !ok {
			t.Errorf("#%d: err is not etcd.Error!", i)
			continue
		}
		if ee.ErrorCode != tt.wcode {
			t.Errorf("#%d: code=%d, want %v", i, ee.ErrorCode, tt.wcode)
			t.Logf("cause: %#v", ee.Cause)
		}
		if !reflect.DeepEqual(got, etcdserverpb.Request{}) {
			t.Errorf("#%d: unexpected non-empty Request: %#v", i, got)
		}
	}
}

func TestGoodParseRequest(t *testing.T) {
	fc := clockwork.NewFakeClock()
	fc.Advance(1111)
	tests := []struct {
		in      *http.Request
		w       etcdserverpb.Request
		noValue bool
	}{
		{
			// good prefix, all other values default
			mustNewRequest(t, "foo"),
			etcdserverpb.Request{
				Method: "GET",
				Path:   path.Join(etcdserver.StoreKeysPrefix, "/foo"),
			},
			false,
		},
		{
			// value specified
			mustNewForm(
				t,
				"foo",
				url.Values{"value": []string{"some_value"}},
			),
			etcdserverpb.Request{
				Method: "PUT",
				Val:    "some_value",
				Path:   path.Join(etcdserver.StoreKeysPrefix, "/foo"),
			},
			false,
		},
		{
			// prevIndex specified
			mustNewForm(
				t,
				"foo",
				url.Values{"prevIndex": []string{"98765"}},
			),
			etcdserverpb.Request{
				Method:    "PUT",
				PrevIndex: 98765,
				Path:      path.Join(etcdserver.StoreKeysPrefix, "/foo"),
			},
			false,
		},
		{
			// recursive specified
			mustNewForm(
				t,
				"foo",
				url.Values{"recursive": []string{"true"}},
			),
			etcdserverpb.Request{
				Method:    "PUT",
				Recursive: true,
				Path:      path.Join(etcdserver.StoreKeysPrefix, "/foo"),
			},
			false,
		},
		{
			// sorted specified
			mustNewForm(
				t,
				"foo",
				url.Values{"sorted": []string{"true"}},
			),
			etcdserverpb.Request{
				Method: "PUT",
				Sorted: true,
				Path:   path.Join(etcdserver.StoreKeysPrefix, "/foo"),
			},
			false,
		},
		{
			// quorum specified
			mustNewForm(
				t,
				"foo",
				url.Values{"quorum": []string{"true"}},
			),
			etcdserverpb.Request{
				Method: "PUT",
				Quorum: true,
				Path:   path.Join(etcdserver.StoreKeysPrefix, "/foo"),
			},
			false,
		},
		{
			// wait specified
			mustNewRequest(t, "foo?wait=true"),
			etcdserverpb.Request{
				Method: "GET",
				Wait:   true,
				Path:   path.Join(etcdserver.StoreKeysPrefix, "/foo"),
			},
			false,
		},
		{
			// empty TTL specified
			mustNewRequest(t, "foo?ttl="),
			etcdserverpb.Request{
				Method:     "GET",
				Path:       path.Join(etcdserver.StoreKeysPrefix, "/foo"),
				Expiration: 0,
			},
			false,
		},
		{
			// non-empty TTL specified
			mustNewRequest(t, "foo?ttl=5678"),
			etcdserverpb.Request{
				Method:     "GET",
				Path:       path.Join(etcdserver.StoreKeysPrefix, "/foo"),
				Expiration: fc.Now().Add(5678 * time.Second).UnixNano(),
			},
			false,
		},
		{
			// zero TTL specified
			mustNewRequest(t, "foo?ttl=0"),
			etcdserverpb.Request{
				Method:     "GET",
				Path:       path.Join(etcdserver.StoreKeysPrefix, "/foo"),
				Expiration: fc.Now().UnixNano(),
			},
			false,
		},
		{
			// dir specified
			mustNewRequest(t, "foo?dir=true"),
			etcdserverpb.Request{
				Method: "GET",
				Dir:    true,
				Path:   path.Join(etcdserver.StoreKeysPrefix, "/foo"),
			},
			false,
		},
		{
			// dir specified negatively
			mustNewRequest(t, "foo?dir=false"),
			etcdserverpb.Request{
				Method: "GET",
				Dir:    false,
				Path:   path.Join(etcdserver.StoreKeysPrefix, "/foo"),
			},
			false,
		},
		{
			// prevExist should be non-null if specified
			mustNewForm(
				t,
				"foo",
				url.Values{"prevExist": []string{"true"}},
			),
			etcdserverpb.Request{
				Method:    "PUT",
				PrevExist: boolp(true),
				Path:      path.Join(etcdserver.StoreKeysPrefix, "/foo"),
			},
			false,
		},
		{
			// prevExist should be non-null if specified
			mustNewForm(
				t,
				"foo",
				url.Values{"prevExist": []string{"false"}},
			),
			etcdserverpb.Request{
				Method:    "PUT",
				PrevExist: boolp(false),
				Path:      path.Join(etcdserver.StoreKeysPrefix, "/foo"),
			},
			false,
		},
		// mix various fields
		{
			mustNewForm(
				t,
				"foo",
				url.Values{
					"value":     []string{"some value"},
					"prevExist": []string{"true"},
					"prevValue": []string{"previous value"},
				},
			),
			etcdserverpb.Request{
				Method:    "PUT",
				PrevExist: boolp(true),
				PrevValue: "previous value",
				Val:       "some value",
				Path:      path.Join(etcdserver.StoreKeysPrefix, "/foo"),
			},
			false,
		},
		// query parameters should be used if given
		{
			mustNewForm(
				t,
				"foo?prevValue=woof",
				url.Values{},
			),
			etcdserverpb.Request{
				Method:    "PUT",
				PrevValue: "woof",
				Path:      path.Join(etcdserver.StoreKeysPrefix, "/foo"),
			},
			false,
		},
		// but form values should take precedence over query parameters
		{
			mustNewForm(
				t,
				"foo?prevValue=woof",
				url.Values{
					"prevValue": []string{"miaow"},
				},
			),
			etcdserverpb.Request{
				Method:    "PUT",
				PrevValue: "miaow",
				Path:      path.Join(etcdserver.StoreKeysPrefix, "/foo"),
			},
			false,
		},
		{
			// noValueOnSuccess specified
			mustNewForm(
				t,
				"foo",
				url.Values{"noValueOnSuccess": []string{"true"}},
			),
			etcdserverpb.Request{
				Method: "PUT",
				Path:   path.Join(etcdserver.StoreKeysPrefix, "/foo"),
			},
			true,
		},
	}

	for i, tt := range tests {
		got, noValueOnSuccess, err := parseKeyRequest(tt.in, fc)
		if err != nil {
			t.Errorf("#%d: err = %v, want %v", i, err, nil)
		}

		if noValueOnSuccess != tt.noValue {
			t.Errorf("#%d: noValue=%t, want %t", i, noValueOnSuccess, tt.noValue)
		}

		if !reflect.DeepEqual(got, tt.w) {
			t.Errorf("#%d: request=%#v, want %#v", i, got, tt.w)
		}
	}
}

func TestServeMembers(t *testing.T) {
	memb1 := membership.Member{ID: 12, Attributes: membership.Attributes{ClientURLs: []string{"http://localhost:8080"}}}
	memb2 := membership.Member{ID: 13, Attributes: membership.Attributes{ClientURLs: []string{"http://localhost:8081"}}}
	cluster := &fakeCluster{
		id:      1,
		members: map[uint64]*membership.Member{1: &memb1, 2: &memb2},
	}
	h := &membersHandler{
		lg:      zap.NewExample(),
		server:  &serverRecorder{},
		clock:   clockwork.NewFakeClock(),
		cluster: cluster,
	}

	wmc := string(`{"members":[{"id":"c","name":"","peerURLs":[],"clientURLs":["http://localhost:8080"]},{"id":"d","name":"","peerURLs":[],"clientURLs":["http://localhost:8081"]}]}`)

	tests := []struct {
		path  string
		wcode int
		wct   string
		wbody string
	}{
		{membersPrefix, http.StatusOK, "application/json", wmc + "\n"},
		{membersPrefix + "/", http.StatusOK, "application/json", wmc + "\n"},
		{path.Join(membersPrefix, "100"), http.StatusNotFound, "application/json", `{"message":"Not found"}`},
		{path.Join(membersPrefix, "foobar"), http.StatusNotFound, "application/json", `{"message":"Not found"}`},
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
		gcid := rw.Header().Get("X-Etcd-Cluster-ID")
		wcid := cluster.ID().String()
		if gcid != wcid {
			t.Errorf("#%d: cid = %s, want %s", i, gcid, wcid)
		}
		if rw.Body.String() != tt.wbody {
			t.Errorf("#%d: body = %q, want %q", i, rw.Body.String(), tt.wbody)
		}
	}
}

// TODO: consolidate **ALL** fake server implementations and add no leader test case.
func TestServeLeader(t *testing.T) {
	memb1 := membership.Member{ID: 1, Attributes: membership.Attributes{ClientURLs: []string{"http://localhost:8080"}}}
	memb2 := membership.Member{ID: 2, Attributes: membership.Attributes{ClientURLs: []string{"http://localhost:8081"}}}
	cluster := &fakeCluster{
		id:      1,
		members: map[uint64]*membership.Member{1: &memb1, 2: &memb2},
	}
	h := &membersHandler{
		lg:      zap.NewExample(),
		server:  &serverRecorder{},
		clock:   clockwork.NewFakeClock(),
		cluster: cluster,
	}

	wmc := string(`{"id":"1","name":"","peerURLs":[],"clientURLs":["http://localhost:8080"]}`)

	tests := []struct {
		path  string
		wcode int
		wct   string
		wbody string
	}{
		{membersPrefix + "leader", http.StatusOK, "application/json", wmc + "\n"},
		// TODO: add no leader case
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
		gcid := rw.Header().Get("X-Etcd-Cluster-ID")
		wcid := cluster.ID().String()
		if gcid != wcid {
			t.Errorf("#%d: cid = %s, want %s", i, gcid, wcid)
		}
		if rw.Body.String() != tt.wbody {
			t.Errorf("#%d: body = %q, want %q", i, rw.Body.String(), tt.wbody)
		}
	}
}

func TestServeMembersCreate(t *testing.T) {
	u := testutil.MustNewURL(t, membersPrefix)
	b := []byte(`{"peerURLs":["http://127.0.0.1:1"]}`)
	req, err := http.NewRequest("POST", u.String(), bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	s := &serverRecorder{}
	h := &membersHandler{
		lg:      zap.NewExample(),
		server:  s,
		clock:   clockwork.NewFakeClock(),
		cluster: &fakeCluster{id: 1},
	}
	rw := httptest.NewRecorder()

	h.ServeHTTP(rw, req)

	wcode := http.StatusCreated
	if rw.Code != wcode {
		t.Errorf("code=%d, want %d", rw.Code, wcode)
	}

	wct := "application/json"
	if gct := rw.Header().Get("Content-Type"); gct != wct {
		t.Errorf("content-type = %s, want %s", gct, wct)
	}
	gcid := rw.Header().Get("X-Etcd-Cluster-ID")
	wcid := h.cluster.ID().String()
	if gcid != wcid {
		t.Errorf("cid = %s, want %s", gcid, wcid)
	}

	wb := `{"id":"c29b431f04be0bc7","name":"","peerURLs":["http://127.0.0.1:1"],"clientURLs":[]}` + "\n"
	g := rw.Body.String()
	if g != wb {
		t.Errorf("got body=%q, want %q", g, wb)
	}

	wm := membership.Member{
		ID: 14022875665250782151,
		RaftAttributes: membership.RaftAttributes{
			PeerURLs: []string{"http://127.0.0.1:1"},
		},
	}

	wactions := []action{{name: "AddMember", params: []interface{}{wm}}}
	if !reflect.DeepEqual(s.actions, wactions) {
		t.Errorf("actions = %+v, want %+v", s.actions, wactions)
	}
}

func TestServeMembersDelete(t *testing.T) {
	req := &http.Request{
		Method: "DELETE",
		URL:    testutil.MustNewURL(t, path.Join(membersPrefix, "BEEF")),
	}
	s := &serverRecorder{}
	h := &membersHandler{
		lg:      zap.NewExample(),
		server:  s,
		cluster: &fakeCluster{id: 1},
	}
	rw := httptest.NewRecorder()

	h.ServeHTTP(rw, req)

	wcode := http.StatusNoContent
	if rw.Code != wcode {
		t.Errorf("code=%d, want %d", rw.Code, wcode)
	}
	gcid := rw.Header().Get("X-Etcd-Cluster-ID")
	wcid := h.cluster.ID().String()
	if gcid != wcid {
		t.Errorf("cid = %s, want %s", gcid, wcid)
	}
	g := rw.Body.String()
	if g != "" {
		t.Errorf("got body=%q, want %q", g, "")
	}
	wactions := []action{{name: "RemoveMember", params: []interface{}{uint64(0xBEEF)}}}
	if !reflect.DeepEqual(s.actions, wactions) {
		t.Errorf("actions = %+v, want %+v", s.actions, wactions)
	}
}

func TestServeMembersUpdate(t *testing.T) {
	u := testutil.MustNewURL(t, path.Join(membersPrefix, "1"))
	b := []byte(`{"peerURLs":["http://127.0.0.1:1"]}`)
	req, err := http.NewRequest("PUT", u.String(), bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	s := &serverRecorder{}
	h := &membersHandler{
		lg:      zap.NewExample(),
		server:  s,
		clock:   clockwork.NewFakeClock(),
		cluster: &fakeCluster{id: 1},
	}
	rw := httptest.NewRecorder()

	h.ServeHTTP(rw, req)

	wcode := http.StatusNoContent
	if rw.Code != wcode {
		t.Errorf("code=%d, want %d", rw.Code, wcode)
	}

	gcid := rw.Header().Get("X-Etcd-Cluster-ID")
	wcid := h.cluster.ID().String()
	if gcid != wcid {
		t.Errorf("cid = %s, want %s", gcid, wcid)
	}

	wm := membership.Member{
		ID: 1,
		RaftAttributes: membership.RaftAttributes{
			PeerURLs: []string{"http://127.0.0.1:1"},
		},
	}

	wactions := []action{{name: "UpdateMember", params: []interface{}{wm}}}
	if !reflect.DeepEqual(s.actions, wactions) {
		t.Errorf("actions = %+v, want %+v", s.actions, wactions)
	}
}

func TestServeMembersFail(t *testing.T) {
	tests := []struct {
		req    *http.Request
		server etcdserver.ServerV2

		wcode int
	}{
		{
			// bad method
			&http.Request{
				Method: "CONNECT",
			},
			&resServer{},

			http.StatusMethodNotAllowed,
		},
		{
			// bad method
			&http.Request{
				Method: "TRACE",
			},
			&resServer{},

			http.StatusMethodNotAllowed,
		},
		{
			// parse body error
			&http.Request{
				URL:    testutil.MustNewURL(t, membersPrefix),
				Method: "POST",
				Body:   ioutil.NopCloser(strings.NewReader("bad json")),
				Header: map[string][]string{"Content-Type": {"application/json"}},
			},
			&resServer{},

			http.StatusBadRequest,
		},
		{
			// bad content type
			&http.Request{
				URL:    testutil.MustNewURL(t, membersPrefix),
				Method: "POST",
				Body:   ioutil.NopCloser(strings.NewReader(`{"PeerURLs": ["http://127.0.0.1:1"]}`)),
				Header: map[string][]string{"Content-Type": {"application/bad"}},
			},
			&errServer{},

			http.StatusUnsupportedMediaType,
		},
		{
			// bad url
			&http.Request{
				URL:    testutil.MustNewURL(t, membersPrefix),
				Method: "POST",
				Body:   ioutil.NopCloser(strings.NewReader(`{"PeerURLs": ["http://a"]}`)),
				Header: map[string][]string{"Content-Type": {"application/json"}},
			},
			&errServer{},

			http.StatusBadRequest,
		},
		{
			// etcdserver.AddMember error
			&http.Request{
				URL:    testutil.MustNewURL(t, membersPrefix),
				Method: "POST",
				Body:   ioutil.NopCloser(strings.NewReader(`{"PeerURLs": ["http://127.0.0.1:1"]}`)),
				Header: map[string][]string{"Content-Type": {"application/json"}},
			},
			&errServer{
				err: errors.New("Error while adding a member"),
			},

			http.StatusInternalServerError,
		},
		{
			// etcdserver.AddMember error
			&http.Request{
				URL:    testutil.MustNewURL(t, membersPrefix),
				Method: "POST",
				Body:   ioutil.NopCloser(strings.NewReader(`{"PeerURLs": ["http://127.0.0.1:1"]}`)),
				Header: map[string][]string{"Content-Type": {"application/json"}},
			},
			&errServer{
				err: membership.ErrIDExists,
			},

			http.StatusConflict,
		},
		{
			// etcdserver.AddMember error
			&http.Request{
				URL:    testutil.MustNewURL(t, membersPrefix),
				Method: "POST",
				Body:   ioutil.NopCloser(strings.NewReader(`{"PeerURLs": ["http://127.0.0.1:1"]}`)),
				Header: map[string][]string{"Content-Type": {"application/json"}},
			},
			&errServer{
				err: membership.ErrPeerURLexists,
			},

			http.StatusConflict,
		},
		{
			// etcdserver.RemoveMember error with arbitrary server error
			&http.Request{
				URL:    testutil.MustNewURL(t, path.Join(membersPrefix, "1")),
				Method: "DELETE",
			},
			&errServer{
				err: errors.New("Error while removing member"),
			},

			http.StatusInternalServerError,
		},
		{
			// etcdserver.RemoveMember error with previously removed ID
			&http.Request{
				URL:    testutil.MustNewURL(t, path.Join(membersPrefix, "0")),
				Method: "DELETE",
			},
			&errServer{
				err: membership.ErrIDRemoved,
			},

			http.StatusGone,
		},
		{
			// etcdserver.RemoveMember error with nonexistent ID
			&http.Request{
				URL:    testutil.MustNewURL(t, path.Join(membersPrefix, "0")),
				Method: "DELETE",
			},
			&errServer{
				err: membership.ErrIDNotFound,
			},

			http.StatusNotFound,
		},
		{
			// etcdserver.RemoveMember error with badly formed ID
			&http.Request{
				URL:    testutil.MustNewURL(t, path.Join(membersPrefix, "bad_id")),
				Method: "DELETE",
			},
			nil,

			http.StatusNotFound,
		},
		{
			// etcdserver.RemoveMember with no ID
			&http.Request{
				URL:    testutil.MustNewURL(t, membersPrefix),
				Method: "DELETE",
			},
			nil,

			http.StatusMethodNotAllowed,
		},
		{
			// parse body error
			&http.Request{
				URL:    testutil.MustNewURL(t, path.Join(membersPrefix, "0")),
				Method: "PUT",
				Body:   ioutil.NopCloser(strings.NewReader("bad json")),
				Header: map[string][]string{"Content-Type": {"application/json"}},
			},
			&resServer{},

			http.StatusBadRequest,
		},
		{
			// bad content type
			&http.Request{
				URL:    testutil.MustNewURL(t, path.Join(membersPrefix, "0")),
				Method: "PUT",
				Body:   ioutil.NopCloser(strings.NewReader(`{"PeerURLs": ["http://127.0.0.1:1"]}`)),
				Header: map[string][]string{"Content-Type": {"application/bad"}},
			},
			&errServer{},

			http.StatusUnsupportedMediaType,
		},
		{
			// bad url
			&http.Request{
				URL:    testutil.MustNewURL(t, path.Join(membersPrefix, "0")),
				Method: "PUT",
				Body:   ioutil.NopCloser(strings.NewReader(`{"PeerURLs": ["http://a"]}`)),
				Header: map[string][]string{"Content-Type": {"application/json"}},
			},
			&errServer{},

			http.StatusBadRequest,
		},
		{
			// etcdserver.UpdateMember error
			&http.Request{
				URL:    testutil.MustNewURL(t, path.Join(membersPrefix, "0")),
				Method: "PUT",
				Body:   ioutil.NopCloser(strings.NewReader(`{"PeerURLs": ["http://127.0.0.1:1"]}`)),
				Header: map[string][]string{"Content-Type": {"application/json"}},
			},
			&errServer{
				err: errors.New("blah"),
			},

			http.StatusInternalServerError,
		},
		{
			// etcdserver.UpdateMember error
			&http.Request{
				URL:    testutil.MustNewURL(t, path.Join(membersPrefix, "0")),
				Method: "PUT",
				Body:   ioutil.NopCloser(strings.NewReader(`{"PeerURLs": ["http://127.0.0.1:1"]}`)),
				Header: map[string][]string{"Content-Type": {"application/json"}},
			},
			&errServer{
				err: membership.ErrPeerURLexists,
			},

			http.StatusConflict,
		},
		{
			// etcdserver.UpdateMember error
			&http.Request{
				URL:    testutil.MustNewURL(t, path.Join(membersPrefix, "0")),
				Method: "PUT",
				Body:   ioutil.NopCloser(strings.NewReader(`{"PeerURLs": ["http://127.0.0.1:1"]}`)),
				Header: map[string][]string{"Content-Type": {"application/json"}},
			},
			&errServer{
				err: membership.ErrIDNotFound,
			},

			http.StatusNotFound,
		},
		{
			// etcdserver.UpdateMember error with badly formed ID
			&http.Request{
				URL:    testutil.MustNewURL(t, path.Join(membersPrefix, "bad_id")),
				Method: "PUT",
			},
			nil,

			http.StatusNotFound,
		},
		{
			// etcdserver.UpdateMember with no ID
			&http.Request{
				URL:    testutil.MustNewURL(t, membersPrefix),
				Method: "PUT",
			},
			nil,

			http.StatusMethodNotAllowed,
		},
	}
	for i, tt := range tests {
		h := &membersHandler{
			lg:      zap.NewExample(),
			server:  tt.server,
			cluster: &fakeCluster{id: 1},
			clock:   clockwork.NewFakeClock(),
		}
		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, tt.req)
		if rw.Code != tt.wcode {
			t.Errorf("#%d: code=%d, want %d", i, rw.Code, tt.wcode)
		}
		if rw.Code != http.StatusMethodNotAllowed {
			gcid := rw.Header().Get("X-Etcd-Cluster-ID")
			wcid := h.cluster.ID().String()
			if gcid != wcid {
				t.Errorf("#%d: cid = %s, want %s", i, gcid, wcid)
			}
		}
	}
}

func TestWriteEvent(t *testing.T) {
	// nil event should not panic
	rec := httptest.NewRecorder()
	writeKeyEvent(rec, etcdserver.Response{}, false)
	h := rec.Header()
	if len(h) > 0 {
		t.Fatalf("unexpected non-empty headers: %#v", h)
	}
	b := rec.Body.String()
	if len(b) > 0 {
		t.Fatalf("unexpected non-empty body: %q", b)
	}

	tests := []struct {
		ev      *v2store.Event
		noValue bool
		idx     string
		// TODO(jonboulle): check body as well as just status code
		code int
		err  error
	}{
		// standard case, standard 200 response
		{
			&v2store.Event{
				Action:   v2store.Get,
				Node:     &v2store.NodeExtern{},
				PrevNode: &v2store.NodeExtern{},
			},
			false,
			"0",
			http.StatusOK,
			nil,
		},
		// check new nodes return StatusCreated
		{
			&v2store.Event{
				Action:   v2store.Create,
				Node:     &v2store.NodeExtern{},
				PrevNode: &v2store.NodeExtern{},
			},
			false,
			"0",
			http.StatusCreated,
			nil,
		},
	}

	for i, tt := range tests {
		rw := httptest.NewRecorder()
		resp := etcdserver.Response{Event: tt.ev, Term: 5, Index: 100}
		writeKeyEvent(rw, resp, tt.noValue)
		if gct := rw.Header().Get("Content-Type"); gct != "application/json" {
			t.Errorf("case %d: bad Content-Type: got %q, want application/json", i, gct)
		}
		if gri := rw.Header().Get("X-Raft-Index"); gri != "100" {
			t.Errorf("case %d: bad X-Raft-Index header: got %s, want %s", i, gri, "100")
		}
		if grt := rw.Header().Get("X-Raft-Term"); grt != "5" {
			t.Errorf("case %d: bad X-Raft-Term header: got %s, want %s", i, grt, "5")
		}
		if gei := rw.Header().Get("X-Etcd-Index"); gei != tt.idx {
			t.Errorf("case %d: bad X-Etcd-Index header: got %s, want %s", i, gei, tt.idx)
		}
		if rw.Code != tt.code {
			t.Errorf("case %d: bad response code: got %d, want %v", i, rw.Code, tt.code)
		}

	}
}

func TestV2DMachinesEndpoint(t *testing.T) {
	tests := []struct {
		method string
		wcode  int
	}{
		{"GET", http.StatusOK},
		{"HEAD", http.StatusOK},
		{"POST", http.StatusMethodNotAllowed},
	}

	m := &machinesHandler{cluster: &fakeCluster{}}
	s := httptest.NewServer(m)
	defer s.Close()

	for _, tt := range tests {
		req, err := http.NewRequest(tt.method, s.URL+machinesPrefix, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}

		if resp.StatusCode != tt.wcode {
			t.Errorf("StatusCode = %d, expected %d", resp.StatusCode, tt.wcode)
		}
	}
}

func TestServeMachines(t *testing.T) {
	cluster := &fakeCluster{
		clientURLs: []string{"http://localhost:8080", "http://localhost:8081", "http://localhost:8082"},
	}
	writer := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	h := &machinesHandler{cluster: cluster}
	h.ServeHTTP(writer, req)
	w := "http://localhost:8080, http://localhost:8081, http://localhost:8082"
	if g := writer.Body.String(); g != w {
		t.Errorf("body = %s, want %s", g, w)
	}
	if writer.Code != http.StatusOK {
		t.Errorf("code = %d, want %d", writer.Code, http.StatusOK)
	}
}

func TestGetID(t *testing.T) {
	tests := []struct {
		path string

		wok   bool
		wid   types.ID
		wcode int
	}{
		{
			"123",
			true, 0x123, http.StatusOK,
		},
		{
			"bad_id",
			false, 0, http.StatusNotFound,
		},
		{
			"",
			false, 0, http.StatusMethodNotAllowed,
		},
	}

	for i, tt := range tests {
		w := httptest.NewRecorder()
		id, ok := getID(zap.NewExample(), tt.path, w)
		if id != tt.wid {
			t.Errorf("#%d: id = %d, want %d", i, id, tt.wid)
		}
		if ok != tt.wok {
			t.Errorf("#%d: ok = %t, want %t", i, ok, tt.wok)
		}
		if w.Code != tt.wcode {
			t.Errorf("#%d code = %d, want %d", i, w.Code, tt.wcode)
		}
	}
}

type dummyStats struct {
	data []byte
}

func (ds *dummyStats) SelfStats() []byte                 { return ds.data }
func (ds *dummyStats) LeaderStats() []byte               { return ds.data }
func (ds *dummyStats) StoreStats() []byte                { return ds.data }
func (ds *dummyStats) UpdateRecvApp(_ types.ID, _ int64) {}

func TestServeSelfStats(t *testing.T) {
	wb := []byte("some statistics")
	w := string(wb)
	sh := &statsHandler{
		lg:    zap.NewExample(),
		stats: &dummyStats{data: wb},
	}
	rw := httptest.NewRecorder()
	sh.serveSelf(rw, &http.Request{Method: "GET"})
	if rw.Code != http.StatusOK {
		t.Errorf("code = %d, want %d", rw.Code, http.StatusOK)
	}
	wct := "application/json"
	if gct := rw.Header().Get("Content-Type"); gct != wct {
		t.Errorf("Content-Type = %q, want %q", gct, wct)
	}
	if g := rw.Body.String(); g != w {
		t.Errorf("body = %s, want %s", g, w)
	}
}

func TestSelfServeStatsBad(t *testing.T) {
	for _, m := range []string{"PUT", "POST", "DELETE"} {
		sh := &statsHandler{lg: zap.NewExample()}
		rw := httptest.NewRecorder()
		sh.serveSelf(
			rw,
			&http.Request{
				Method: m,
			},
		)
		if rw.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: code=%d, want %d", m, rw.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestLeaderServeStatsBad(t *testing.T) {
	for _, m := range []string{"PUT", "POST", "DELETE"} {
		sh := &statsHandler{lg: zap.NewExample()}
		rw := httptest.NewRecorder()
		sh.serveLeader(
			rw,
			&http.Request{
				Method: m,
			},
		)
		if rw.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: code=%d, want %d", m, rw.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestServeLeaderStats(t *testing.T) {
	wb := []byte("some statistics")
	w := string(wb)
	sh := &statsHandler{
		lg:    zap.NewExample(),
		stats: &dummyStats{data: wb},
	}
	rw := httptest.NewRecorder()
	sh.serveLeader(rw, &http.Request{Method: "GET"})
	if rw.Code != http.StatusOK {
		t.Errorf("code = %d, want %d", rw.Code, http.StatusOK)
	}
	wct := "application/json"
	if gct := rw.Header().Get("Content-Type"); gct != wct {
		t.Errorf("Content-Type = %q, want %q", gct, wct)
	}
	if g := rw.Body.String(); g != w {
		t.Errorf("body = %s, want %s", g, w)
	}
}

func TestServeStoreStats(t *testing.T) {
	wb := []byte("some statistics")
	w := string(wb)
	sh := &statsHandler{
		lg:    zap.NewExample(),
		stats: &dummyStats{data: wb},
	}
	rw := httptest.NewRecorder()
	sh.serveStore(rw, &http.Request{Method: "GET"})
	if rw.Code != http.StatusOK {
		t.Errorf("code = %d, want %d", rw.Code, http.StatusOK)
	}
	wct := "application/json"
	if gct := rw.Header().Get("Content-Type"); gct != wct {
		t.Errorf("Content-Type = %q, want %q", gct, wct)
	}
	if g := rw.Body.String(); g != w {
		t.Errorf("body = %s, want %s", g, w)
	}

}

func TestBadServeKeys(t *testing.T) {
	testBadCases := []struct {
		req    *http.Request
		server etcdserver.ServerV2

		wcode int
		wbody string
	}{
		{
			// bad method
			&http.Request{
				Method: "CONNECT",
			},
			&resServer{},

			http.StatusMethodNotAllowed,
			"Method Not Allowed",
		},
		{
			// bad method
			&http.Request{
				Method: "TRACE",
			},
			&resServer{},

			http.StatusMethodNotAllowed,
			"Method Not Allowed",
		},
		{
			// parseRequest error
			&http.Request{
				Body:   nil,
				Method: "PUT",
			},
			&resServer{},

			http.StatusBadRequest,
			`{"errorCode":210,"message":"Invalid POST form","cause":"missing form body","index":0}`,
		},
		{
			// etcdserver.Server error
			mustNewRequest(t, "foo"),
			&errServer{
				err: errors.New("Internal Server Error"),
			},

			http.StatusInternalServerError,
			`{"errorCode":300,"message":"Raft Internal Error","cause":"Internal Server Error","index":0}`,
		},
		{
			// etcdserver.Server etcd error
			mustNewRequest(t, "foo"),
			&errServer{
				err: v2error.NewError(v2error.EcodeKeyNotFound, "/1/pant", 0),
			},

			http.StatusNotFound,
			`{"errorCode":100,"message":"Key not found","cause":"/pant","index":0}`,
		},
		{
			// non-event/watcher response from etcdserver.Server
			mustNewRequest(t, "foo"),
			&resServer{
				res: etcdserver.Response{},
			},

			http.StatusInternalServerError,
			`{"errorCode":300,"message":"Raft Internal Error","cause":"received response with no Event/Watcher","index":0}`,
		},
	}
	for i, tt := range testBadCases {
		h := &keysHandler{
			lg:      zap.NewExample(),
			timeout: 0, // context times out immediately
			server:  tt.server,
			cluster: &fakeCluster{id: 1},
		}
		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, tt.req)
		if rw.Code != tt.wcode {
			t.Errorf("#%d: got code=%d, want %d", i, rw.Code, tt.wcode)
		}
		if rw.Code != http.StatusMethodNotAllowed {
			gcid := rw.Header().Get("X-Etcd-Cluster-ID")
			wcid := h.cluster.ID().String()
			if gcid != wcid {
				t.Errorf("#%d: cid = %s, want %s", i, gcid, wcid)
			}
		}
		if g := strings.TrimSuffix(rw.Body.String(), "\n"); g != tt.wbody {
			t.Errorf("#%d: body = %s, want %s", i, g, tt.wbody)
		}
	}
}

func TestServeKeysGood(t *testing.T) {
	tests := []struct {
		req   *http.Request
		wcode int
	}{
		{
			mustNewMethodRequest(t, "HEAD", "foo"),
			http.StatusOK,
		},
		{
			mustNewMethodRequest(t, "GET", "foo"),
			http.StatusOK,
		},
		{
			mustNewForm(t, "foo", url.Values{"value": []string{"bar"}}),
			http.StatusOK,
		},
		{
			mustNewMethodRequest(t, "DELETE", "foo"),
			http.StatusOK,
		},
		{
			mustNewPostForm(t, "foo", url.Values{"value": []string{"bar"}}),
			http.StatusOK,
		},
	}
	server := &resServer{
		res: etcdserver.Response{
			Event: &v2store.Event{
				Action: v2store.Get,
				Node:   &v2store.NodeExtern{},
			},
		},
	}
	for i, tt := range tests {
		h := &keysHandler{
			lg:      zap.NewExample(),
			timeout: time.Hour,
			server:  server,
			cluster: &fakeCluster{id: 1},
		}
		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, tt.req)
		if rw.Code != tt.wcode {
			t.Errorf("#%d: got code=%d, want %d", i, rw.Code, tt.wcode)
		}
	}
}

func TestServeKeysEvent(t *testing.T) {
	tests := []struct {
		req   *http.Request
		rsp   etcdserver.Response
		wcode int
		event *v2store.Event
	}{
		{
			mustNewRequest(t, "foo"),
			etcdserver.Response{
				Event: &v2store.Event{
					Action: v2store.Get,
					Node:   &v2store.NodeExtern{},
				},
			},
			http.StatusOK,
			&v2store.Event{
				Action: v2store.Get,
				Node:   &v2store.NodeExtern{},
			},
		},
		{
			mustNewForm(
				t,
				"foo",
				url.Values{"noValueOnSuccess": []string{"true"}},
			),
			etcdserver.Response{
				Event: &v2store.Event{
					Action: v2store.CompareAndSwap,
					Node:   &v2store.NodeExtern{},
				},
			},
			http.StatusOK,
			&v2store.Event{
				Action: v2store.CompareAndSwap,
				Node:   nil,
			},
		},
	}

	server := &resServer{}
	h := &keysHandler{
		lg:      zap.NewExample(),
		timeout: time.Hour,
		server:  server,
		cluster: &fakeCluster{id: 1},
	}

	for _, tt := range tests {
		server.res = tt.rsp
		rw := httptest.NewRecorder()
		h.ServeHTTP(rw, tt.req)

		wbody := mustMarshalEvent(
			t,
			tt.event,
		)

		if rw.Code != tt.wcode {
			t.Errorf("got code=%d, want %d", rw.Code, tt.wcode)
		}
		gcid := rw.Header().Get("X-Etcd-Cluster-ID")
		wcid := h.cluster.ID().String()
		if gcid != wcid {
			t.Errorf("cid = %s, want %s", gcid, wcid)
		}
		g := rw.Body.String()
		if g != wbody {
			t.Errorf("got body=%#v, want %#v", g, wbody)
		}
	}
}

func TestServeKeysWatch(t *testing.T) {
	req := mustNewRequest(t, "/foo/bar")
	ec := make(chan *v2store.Event)
	dw := &dummyWatcher{
		echan: ec,
	}
	server := &resServer{
		res: etcdserver.Response{
			Watcher: dw,
		},
	}
	h := &keysHandler{
		lg:      zap.NewExample(),
		timeout: time.Hour,
		server:  server,
		cluster: &fakeCluster{id: 1},
	}
	go func() {
		ec <- &v2store.Event{
			Action: v2store.Get,
			Node:   &v2store.NodeExtern{},
		}
	}()
	rw := httptest.NewRecorder()

	h.ServeHTTP(rw, req)

	wcode := http.StatusOK
	wbody := mustMarshalEvent(
		t,
		&v2store.Event{
			Action: v2store.Get,
			Node:   &v2store.NodeExtern{},
		},
	)

	if rw.Code != wcode {
		t.Errorf("got code=%d, want %d", rw.Code, wcode)
	}
	gcid := rw.Header().Get("X-Etcd-Cluster-ID")
	wcid := h.cluster.ID().String()
	if gcid != wcid {
		t.Errorf("cid = %s, want %s", gcid, wcid)
	}
	g := rw.Body.String()
	if g != wbody {
		t.Errorf("got body=%#v, want %#v", g, wbody)
	}
}

type recordingCloseNotifier struct {
	*httptest.ResponseRecorder
	cn chan bool
}

func (rcn *recordingCloseNotifier) CloseNotify() <-chan bool {
	return rcn.cn
}

func TestHandleWatch(t *testing.T) {
	defaultRwRr := func() (http.ResponseWriter, *httptest.ResponseRecorder) {
		r := httptest.NewRecorder()
		return r, r
	}
	noopEv := func(chan *v2store.Event) {}

	tests := []struct {
		getCtx   func() context.Context
		getRwRr  func() (http.ResponseWriter, *httptest.ResponseRecorder)
		doToChan func(chan *v2store.Event)

		wbody string
	}{
		{
			// Normal case: one event
			context.Background,
			defaultRwRr,
			func(ch chan *v2store.Event) {
				ch <- &v2store.Event{
					Action: v2store.Get,
					Node:   &v2store.NodeExtern{},
				}
			},

			mustMarshalEvent(
				t,
				&v2store.Event{
					Action: v2store.Get,
					Node:   &v2store.NodeExtern{},
				},
			),
		},
		{
			// Channel is closed, no event
			context.Background,
			defaultRwRr,
			func(ch chan *v2store.Event) {
				close(ch)
			},

			"",
		},
		{
			// Simulate a timed-out context
			func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			defaultRwRr,
			noopEv,

			"",
		},
		{
			// Close-notifying request
			context.Background,
			func() (http.ResponseWriter, *httptest.ResponseRecorder) {
				rw := &recordingCloseNotifier{
					ResponseRecorder: httptest.NewRecorder(),
					cn:               make(chan bool, 1),
				}
				rw.cn <- true
				return rw, rw.ResponseRecorder
			},
			noopEv,

			"",
		},
	}

	for i, tt := range tests {
		rw, rr := tt.getRwRr()
		wa := &dummyWatcher{
			echan: make(chan *v2store.Event, 1),
			sidx:  10,
		}
		tt.doToChan(wa.echan)

		resp := etcdserver.Response{Term: 5, Index: 100, Watcher: wa}
		handleKeyWatch(tt.getCtx(), zap.NewExample(), rw, resp, false)

		wcode := http.StatusOK
		wct := "application/json"
		wei := "10"
		wri := "100"
		wrt := "5"

		if rr.Code != wcode {
			t.Errorf("#%d: got code=%d, want %d", i, rr.Code, wcode)
		}
		h := rr.Header()
		if ct := h.Get("Content-Type"); ct != wct {
			t.Errorf("#%d: Content-Type=%q, want %q", i, ct, wct)
		}
		if ei := h.Get("X-Etcd-Index"); ei != wei {
			t.Errorf("#%d: X-Etcd-Index=%q, want %q", i, ei, wei)
		}
		if ri := h.Get("X-Raft-Index"); ri != wri {
			t.Errorf("#%d: X-Raft-Index=%q, want %q", i, ri, wri)
		}
		if rt := h.Get("X-Raft-Term"); rt != wrt {
			t.Errorf("#%d: X-Raft-Term=%q, want %q", i, rt, wrt)
		}
		g := rr.Body.String()
		if g != tt.wbody {
			t.Errorf("#%d: got body=%#v, want %#v", i, g, tt.wbody)
		}
	}
}

func TestHandleWatchStreaming(t *testing.T) {
	rw := &flushingRecorder{
		httptest.NewRecorder(),
		make(chan struct{}, 1),
	}
	wa := &dummyWatcher{
		echan: make(chan *v2store.Event),
	}

	// Launch the streaming handler in the background with a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		resp := etcdserver.Response{Watcher: wa}
		handleKeyWatch(ctx, zap.NewExample(), rw, resp, true)
		close(done)
	}()

	// Expect one Flush for the headers etc.
	select {
	case <-rw.ch:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for flush")
	}

	// Expect headers but no body
	wcode := http.StatusOK
	wct := "application/json"
	wbody := ""

	if rw.Code != wcode {
		t.Errorf("got code=%d, want %d", rw.Code, wcode)
	}
	h := rw.Header()
	if ct := h.Get("Content-Type"); ct != wct {
		t.Errorf("Content-Type=%q, want %q", ct, wct)
	}
	g := rw.Body.String()
	if g != wbody {
		t.Errorf("got body=%#v, want %#v", g, wbody)
	}

	// Now send the first event
	select {
	case wa.echan <- &v2store.Event{
		Action: v2store.Get,
		Node:   &v2store.NodeExtern{},
	}:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for send")
	}

	// Wait for it to be flushed...
	select {
	case <-rw.ch:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for flush")
	}

	// And check the body is as expected
	wbody = mustMarshalEvent(
		t,
		&v2store.Event{
			Action: v2store.Get,
			Node:   &v2store.NodeExtern{},
		},
	)
	g = rw.Body.String()
	if g != wbody {
		t.Errorf("got body=%#v, want %#v", g, wbody)
	}

	// Rinse and repeat
	select {
	case wa.echan <- &v2store.Event{
		Action: v2store.Get,
		Node:   &v2store.NodeExtern{},
	}:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for send")
	}

	select {
	case <-rw.ch:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for flush")
	}

	// This time, we expect to see both events
	wbody = wbody + wbody
	g = rw.Body.String()
	if g != wbody {
		t.Errorf("got body=%#v, want %#v", g, wbody)
	}

	// Finally, time out the connection and ensure the serving goroutine returns
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for done")
	}
}

func TestTrimEventPrefix(t *testing.T) {
	pre := "/abc"
	tests := []struct {
		ev  *v2store.Event
		wev *v2store.Event
	}{
		{
			nil,
			nil,
		},
		{
			&v2store.Event{},
			&v2store.Event{},
		},
		{
			&v2store.Event{Node: &v2store.NodeExtern{Key: "/abc/def"}},
			&v2store.Event{Node: &v2store.NodeExtern{Key: "/def"}},
		},
		{
			&v2store.Event{PrevNode: &v2store.NodeExtern{Key: "/abc/ghi"}},
			&v2store.Event{PrevNode: &v2store.NodeExtern{Key: "/ghi"}},
		},
		{
			&v2store.Event{
				Node:     &v2store.NodeExtern{Key: "/abc/def"},
				PrevNode: &v2store.NodeExtern{Key: "/abc/ghi"},
			},
			&v2store.Event{
				Node:     &v2store.NodeExtern{Key: "/def"},
				PrevNode: &v2store.NodeExtern{Key: "/ghi"},
			},
		},
	}
	for i, tt := range tests {
		ev := trimEventPrefix(tt.ev, pre)
		if !reflect.DeepEqual(ev, tt.wev) {
			t.Errorf("#%d: event = %+v, want %+v", i, ev, tt.wev)
		}
	}
}

func TestTrimNodeExternPrefix(t *testing.T) {
	pre := "/abc"
	tests := []struct {
		n  *v2store.NodeExtern
		wn *v2store.NodeExtern
	}{
		{
			nil,
			nil,
		},
		{
			&v2store.NodeExtern{Key: "/abc/def"},
			&v2store.NodeExtern{Key: "/def"},
		},
		{
			&v2store.NodeExtern{
				Key: "/abc/def",
				Nodes: []*v2store.NodeExtern{
					{Key: "/abc/def/1"},
					{Key: "/abc/def/2"},
				},
			},
			&v2store.NodeExtern{
				Key: "/def",
				Nodes: []*v2store.NodeExtern{
					{Key: "/def/1"},
					{Key: "/def/2"},
				},
			},
		},
	}
	for i, tt := range tests {
		trimNodeExternPrefix(tt.n, pre)
		if !reflect.DeepEqual(tt.n, tt.wn) {
			t.Errorf("#%d: node = %+v, want %+v", i, tt.n, tt.wn)
		}
	}
}

func TestTrimPrefix(t *testing.T) {
	tests := []struct {
		in     string
		prefix string
		w      string
	}{
		{"/v2/members", "/v2/members", ""},
		{"/v2/members/", "/v2/members", ""},
		{"/v2/members/foo", "/v2/members", "foo"},
	}
	for i, tt := range tests {
		if g := trimPrefix(tt.in, tt.prefix); g != tt.w {
			t.Errorf("#%d: trimPrefix = %q, want %q", i, g, tt.w)
		}
	}
}

func TestNewMemberCollection(t *testing.T) {
	fixture := []*membership.Member{
		{
			ID:             12,
			Attributes:     membership.Attributes{ClientURLs: []string{"http://localhost:8080", "http://localhost:8081"}},
			RaftAttributes: membership.RaftAttributes{PeerURLs: []string{"http://localhost:8082", "http://localhost:8083"}},
		},
		{
			ID:             13,
			Attributes:     membership.Attributes{ClientURLs: []string{"http://localhost:9090", "http://localhost:9091"}},
			RaftAttributes: membership.RaftAttributes{PeerURLs: []string{"http://localhost:9092", "http://localhost:9093"}},
		},
	}
	got := newMemberCollection(fixture)

	want := httptypes.MemberCollection([]httptypes.Member{
		{
			ID:         "c",
			ClientURLs: []string{"http://localhost:8080", "http://localhost:8081"},
			PeerURLs:   []string{"http://localhost:8082", "http://localhost:8083"},
		},
		{
			ID:         "d",
			ClientURLs: []string{"http://localhost:9090", "http://localhost:9091"},
			PeerURLs:   []string{"http://localhost:9092", "http://localhost:9093"},
		},
	})

	if !reflect.DeepEqual(&want, got) {
		t.Fatalf("newMemberCollection failure: want=%#v, got=%#v", &want, got)
	}
}

func TestNewMember(t *testing.T) {
	fixture := &membership.Member{
		ID:             12,
		Attributes:     membership.Attributes{ClientURLs: []string{"http://localhost:8080", "http://localhost:8081"}},
		RaftAttributes: membership.RaftAttributes{PeerURLs: []string{"http://localhost:8082", "http://localhost:8083"}},
	}
	got := newMember(fixture)

	want := httptypes.Member{
		ID:         "c",
		ClientURLs: []string{"http://localhost:8080", "http://localhost:8081"},
		PeerURLs:   []string{"http://localhost:8082", "http://localhost:8083"},
	}

	if !reflect.DeepEqual(want, got) {
		t.Fatalf("newMember failure: want=%#v, got=%#v", want, got)
	}
}
