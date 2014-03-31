package log

import (
	golog "github.com/coreos/etcd/third_party/github.com/coreos/go-log/log"
	"os"
)

// The Verbose flag turns on verbose logging.
var (
	Verbose bool = false
	logger Logger = golog.New("etcd", false,
		golog.CombinedSink(os.Stdout, "[%s] %s %-9s | %s\n", []string{"prefix", "time", "priority", "message"}))
)

type Logger interface {
	Infof(format string, v ...interface{})
	Debugf(format string, v ...interface{})
	Debug(v ...interface{})
	Warningf(format string, v ...interface{})
	Warning(v ...interface{})
	Fatalf(format string, v ...interface{})
	Fatalln(v ...interface{})
}

func SetLogger(l Logger) {
	logger = l
}

func GetLogger() Logger {
	return logger
}

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
