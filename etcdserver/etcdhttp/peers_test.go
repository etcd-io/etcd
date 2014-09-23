package etcdhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coreos/etcd/raft/raftpb"
)

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
		ps := Cluster{
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
