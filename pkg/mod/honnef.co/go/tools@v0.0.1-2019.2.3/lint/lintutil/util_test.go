package lintutil

import (
	"go/token"
	"testing"
)

func TestParsePos(t *testing.T) {
	var tests = []struct {
		in  string
		out token.Position
	}{
		{
			"/tmp/gopackages280076669/go-build/net/cgo_linux.cgo1.go:1",
			token.Position{
				Filename: "/tmp/gopackages280076669/go-build/net/cgo_linux.cgo1.go",
				Line:     1,
				Column:   0,
			},
		},
		{
			"/tmp/gopackages280076669/go-build/net/cgo_linux.cgo1.go:1:",
			token.Position{
				Filename: "/tmp/gopackages280076669/go-build/net/cgo_linux.cgo1.go",
				Line:     1,
				Column:   0,
			},
		},
		{
			"/tmp/gopackages280076669/go-build/net/cgo_linux.cgo1.go:23:43",
			token.Position{
				Filename: "/tmp/gopackages280076669/go-build/net/cgo_linux.cgo1.go",
				Line:     23,
				Column:   43,
			},
		},
	}

	for _, tt := range tests {
		res := parsePos(tt.in)
		if res != tt.out {
			t.Errorf("failed to parse %q correctly", tt.in)
		}
	}
}
