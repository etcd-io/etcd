package logutils

type Log interface {
	Fatalf(format string, args ...any)
	Panicf(format string, args ...any)
	Errorf(format string, args ...any)
	Warnf(format string, args ...any)
	Infof(format string, args ...any)

	Child(name string) Log
	SetLevel(level LogLevel)
}

type LogLevel int

const (
	// LogLevelDebug Debug messages, write to debug logs only by logutils.Debug.
	LogLevelDebug LogLevel = 0

	// LogLevelInfo Information messages, don't write too many messages,
	// only useful ones: they are shown when running with -v.
	LogLevelInfo LogLevel = 1

	// LogLevelWarn Hidden errors: non-critical errors: work can be continued, no need to fail whole program;
	// tests will crash if any warning occurred.
	LogLevelWarn LogLevel = 2

	// LogLevelError Only not hidden from user errors: whole program failing, usually
	// error logging happens in 1-2 places: in the "main" function.
	LogLevelError LogLevel = 3
)
