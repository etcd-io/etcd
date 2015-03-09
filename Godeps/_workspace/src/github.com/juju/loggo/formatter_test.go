// Copyright 2014 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package loggo_test

import (
	"time"

	gc "gopkg.in/check.v1"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/juju/loggo"
)

type formatterSuite struct{}

var _ = gc.Suite(&formatterSuite{})

func (*formatterSuite) TestDefaultFormat(c *gc.C) {
	location, err := time.LoadLocation("UTC")
	c.Assert(err, gc.IsNil)
	testTime := time.Date(2013, 5, 3, 10, 53, 24, 123456, location)
	formatter := &loggo.DefaultFormatter{}
	formatted := formatter.Format(loggo.WARNING, "test.module", "some/deep/filename", 42, testTime, "hello world!")
	c.Assert(formatted, gc.Equals, "2013-05-03 10:53:24 WARNING test.module filename:42 hello world!")
}
