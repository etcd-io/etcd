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

package etcdserver

import (
	"errors"
	"io/ioutil"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/coreos/etcd/etcdserver/stats"
	"github.com/coreos/etcd/pkg/testutil"
	"github.com/coreos/etcd/pkg/types"
)

func TestSendHubInitSenders(t *testing.T) {
	membs := []*Member{
		newTestMember(1, []string{"http://a"}, "", nil),
		newTestMember(2, []string{"http://b"}, "", nil),
		newTestMember(3, []string{"http://c"}, "", nil),
	}
	cl := newTestCluster(membs)
	ls := stats.NewLeaderStats("")
	h := newSendHub(nil, cl, nil, ls)

	ids := cl.MemberIDs()
	if len(h.senders) != len(ids) {
		t.Errorf("len(ids) = %d, want %d", len(h.senders), len(ids))
	}
	for _, id := range ids {
		if _, ok := h.senders[id]; !ok {
			t.Errorf("senders[%s] is nil, want exists", id)
		}
	}
}

func TestSendHubAdd(t *testing.T) {
	cl := newTestCluster(nil)
	ls := stats.NewLeaderStats("")
	h := newSendHub(nil, cl, nil, ls)
	m := newTestMember(1, []string{"http://a"}, "", nil)
	h.Add(m)

	if _, ok := ls.Followers["1"]; !ok {
		t.Errorf("FollowerStats[1] is nil, want exists")
	}
	s, ok := h.senders[types.ID(1)]
	if !ok {
		t.Fatalf("senders[1] is nil, want exists")
	}
	if s.u != "http://a/raft" {
		t.Errorf("url = %s, want %s", s.u, "http://a/raft")
	}

	h.Add(m)
	ns := h.senders[types.ID(1)]
	if s != ns {
		t.Errorf("sender = %p, want %p", ns, s)
	}
}

func TestSendHubRemove(t *testing.T) {
	membs := []*Member{
		newTestMember(1, []string{"http://a"}, "", nil),
	}
	cl := newTestCluster(membs)
	ls := stats.NewLeaderStats("")
	h := newSendHub(nil, cl, nil, ls)
	h.Remove(types.ID(1))

	if _, ok := h.senders[types.ID(1)]; ok {
		t.Fatalf("senders[1] exists, want removed")
	}
}

// TestSenderSend tests that send func could post data using roundtripper
// and increase success count in stats.
func TestSenderSend(t *testing.T) {
	tr := &roundTripperRecorder{}
	fs := &stats.FollowerStats{}
	s := newSender(tr, "http://10.0.0.1", types.ID(1), fs)
	// wait for handle goroutines start
	// TODO: wait for goroutines ready before return newSender
	time.Sleep(10 * time.Millisecond)
	if err := s.send([]byte("some data")); err != nil {
		t.Fatalf("unexpect send error: %v", err)
	}
	s.stop()
	// wait for goroutines end
	// TODO: elegant stop
	time.Sleep(10 * time.Millisecond)

	if tr.Request() == nil {
		t.Errorf("sender fails to post the data")
	}
	fs.Lock()
	defer fs.Unlock()
	if fs.Counts.Success != 1 {
		t.Errorf("success = %d, want 1", fs.Counts.Success)
	}
}

func TestSenderExceedMaximalServing(t *testing.T) {
	tr := newRoundTripperBlocker()
	fs := &stats.FollowerStats{}
	s := newSender(tr, "http://10.0.0.1", types.ID(1), fs)
	// wait for handle goroutines start
	// TODO: wait for goroutines ready before return newSender
	time.Sleep(10 * time.Millisecond)
	// It could handle that many requests at the same time.
	for i := 0; i < connPerSender; i++ {
		if err := s.send([]byte("some data")); err != nil {
			t.Errorf("send err = %v, want nil", err)
		}
	}
	// This one exceeds its maximal serving ability
	if err := s.send([]byte("some data")); err == nil {
		t.Errorf("unexpect send success")
	}
	tr.unblock()
	// Make handles finish their post
	testutil.ForceGosched()
	// It could send new data after previous ones succeed
	if err := s.send([]byte("some data")); err != nil {
		t.Errorf("send err = %v, want nil", err)
	}
	s.stop()
}

// TestSenderSendFailed tests that when send func meets the post error,
// it increases fail count in stats.
func TestSenderSendFailed(t *testing.T) {
	fs := &stats.FollowerStats{}
	s := newSender(newRespRoundTripper(0, errors.New("blah")), "http://10.0.0.1", types.ID(1), fs)
	// wait for handle goroutines start
	// TODO: wait for goroutines ready before return newSender
	time.Sleep(10 * time.Millisecond)
	if err := s.send([]byte("some data")); err != nil {
		t.Fatalf("unexpect send error: %v", err)
	}
	s.stop()
	// wait for goroutines end
	// TODO: elegant stop
	time.Sleep(10 * time.Millisecond)

	fs.Lock()
	defer fs.Unlock()
	if fs.Counts.Fail != 1 {
		t.Errorf("fail = %d, want 1", fs.Counts.Fail)
	}
}

func TestSenderPost(t *testing.T) {
	tr := &roundTripperRecorder{}
	s := newSender(tr, "http://10.0.0.1", types.ID(1), nil)
	if err := s.post([]byte("some data")); err != nil {
		t.Fatalf("unexpect post error: %v", err)
	}
	s.stop()

	if g := tr.Request().Method; g != "POST" {
		t.Errorf("method = %s, want %s", g, "POST")
	}
	if g := tr.Request().URL.String(); g != "http://10.0.0.1" {
		t.Errorf("url = %s, want %s", g, "http://10.0.0.1")
	}
	if g := tr.Request().Header.Get("Content-Type"); g != "application/protobuf" {
		t.Errorf("content type = %s, want %s", g, "application/protobuf")
	}
	if g := tr.Request().Header.Get("X-Etcd-Cluster-ID"); g != "1" {
		t.Errorf("cluster id = %s, want %s", g, "1")
	}
	b, err := ioutil.ReadAll(tr.Request().Body)
	if err != nil {
		t.Fatalf("unexpected ReadAll error: %v", err)
	}
	if string(b) != "some data" {
		t.Errorf("body = %s, want %s", b, "some data")
	}
}

func TestSenderPostBad(t *testing.T) {
	tests := []struct {
		u    string
		code int
		err  error
	}{
		// bad url
		{":bad url", http.StatusNoContent, nil},
		// RoundTrip returns error
		{"http://10.0.0.1", 0, errors.New("blah")},
		// unexpected response status code
		{"http://10.0.0.1", http.StatusOK, nil},
		{"http://10.0.0.1", http.StatusCreated, nil},
	}
	for i, tt := range tests {
		s := newSender(newRespRoundTripper(tt.code, tt.err), tt.u, types.ID(1), nil)
		err := s.post([]byte("some data"))
		s.stop()

		if err == nil {
			t.Errorf("#%d: err = nil, want not nil", i)
		}
	}
}

type roundTripperBlocker struct {
	c chan struct{}
}

func newRoundTripperBlocker() *roundTripperBlocker {
	return &roundTripperBlocker{c: make(chan struct{})}
}
func (t *roundTripperBlocker) RoundTrip(req *http.Request) (*http.Response, error) {
	<-t.c
	return &http.Response{StatusCode: http.StatusNoContent, Body: &nopReadCloser{}}, nil
}
func (t *roundTripperBlocker) unblock() {
	close(t.c)
}

type respRoundTripper struct {
	code int
	err  error
}

func newRespRoundTripper(code int, err error) *respRoundTripper {
	return &respRoundTripper{code: code, err: err}
}
func (t *respRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: t.code, Body: &nopReadCloser{}}, t.err
}

type roundTripperRecorder struct {
	req *http.Request
	sync.Mutex
}

func (t *roundTripperRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	t.Lock()
	defer t.Unlock()
	t.req = req
	return &http.Response{StatusCode: http.StatusNoContent, Body: &nopReadCloser{}}, nil
}
func (t *roundTripperRecorder) Request() *http.Request {
	t.Lock()
	defer t.Unlock()
	return t.req
}

type nopReadCloser struct{}

func (n *nopReadCloser) Read(p []byte) (int, error) { return 0, nil }
func (n *nopReadCloser) Close() error               { return nil }
