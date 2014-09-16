package etcdhttp

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/coreos/etcd/raft/raftpb"
)

func TestPeers(t *testing.T) {
	tests := []struct {
		in      string
		wids    []int64
		wep     []string
		wstring string
	}{
		{
			"1=1.1.1.1",
			[]int64{1},
			[]string{"http://1.1.1.1"},
			"1=1.1.1.1",
		},
		{
			"2=2.2.2.2",
			[]int64{2},
			[]string{"http://2.2.2.2"},
			"2=2.2.2.2",
		},
		{
			"1=1.1.1.1&1=1.1.1.2&2=2.2.2.2",
			[]int64{1, 2},
			[]string{"http://1.1.1.1", "http://1.1.1.2", "http://2.2.2.2"},
			"1=1.1.1.1&1=1.1.1.2&2=2.2.2.2",
		},
		{
			"3=3.3.3.3&4=4.4.4.4&1=1.1.1.1&1=1.1.1.2&2=2.2.2.2",
			[]int64{1, 2, 3, 4},
			[]string{"http://1.1.1.1", "http://1.1.1.2", "http://2.2.2.2",
				"http://3.3.3.3", "http://4.4.4.4"},
			"1=1.1.1.1&1=1.1.1.2&2=2.2.2.2&3=3.3.3.3&4=4.4.4.4",
		},
	}
	for i, tt := range tests {
		p := &Peers{}
		err := p.Set(tt.in)
		if err != nil {
			t.Errorf("#%d: err=%v, want nil", i, err)
		}
		ids := int64Slice(p.IDs())
		sort.Sort(ids)
		if !reflect.DeepEqual([]int64(ids), tt.wids) {
			t.Errorf("#%d: IDs=%#v, want %#v", i, []int64(ids), tt.wids)
		}
		ep := p.Endpoints()
		if !reflect.DeepEqual(ep, tt.wep) {
			t.Errorf("#%d: Endpoints=%#v, want %#v", i, ep, tt.wep)
		}
		s := p.String()
		if s != tt.wstring {
			t.Errorf("#%d: string=%q, want %q", i, s, tt.wstring)
		}
	}
}

type int64Slice []int64

func (p int64Slice) Len() int           { return len(p) }
func (p int64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p int64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func TestPeersSetBad(t *testing.T) {
	tests := []string{
		// garbage URL
		"asdf%%",
		// non-int64 keys
		"a=1.2.3.4",
		"-1-23=1.2.3.4",
	}
	for i, tt := range tests {
		p := &Peers{}
		if err := p.Set(tt); err == nil {
			t.Errorf("#%d: err=nil unexpectedly", i)
		}
	}
}

func TestPeersPick(t *testing.T) {
	ps := &Peers{
		1: []string{"abc", "def", "ghi", "jkl", "mno", "pqr", "stu"},
		2: []string{"xyz"},
		3: []string{},
	}
	ids := map[string]bool{
		"http://abc": true,
		"http://def": true,
		"http://ghi": true,
		"http://jkl": true,
		"http://mno": true,
		"http://pqr": true,
		"http://stu": true,
	}
	for i := 0; i < 1000; i++ {
		a := ps.Pick(1)
		if _, ok := ids[a]; !ok {
			t.Errorf("returned ID %q not in expected range!", a)
			break
		}
	}
	if b := ps.Pick(2); b != "http://xyz" {
		t.Errorf("id=%q, want %q", b, "http://xyz")
	}
	if c := ps.Pick(3); c != "" {
		t.Errorf("id=%q, want \"\"", c)
	}
}

func TestHttpPost(t *testing.T) {
	var tr *http.Request
	tests := []struct {
		h http.HandlerFunc
		w bool
	}{
		{
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tr = r
				w.WriteHeader(http.StatusNoContent)
			}),
			true,
		},
		{
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tr = r
				w.WriteHeader(http.StatusNotFound)
			}),
			false,
		},
		{
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tr = r
				w.WriteHeader(http.StatusInternalServerError)
			}),
			false,
		},
	}
	for i, tt := range tests {
		ts := httptest.NewServer(tt.h)
		if g := httpPost(ts.URL, []byte("adsf")); g != tt.w {
			t.Errorf("#%d: httpPost()=%t, want %t", i, g, tt.w)
		}
		if tr.Method != "POST" {
			t.Errorf("#%d: Method=%q, want %q", i, tr.Method, "POST")
		}
		if ct := tr.Header.Get("Content-Type"); ct != "application/protobuf" {
			t.Errorf("#%d: Content-Type=%q, want %q", i, ct, "application/protobuf")
		}
		tr = nil
		ts.Close()
	}

	if httpPost("garbage url", []byte("data")) {
		t.Errorf("httpPost with bad URL returned true unexpectedly!")
	}
}

func TestSend(t *testing.T) {
	var tr *http.Request
	var rc int
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tr = r
		w.WriteHeader(rc)
	})
	tests := []struct {
		m    raftpb.Message
		code int

		ok bool
	}{
		{
			// all good
			raftpb.Message{
				To:   42,
				Type: 4,
			},
			http.StatusNoContent,
			true,
		},
		{
			// bad response from server should be silently ignored
			raftpb.Message{
				To:   42,
				Type: 2,
			},
			http.StatusInternalServerError,
			true,
		},
		{
			// unknown destination!
			raftpb.Message{
				To:   3,
				Type: 2,
			},
			0,
			false,
		},
	}

	for i, tt := range tests {
		tr = nil
		rc = tt.code
		ts := httptest.NewServer(h)
		ps := Peers{
			42: []string{strings.TrimPrefix(ts.URL, "http://")},
		}
		send(ps, tt.m)

		if !tt.ok {
			if tr != nil {
				t.Errorf("#%d: got request=%#v, want nil", i, tr)
			}
			ts.Close()
			continue
		}

		if tr.Method != "POST" {
			t.Errorf("#%d: Method=%q, want %q", i, tr.Method, "POST")
		}
		if ct := tr.Header.Get("Content-Type"); ct != "application/protobuf" {
			t.Errorf("#%d: Content-Type=%q, want %q", i, ct, "application/protobuf")
		}
		if tr.URL.String() != "/raft" {
			t.Errorf("#%d: URL=%q, want %q", i, tr.URL.String(), "/raft")
		}
		ts.Close()
	}
}
