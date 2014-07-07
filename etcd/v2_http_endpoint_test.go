package etcd

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

func TestMachinesEndPoint(t *testing.T) {
	es, hs := buildCluster(3)
	waitCluster(t, es)

	us := make([]string, len(hs))
	for i := range hs {
		us[i] = hs[i].URL
	}
	w := strings.Join(us, ",")

	for i := range hs {
		r, err := http.Get(hs[i].URL + v2machinePrefix)
		if err != nil {
			t.Errorf("%v", err)
			break
		}
		b, err := ioutil.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			t.Errorf("%v", err)
			break
		}
		if string(b) != w {
			t.Errorf("machines = %v, want %v", string(b), w)
		}
	}

	for i := range es {
		es[len(es)-i-1].Stop()
	}
	for i := range hs {
		hs[len(hs)-i-1].Close()
	}
	afterTest(t)
}

func TestLeaderEndPoint(t *testing.T) {
	es, hs := buildCluster(3)
	waitCluster(t, es)

	us := make([]string, len(hs))
	for i := range hs {
		us[i] = hs[i].URL
	}
	// todo(xiangli) change this to raft port...
	w := hs[0].URL + "/raft"

	for i := range hs {
		r, err := http.Get(hs[i].URL + v2LeaderPrefix)
		if err != nil {
			t.Errorf("%v", err)
			break
		}
		b, err := ioutil.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			t.Errorf("%v", err)
			break
		}
		if string(b) != w {
			t.Errorf("leader = %v, want %v", string(b), w)
		}
	}

	for i := range es {
		es[len(es)-i-1].Stop()
	}
	for i := range hs {
		hs[len(hs)-i-1].Close()
	}
	afterTest(t)
}
