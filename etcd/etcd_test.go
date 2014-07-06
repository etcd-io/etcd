package etcd

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMultipleNodes(t *testing.T) {
	tests := []int{1, 3, 5, 9, 11}

	for _, tt := range tests {
		es, hs := buildCluster(tt)
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

func buildCluster(number int) ([]*Server, []*httptest.Server) {
	bootstrapper := 0
	es := make([]*Server, number)
	hs := make([]*httptest.Server, number)
	var seed string

	for i := range es {
		es[i] = New(i, "", []string{seed})
		es[i].SetTick(time.Millisecond * 5)
		hs[i] = httptest.NewServer(es[i])
		es[i].pubAddr = hs[i].URL

		if i == bootstrapper {
			seed = hs[i].URL
			go es[i].Bootstrap()
		} else {
			// wait for the previous configuration change to be committed
			// or this configuration request might be dropped
			w, err := es[0].Watch(nodePrefix, true, false, uint64(i))
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
			w, err := e.Watch(nodePrefix, true, false, uint64(k))
			if err != nil {
				panic(err)
			}
			v := <-w.EventChan
			ww := fmt.Sprintf("%s/%d", nodePrefix, k-1)
			if v.Node.Key != ww {
				t.Errorf("#%d path = %v, want %v", i, v.Node.Key, w)
			}
		}
	}
}
