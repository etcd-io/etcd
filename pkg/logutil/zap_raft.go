package logutil

import (
	"github.com/coreos/etcd/raft"
	"go.uber.org/zap"
)

// NewRaftLogger converts "*zap.Logger" to "raft.Logger".
func NewRaftLogger(lcfg zap.Config) (raft.Logger, error) {
	lg, err := lcfg.Build(zap.AddCallerSkip(1)) // to annotate caller outside of "logutil"
	if err != nil {
		return nil, err
	}
	return &zapRaftLogger{lg: lg, sugar: lg.Sugar()}, nil
}

type zapRaftLogger struct {
	lg    *zap.Logger
	sugar *zap.SugaredLogger
}

func (zl *zapRaftLogger) Debug(args ...interface{}) {
	zl.sugar.Debug(args...)
}

func (zl *zapRaftLogger) Debugf(format string, args ...interface{}) {
	zl.sugar.Debugf(format, args...)
}

func (zl *zapRaftLogger) Error(args ...interface{}) {
	zl.sugar.Error(args...)
}

func (zl *zapRaftLogger) Errorf(format string, args ...interface{}) {
	zl.sugar.Errorf(format, args...)
}

func (zl *zapRaftLogger) Info(args ...interface{}) {
	zl.sugar.Info(args...)
}

func (zl *zapRaftLogger) Infof(format string, args ...interface{}) {
	zl.sugar.Infof(format, args...)
}

func (zl *zapRaftLogger) Warning(args ...interface{}) {
	zl.sugar.Warn(args...)
}

func (zl *zapRaftLogger) Warningf(format string, args ...interface{}) {
	zl.sugar.Warnf(format, args...)
}

func (zl *zapRaftLogger) Fatal(args ...interface{}) {
	zl.sugar.Fatal(args...)
}

func (zl *zapRaftLogger) Fatalf(format string, args ...interface{}) {
	zl.sugar.Fatalf(format, args...)
}

func (zl *zapRaftLogger) Panic(args ...interface{}) {
	zl.sugar.Panic(args...)
}

func (zl *zapRaftLogger) Panicf(format string, args ...interface{}) {
	zl.sugar.Panicf(format, args...)
}
