package log

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

const teststring = "asdf123"

func TestOutputf(t *testing.T) {
	buf := new(bytes.Buffer)
	log.SetOutput(buf)
	Level = LevelDebug
	outputf(LevelDebug, teststring, nil)

	// outputf correctly prints string
	if !strings.Contains(buf.String(), teststring) {
		t.Fail()
	}
	return
}

func TestOutput(t *testing.T) {
	buf := new(bytes.Buffer)
	log.SetOutput(buf)
	Level = LevelDebug
	output(LevelDebug, nil)

	// outputf correctly prints string with proper Debug prefix
	if !strings.Contains(buf.String(), levelPrefix[LevelDebug]) {
		t.Fail()
	}
	return
}

func TestCriticalf(t *testing.T) {
	buf := new(bytes.Buffer)
	log.SetOutput(buf)
	Criticalf(teststring, nil)

	// outputf correctly prints string
	// should never fail because critical > debug
	if !strings.Contains(buf.String(), teststring) {
		t.Fail()
	}
	return
}

func TestCritical(t *testing.T) {
	buf := new(bytes.Buffer)
	log.SetOutput(buf)
	Critical(nil)

	// outputf correctly prints string
	if !strings.Contains(buf.String(), levelPrefix[LevelCritical]) {
		t.Fail()
	}
	return
}

func TestWarningf(t *testing.T) {
	buf := new(bytes.Buffer)
	log.SetOutput(buf)
	Warningf(teststring, nil)

	// outputf correctly prints string
	// should never fail because fatal critical > debug
	if !strings.Contains(buf.String(), teststring) {
		t.Fail()
	}
	return
}

func TestWarning(t *testing.T) {
	buf := new(bytes.Buffer)
	log.SetOutput(buf)
	Warning(nil)

	// outputf correctly prints string
	if !strings.Contains(buf.String(), levelPrefix[LevelWarning]) {
		t.Fail()
	}
	return
}

func TestInfof(t *testing.T) {
	buf := new(bytes.Buffer)
	log.SetOutput(buf)
	Infof(teststring, nil)

	// outputf correctly prints string
	// should never fail because fatal info > debug
	if !strings.Contains(buf.String(), teststring) {
		t.Fail()
	}
	return
}

func TestInfo(t *testing.T) {
	buf := new(bytes.Buffer)
	log.SetOutput(buf)
	Info(nil)

	// outputf correctly prints string
	if !strings.Contains(buf.String(), levelPrefix[LevelInfo]) {
		t.Fail()
	}
	return
}

func TestDebugf(t *testing.T) {
	buf := new(bytes.Buffer)
	log.SetOutput(buf)
	Level = LevelDebug
	Debugf(teststring, nil)

	// outputf correctly prints string
	// should never fail because fatal debug >= debug
	if !strings.Contains(buf.String(), teststring) {
		t.Fail()
	}
	return
}

func TestDebug(t *testing.T) {
	buf := new(bytes.Buffer)
	log.SetOutput(buf)
	Level = LevelDebug
	Debug(nil)

	// outputf correctly prints string
	if !strings.Contains(buf.String(), levelPrefix[LevelDebug]) {
		t.Fail()
	}
	return
}

type testSyslogger struct {
	*bytes.Buffer
}

func (l testSyslogger) Debug(s string) {
	l.WriteString("[DEBUG] ")
	_, _ = l.WriteString(s)
}

func (l testSyslogger) Info(s string) {
	l.WriteString("[INFO] ")
	_, _ = l.WriteString(s)
}

func (l testSyslogger) Warning(s string) {
	l.WriteString("[WARN] ")
	_, _ = l.WriteString(s)
}

func (l testSyslogger) Err(s string) {
	l.WriteString("[ERROR] ")
	_, _ = l.WriteString(s)
}

func (l testSyslogger) Crit(s string) {
	l.WriteString("[CRIT] ")
	_, _ = l.WriteString(s)
}

func (l testSyslogger) Emerg(s string) {
	l.WriteString("[FATAL] ")
	_, _ = l.WriteString(s)
}

func TestSetLogger(t *testing.T) {
	buf := new(bytes.Buffer)
	SetLogger(testSyslogger{buf})
	Level = LevelDebug
	outputf(LevelDebug, teststring, nil)

	// outputf correctly prints string
	if !strings.Contains(buf.String(), teststring) {
		t.Fail()
	}
	SetLogger(nil)
	return
}
