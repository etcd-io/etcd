package capnslog

import (
	"fmt"
	"os"
)

type PackageLogger struct {
	pkg   string
	level LogLevel
}

const calldepth = 3

func (p *PackageLogger) internalLog(depth int, inLevel LogLevel, entries ...LogEntry) {
	logger.Lock()
	defer logger.Unlock()
	if logger.formatter != nil {
		logger.formatter.Format(p.pkg, inLevel, depth+1, entries...)
	}
}

func (p *PackageLogger) LevelAt(l LogLevel) bool {
	return p.level >= l
}

// log stdlib compatibility

func (p *PackageLogger) Println(args ...interface{}) {
	if p.level < INFO {
		return
	}
	p.internalLog(calldepth, INFO, BaseLogEntry(fmt.Sprintln(args...)))
}

func (p *PackageLogger) Printf(format string, args ...interface{}) {
	if p.level < INFO {
		return
	}
	p.internalLog(calldepth, INFO, BaseLogEntry(fmt.Sprintf(format, args...)))
}

func (p *PackageLogger) Print(args ...interface{}) {
	if p.level < INFO {
		return
	}
	p.internalLog(calldepth, INFO, BaseLogEntry(fmt.Sprint(args...)))
}

// Panic and fatal

func (p *PackageLogger) Panicf(format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	p.internalLog(calldepth, CRITICAL, BaseLogEntry(s))
	panic(s)
}

func (p *PackageLogger) Panic(args ...interface{}) {
	s := fmt.Sprint(args...)
	p.internalLog(calldepth, CRITICAL, BaseLogEntry(s))
	panic(s)
}

func (p *PackageLogger) Fatalf(format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	p.internalLog(calldepth, CRITICAL, BaseLogEntry(s))
	os.Exit(1)
}

func (p *PackageLogger) Fatal(args ...interface{}) {
	s := fmt.Sprint(args...)
	p.internalLog(calldepth, CRITICAL, BaseLogEntry(s))
	os.Exit(1)
}

// Error Functions

func (p *PackageLogger) Errorf(format string, args ...interface{}) {
	if p.level < ERROR {
		return
	}
	p.internalLog(calldepth, ERROR, BaseLogEntry(fmt.Sprintf(format, args...)))
}

func (p *PackageLogger) Error(entries ...LogEntry) {
	if p.level < ERROR {
		return
	}
	p.internalLog(calldepth, ERROR, entries...)
}

// Warning Functions

func (p *PackageLogger) Warningf(format string, args ...interface{}) {
	if p.level < WARNING {
		return
	}
	p.internalLog(calldepth, WARNING, BaseLogEntry(fmt.Sprintf(format, args...)))
}

func (p *PackageLogger) Warning(entries ...LogEntry) {
	if p.level < WARNING {
		return
	}
	p.internalLog(calldepth, WARNING, entries...)
}

// Notice Functions

func (p *PackageLogger) Noticef(format string, args ...interface{}) {
	if p.level < NOTICE {
		return
	}
	p.internalLog(calldepth, NOTICE, BaseLogEntry(fmt.Sprintf(format, args...)))
}

func (p *PackageLogger) Notice(entries ...LogEntry) {
	if p.level < NOTICE {
		return
	}
	p.internalLog(calldepth, NOTICE, entries...)
}

// Info Functions

func (p *PackageLogger) Infof(format string, args ...interface{}) {
	if p.level < INFO {
		return
	}
	p.internalLog(calldepth, INFO, BaseLogEntry(fmt.Sprintf(format, args...)))
}

func (p *PackageLogger) Info(entries ...LogEntry) {
	if p.level < INFO {
		return
	}
	p.internalLog(calldepth, INFO, entries...)
}

// Debug Functions

func (p *PackageLogger) Debugf(format string, args ...interface{}) {
	if p.level < DEBUG {
		return
	}
	p.internalLog(calldepth, DEBUG, BaseLogEntry(fmt.Sprintf(format, args...)))
}

func (p *PackageLogger) Debug(entries ...LogEntry) {
	if p.level < DEBUG {
		return
	}
	p.internalLog(calldepth, DEBUG, entries...)
}

// Trace Functions

func (p *PackageLogger) Tracef(format string, args ...interface{}) {
	if p.level < TRACE {
		return
	}
	p.internalLog(calldepth, TRACE, BaseLogEntry(fmt.Sprintf(format, args...)))
}

func (p *PackageLogger) Trace(entries ...LogEntry) {
	if p.level < TRACE {
		return
	}
	p.internalLog(calldepth, TRACE, entries...)
}

func (p *PackageLogger) Flush() {
	logger.Lock()
	defer logger.Unlock()
	logger.formatter.Flush()
}
