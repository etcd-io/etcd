
# loggo
    import "github.com/juju/loggo"

[![GoDoc](https://godoc.org/github.com/juju/loggo?status.svg)](https://godoc.org/github.com/juju/loggo)

### Module level logging for Go
This package provides an alternative to the standard library log package.

The actual logging functions never return errors.  If you are logging
something, you really don't want to be worried about the logging
having trouble.

Modules have names that are defined by dotted strings.


	"first.second.third"

There is a root module that has the name `""`.  Each module
(except the root module) has a parent, identified by the part of
the name without the last dotted value.
* the parent of "first.second.third" is "first.second"
* the parent of "first.second" is "first"
* the parent of "first" is "" (the root module)

Each module can specify its own severity level.  Logging calls that are of
a lower severity than the module's effective severity level are not written
out.

Loggers are created using the GetLogger function.


	logger := loggo.GetLogger("foo.bar")

By default there is one writer registered, which will write to Stderr,
and the root module, which will only emit warnings and above.
If you want to continue using the default
logger, but have it emit all logging levels you need to do the following.


	writer, _, err := loggo.RemoveWriter("default")
	// err is non-nil if and only if the name isn't found.
	loggo.RegisterWriter("default", writer, loggo.TRACE)






## func ConfigureLoggers
``` go
func ConfigureLoggers(specification string) error
```
ConfigureLoggers configures loggers according to the given string
specification, which specifies a set of modules and their associated
logging levels.  Loggers are colon- or semicolon-separated; each
module is specified as <modulename>=<level>.  White space outside of
module names and levels is ignored.  The root module is specified
with the name "<root>".

An example specification:


	`<root>=ERROR; foo.bar=WARNING`


## func LoggerInfo
``` go
func LoggerInfo() string
```
LoggerInfo returns information about the configured loggers and their logging
levels.  The information is returned in the format expected by
ConfigureModules. Loggers with UNSPECIFIED level will not
be included.


## func ParseConfigurationString
``` go
func ParseConfigurationString(specification string) (map[string]Level, error)
```
ParseConfigurationString parses a logger configuration string into a map of
logger names and their associated log level. This method is provided to
allow other programs to pre-validate a configuration string rather than
just calling ConfigureLoggers.

Loggers are colon- or semicolon-separated; each module is specified as
<modulename>=<level>.  White space outside of module names and levels is
ignored.  The root module is specified with the name "<root>".

As a special case, a log level may be specified on its own.
This is equivalent to specifying the level of the root module,
so "DEBUG" is equivalent to `<root>=DEBUG`

An example specification:


	`<root>=ERROR; foo.bar=WARNING`


## func RegisterWriter
``` go
func RegisterWriter(name string, writer Writer, minLevel Level) error
```
RegisterWriter adds the writer to the list of writers that get notified
when logging.  When registering, the caller specifies the minimum logging
level that will be written, and a name for the writer.  If there is already
a registered writer with that name, an error is returned.


## func ResetLoggers
``` go
func ResetLoggers()
```
ResetLogging iterates through the known modules and sets the levels of all
to UNSPECIFIED, except for <root> which is set to WARNING.


## func ResetWriters
``` go
func ResetWriters()
```
ResetWriters puts the list of writers back into the initial state.


## func WillWrite
``` go
func WillWrite(level Level) bool
```
WillWrite returns whether there are any writers registered
at or above the given severity level. If it returns
false, a log message at the given level will be discarded.



## type DefaultFormatter
``` go
type DefaultFormatter struct{}
```
DefaultFormatter provides a simple concatenation of all the components.











### func (\*DefaultFormatter) Format
``` go
func (*DefaultFormatter) Format(level Level, module, filename string, line int, timestamp time.Time, message string) string
```
Format returns the parameters separated by spaces except for filename and
line which are separated by a colon.  The timestamp is shown to second
resolution in UTC.



## type Formatter
``` go
type Formatter interface {
    Format(level Level, module, filename string, line int, timestamp time.Time, message string) string
}
```
Formatter defines the single method Format, which takes the logging
information, and converts it to a string.











## type Level
``` go
type Level uint32
```
Level holds a severity level.



``` go
const (
    UNSPECIFIED Level = iota
    TRACE
    DEBUG
    INFO
    WARNING
    ERROR
    CRITICAL
)
```
The severity levels. Higher values are more considered more
important.







### func ParseLevel
``` go
func ParseLevel(level string) (Level, bool)
```
ParseLevel converts a string representation of a logging level to a
Level. It returns the level and whether it was valid or not.




### func (Level) String
``` go
func (level Level) String() string
```


## type Logger
``` go
type Logger struct {
    // contains filtered or unexported fields
}
```
A Logger represents a logging module. It has an associated logging
level which can be changed; messages of lesser severity will
be dropped. Loggers have a hierarchical relationship - see
the package documentation.

The zero Logger value is usable - any messages logged
to it will be sent to the root Logger.









### func GetLogger
``` go
func GetLogger(name string) Logger
```
GetLogger returns a Logger for the given module name,
creating it and its parents if necessary.




### func (Logger) Criticalf
``` go
func (logger Logger) Criticalf(message string, args ...interface{})
```
Criticalf logs the printf-formatted message at critical level.



### func (Logger) Debugf
``` go
func (logger Logger) Debugf(message string, args ...interface{})
```
Debugf logs the printf-formatted message at debug level.



### func (Logger) EffectiveLogLevel
``` go
func (logger Logger) EffectiveLogLevel() Level
```
EffectiveLogLevel returns the effective log level of
the receiver - that is, messages with a lesser severity
level will be discarded.

If the log level of the receiver is unspecified,
it will be taken from the effective log level of its
parent.



### func (Logger) Errorf
``` go
func (logger Logger) Errorf(message string, args ...interface{})
```
Errorf logs the printf-formatted message at error level.



### func (Logger) Infof
``` go
func (logger Logger) Infof(message string, args ...interface{})
```
Infof logs the printf-formatted message at info level.



### func (Logger) IsDebugEnabled
``` go
func (logger Logger) IsDebugEnabled() bool
```
IsDebugEnabled returns whether debugging is enabled
at debug level.



### func (Logger) IsErrorEnabled
``` go
func (logger Logger) IsErrorEnabled() bool
```
IsErrorEnabled returns whether debugging is enabled
at error level.



### func (Logger) IsInfoEnabled
``` go
func (logger Logger) IsInfoEnabled() bool
```
IsInfoEnabled returns whether debugging is enabled
at info level.



### func (Logger) IsLevelEnabled
``` go
func (logger Logger) IsLevelEnabled(level Level) bool
```
IsLevelEnabled returns whether debugging is enabled
for the given log level.



### func (Logger) IsTraceEnabled
``` go
func (logger Logger) IsTraceEnabled() bool
```
IsTraceEnabled returns whether debugging is enabled
at trace level.



### func (Logger) IsWarningEnabled
``` go
func (logger Logger) IsWarningEnabled() bool
```
IsWarningEnabled returns whether debugging is enabled
at warning level.



### func (Logger) LogCallf
``` go
func (logger Logger) LogCallf(calldepth int, level Level, message string, args ...interface{})
```
LogCallf logs a printf-formatted message at the given level.
The location of the call is indicated by the calldepth argument.
A calldepth of 1 means the function that called this function.
A message will be discarded if level is less than the
the effective log level of the logger.
Note that the writers may also filter out messages that
are less than their registered minimum severity level.



### func (Logger) LogLevel
``` go
func (logger Logger) LogLevel() Level
```
LogLevel returns the configured log level of the logger.



### func (Logger) Logf
``` go
func (logger Logger) Logf(level Level, message string, args ...interface{})
```
Logf logs a printf-formatted message at the given level.
A message will be discarded if level is less than the
the effective log level of the logger.
Note that the writers may also filter out messages that
are less than their registered minimum severity level.



### func (Logger) Name
``` go
func (logger Logger) Name() string
```
Name returns the logger's module name.



### func (Logger) SetLogLevel
``` go
func (logger Logger) SetLogLevel(level Level)
```
SetLogLevel sets the severity level of the given logger.
The root logger cannot be set to UNSPECIFIED level.
See EffectiveLogLevel for how this affects the
actual messages logged.



### func (Logger) Tracef
``` go
func (logger Logger) Tracef(message string, args ...interface{})
```
Tracef logs the printf-formatted message at trace level.



### func (Logger) Warningf
``` go
func (logger Logger) Warningf(message string, args ...interface{})
```
Warningf logs the printf-formatted message at warning level.



## type TestLogValues
``` go
type TestLogValues struct {
    Level     Level
    Module    string
    Filename  string
    Line      int
    Timestamp time.Time
    Message   string
}
```
TestLogValues represents a single logging call.











## type TestWriter
``` go
type TestWriter struct {
    // contains filtered or unexported fields
}
```
TestWriter is a useful Writer for testing purposes.  Each component of the
logging message is stored in the Log array.











### func (\*TestWriter) Clear
``` go
func (writer *TestWriter) Clear()
```
Clear removes any saved log messages.



### func (\*TestWriter) Log
``` go
func (writer *TestWriter) Log() []TestLogValues
```
Log returns a copy of the current logged values.



### func (\*TestWriter) Write
``` go
func (writer *TestWriter) Write(level Level, module, filename string, line int, timestamp time.Time, message string)
```
Write saves the params as members in the TestLogValues struct appended to the Log array.



## type Writer
``` go
type Writer interface {
    // Write writes a message to the Writer with the given
    // level and module name. The filename and line hold
    // the file name and line number of the code that is
    // generating the log message; the time stamp holds
    // the time the log message was generated, and
    // message holds the log message itself.
    Write(level Level, name, filename string, line int, timestamp time.Time, message string)
}
```
Writer is implemented by any recipient of log messages.









### func NewSimpleWriter
``` go
func NewSimpleWriter(writer io.Writer, formatter Formatter) Writer
```
NewSimpleWriter returns a new writer that writes
log messages to the given io.Writer formatting the
messages with the given formatter.


### func RemoveWriter
``` go
func RemoveWriter(name string) (Writer, Level, error)
```
RemoveWriter removes the Writer identified by 'name' and returns it.
If the Writer is not found, an error is returned.


### func ReplaceDefaultWriter
``` go
func ReplaceDefaultWriter(writer Writer) (Writer, error)
```
ReplaceDefaultWriter is a convenience method that does the equivalent of
RemoveWriter and then RegisterWriter with the name "default".  The previous
default writer, if any is returned.










- - -
Generated by [godoc2md](http://godoc.org/github.com/davecheney/godoc2md)
