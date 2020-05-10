package pkg

import (
	"strings"
	"testing"
)

func TestFoo(t *testing.T) {
	strings.Replace("", "", "", 1) // want `is a pure function but its return value is ignored`
}

func BenchmarkFoo(b *testing.B) {
	strings.Replace("", "", "", 1)
}

func doBenchmark(s string, b *testing.B) {
	strings.Replace("", "", "", 1)
}

func BenchmarkBar(b *testing.B) {
	doBenchmark("", b)
}
