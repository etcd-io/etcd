package log

import (
	"context"
	"io"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const DefaultLogLevel = "info"

// Mapped to logrus
const (
	PanicLevel Level = Level(logrus.PanicLevel)
	FatalLevel Level = Level(logrus.FatalLevel)
	ErrorLevel Level = Level(logrus.ErrorLevel)
	WarnLevel  Level = Level(logrus.WarnLevel)
	InfoLevel  Level = Level(logrus.InfoLevel)
	DebugLevel Level = Level(logrus.DebugLevel)
)

var (
	logger = &Logger{logrus.StandardLogger()}
)

// Fields type, used to pass to `WithFields`
type Fields logrus.Fields

// Formatter interface is used to implement a custom Formatter
type Formatter logrus.Formatter

// Level type
type Level logrus.Level

// Hook to be fired when logging on the logging levels returned from
// `Levels()` on your implementation of the interface.
type Hook logrus.Hook

// Logger is a wrap around a logrus FieldLogger
type Logger struct {
	logrus.FieldLogger
}

func standardFormat(logger *logrus.Logger) {
	formatter := new(logrus.TextFormatter)
	formatter.TimestampFormat = time.RFC3339Nano
	formatter.FullTimestamp = true
	logger.SetFormatter(formatter)
}

func (l *Logger) SetLogLevelFromEnv(envName string) {
	ll := strings.TrimSpace(os.Getenv(envName))
	if ll == "" {
		ll = DefaultLogLevel
	}

	logLevel, err := logrus.ParseLevel(ll)
	if err != nil {
		logrus.WithFields(logrus.Fields{"level": ll}).Warn("Could not parse log level, setting to INFO")
		logLevel = logrus.InfoLevel
	}
	// this is unfortunate
	reall, ok := l.FieldLogger.(*logrus.Logger)
	if !ok {
		panic(errors.New("could not implementation with SetLevel and SetFormatter"))
	}
	reall.SetLevel(logLevel)
	standardFormat(reall)
}

type option func(logger *logrus.Logger)

// ConfigureLevel returns an opaque object that can be passed to New
// to set the new logger's level.
func ConfigureLevel(l Level) option {
	return func(logger *logrus.Logger) {
		logger.SetLevel(logrus.Level(l))
	}
}

// New creates new logger, configured with the relevant options
func New(options ...option) *Logger {
	l := logrus.New()
	for _, option := range options {
		option(l)
	}
	standardFormat(l)
	return &Logger{l}
}

// StandardLogger returns a new standard logger, which is the same
// global log object  used by the package level log functions
// (e.g. log.Infof)
func StandardLogger() *Logger {
	return logger
}

type contextKey string

// WithLogger stores the logger.
func withLogger(ctx context.Context, l logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, contextKey("logger"), l)
}

// LoggerFromContext returns the structured logger stored in the context, if present
// or a new standard logger.
func LoggerFromContext(ctx context.Context) *Logger {
	l, ok := ctx.Value(contextKey("logger")).(logrus.FieldLogger)
	if !ok {
		l = logrus.StandardLogger()
	}
	return &Logger{l}
}

// WithLogger returns a new context which is a copy of the parent context
// with the addition of the logger
func WithLogger(ctx context.Context, l *Logger) context.Context {
	return withLogger(ctx, l.FieldLogger)
}

// WithField wraps logrus
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{l.FieldLogger.WithField(key, value)}
}

// WithFields wraps logrus
func (l *Logger) WithFields(fields Fields) *Logger {
	return &Logger{l.FieldLogger.WithFields(logrus.Fields(fields))}
}

// AddFields adds some fields globally to the standard logger
func AddFields(fields Fields) {
	logger = logger.WithFields(fields)
}

// SetOutput sets the standard logger output.
func SetOutput(out io.Writer) {
	logrus.SetOutput(out)
}

// SetFormatter sets the standard logger formatter.
func SetFormatter(formatter Formatter) {
	logrus.SetFormatter(logrus.Formatter(formatter))
}

// SetLevel sets the standard logger level.
func SetLevel(level Level) {
	logrus.SetLevel(logrus.Level(level))
}

// GetLevel returns the standard logger level.
func GetLevel() Level {
	return Level(logrus.GetLevel())
}

// AddHook adds a hook to the standard logger hooks.
func AddHook(hook Hook) {
	logrus.AddHook(logrus.Hook(hook))
}

// WithError creates an logger from the standard logger and adds an error to it, using the value defined in ErrorKey as key.
func WithError(err error) *Logger {
	return logger.WithField(logrus.ErrorKey, err)
}

// WithField creates an logger from the standard logger and adds a field to
// it. If you want multiple fields, use `WithFields`.
//
// Note that it doesn't log until you call Debug, Print, Info, Warn, Fatal
// or Panic on the Entry it returns.
func WithField(key string, value interface{}) *Logger {
	return logger.WithField(key, value)
}

// WithFields creates an logger from the standard logger and adds multiple
// fields to it. This is simply a helper for `WithField`, invoking it
// once for each field.
//
// Note that it doesn't log until you call Debug, Print, Info, Warn, Fatal
// or Panic on the Entry it returns.
func WithFields(fields Fields) *Logger {
	return logger.WithFields(fields)
}

// Debug logs a message at level Debug on the standard logger.
func Debug(args ...interface{}) {
	logger.Debug(args...)
}

// Print logs a message at level Info on the standard logger.
func Print(args ...interface{}) {
	logger.Print(args...)
}

// Info logs a message at level Info on the standard logger.
func Info(args ...interface{}) {
	logger.Info(args...)
}

// Warn logs a message at level Warn on the standard logger.
func Warn(args ...interface{}) {
	logger.Warn(args...)
}

// Warning logs a message at level Warn on the standard logger.
func Warning(args ...interface{}) {
	logger.Warning(args...)
}

// Error logs a message at level Error on the standard logger.
func Error(args ...interface{}) {
	logger.Error(args...)
}

// Panic logs a message at level Panic on the standard logger.
func Panic(args ...interface{}) {
	logger.Panic(args...)
}

// Fatal logs a message at level Fatal on the standard logger.
func Fatal(args ...interface{}) {
	logger.Fatal(args...)
}

// Debugf logs a message at level Debug on the standard logger.
func Debugf(format string, args ...interface{}) {
	logger.Debugf(format, args...)
}

// Printf logs a message at level Info on the standard logger.
func Printf(format string, args ...interface{}) {
	logger.Printf(format, args...)
}

// Infof logs a message at level Info on the standard logger.
func Infof(format string, args ...interface{}) {
	logger.Infof(format, args...)
}

// Warnf logs a message at level Warn on the standard logger.
func Warnf(format string, args ...interface{}) {
	logger.Warnf(format, args...)
}

// Warningf logs a message at level Warn on the standard logger.
func Warningf(format string, args ...interface{}) {
	logger.Warningf(format, args...)
}

// Errorf logs a message at level Error on the standard logger.
func Errorf(format string, args ...interface{}) {
	logger.Errorf(format, args...)
}

// Panicf logs a message at level Panic on the standard logger.
func Panicf(format string, args ...interface{}) {
	logger.Panicf(format, args...)
}

// Fatalf logs a message at level Fatal on the standard logger.
func Fatalf(format string, args ...interface{}) {
	logger.Fatalf(format, args...)
}

// Debugln logs a message at level Debug on the standard logger.
func Debugln(args ...interface{}) {
	logger.Debugln(args...)
}

// Println logs a message at level Info on the standard logger.
func Println(args ...interface{}) {
	logger.Println(args...)
}

// Infoln logs a message at level Info on the standard logger.
func Infoln(args ...interface{}) {
	logger.Infoln(args...)
}

// Warnln logs a message at level Warn on the standard logger.
func Warnln(args ...interface{}) {
	logger.Warnln(args...)
}

// Warningln logs a message at level Warn on the standard logger.
func Warningln(args ...interface{}) {
	logger.Warningln(args...)
}

// Errorln logs a message at level Error on the standard logger.
func Errorln(args ...interface{}) {
	logger.Errorln(args...)
}

// Panicln logs a message at level Panic on the standard logger.
func Panicln(args ...interface{}) {
	logger.Panicln(args...)
}

// Fatalln logs a message at level Fatal on the standard logger.
func Fatalln(args ...interface{}) {
	logger.Fatalln(args...)
}

// RootLogger sets the "root" field to some string
func RootLogger(logger *Logger, root string) *Logger {
	return logger.WithFields(Fields{"root": root})
}
