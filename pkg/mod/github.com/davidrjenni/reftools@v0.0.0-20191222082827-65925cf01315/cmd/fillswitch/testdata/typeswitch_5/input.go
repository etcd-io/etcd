package p

import (
	"io"

	_ "github.com/davidrjenni/reftools/cmd/fillswitch/testdata/typeswitch_5/internal/foo"
)

func test(r io.Reader) {
	switch r := r.(type) {
	}
}

type panicReader struct{}

func (r *panicReader) Read(p []byte) (int, error) {
	panic("not implemented")
}

type myReadWriter interface {
	Read(p []byte) (int, error)
	Writer(p []byte) (int, error)
}
