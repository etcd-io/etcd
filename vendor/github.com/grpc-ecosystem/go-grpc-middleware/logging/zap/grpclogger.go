// Copyright 2017 Michal Witkowski. All Rights Reserved.
// See LICENSE for licensing terms.

package grpc_zap

import (
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc/grpclog"
)

// ReplaceGrpcLogger sets the given zap.Logger as a gRPC-level logger.
// This should be called *before* any other initialization, preferably from init() functions.
func ReplaceGrpcLogger(logger *zap.Logger) {
	zgl := &zapGrpcLogger{logger.With(SystemField, zap.Bool("grpc_log", true))}
	grpclog.SetLogger(zgl)
}

type zapGrpcLogger struct {
	logger *zap.Logger
}

func (l *zapGrpcLogger) Fatal(args ...interface{}) {
	l.logger.Fatal(fmt.Sprint(args...))
}

func (l *zapGrpcLogger) Fatalf(format string, args ...interface{}) {
	l.logger.Fatal(fmt.Sprintf(format, args...))
}

func (l *zapGrpcLogger) Fatalln(args ...interface{}) {
	l.logger.Fatal(fmt.Sprint(args...))
}

func (l *zapGrpcLogger) Print(args ...interface{}) {
	l.logger.Info(fmt.Sprint(args...))
}

func (l *zapGrpcLogger) Printf(format string, args ...interface{}) {
	l.logger.Info(fmt.Sprintf(format, args...))
}

func (l *zapGrpcLogger) Println(args ...interface{}) {
	l.logger.Info(fmt.Sprint(args...))
}
