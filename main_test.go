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
	os.Setenv("ETCD_ID", "woof")
	if err := flag.Set("id", "quack"); err != nil {
		t.Fatal(err)
	}

	// first verify that flags are as expected before reading the env
	for f, want := range map[string]string{
		"data-dir":       "",
		"peer-bind-addr": "1.2.3.4",
		"id":             "quack",
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
		"id":             "quack",
	} {
		if got := flag.Lookup(f).Value.String(); got != want {
			t.Errorf("flag %q=%q, want %q", f, got, want)
		}
	}
}
