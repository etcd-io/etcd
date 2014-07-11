package etcd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"testing"
	"time"

	"github.com/coreos/etcd/config"
)

func TestMultipleNodes(t *testing.T) {
	tests := []int{1, 3, 5, 9, 11}

	for _, tt := range tests {
		es, hs := buildCluster(tt, false)
		waitCluster(t, es)
		for i := range es {
			es[len(es)-i-1].Stop()
		}
		for i := range hs {
			hs[len(hs)-i-1].Close()
		}
	}
	afterTest(t)
}

func TestMultipleTLSNodes(t *testing.T) {
	tests := []int{1, 3, 5}

	for _, tt := range tests {
		es, hs := buildCluster(tt, true)
		waitCluster(t, es)
		for i := range es {
			es[len(es)-i-1].Stop()
		}
		for i := range hs {
			hs[len(hs)-i-1].Close()
		}
	}
	afterTest(t)
}

func TestV2Redirect(t *testing.T) {
	es, hs := buildCluster(3, false)
	waitCluster(t, es)
	u := hs[1].URL
	ru := fmt.Sprintf("%s%s", hs[0].URL, "/v2/keys/foo")
	tc := NewTestClient()

	v := url.Values{}
	v.Set("value", "XXX")
	resp, _ := tc.PutForm(fmt.Sprintf("%s%s", u, "/v2/keys/foo"), v)
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusTemporaryRedirect)
	}
	location, err := resp.Location()
	if err != nil {
		t.Errorf("want err = %, want nil", err)
	}

	if location.String() != ru {
		t.Errorf("location = %v, want %v", location.String(), ru)
	}

	resp.Body.Close()
	for i := range es {
		es[len(es)-i-1].Stop()
	}
	for i := range hs {
		hs[len(hs)-i-1].Close()
	}
	afterTest(t)
}

func TestRemove(t *testing.T) {
	tests := []struct {
		size  int
		round int
	}{
		{3, 5},
		{4, 5},
		{5, 5},
		{6, 5},
	}

	for _, tt := range tests {
		es, hs := buildCluster(tt.size, false)
		waitCluster(t, es)

		// we don't remove the machine from 2-node cluster because it is
		// not 100 percent safe in our raft.
		// TODO(yichengq): improve it later.
		for i := 0; i < tt.size-2; i++ {
			// wait for leader to be stable for all live machines
			// TODO(yichengq): change it later
			var prevLead int64
			var prevTerm int64
			for j := i; j < tt.size; j++ {
				id := int64(i)
				lead := es[j].node.Leader()
				term := es[j].node.Term()
				fit := true
				if j == i {
					if lead < id {
						fit = false
					}
				} else {
					if lead != prevLead || term != prevTerm {
						fit = false
					}
				}
				if !fit {
					j = i - 1
					runtime.Gosched()
					continue
				}
				prevLead = lead
				prevTerm = term
			}

			index := es[i].Index()
			es[i].Remove(i)

			// i-th machine cannot be promised to apply the removal command of
			// its own due to our non-optimized raft.
			// TODO(yichengq): it should work when
			// https://github.com/etcd-team/etcd/pull/7 is merged.
			for j := i + 1; j < tt.size; j++ {
				w, err := es[j].Watch(v2machineKVPrefix, true, false, index+1)
				if err != nil {
					t.Errorf("#%d on %d: %v", i, j, err)
					break
				}
				v := <-w.EventChan
				ww := fmt.Sprintf("%s/%d", v2machineKVPrefix, i)
				if v.Node.Key != ww {
					t.Errorf("#%d on %d: path = %v, want %v", i, j, v.Node.Key, ww)
				}
			}

			// may need to wait for msgDenial
			// TODO(yichengq): no need to sleep here when previous issue is merged.
			if es[i].mode == stop {
				continue
			}
			time.Sleep(defaultElection * defaultTickDuration)
			if g := es[i].mode; g != stop {
				t.Errorf("#%d: mode = %d, want stop", i, g)
			}
		}

		for i := range hs {
			es[len(hs)-i-1].Stop()
		}
		for i := range hs {
			hs[len(hs)-i-1].Close()
		}
		afterTest(t)
	}
}

func buildCluster(number int, tls bool) ([]*Server, []*httptest.Server) {
	bootstrapper := 0
	es := make([]*Server, number)
	hs := make([]*httptest.Server, number)
	var seed string

	for i := range es {
		c := config.New()
		c.Peers = []string{seed}
		es[i] = New(c, int64(i))
		es[i].SetTick(time.Millisecond * 5)
		m := http.NewServeMux()
		m.Handle("/", es[i])
		m.Handle("/raft", es[i].t)
		m.Handle("/raft/", es[i].t)

		if tls {
			hs[i] = httptest.NewTLSServer(m)
		} else {
			hs[i] = httptest.NewServer(m)
		}

		es[i].raftPubAddr = hs[i].URL
		es[i].pubAddr = hs[i].URL

		if i == bootstrapper {
			seed = hs[i].URL
			go es[i].Bootstrap()
		} else {
			// wait for the previous configuration change to be committed
			// or this configuration request might be dropped
			w, err := es[0].Watch(v2machineKVPrefix, true, false, uint64(i))
			if err != nil {
				panic(err)
			}
			<-w.EventChan
			go es[i].Join()
		}
	}
	return es, hs
}

func waitCluster(t *testing.T, es []*Server) {
	n := len(es)
	for i, e := range es {
		for k := 1; k < n+1; k++ {
			w, err := e.Watch(v2machineKVPrefix, true, false, uint64(k))
			if err != nil {
				panic(err)
			}
			v := <-w.EventChan
			ww := fmt.Sprintf("%s/%d", v2machineKVPrefix, k-1)
			if v.Node.Key != ww {
				t.Errorf("#%d path = %v, want %v", i, v.Node.Key, w)
			}
		}
	}
}
