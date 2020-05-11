// Copyright (c) Improbable Worlds Ltd, All Rights Reserved

package grpc_logrus

import (
	"path"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus/ctxlogrus"
	"github.com/grpc-ecosystem/go-grpc-middleware/tags/logrus"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var (
	// SystemField is used in every log statement made through grpc_logrus. Can be overwritten before any initialization code.
	SystemField = "system"

	// KindField describes the log gield used to incicate whether this is a server or a client log statment.
	KindField = "span.kind"
)

// UnaryServerInterceptor returns a new unary server interceptors that adds logrus.Entry to the context.
func UnaryServerInterceptor(entry *logrus.Entry, opts ...Option) grpc.UnaryServerInterceptor {
	o := evaluateServerOpt(opts)
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()
		newCtx := newLoggerForCall(ctx, entry, info.FullMethod, startTime)

		resp, err := handler(newCtx, req)

		if !o.shouldLog(info.FullMethod, err) {
			return resp, err
		}
		code := o.codeFunc(err)
		level := o.levelFunc(code)
		durField, durVal := o.durationFunc(time.Since(startTime))
		fields := logrus.Fields{
			"grpc.code": code.String(),
			durField:    durVal,
		}
		if err != nil {
			fields[logrus.ErrorKey] = err
		}

		levelLogf(
			ctx_logrus.Extract(newCtx).WithFields(fields), // re-extract logger from newCtx, as it may have extra fields that changed in the holder.
			level,
			"finished unary call with code "+code.String())

		return resp, err
	}
}

// StreamServerInterceptor returns a new streaming server interceptor that adds logrus.Entry to the context.
func StreamServerInterceptor(entry *logrus.Entry, opts ...Option) grpc.StreamServerInterceptor {
	o := evaluateServerOpt(opts)
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startTime := time.Now()
		newCtx := newLoggerForCall(stream.Context(), entry, info.FullMethod, startTime)
		wrapped := grpc_middleware.WrapServerStream(stream)
		wrapped.WrappedContext = newCtx

		err := handler(srv, wrapped)

		if !o.shouldLog(info.FullMethod, err) {
			return err
		}
		code := o.codeFunc(err)
		level := o.levelFunc(code)
		durField, durVal := o.durationFunc(time.Since(startTime))
		fields := logrus.Fields{
			"grpc.code": code.String(),
			durField:    durVal,
		}
		if err != nil {
			fields[logrus.ErrorKey] = err
		}

		levelLogf(
			ctx_logrus.Extract(newCtx).WithFields(fields), // re-extract logger from newCtx, as it may have extra fields that changed in the holder.
			level,
			"finished streaming call with code "+code.String())

		return err
	}
}

func levelLogf(entry *logrus.Entry, level logrus.Level, format string, args ...interface{}) {
	switch level {
	case logrus.DebugLevel:
		entry.Debugf(format, args...)
	case logrus.InfoLevel:
		entry.Infof(format, args...)
	case logrus.WarnLevel:
		entry.Warningf(format, args...)
	case logrus.ErrorLevel:
		entry.Errorf(format, args...)
	case logrus.FatalLevel:
		entry.Fatalf(format, args...)
	case logrus.PanicLevel:
		entry.Panicf(format, args...)
	}
}

func newLoggerForCall(ctx context.Context, entry *logrus.Entry, fullMethodString string, start time.Time) context.Context {
	service := path.Dir(fullMethodString)[1:]
	method := path.Base(fullMethodString)
	callLog := entry.WithFields(
		logrus.Fields{
			SystemField:       "grpc",
			KindField:         "server",
			"grpc.service":    service,
			"grpc.method":     method,
			"grpc.start_time": start.Format(time.RFC3339),
		})

	if d, ok := ctx.Deadline(); ok {
		callLog = callLog.WithFields(
			logrus.Fields{
				"grpc.request.deadline": d.Format(time.RFC3339),
			})
	}

	callLog = callLog.WithFields(ctx_logrus.Extract(ctx).Data)
	return ctxlogrus.ToContext(ctx, callLog)
}
