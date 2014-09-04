package elog

import (
	. "log"
	"runtime"
)

func TODO() {
	_, file, line, _ := runtime.Caller(1)
	Printf("TODO: %s:%d", file, line)
}
