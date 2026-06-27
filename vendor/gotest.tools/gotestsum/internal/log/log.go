package log

import (
	"fmt"

	"github.com/fatih/color"
)

type Level uint8

const (
	ErrorLevel Level = iota
	WarnLevel
	InfoLevel
	DebugLevel
)

var (
	level = WarnLevel
	out   = color.Error
)

// SetLevel for the global logger.
func SetLevel(l Level) {
	level = l
}

// Warnf prints the message to stderr, with a yellow WARN prefix.
func Warnf(format string, args ...interface{}) {
	if level < WarnLevel {
		return
	}
	fmt.Fprint(out, color.YellowString("WARN "))
	fmt.Fprintf(out, format, args...)
	fmt.Fprint(out, "\n")
}

// Debugf prints the message to stderr, with no prefix.
func Debugf(format string, args ...interface{}) {
	if level < DebugLevel {
		return
	}
	fmt.Fprintf(out, format, args...)
	fmt.Fprint(out, "\n")
}

// Infof prints the message to stderr, with no prefix.
func Infof(format string, args ...interface{}) {
	if level < InfoLevel {
		return
	}
	fmt.Fprintf(out, format, args...)
	fmt.Fprint(out, "\n")
}

// Errorf prints the message to stderr, with a red ERROR prefix.
func Errorf(format string, args ...interface{}) {
	if level < ErrorLevel {
		return
	}
	fmt.Fprint(out, color.RedString("ERROR "))
	fmt.Fprintf(out, format, args...)
	fmt.Fprint(out, "\n")
}

// Error prints the message to stderr, with a red ERROR prefix.
func Error(msg string) {
	if level < ErrorLevel {
		return
	}
	fmt.Fprint(out, color.RedString("ERROR "))
	fmt.Fprintln(out, msg)
}
