package p

import (
	"io"
	"os"
)

func test(r io.Reader) {
	switch r := r.(type) {
	case *os.File:
		println("file")
	}
}
