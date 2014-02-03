package etcd

import (
	"io/ioutil"
	"log"
	"strings"
)

type Logger interface {
	Debug(args ...interface{})
	Debugf(fmt string, args ...interface{})
	Warning(args ...interface{})
	Warningf(fmt string, args ...interface{})
}

var logger Logger

func SetLogger(log Logger) {
	logger = log
}

func GetLogger() Logger {
	return logger
}

type defaultLogger struct {
	log *log.Logger
}

func (p *defaultLogger) Debug(args ...interface{}) {
	p.log.Println(args)
}

func (p *defaultLogger) Debugf(fmt string, args ...interface{}) {
	// Append newline if necessary
	if !strings.HasSuffix(fmt, "\n") {
		fmt = fmt + "\n"
	}
	p.log.Printf(fmt, args)
}

func (p *defaultLogger) Warning(args ...interface{}) {
	p.Debug(args)
}

func (p *defaultLogger) Warningf(fmt string, args ...interface{}) {
	p.Debugf(fmt, args)
}

func init() {
	// Default logger uses the go default log.
	SetLogger(&defaultLogger{log.New(ioutil.Discard, "go-etcd", log.LstdFlags)})
}
