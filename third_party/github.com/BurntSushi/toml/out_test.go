package toml

import (
	"flag"
	"fmt"
)

var flagOut = false

func init() {
	flag.BoolVar(&flagOut, "out", flagOut, "Print debug output.")
	flag.Parse()
}

func testf(format string, v ...interface{}) {
	if flagOut {
		fmt.Printf(format, v...)
	}
}
