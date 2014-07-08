package etcd

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/config"
	"github.com/coreos/etcd/store"
)

func TestMachinesEndPoint(t *testing.T) {
	es, hs := buildCluster(3, false)
	waitCluster(t, es)

	w := make([]string, len(hs))
	for i := range hs {
		w[i] = hs[i].URL
	}

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
		g := strings.Split(string(b), ",")
		sort.Strings(g)
		if !reflect.DeepEqual(w, g) {
			t.Errorf("machines = %v, want %v", g, w)
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
	es, hs := buildCluster(3, false)
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

func TestStoreStatsEndPoint(t *testing.T) {
	es, hs := buildCluster(1, false)
	waitCluster(t, es)

	resp, err := http.Get(hs[0].URL + v2StoreStatsPrefix)
	if err != nil {
		t.Errorf("%v", err)
	}
	stats := new(store.Stats)
	d := json.NewDecoder(resp.Body)
	err = d.Decode(stats)
	resp.Body.Close()
	if err != nil {
		t.Errorf("%v", err)
	}

	if stats.SetSuccess != 1 {
		t.Errorf("setSuccess = %d, want 1", stats.SetSuccess)
	}

	for i := range es {
		es[len(es)-i-1].Stop()
	}
	for i := range hs {
		hs[len(hs)-i-1].Close()
	}
	afterTest(t)
}

func TestGetAdminConfigEndPoint(t *testing.T) {
	es, hs := buildCluster(3, false)
	waitCluster(t, es)

	for i := range hs {
		r, err := http.Get(hs[i].URL + v2adminConfigPrefix)
		if err != nil {
			t.Errorf("%v", err)
			continue
		}
		if g := r.StatusCode; g != 200 {
			t.Errorf("#%d: status = %d, want %d", i, g, 200)
		}
		if g := r.Header.Get("Content-Type"); g != "application/json" {
			t.Errorf("#%d: ContentType = %d, want application/json", i, g)
		}

		conf := new(config.ClusterConfig)
		err = json.NewDecoder(r.Body).Decode(conf)
		r.Body.Close()
		if err != nil {
			t.Errorf("%v", err)
			continue
		}
		w := config.NewClusterConfig()
		if !reflect.DeepEqual(conf, w) {
			t.Errorf("#%d: config = %+v, want %+v", i, conf, w)
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

func TestPutAdminConfigEndPoint(t *testing.T) {
	tests := []struct {
		c, wc string
	}{
		{
			`{"activeSize":1,"removeDelay":1,"syncInterval":1}`,
			`{"activeSize":3,"removeDelay":2,"syncInterval":1}`,
		},
		{
			`{"activeSize":5,"removeDelay":20.5,"syncInterval":1.5}`,
			`{"activeSize":5,"removeDelay":20.5,"syncInterval":1.5}`,
		},
		{
			`{"activeSize":5 ,  "removeDelay":20 ,  "syncInterval": 2 }`,
			`{"activeSize":5,"removeDelay":20,"syncInterval":2}`,
		},
		{
			`{"activeSize":3, "removeDelay":60}`,
			`{"activeSize":3,"removeDelay":60,"syncInterval":5}`,
		},
	}

	for i, tt := range tests {
		es, hs := buildCluster(3, false)
		waitCluster(t, es)

		r, err := NewTestClient().Put(hs[0].URL+v2adminConfigPrefix, "application/json", bytes.NewBufferString(tt.c))
		if err != nil {
			t.Fatalf("%v", err)
		}
		b, err := ioutil.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			t.Fatalf("%v", err)
		}
		if wbody := append([]byte(tt.wc), '\n'); !reflect.DeepEqual(b, wbody) {
			t.Errorf("#%d: put result = %s, want %s", i, b, wbody)
		}

		barrier(t, 0, es)

		for j := range es {
			e, err := es[j].Get(v2configKVPrefix, false, false)
			if err != nil {
				t.Errorf("%v", err)
				continue
			}
			if g := *e.Node.Value; g != tt.wc {
				t.Errorf("#%d.%d: %s = %s, want %s", i, j, v2configKVPrefix, g, tt.wc)
			}
		}

		for j := range es {
			es[len(es)-j-1].Stop()
		}
		for j := range hs {
			hs[len(hs)-j-1].Close()
		}
		afterTest(t)
	}
}

// barrier ensures that all servers have made further progress on applied index
// compared to the base one.
func barrier(t *testing.T, base int, es []*Server) {
	applied := es[base].node.Applied()
	// time used for goroutine scheduling
	time.Sleep(5 * time.Millisecond)
	for i, e := range es {
		for j := 0; ; j++ {
			if e.node.Applied() >= applied {
				break
			}
			time.Sleep(defaultHeartbeat * defaultTickDuration)
			if j == 2 {
				t.Fatalf("#%d: applied = %d, want >= %d", i, e.node.Applied(), applied)
			}
		}
	}
}
