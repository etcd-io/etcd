package test

import (
	"syscall"

	golog "github.com/coreos/etcd/third_party/github.com/coreos/go-log/log"

	"github.com/coreos/etcd/log"
)

func init() {
	// Set higher number of file limit.
	// Or it will suffer from lack of file descriptors.
	// TODO(yichengq): set it lower later. It is not supposed to
	// use that many.
	var rlim syscall.Rlimit
	syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlim)
	rlim.Cur = 65536
	syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rlim)

	// Currently Fatal and Fatalf are used in the project.
	// Overwrite them using Panic, so it could catch the error and don't
	// exit the process.
	// It should not leak goroutines because Fatal is only used before
	// starting the peer server and client server.
	// TODO(yichengq): Remove Panic calls in the project.
	logger := &Logger{(log.GetLogger()).(*golog.Logger)}
	log.SetLogger(logger)
}

type Logger struct {
	*golog.Logger
}

func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.Panicf(format, v...)
}

func (l *Logger) Fatalln(v ...interface{}) {
	l.Panicln(v...)
}
