package main

import (
	"fmt"
	"reflect"
	"testing"
)

func TestProxyFlagSet(t *testing.T) {
	tests := []struct {
		val  string
		pass bool
	}{
		// known values
		{"on", true},
		{"off", true},

		// unrecognized values
		{"foo", false},
		{"", false},
	}

	for i, tt := range tests {
		pf := new(ProxyFlag)
		err := pf.Set(tt.val)
		if tt.pass != (err == nil) {
			t.Errorf("#%d: want pass=%t, but got err=%v", i, tt.pass, err)
		}
	}
}

func TestBadValidateAddr(t *testing.T) {
	tests := []string{
		// bad IP specification
		":4001",
		"127.0:8080",
		"123:456",
		// bad port specification
		"127.0.0.1:foo",
		"127.0.0.1:",
		// unix sockets not supported
		"unix://",
		"unix://tmp/etcd.sock",
		// bad strings
		"somewhere",
		"234#$",
		"file://foo/bar",
		"http://hello",
	}
	for i, in := range tests {
		if err := validateAddr(in); err == nil {
			t.Errorf(`#%d: unexpected nil error for in=%q`, i, in)
		}
	}
}

func TestValidateAddr(t *testing.T) {
	tests := []string{
		"1.2.3.4:8080",
		"10.1.1.1:80",
	}
	for i, in := range tests {
		if err := validateAddr(in); err != nil {
			t.Errorf("#%d: err=%v, want nil for in=%q", i, err, in)
		}
	}
}

func TestReconfigFlagSet(t *testing.T) {
	tests := []struct {
		val  string
		pass bool
	}{
		// known values
		{"add", true},
		{"remove", true},
		{"off", true},

		// unrecognized values
		{"foo", false},
		{"", false},
	}

	for i, tt := range tests {
		rf := new(ReconfigFlag)
		err := rf.Set(tt.val)
		if tt.pass != (err == nil) {
			t.Errorf("#%d: want pass=%v, but got err=%v", i, tt.pass, err)
		}
	}
}

func TestBadEndpointTraversal(t *testing.T) {
	called := make(map[string]int)
	f := func(endpoint string) error {
		called[endpoint]++
		return fmt.Errorf("blah")
	}
	err := endpointTraversal([]string{"foo", "bar"}, f, 5, 0)

	if err == nil {
		t.Errorf("unexpected nil error")
	}
	wcalled := map[string]int{"foo": 5, "bar": 5}
	if !reflect.DeepEqual(called, wcalled) {
		t.Errorf("called = %v, want %v", called, wcalled)
	}
}

func TestEndpointTraversal(t *testing.T) {
	f := func(endpoint string) error {
		return nil
	}
	if err := endpointTraversal([]string{"foo"}, f, 1, 0); err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}
