package gopkgs

import (
	"testing"
)

func TestVendor(t *testing.T) {
	cases := []struct {
		workDir   string
		vendorDir string
		visible   bool
	}{
		{
			workDir:   "/home/foo/go/src/github.com/rogpeppe/godef",
			vendorDir: "/home/foo/go/src/github.com/rogpeppe/godef",
			visible:   true,
		},
		{
			workDir:   "/home/foo/go/src/github.com/rogpeppe/godef/go",
			vendorDir: "/home/foo/go/src/github.com/rogpeppe/godef",
			visible:   true,
		},
		{
			workDir:   "/home/foo/go/src/github.com/rogpeppe/godef/go/ast",
			vendorDir: "/home/foo/go/src/github.com/rogpeppe/godef",
			visible:   true,
		},
		{
			workDir:   "/home/foo/go/src/github.com/rogpeppe/godef",
			vendorDir: "/home/foo/go/src/github.com/uudashr/gopkgs/vendor/github.com/pkg/errors",
			visible:   false,
		},
	}

	for i, c := range cases {
		if got, want := visibleVendor(c.workDir, c.vendorDir), c.visible; got != want {
			t.Error("got:", got, "want:", want, "case:", i)
		}
	}
}
