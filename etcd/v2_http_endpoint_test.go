/*
Copyright 2014 CoreOS Inc.

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

package etcd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/coreos/etcd/conf"
	"github.com/coreos/etcd/store"
)

func TestMachinesEndPoint(t *testing.T) {
	cl := &testCluster{Size: 3}
	cl.Start()

	w := make([]string, cl.Size)
	for i := 0; i < cl.Size; i++ {
		w[i] = cl.URL(i)
	}

	for i := 0; i < cl.Size; i++ {
		r, err := http.Get(cl.URL(i) + v2machinePrefix)
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

	cl.Destroy()
}

func TestLeaderEndPoint(t *testing.T) {
	cl := &testCluster{Size: 3}
	cl.Start()

	us := make([]string, cl.Size)
	for i := 0; i < cl.Size; i++ {
		us[i] = cl.URL(i)
	}
	// todo(xiangli) change this to raft port...
	w := cl.URL(0) + "/raft"

	for i := 0; i < cl.Size; i++ {
		r, err := http.Get(cl.URL(i) + v2LeaderPrefix)
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

	cl.Destroy()
}

func TestStoreStatsEndPoint(t *testing.T) {
	cl := &testCluster{Size: 1}
	cl.Start()

	resp, err := http.Get(cl.URL(0) + v2StoreStatsPrefix)
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

	cl.Destroy()
}

func TestGetAdminConfigEndPoint(t *testing.T) {
	cl := &testCluster{Size: 1}
	cl.Start()

	r, err := http.Get(cl.URL(0) + v2adminConfigPrefix)
	if err != nil {
		t.Errorf("%v", err)
	}
	if g := r.StatusCode; g != 200 {
		t.Errorf("status = %d, want %d", g, 200)
	}
	if g := r.Header.Get("Content-Type"); g != "application/json" {
		t.Errorf("ContentType = %s, want application/json", g)
	}

	cfg := new(conf.ClusterConfig)
	err = json.NewDecoder(r.Body).Decode(cfg)
	r.Body.Close()
	if err != nil {
		t.Errorf("%v", err)
	}
	w := conf.NewClusterConfig()
	if !reflect.DeepEqual(cfg, w) {
		t.Errorf("config = %+v, want %+v", cfg, w)
	}

	cl.Destroy()
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
		cl := &testCluster{Size: 1}
		cl.Start()
		index := cl.Participant(0).Index()

		r, err := NewTestClient().Put(cl.URL(0)+v2adminConfigPrefix, "application/json", bytes.NewBufferString(tt.c))
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

		w, err := cl.Participant(0).Watch(v2configKVPrefix, false, false, index)
		if err != nil {
			t.Errorf("%v", err)
			continue
		}
		e := <-w.EventChan
		if g := *e.Node.Value; g != tt.wc {
			t.Errorf("#%d: %s = %s, want %s", i, v2configKVPrefix, g, tt.wc)
		}

		cl.Destroy()
	}
}

func TestGetAdminMachineEndPoint(t *testing.T) {
	cl := &testCluster{Size: 3}
	cl.Start()

	for i := 0; i < cl.Size; i++ {
		for j := 0; j < cl.Size; j++ {
			name := fmt.Sprint(cl.Node(i).Id)
			r, err := http.Get(cl.URL(j) + v2adminMachinesPrefix + name)
			if err != nil {
				t.Errorf("%v", err)
				continue
			}
			if g := r.StatusCode; g != 200 {
				t.Errorf("#%d on %d: status = %d, want %d", i, j, g, 200)
			}
			if g := r.Header.Get("Content-Type"); g != "application/json" {
				t.Errorf("#%d on %d: ContentType = %s, want application/json", i, j, g)
			}

			m := new(machineMessage)
			err = json.NewDecoder(r.Body).Decode(m)
			r.Body.Close()
			if err != nil {
				t.Errorf("%v", err)
				continue
			}
			wm := &machineMessage{
				Name:      name,
				State:     stateFollower,
				ClientURL: cl.URL(i),
				PeerURL:   cl.URL(i),
			}
			if i == 0 {
				wm.State = stateLeader
			}
			if !reflect.DeepEqual(m, wm) {
				t.Errorf("#%d on %d: body = %+v, want %+v", i, j, m, wm)
			}
		}
	}

	cl.Destroy()
}

func TestGetAdminMachinesEndPoint(t *testing.T) {
	cl := &testCluster{Size: 3}
	cl.Start()

	w := make([]*machineMessage, cl.Size)
	for i := 0; i < cl.Size; i++ {
		w[i] = &machineMessage{
			Name:      fmt.Sprint(cl.Node(i).Id),
			State:     stateFollower,
			ClientURL: cl.URL(i),
			PeerURL:   cl.URL(i),
		}
	}
	w[0].State = stateLeader

	for i := 0; i < cl.Size; i++ {
		r, err := http.Get(cl.URL(i) + v2adminMachinesPrefix)
		if err != nil {
			t.Errorf("%v", err)
			continue
		}
		m := make([]*machineMessage, 0)
		err = json.NewDecoder(r.Body).Decode(&m)
		r.Body.Close()
		if err != nil {
			t.Errorf("%v", err)
			continue
		}

		sm := machineSlice(m)
		sw := machineSlice(w)
		sort.Sort(sm)
		sort.Sort(sw)

		if !reflect.DeepEqual(sm, sw) {
			t.Errorf("on %d: machines = %+v, want %+v", i, sm, sw)
		}
	}

	cl.Destroy()
}

// int64Slice implements sort interface
type machineSlice []*machineMessage

func (s machineSlice) Len() int           { return len(s) }
func (s machineSlice) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s machineSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
