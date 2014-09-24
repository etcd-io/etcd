package pkg

import (
	"flag"
	"os"
	"testing"
)

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
