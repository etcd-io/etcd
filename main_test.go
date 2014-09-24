package main

import "os"
import "flag"
import "testing"

func TestSetFlagsFromEnv(t *testing.T) {
	fs := flag.NewFlagSet("testing", flag.ExitOnError)
	fs.String("a", "", "")
	fs.String("b", "", "")
	fs.String("c", "", "")
	fs.Parse([]string{})

	os.Clearenv()
	// flags should be settable using env vars
	os.Setenv("ETCD_A", "foo")
	// and command-line flags
	if err := fs.Set("b", "bar"); err != nil {
		t.Fatal(err)
	}
	// command-line flags take precedence over env vars
	os.Setenv("ETCD_C", "woof")
	if err := fs.Set("c", "quack"); err != nil {
		t.Fatal(err)
	}

	// first verify that flags are as expected before reading the env
	for f, want := range map[string]string{
		"a": "",
		"b": "bar",
		"c": "quack",
	} {
		if got := fs.Lookup(f).Value.String(); got != want {
			t.Fatalf("flag %q=%q, want %q", f, got, want)
		}
	}

	// now read the env and verify flags were updated as expected
	SetFlagsFromEnv(fs)
	for f, want := range map[string]string{
		"a": "foo",
		"b": "bar",
		"c": "quack",
	} {
		if got := fs.Lookup(f).Value.String(); got != want {
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
