// Copyright 2017 Michal Witkowski. All Rights Reserved.
// See LICENSE for licensing terms.

package grpc_logrus

import (
	"path"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// UnaryClientInterceptor returns a new unary client interceptor that optionally logs the execution of external gRPC calls.
func UnaryClientInterceptor(entry *logrus.Entry, opts ...Option) grpc.UnaryClientInterceptor {
	o := evaluateClientOpt(opts)
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		fields := newClientLoggerFields(ctx, method)
		startTime := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		logFinalClientLine(o, entry.WithFields(fields), startTime, err, "finished client unary call")
		return err
	}
}

// StreamServerInterceptor returns a new streaming client interceptor that optionally logs the execution of external gRPC calls.
func StreamClientInterceptor(entry *logrus.Entry, opts ...Option) grpc.StreamClientInterceptor {
	o := evaluateClientOpt(opts)
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		fields := newClientLoggerFields(ctx, method)
		startTime := time.Now()
		clientStream, err := streamer(ctx, desc, cc, method, opts...)
		logFinalClientLine(o, entry.WithFields(fields), startTime, err, "finished client streaming call")
		return clientStream, err
	}
}

func logFinalClientLine(o *options, entry *logrus.Entry, startTime time.Time, err error, msg string) {
	code := o.codeFunc(err)
	level := o.levelFunc(code)
	durField, durVal := o.durationFunc(time.Now().Sub(startTime))
	fields := logrus.Fields{
		"grpc.code": code.String(),
		durField:    durVal,
	}
	if err != nil {
		fields[logrus.ErrorKey] = err
	}
	levelLogf(
		entry.WithFields(fields),
		level,
		msg)
}

func newClientLoggerFields(ctx context.Context, fullMethodString string) logrus.Fields {
	service := path.Dir(fullMethodString)[1:]
	method := path.Base(fullMethodString)
	return logrus.Fields{
		SystemField:    "grpc",
		KindField:      "client",
		"grpc.service": service,
		"grpc.method":  method,
	}
}
