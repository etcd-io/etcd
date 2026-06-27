package log

import (
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Use L to log with Zap
var logger *zap.Logger

// Keep the config to reference the atomicLevel for changing levels
var logConfig zap.Config

var doOnce sync.Once

// InitLogger sets up the logger
func InitLogger() {
	doOnce.Do(func() {
		logConfig = zap.NewDevelopmentConfig()

		logConfig.EncoderConfig.TimeKey = "timestamp"
		logConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		logConfig.Level.SetLevel(zapcore.InfoLevel)
		logConfig.OutputPaths = []string{"stderr"}

		var err error
		logger, err = logConfig.Build()
		if err != nil {
			panic(err)
		}
	})
}

// SetLevel allows you to set the level of the default gci logger.
// This will not work if you replace the logger
func SetLevel(level zapcore.Level) {
	logConfig.Level.SetLevel(level)
}

// L returns the logger
func L() *zap.Logger {
	return logger
}

// SetLogger allows you to set the logger to whatever you want
func SetLogger(l *zap.Logger) {
	logger = l
}
