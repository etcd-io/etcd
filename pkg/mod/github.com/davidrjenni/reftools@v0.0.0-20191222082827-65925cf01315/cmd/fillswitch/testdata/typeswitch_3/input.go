package p

import (
	"io"
	_ "os"
)

func test(r io.Reader) {
	switch r := r.(type) {
	}
}
