package capnslog

import (
	"log/syslog"
)

func NewSyslogFormatter(w *syslog.Writer) Formatter {
	return &syslogFormatter{w}
}

func NewDefaultSyslogFormatter(tag string) (Formatter, error) {
	w, err := syslog.New(syslog.LOG_DEBUG, tag)
	if err != nil {
		return nil, err
	}
	return NewSyslogFormatter(w), nil
}

type syslogFormatter struct {
	w *syslog.Writer
}

func (s *syslogFormatter) Format(pkg string, l LogLevel, _ int, entries ...LogEntry) {
	for _, entry := range entries {
		str := entry.LogString()
		switch l {
		case CRITICAL:
			s.w.Crit(str)
		case ERROR:
			s.w.Err(str)
		case WARNING:
			s.w.Warning(str)
		case NOTICE:
			s.w.Notice(str)
		case INFO:
			s.w.Info(str)
		case DEBUG:
			s.w.Debug(str)
		case TRACE:
			s.w.Debug(str)
		default:
			panic("Unhandled loglevel")
		}
	}
}

func (s *syslogFormatter) Flush() {
}
