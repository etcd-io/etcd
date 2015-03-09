// Copyright 2014 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package loggo

import (
	"fmt"
	"path/filepath"
	"time"
)

// Formatter defines the single method Format, which takes the logging
// information, and converts it to a string.
type Formatter interface {
	Format(level Level, module, filename string, line int, timestamp time.Time, message string) string
}

// DefaultFormatter provides a simple concatenation of all the components.
type DefaultFormatter struct{}

// Format returns the parameters separated by spaces except for filename and
// line which are separated by a colon.  The timestamp is shown to second
// resolution in UTC.
func (*DefaultFormatter) Format(level Level, module, filename string, line int, timestamp time.Time, message string) string {
	ts := timestamp.In(time.UTC).Format("2006-01-02 15:04:05")
	// Just get the basename from the filename
	filename = filepath.Base(filename)
	return fmt.Sprintf("%s %s %s %s:%d %s", ts, level, module, filename, line, message)
}
