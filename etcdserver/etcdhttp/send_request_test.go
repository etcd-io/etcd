package etcdhttp

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestSendAddMemberRequest(t *testing.T) {
	var tr *http.Request
	tests := []struct {
		h     http.HandlerFunc
		wpass bool
	}{
		{
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.ParseForm()
				tr = r
				w.WriteHeader(http.StatusCreated)
			}),
			true,
		},
		{
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.ParseForm()
				tr = r
				w.WriteHeader(http.StatusInternalServerError)
			}),
			false,
		},
	}
	for i, tt := range tests {
		ts := httptest.NewServer(tt.h)
		wpeers := []string{"foo", "bar"}
		if g := SendAddMemberRequest(ts.URL, 1, wpeers); g == nil != tt.wpass {
			t.Errorf("#%d: err = %v, want pass = %v", i, g, tt.wpass)
		}
		if tr.Method != "PUT" {
			t.Errorf("#%d: Method = %q, want %q", i, tr.Method, "PUT")
		}
		if w := adminMembersPrefix + "1"; tr.URL.String() != w {
			t.Errorf("#%d: url = %q, want %q", i, tr.URL.String(), w)
		}
		if ct := tr.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("#%d: Content-Type = %q, want %q", i, ct, "application/x-www-form-urlencoded")
		}
		if peers := tr.PostForm["PeerURLs"]; !reflect.DeepEqual(peers, wpeers) {
			t.Errorf("#%d: peers = %v, want %v", i, peers, wpeers)
			t.Errorf("%v", tr.PostForm)
		}

		tr = nil
		ts.Close()
	}

	if SendAddMemberRequest("garbage url", 1, []string{}) == nil {
		t.Errorf("addOnce with bad URL succeeds unexpectedly!")
	}
}

func TestSendRemoveMemberRequest(t *testing.T) {
	var tr *http.Request
	tests := []struct {
		h     http.HandlerFunc
		wpass bool
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
				w.WriteHeader(http.StatusInternalServerError)
			}),
			false,
		},
	}
	for i, tt := range tests {
		ts := httptest.NewServer(tt.h)
		if g := SendRemoveMemberRequest(ts.URL, 1); g == nil != tt.wpass {
			t.Errorf("#%d: err = %v, want pass = %v", i, g, tt.wpass)
		}
		if tr.Method != "DELETE" {
			t.Errorf("#%d: Method = %q, want %q", i, tr.Method, "DELETE")
		}
		if w := adminMembersPrefix + "1"; tr.URL.String() != w {
			t.Errorf("#%d: url = %q, want %q", i, tr.URL.String(), w)
		}

		tr = nil
		ts.Close()
	}

	if SendRemoveMemberRequest("garbage url", 1) == nil {
		t.Errorf("removeOnce with bad URL succeeds unexpectedly!")
	}
}
