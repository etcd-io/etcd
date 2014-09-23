package main

import "os"
import "flag"
import "testing"

func TestSetFlagsFromEnv(t *testing.T) {
	os.Clearenv()
	// flags should be settable using env vars
	os.Setenv("ETCD_DATA_DIR", "/foo/bar")
	// and command-line flags
	if err := flag.Set("peer-bind-addr", "1.2.3.4"); err != nil {
		t.Fatal(err)
	}
	// command-line flags take precedence over env vars
	os.Setenv("ETCD_NAME", "woof")
	if err := flag.Set("name", "quack"); err != nil {
		t.Fatal(err)
	}

	// first verify that flags are as expected before reading the env
	for f, want := range map[string]string{
		"data-dir":       "",
		"peer-bind-addr": "1.2.3.4",
		"name":           "quack",
	} {
		if got := flag.Lookup(f).Value.String(); got != want {
			t.Fatalf("flag %q=%q, want %q", f, got, want)
		}
	}

	// now read the env and verify flags were updated as expected
	setFlagsFromEnv()
	for f, want := range map[string]string{
		"data-dir":       "/foo/bar",
		"peer-bind-addr": "1.2.3.4",
		"name":           "quack",
	} {
		if got := flag.Lookup(f).Value.String(); got != want {
			t.Errorf("flag %q=%q, want %q", f, got, want)
		}
	}
}

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
