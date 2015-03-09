// Copyright 2014 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package loggo_test

import (
	"fmt"
	"time"

	gc "gopkg.in/check.v1"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/juju/loggo"
)

type writerBasicsSuite struct{}

var _ = gc.Suite(&writerBasicsSuite{})

func (s *writerBasicsSuite) TearDownTest(c *gc.C) {
	loggo.ResetWriters()
}

func (*writerBasicsSuite) TestRemoveDefaultWriter(c *gc.C) {
	defaultWriter, level, err := loggo.RemoveWriter("default")
	c.Assert(err, gc.IsNil)
	c.Assert(level, gc.Equals, loggo.TRACE)
	c.Assert(defaultWriter, gc.NotNil)

	// Trying again fails.
	defaultWriter, level, err = loggo.RemoveWriter("default")
	c.Assert(err, gc.ErrorMatches, `Writer "default" is not registered`)
	c.Assert(level, gc.Equals, loggo.UNSPECIFIED)
	c.Assert(defaultWriter, gc.IsNil)
}

func (*writerBasicsSuite) TestRegisterWriterExistingName(c *gc.C) {
	err := loggo.RegisterWriter("default", &loggo.TestWriter{}, loggo.INFO)
	c.Assert(err, gc.ErrorMatches, `there is already a Writer registered with the name "default"`)
}

func (*writerBasicsSuite) TestRegisterNilWriter(c *gc.C) {
	err := loggo.RegisterWriter("nil", nil, loggo.INFO)
	c.Assert(err, gc.ErrorMatches, `Writer cannot be nil`)
}

func (*writerBasicsSuite) TestRegisterWriterTypedNil(c *gc.C) {
	// If the interface is a typed nil, we have to trust the user.
	var writer *loggo.TestWriter
	err := loggo.RegisterWriter("nil", writer, loggo.INFO)
	c.Assert(err, gc.IsNil)
}

func (*writerBasicsSuite) TestReplaceDefaultWriter(c *gc.C) {
	oldWriter, err := loggo.ReplaceDefaultWriter(&loggo.TestWriter{})
	c.Assert(oldWriter, gc.NotNil)
	c.Assert(err, gc.IsNil)
}

func (*writerBasicsSuite) TestReplaceDefaultWriterWithNil(c *gc.C) {
	oldWriter, err := loggo.ReplaceDefaultWriter(nil)
	c.Assert(oldWriter, gc.IsNil)
	c.Assert(err, gc.ErrorMatches, "Writer cannot be nil")
}

func (*writerBasicsSuite) TestReplaceDefaultWriterNoDefault(c *gc.C) {
	loggo.RemoveWriter("default")
	oldWriter, err := loggo.ReplaceDefaultWriter(&loggo.TestWriter{})
	c.Assert(oldWriter, gc.IsNil)
	c.Assert(err, gc.ErrorMatches, `there is no "default" writer`)
}

func (s *writerBasicsSuite) TestWillWrite(c *gc.C) {
	// By default, the root logger watches TRACE messages
	c.Assert(loggo.WillWrite(loggo.TRACE), gc.Equals, true)
	// Note: ReplaceDefaultWriter doesn't let us change the default log
	//	 level :(
	writer, _, err := loggo.RemoveWriter("default")
	c.Assert(err, gc.IsNil)
	c.Assert(writer, gc.NotNil)
	err = loggo.RegisterWriter("default", writer, loggo.CRITICAL)
	c.Assert(err, gc.IsNil)
	c.Assert(loggo.WillWrite(loggo.TRACE), gc.Equals, false)
	c.Assert(loggo.WillWrite(loggo.DEBUG), gc.Equals, false)
	c.Assert(loggo.WillWrite(loggo.INFO), gc.Equals, false)
	c.Assert(loggo.WillWrite(loggo.WARNING), gc.Equals, false)
	c.Assert(loggo.WillWrite(loggo.CRITICAL), gc.Equals, true)
}

type writerSuite struct {
	logger loggo.Logger
}

var _ = gc.Suite(&writerSuite{})

func (s *writerSuite) SetUpTest(c *gc.C) {
	loggo.ResetLoggers()
	loggo.RemoveWriter("default")
	s.logger = loggo.GetLogger("test.writer")
	// Make it so the logger itself writes all messages.
	s.logger.SetLogLevel(loggo.TRACE)
}

func (s *writerSuite) TearDownTest(c *gc.C) {
	loggo.ResetWriters()
}

func (s *writerSuite) TearDownSuite(c *gc.C) {
	loggo.ResetLoggers()
}

func (s *writerSuite) TestWritingCapturesFileAndLineAndModule(c *gc.C) {
	writer := &loggo.TestWriter{}
	err := loggo.RegisterWriter("test", writer, loggo.INFO)
	c.Assert(err, gc.IsNil)

	s.logger.Infof("Info message") //tag capture

	log := writer.Log()
	c.Assert(log, gc.HasLen, 1)
	assertLocation(c, log[0], "capture")
	c.Assert(log[0].Module, gc.Equals, "test.writer")
}

func (s *writerSuite) TestWritingLimitWarning(c *gc.C) {
	writer := &loggo.TestWriter{}
	err := loggo.RegisterWriter("test", writer, loggo.WARNING)
	c.Assert(err, gc.IsNil)

	start := time.Now()
	s.logger.Criticalf("Something critical.")
	s.logger.Errorf("An error.")
	s.logger.Warningf("A warning message")
	s.logger.Infof("Info message")
	s.logger.Tracef("Trace the function")
	end := time.Now()

	log := writer.Log()
	c.Assert(log, gc.HasLen, 3)
	c.Assert(log[0].Level, gc.Equals, loggo.CRITICAL)
	c.Assert(log[0].Message, gc.Equals, "Something critical.")
	c.Assert(log[0].Timestamp, Between(start, end))

	c.Assert(log[1].Level, gc.Equals, loggo.ERROR)
	c.Assert(log[1].Message, gc.Equals, "An error.")
	c.Assert(log[1].Timestamp, Between(start, end))

	c.Assert(log[2].Level, gc.Equals, loggo.WARNING)
	c.Assert(log[2].Message, gc.Equals, "A warning message")
	c.Assert(log[2].Timestamp, Between(start, end))
}

func (s *writerSuite) TestWritingLimitTrace(c *gc.C) {
	writer := &loggo.TestWriter{}
	err := loggo.RegisterWriter("test", writer, loggo.TRACE)
	c.Assert(err, gc.IsNil)

	start := time.Now()
	s.logger.Criticalf("Something critical.")
	s.logger.Errorf("An error.")
	s.logger.Warningf("A warning message")
	s.logger.Infof("Info message")
	s.logger.Tracef("Trace the function")
	end := time.Now()

	log := writer.Log()
	c.Assert(log, gc.HasLen, 5)
	c.Assert(log[0].Level, gc.Equals, loggo.CRITICAL)
	c.Assert(log[0].Message, gc.Equals, "Something critical.")
	c.Assert(log[0].Timestamp, Between(start, end))

	c.Assert(log[1].Level, gc.Equals, loggo.ERROR)
	c.Assert(log[1].Message, gc.Equals, "An error.")
	c.Assert(log[1].Timestamp, Between(start, end))

	c.Assert(log[2].Level, gc.Equals, loggo.WARNING)
	c.Assert(log[2].Message, gc.Equals, "A warning message")
	c.Assert(log[2].Timestamp, Between(start, end))

	c.Assert(log[3].Level, gc.Equals, loggo.INFO)
	c.Assert(log[3].Message, gc.Equals, "Info message")
	c.Assert(log[3].Timestamp, Between(start, end))

	c.Assert(log[4].Level, gc.Equals, loggo.TRACE)
	c.Assert(log[4].Message, gc.Equals, "Trace the function")
	c.Assert(log[4].Timestamp, Between(start, end))
}

func (s *writerSuite) TestMultipleWriters(c *gc.C) {
	errorWriter := &loggo.TestWriter{}
	err := loggo.RegisterWriter("error", errorWriter, loggo.ERROR)
	c.Assert(err, gc.IsNil)
	warningWriter := &loggo.TestWriter{}
	err = loggo.RegisterWriter("warning", warningWriter, loggo.WARNING)
	c.Assert(err, gc.IsNil)
	infoWriter := &loggo.TestWriter{}
	err = loggo.RegisterWriter("info", infoWriter, loggo.INFO)
	c.Assert(err, gc.IsNil)
	traceWriter := &loggo.TestWriter{}
	err = loggo.RegisterWriter("trace", traceWriter, loggo.TRACE)
	c.Assert(err, gc.IsNil)

	s.logger.Errorf("An error.")
	s.logger.Warningf("A warning message")
	s.logger.Infof("Info message")
	s.logger.Tracef("Trace the function")

	c.Assert(errorWriter.Log(), gc.HasLen, 1)
	c.Assert(warningWriter.Log(), gc.HasLen, 2)
	c.Assert(infoWriter.Log(), gc.HasLen, 3)
	c.Assert(traceWriter.Log(), gc.HasLen, 4)
}

func Between(start, end time.Time) gc.Checker {
	if end.Before(start) {
		return &betweenChecker{end, start}
	}
	return &betweenChecker{start, end}
}

type betweenChecker struct {
	start, end time.Time
}

func (checker *betweenChecker) Info() *gc.CheckerInfo {
	info := gc.CheckerInfo{
		Name:   "Between",
		Params: []string{"obtained"},
	}
	return &info
}

func (checker *betweenChecker) Check(params []interface{}, names []string) (result bool, error string) {
	when, ok := params[0].(time.Time)
	if !ok {
		return false, "obtained value type must be time.Time"
	}
	if when.Before(checker.start) {
		return false, fmt.Sprintf("obtained time %q is before start time %q", when, checker.start)
	}
	if when.After(checker.end) {
		return false, fmt.Sprintf("obtained time %q is after end time %q", when, checker.end)
	}
	return true, ""
}
