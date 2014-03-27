package log

import (
	"os"

	golog "github.com/coreos/etcd/third_party/github.com/coreos/go-log/log"
)

// The Verbose flag turns on verbose logging.
var Verbose bool = false

var logger *golog.Logger = golog.New("etcd", false,
	golog.CombinedSink(os.Stdout, "[%s] %s %-9s | %s\n", []string{"prefix", "time", "priority", "message"}))

func Infof(format string, v ...interface{}) {
	logger.Infof(format, v...)
}

func Debugf(format string, v ...interface{}) {
	if Verbose {
		logger.Debugf(format, v...)
	}
}

func Debug(v ...interface{}) {
	if Verbose {
		logger.Debug(v...)
	}
}

func Warnf(format string, v ...interface{}) {
	logger.Warningf(format, v...)
}

func Warn(v ...interface{}) {
	logger.Warning(v...)
}

func Fatalf(format string, v ...interface{}) {
	logger.Fatalf(format, v...)
}

func Fatal(v ...interface{}) {
	logger.Fatalln(v...)
}
