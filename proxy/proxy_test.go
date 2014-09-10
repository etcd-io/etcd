package proxy

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

func TestNewDirector(t *testing.T) {
	tests := []struct {
		good      bool
		endpoints []string
	}{
		{true, []string{"http://192.0.2.8"}},
		{true, []string{"http://192.0.2.8:8001"}},
		{true, []string{"http://example.com"}},
		{true, []string{"http://example.com:8001"}},
		{true, []string{"http://192.0.2.8:8001", "http://example.com:8002"}},

		{false, []string{"192.0.2.8"}},
		{false, []string{"192.0.2.8:8001"}},
		{false, []string{""}},
	}

	for i, tt := range tests {
		_, err := newDirector(tt.endpoints)
		if tt.good != (err == nil) {
			t.Errorf("#%d: expected success = %t, got err = %v", i, tt.good, err)
		}
	}
}

func TestDirectorDirect(t *testing.T) {
	d := &director{
		endpoints: []url.URL{
			url.URL{
				Scheme: "http",
				Host:   "bar.example.com",
			},
		},
	}

	req := &http.Request{
		Method: "GET",
		Host:   "foo.example.com",
		URL: &url.URL{
			Host: "foo.example.com",
			Path: "/v2/keys/baz",
		},
	}

	d.direct(req)

	want := &http.Request{
		Method: "GET",
		// this field must not change
		Host: "foo.example.com",
		URL: &url.URL{
			// the Scheme field is updated per the director's first endpoint
			Scheme: "http",
			// the Host field is updated per the director's first endpoint
			Host: "bar.example.com",
			Path: "/v2/keys/baz",
		},
	}

	if !reflect.DeepEqual(want, req) {
		t.Fatalf("HTTP request does not match expected criteria: want=%#v got=%#v", want, req)
	}
}
