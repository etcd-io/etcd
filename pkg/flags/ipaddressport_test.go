package flags

import (
	"testing"
)

func TestIPAddressPortSet(t *testing.T) {
	pass := []string{
		"1.2.3.4:8080",
		"10.1.1.1:80",
	}

	fail := []string{
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

	for i, tt := range pass {
		f := &IPAddressPort{}
		if err := f.Set(tt); err != nil {
			t.Errorf("#%d: unexpected error from IPAddressPort.Set(%q): %v", i, tt, err)
		}
	}

	for i, tt := range fail {
		f := &IPAddressPort{}
		if err := f.Set(tt); err == nil {
			t.Errorf("#%d: expected error from IPAddressPort.Set(%q)", i, tt, err)
		}
	}
}

func TestIPAddressPortString(t *testing.T) {
	f := &IPAddressPort{}
	if err := f.Set("127.0.0.1:4001"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "127.0.0.1:4001"
	got := f.String()
	if want != got {
		t.Fatalf("IPAddressPort.String() value should be %q, got %q", want, got)
	}
}
