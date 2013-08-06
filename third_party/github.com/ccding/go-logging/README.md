#go-logging
go-logging is a high-performance logging library for golang.
* Simple: It supports only necessary operations and easy to get start.
* Fast: Asynchronous logging without runtime-related fields has an extremely
  low delay of about 800 nano-seconds.

## Getting Started
The stable version is under the `stable` branch, which does never revert and
is fully tested. The tags in `stable` branch indicate the version numbers.

However, `master` branch is unstable version, and `dev` branch is development
branch. `master` branch merges `dev` branch periodically.

Btw, all the pull request should be sent to the `dev` branch.

### Installation
The step below will download the library source code to
`${GOPATH}/src/github.com/ccding/go-logging`.
```bash
go get github.com/ccding/go-logging/logging
```

Given the source code downloaded, it makes you be able to run the examples,
tests, and benchmarks.
```bash
cd ${GOPATH}/src/github.com/ccding/go-logging/logging
go get
go run ../example.go
go test -v -bench .
```

### Example
go-logging is used like any other Go libraries. You can simply use the library
in this way.
```go
import "github.com/ccding/go-logging/logging"
```

Here is a simple example.
```go
package main

import (
	"github.com/ccding/go-logging/logging"
)

func main() {
	logger, _ := logging.SimpleLogger("main")
	logger.SetLevel(logging.DEBUG)
	logger.Error("this is a test from error")
	logger.Destroy()
}
```

### Configuration
#### Construction Functions
It has the following functions to create a logger.
```go
// with BasicFormat and writing to stdout
SimpleLogger(name string) (*Logger, error)
// with BasicFormat and writing to DefaultFileName
BasicLogger(name string) (*Logger, error)
// with RichFormatand writing to DefaultFileName
RichLogger(name string) (*Logger, error)
// with detailed configuration and writing to file
FileLogger(name string, level Level, format string, timeFormat string, file string, sync bool) (*Logger, error)
// with detailed configuration and writing to a writer
WriterLogger(name string, level Level, format string, timeFormat string, out io.Writer, sync bool) (*Logger, error)
// read configurations from a config file
ConfigLogger(filename string) (*Logger, error)
```
The meanings of these fields are
```go
name           string        // logger name
level          Level         // record level higher than this will be printed
format         string        // format configuration
timeFormat     string        // format for time
file           string        // file name for logging
out            io.Writer     // writer for logging
sync           bool          // use sync or async way to record logs
```
The detailed description of these fields will be presented later.

#### Logging Functions
It supports the following functions for logging. All of these functions are
thread-safe.
```go
(*Logger) Logf(level Level, format string, v ...interface{})
(*Logger) Log(level Level, v ...interface{})
(*Logger) Criticalf(format string, v ...interface{})
(*Logger) Critical(v ...interface{})
(*Logger) Fatalf(format string, v ...interface{})
(*Logger) Fatal(v ...interface{})
(*Logger) Errorf(format string, v ...interface{})
(*Logger) Error(v ...interface{})
(*Logger) Warningf(format string, v ...interface{})
(*Logger) Warning(v ...interface{})
(*Logger) Warnf(format string, v ...interface{})
(*Logger) Warn(v ...interface{})
(*Logger) Infof(format string, v ...interface{})
(*Logger) Info(v ...interface{})
(*Logger) Debugf(format string, v ...interface{})
(*Logger) Debug(v ...interface{})
(*Logger) Notsetf(format string, v ...interface{})
(*Logger) Notset(v ...interface{})
```

#### Logger Operations
The logger supports the following operations.  In these functions, `SetWriter`
and `Destroy` are not thread-safe, while others are. All these functions are
running in a synchronous way.
```go
// Getter functions
(*Logger) Name() string                    // get name
(*Logger) TimeFormat() string              // get time format
(*Logger) Level() Level                    // get level  [this function is thread safe]
(*Logger) RecordFormat() string            // get the first part of the format
(*Logger) RecordArgs() []string            // get the second part of the format
(*Logger) Writer() io.Writer               // get writer
(*Logger) Sync() bool                      // get sync or async

// Setter functions
(*Logger) SetLevel(level Level)            // set level  [this function is thread safe]
(*Logger) SetWriter(out ...io.Writer)      // set multiple writers

// Other functions
(*Logger) Flush()             // flush the writer
(*Logger) Destroy()           // destroy the logger
```

#### Fields Description

##### Name
Name field is a string, which can be written to the logging and used to
separate multiple loggers. It allows two logger having the same name.  There
is not any default value for name.

##### Logging Levels
There are these levels in logging.
```go
CRITICAL     50
FATAL        CRITICAL
ERROR        40
WARNING      30
WARN         WARNING
INFO         20
DEBUG        10
NOTSET       0
```

##### Record Format
The record format is described by a string, which has two parts separated by
`\n`. The first part describes the format of the log, and the second part
lists all the fields to be shown in the log. In other word, the first part is
the first parameter `format` of `fmt.Printf(format string, v ...interface{})`,
and the second part describes the second parameter `v` of it. It is not
allowed to have `\n` in the first part.  The fields in the second part are
separated by comma `,`, while extra blank spaces are allowed.  An example of
the format string is
```go
const BasicFormat = "%s [%6s] %30s - %s\n name,levelname,time,message"
```
which is the pre-defined `BasicFormat` used by `BasicLogger()` and
`SimpleLogger()`.

It supports the following fields for the second part of the format.
```go
"name"          string     %s      // name of the logger
"seqid"         uint64     %d      // sequence number
"levelno"       int32      %d      // level number
"levelname"     string     %s      // level name
"created"       int64      %d      // starting time of the logger
"nsecs"         int64      %d      // nanosecond of the starting time
"time"          string     %s      // record created time
"timestamp"     int64      %d      // timestamp of record
"rtime"         int64      %d      // relative time since started
"filename"      string     %s      // source filename of the caller
"pathname"      string     %s      // filename with path
"module"        string     %s      // executable filename
"lineno"        int        %d      // line number in source code
"funcname"      string     %s      // function name of the caller
"thread"        int32      %d      // thread id
"process"       int        %d      // process id
"message"       string     %d      // logger message
```
The following runtime-related fields is extremely expensive and slow, please
be careful when using them.
```go
"filename"      string     %s      // source filename of the caller
"pathname"      string     %s      // filename with path
"lineno"        int        %d      // line number in source code
"funcname"      string     %s      // function name of the caller
"thread"        int32      %d      // thread id
```

There are a few pre-defined values for record format.
```go
BasicFormat = "%s [%6s] %30s - %s\n name,levelname,time,message"
RichFormat  = "%s [%6s] %d %30s - %d - %s:%s:%d - %s\n name, levelname, seqid, time, thread, filename, funcname, lineno, message"
```

##### Time Format
We use the same time format as golang.  The default time format is
```go
DefaultTimeFormat     = "2006-01-02 15:04:05.999999999" // default time format
```

##### File Name, Writer, and Sync
The meaning of these fields are obvious. Filename is used to create writer.
We also allow the user create a writer by herself and pass it to the logger.
Sync describes whether the user would like to use synchronous or asynchronous
method to write logs. `true` value means synchronous method, and `false` value
means asynchronous way.  We suggest you use asynchronous way because it causes
extremely low extra delay by the logging functions.

## Contributors
In alphabetical order
* Cong Ding ([ccding][ccding])
* Xiang Li ([xiangli-cmu][xiangli])
* Zifei Tong ([5kg][5kg])
[ccding]: //github.com/ccding
[xiangli]: //github.com/xiangli-cmu
[5kg]: //github.com/5kg

## TODO List
1. logging server
