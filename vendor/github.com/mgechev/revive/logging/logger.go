// Package logging provides a logger and related methods.
package logging

import (
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"
)

// GetLogger retrieves an instance of an application logger.
// The log level can be configured via the REVIVE_LOG_LEVEL environment variable.
// If REVIVE_LOG_LEVEL is unset or empty, logging is disabled.
// If it is set to an invalid value, the log level defaults to WARN.
//
//nolint:unparam // err is always nil, but is included in the signature for future extensibility.
func GetLogger() (*slog.Logger, error) {
	return getLogger(), nil
}

var getLogger = sync.OnceValue(initLogger(os.Stderr))

func initLogger(out io.Writer) func() *slog.Logger {
	return func() *slog.Logger {
		logLevel := os.Getenv("REVIVE_LOG_LEVEL")
		if logLevel == "" {
			return slog.New(slog.DiscardHandler)
		}

		leveler := &slog.LevelVar{}
		opts := &slog.HandlerOptions{Level: leveler}

		level := slog.LevelWarn
		_ = level.UnmarshalText([]byte(logLevel)) // Ignore error and default to WARN if invalid
		leveler.Set(level)
		logger := slog.New(slog.NewTextHandler(out, opts))

		logger.Info("Logger initialized", "logLevel", logLevel)

		return logger
	}
}

// InitForTesting initializes the logger singleton cache for testing purposes.
// This function should only be called in tests.
func InitForTesting(tb testing.TB, out io.Writer) {
	tb.Helper()
	getLogger = sync.OnceValue(initLogger(out))
}
