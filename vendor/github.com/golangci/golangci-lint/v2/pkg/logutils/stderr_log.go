package logutils

import (
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/golangci/golangci-lint/v2/pkg/exitcodes"
)

const (
	// envLogLevel values: "error", "err", "warning", "warn","info"
	envLogLevel = "LOG_LEVEL"
	// envLogTimestamp value: "1"
	envLogTimestamp = "LOG_TIMESTAMP"
)

var _ Log = NewStderrLog(DebugKeyEmpty)

type StderrLog struct {
	name   string
	logger *logrus.Logger
	level  LogLevel
}

func NewStderrLog(name string) *StderrLog {
	sl := &StderrLog{
		name:   name,
		logger: logrus.New(),
		level:  LogLevelWarn,
	}

	switch os.Getenv(envLogLevel) {
	case "error", "err":
		sl.logger.SetLevel(logrus.ErrorLevel)
	case "warning", "warn":
		sl.logger.SetLevel(logrus.WarnLevel)
	case "info":
		sl.logger.SetLevel(logrus.InfoLevel)
	default:
		sl.logger.SetLevel(logrus.DebugLevel)
	}

	sl.logger.Out = StdErr
	sl.logger.Formatter = logFormatter

	return sl
}

func (sl StderrLog) prefix() string {
	prefix := ""
	if sl.name != "" {
		prefix = fmt.Sprintf("[%s] ", sl.name)
	}

	return prefix
}

func (sl StderrLog) Fatalf(format string, args ...any) {
	sl.logger.Errorf("%s%s", sl.prefix(), fmt.Sprintf(format, args...))
	os.Exit(exitcodes.Failure)
}

func (sl StderrLog) Panicf(format string, args ...any) {
	v := fmt.Sprintf("%s%s", sl.prefix(), fmt.Sprintf(format, args...))
	panic(v)
}

func (sl StderrLog) Errorf(format string, args ...any) {
	if sl.level > LogLevelError {
		return
	}

	sl.logger.Errorf("%s%s", sl.prefix(), fmt.Sprintf(format, args...))
	// don't call exitIfTest() because the idea is to
	// crash on hidden errors (warnings); but Errorf MUST NOT be
	// called on hidden errors, see log levels comments.
}

func (sl StderrLog) Warnf(format string, args ...any) {
	if sl.level > LogLevelWarn {
		return
	}

	sl.logger.Warnf("%s%s", sl.prefix(), fmt.Sprintf(format, args...))
}

func (sl StderrLog) Infof(format string, args ...any) {
	if sl.level > LogLevelInfo {
		return
	}

	sl.logger.Infof("%s%s", sl.prefix(), fmt.Sprintf(format, args...))
}

func (sl StderrLog) Debugf(format string, args ...any) {
	if sl.level > LogLevelDebug {
		return
	}

	sl.logger.Debugf("%s%s", sl.prefix(), fmt.Sprintf(format, args...))
}

func (sl StderrLog) Child(name string) Log {
	prefix := ""
	if sl.name != "" {
		prefix = sl.name + "/"
	}

	child := sl
	child.name = prefix + name

	return &child
}

func (sl *StderrLog) SetLevel(level LogLevel) {
	sl.level = level
}

var logFormatter = newLogFormatter()

func DisableColors(disable bool) {
	logFormatter.DisableColors = disable
}

func newLogFormatter() *logrus.TextFormatter {
	formatter := &logrus.TextFormatter{
		DisableTimestamp:          true, // `INFO[0007] msg` -> `INFO msg`
		EnvironmentOverrideColors: true,
	}

	if os.Getenv(envLogTimestamp) == "1" {
		formatter.DisableTimestamp = false
		formatter.FullTimestamp = true
		formatter.TimestampFormat = time.StampMilli
	}

	return formatter
}
