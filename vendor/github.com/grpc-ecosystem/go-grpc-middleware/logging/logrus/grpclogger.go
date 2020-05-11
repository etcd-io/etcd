// Copyright 2017 Michal Witkowski. All Rights Reserved.
// See LICENSE for licensing terms.

package grpc_logrus

import (
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/grpclog"
)

// ReplaceGrpcLogger sets the given logrus.Logger as a gRPC-level logger.
// This should be called *before* any other initialization, preferably from init() functions.
func ReplaceGrpcLogger(logger *logrus.Entry) {
	grpclog.SetLogger(logger.WithField("system", SystemField))
}
