package flags

import (
	"testing"
)

func TestValidateURLsValueBad(t *testing.T) {
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
		"http://hello/asdf",
		"http://10.1.1.1",
	}
	for i, in := range tests {
		u := URLsValue{}
		if err := u.Set(in); err == nil {
			t.Errorf(`#%d: unexpected nil error for in=%q`, i, in)
		}
	}
}

func TestValidateURLsValueGood(t *testing.T) {
	tests := []string{
		"https://1.2.3.4:8080",
		"http://10.1.1.1:80",
		"http://localhost:80",
		"http://:80",
	}
	for i, in := range tests {
		u := URLsValue{}
		if err := u.Set(in); err != nil {
			t.Errorf("#%d: err=%v, want nil for in=%q", i, err, in)
		}
	}
}
