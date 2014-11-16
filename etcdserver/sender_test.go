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
	"net/http"
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

func TestSendHubShouldStop(t *testing.T) {
	membs := []*Member{
		newTestMember(1, []string{"http://a"}, "", nil),
	}
	tr := newRespRoundTripper(http.StatusForbidden, nil)
	cl := newTestCluster(membs)
	ls := stats.NewLeaderStats("")
	h := newSendHub(tr, cl, nil, ls)

	shouldstop := h.ShouldStopNotify()
	select {
	case <-shouldstop:
		t.Fatalf("received unexpected shouldstop notification")
	case <-time.After(10 * time.Millisecond):
	}
	h.senders[1].Send([]byte("somedata"))

	testutil.ForceGosched()
	select {
	case <-shouldstop:
	default:
		t.Fatalf("cannot receive stop notification")
	}
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

type nopReadCloser struct{}

func (n *nopReadCloser) Read(p []byte) (int, error) { return 0, nil }
func (n *nopReadCloser) Close() error               { return nil }
