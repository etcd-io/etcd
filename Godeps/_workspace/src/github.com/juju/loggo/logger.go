// Copyright 2014 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package loggo

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Level holds a severity level.
type Level uint32

// The severity levels. Higher values are more considered more
// important.
const (
	UNSPECIFIED Level = iota
	TRACE
	DEBUG
	INFO
	WARNING
	ERROR
	CRITICAL
)

// A Logger represents a logging module. It has an associated logging
// level which can be changed; messages of lesser severity will
// be dropped. Loggers have a hierarchical relationship - see
// the package documentation.
//
// The zero Logger value is usable - any messages logged
// to it will be sent to the root Logger.
type Logger struct {
	impl *module
}

type module struct {
	name   string
	level  Level
	parent *module
}

// Initially the modules map only contains the root module.
var (
	root         = &module{level: WARNING}
	modulesMutex sync.Mutex
	modules      = map[string]*module{
		"": root,
	}
)

func (level Level) String() string {
	switch level {
	case UNSPECIFIED:
		return "UNSPECIFIED"
	case TRACE:
		return "TRACE"
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARNING:
		return "WARNING"
	case ERROR:
		return "ERROR"
	case CRITICAL:
		return "CRITICAL"
	}
	return "<unknown>"
}

// get atomically gets the value of the given level.
func (level *Level) get() Level {
	return Level(atomic.LoadUint32((*uint32)(level)))
}

// set atomically sets the value of the receiver
// to the given level.
func (level *Level) set(newLevel Level) {
	atomic.StoreUint32((*uint32)(level), uint32(newLevel))
}

// getLoggerInternal assumes that the modulesMutex is locked.
func getLoggerInternal(name string) Logger {
	impl, found := modules[name]
	if found {
		return Logger{impl}
	}
	parentName := ""
	if i := strings.LastIndex(name, "."); i >= 0 {
		parentName = name[0:i]
	}
	parent := getLoggerInternal(parentName).impl
	impl = &module{name, UNSPECIFIED, parent}
	modules[name] = impl
	return Logger{impl}
}

// GetLogger returns a Logger for the given module name,
// creating it and its parents if necessary.
func GetLogger(name string) Logger {
	// Lowercase the module name, and look for it in the modules map.
	name = strings.ToLower(name)
	modulesMutex.Lock()
	defer modulesMutex.Unlock()
	return getLoggerInternal(name)
}

// LoggerInfo returns information about the configured loggers and their logging
// levels.  The information is returned in the format expected by
// ConfigureModules. Loggers with UNSPECIFIED level will not
// be included.
func LoggerInfo() string {
	output := []string{}
	// output in alphabetical order.
	keys := []string{}
	modulesMutex.Lock()
	defer modulesMutex.Unlock()
	for key := range modules {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, name := range keys {
		mod := modules[name]
		severity := mod.level
		if severity == UNSPECIFIED {
			continue
		}
		output = append(output, fmt.Sprintf("%s=%s", mod.Name(), severity))
	}
	return strings.Join(output, ";")
}

// ParseConfigurationString parses a logger configuration string into a map of
// logger names and their associated log level. This method is provided to
// allow other programs to pre-validate a configuration string rather than
// just calling ConfigureLoggers.
//
// Loggers are colon- or semicolon-separated; each module is specified as
// <modulename>=<level>.  White space outside of module names and levels is
// ignored.  The root module is specified with the name "<root>".
//
// As a special case, a log level may be specified on its own.
// This is equivalent to specifying the level of the root module,
// so "DEBUG" is equivalent to `<root>=DEBUG`
//
// An example specification:
//	`<root>=ERROR; foo.bar=WARNING`
func ParseConfigurationString(specification string) (map[string]Level, error) {
	levels := make(map[string]Level)
	if level, ok := ParseLevel(specification); ok {
		levels[""] = level
		return levels, nil
	}
	values := strings.FieldsFunc(specification, func(r rune) bool { return r == ';' || r == ':' })
	for _, value := range values {
		s := strings.SplitN(value, "=", 2)
		if len(s) < 2 {
			return nil, fmt.Errorf("logger specification expected '=', found %q", value)
		}
		name := strings.TrimSpace(s[0])
		levelStr := strings.TrimSpace(s[1])
		if name == "" || levelStr == "" {
			return nil, fmt.Errorf("logger specification %q has blank name or level", value)
		}
		if name == "<root>" {
			name = ""
		}
		level, ok := ParseLevel(levelStr)
		if !ok {
			return nil, fmt.Errorf("unknown severity level %q", levelStr)
		}
		levels[name] = level
	}
	return levels, nil
}

// ConfigureLoggers configures loggers according to the given string
// specification, which specifies a set of modules and their associated
// logging levels.  Loggers are colon- or semicolon-separated; each
// module is specified as <modulename>=<level>.  White space outside of
// module names and levels is ignored.  The root module is specified
// with the name "<root>".
//
// An example specification:
//	`<root>=ERROR; foo.bar=WARNING`
func ConfigureLoggers(specification string) error {
	if specification == "" {
		return nil
	}
	levels, err := ParseConfigurationString(specification)
	if err != nil {
		return err
	}
	for name, level := range levels {
		GetLogger(name).SetLogLevel(level)
	}
	return nil
}

// ResetLogging iterates through the known modules and sets the levels of all
// to UNSPECIFIED, except for <root> which is set to WARNING.
func ResetLoggers() {
	modulesMutex.Lock()
	defer modulesMutex.Unlock()
	for name, module := range modules {
		if name == "" {
			module.level.set(WARNING)
		} else {
			module.level.set(UNSPECIFIED)
		}
	}
}

// ParseLevel converts a string representation of a logging level to a
// Level. It returns the level and whether it was valid or not.
func ParseLevel(level string) (Level, bool) {
	level = strings.ToUpper(level)
	switch level {
	case "UNSPECIFIED":
		return UNSPECIFIED, true
	case "TRACE":
		return TRACE, true
	case "DEBUG":
		return DEBUG, true
	case "INFO":
		return INFO, true
	case "WARN", "WARNING":
		return WARNING, true
	case "ERROR":
		return ERROR, true
	case "CRITICAL":
		return CRITICAL, true
	}
	return UNSPECIFIED, false
}

func (logger Logger) getModule() *module {
	if logger.impl == nil {
		return root
	}
	return logger.impl
}

// Name returns the logger's module name.
func (logger Logger) Name() string {
	return logger.getModule().Name()
}

// LogLevel returns the configured log level of the logger.
func (logger Logger) LogLevel() Level {
	return logger.getModule().level.get()
}

func (module *module) getEffectiveLogLevel() Level {
	// Note: the root module is guaranteed to have a
	// specified logging level, so acts as a suitable sentinel
	// for this loop.
	for {
		if level := module.level.get(); level != UNSPECIFIED {
			return level
		}
		module = module.parent
	}
	panic("unreachable")
}

func (module *module) Name() string {
	if module.name == "" {
		return "<root>"
	}
	return module.name
}

// EffectiveLogLevel returns the effective log level of
// the receiver - that is, messages with a lesser severity
// level will be discarded.
//
// If the log level of the receiver is unspecified,
// it will be taken from the effective log level of its
// parent.
func (logger Logger) EffectiveLogLevel() Level {
	return logger.getModule().getEffectiveLogLevel()
}

// SetLogLevel sets the severity level of the given logger.
// The root logger cannot be set to UNSPECIFIED level.
// See EffectiveLogLevel for how this affects the
// actual messages logged.
func (logger Logger) SetLogLevel(level Level) {
	module := logger.getModule()
	// The root module can't be unspecified.
	if module.name == "" && level == UNSPECIFIED {
		level = WARNING
	}
	module.level.set(level)
}

// Logf logs a printf-formatted message at the given level.
// A message will be discarded if level is less than the
// the effective log level of the logger.
// Note that the writers may also filter out messages that
// are less than their registered minimum severity level.
func (logger Logger) Logf(level Level, message string, args ...interface{}) {
	logger.LogCallf(2, level, message, args...)
}

// LogCallf logs a printf-formatted message at the given level.
// The location of the call is indicated by the calldepth argument.
// A calldepth of 1 means the function that called this function.
// A message will be discarded if level is less than the
// the effective log level of the logger.
// Note that the writers may also filter out messages that
// are less than their registered minimum severity level.
func (logger Logger) LogCallf(calldepth int, level Level, message string, args ...interface{}) {
	if logger.getModule().getEffectiveLogLevel() > level ||
		!WillWrite(level) ||
		level < TRACE ||
		level > CRITICAL {
		return
	}
	// Gather time, and filename, line number.
	now := time.Now() // get this early.
	// Param to Caller is the call depth.  Since this method is called from
	// the Logger methods, we want the place that those were called from.
	_, file, line, ok := runtime.Caller(calldepth + 1)
	if !ok {
		file = "???"
		line = 0
	}
	// Trim newline off format string, following usual
	// Go logging conventions.
	if len(message) > 0 && message[len(message)-1] == '\n' {
		message = message[0 : len(message)-1]
	}

	// To avoid having a proliferation of Info/Infof methods,
	// only use Sprintf if there are any args, and rely on the
	// `go vet` tool for the obvious cases where someone has forgotten
	// to provide an arg.
	formattedMessage := message
	if len(args) > 0 {
		formattedMessage = fmt.Sprintf(message, args...)
	}
	writeToWriters(level, logger.impl.name, file, line, now, formattedMessage)
}

// Criticalf logs the printf-formatted message at critical level.
func (logger Logger) Criticalf(message string, args ...interface{}) {
	logger.Logf(CRITICAL, message, args...)
}

// Errorf logs the printf-formatted message at error level.
func (logger Logger) Errorf(message string, args ...interface{}) {
	logger.Logf(ERROR, message, args...)
}

// Warningf logs the printf-formatted message at warning level.
func (logger Logger) Warningf(message string, args ...interface{}) {
	logger.Logf(WARNING, message, args...)
}

// Infof logs the printf-formatted message at info level.
func (logger Logger) Infof(message string, args ...interface{}) {
	logger.Logf(INFO, message, args...)
}

// Debugf logs the printf-formatted message at debug level.
func (logger Logger) Debugf(message string, args ...interface{}) {
	logger.Logf(DEBUG, message, args...)
}

// Tracef logs the printf-formatted message at trace level.
func (logger Logger) Tracef(message string, args ...interface{}) {
	logger.Logf(TRACE, message, args...)
}

// IsLevelEnabled returns whether debugging is enabled
// for the given log level.
func (logger Logger) IsLevelEnabled(level Level) bool {
	return logger.getModule().getEffectiveLogLevel() <= level
}

// IsErrorEnabled returns whether debugging is enabled
// at error level.
func (logger Logger) IsErrorEnabled() bool {
	return logger.IsLevelEnabled(ERROR)
}

// IsWarningEnabled returns whether debugging is enabled
// at warning level.
func (logger Logger) IsWarningEnabled() bool {
	return logger.IsLevelEnabled(WARNING)
}

// IsInfoEnabled returns whether debugging is enabled
// at info level.
func (logger Logger) IsInfoEnabled() bool {
	return logger.IsLevelEnabled(INFO)
}

// IsDebugEnabled returns whether debugging is enabled
// at debug level.
func (logger Logger) IsDebugEnabled() bool {
	return logger.IsLevelEnabled(DEBUG)
}

// IsTraceEnabled returns whether debugging is enabled
// at trace level.
func (logger Logger) IsTraceEnabled() bool {
	return logger.IsLevelEnabled(TRACE)
}
