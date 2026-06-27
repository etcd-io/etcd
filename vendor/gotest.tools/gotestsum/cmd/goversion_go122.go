//go:build go1.22
// +build go1.22

package cmd

import (
	goversion "go/version"
	"runtime"
)

func isGoVersionAtLeast(v string) bool {
	return goversion.Compare(v, runtime.Version()) < 0
}
