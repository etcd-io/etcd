package proxy

import (
	"net/url"
	"reflect"
	"testing"
)

func TestNewDirectorEndpointValidation(t *testing.T) {
	tests := []struct {
		good      bool
		endpoints []string
	}{
		{true, []string{"http://192.0.2.8"}},
		{true, []string{"http://192.0.2.8:8001"}},
		{true, []string{"http://example.com"}},
		{true, []string{"http://example.com:8001"}},
		{true, []string{"http://192.0.2.8:8001", "http://example.com:8002"}},

		{false, []string{"://"}},
		{false, []string{"http://"}},
		{false, []string{"192.0.2.8"}},
		{false, []string{"192.0.2.8:8001"}},
		{false, []string{""}},
		{false, []string{}},
	}

	for i, tt := range tests {
		_, err := newDirector(tt.endpoints)
		if tt.good != (err == nil) {
			t.Errorf("#%d: expected success = %t, got err = %v", i, tt.good, err)
		}
	}
}

func TestDirectorEndpointsFiltering(t *testing.T) {
	d := director{
		ep: []*endpoint{
			&endpoint{
				URL:       url.URL{Scheme: "http", Host: "192.0.2.5:5050"},
				Available: false,
			},
			&endpoint{
				URL:       url.URL{Scheme: "http", Host: "192.0.2.4:4000"},
				Available: true,
			},
		},
	}

	got := d.endpoints()
	want := []*endpoint{
		&endpoint{
			URL:       url.URL{Scheme: "http", Host: "192.0.2.4:4000"},
			Available: true,
		},
	}

	if !reflect.DeepEqual(want, got) {
		t.Fatalf("directed to incorrect endpoint: want = %#v, got = %#v", want, got)
	}
}
