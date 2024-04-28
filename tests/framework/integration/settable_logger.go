// Copyright 2016 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integration

import (
	"io"
	"sync"

	"google.golang.org/grpc/grpclog"
)

// settableLoggerV2 is thread-safe.
// Copied from https://github.com/grpc-ecosystem/go-grpc-middleware/tree/v1.4.0/logging/settable.
type settableLoggerV2 interface {
	grpclog.LoggerV2
	// Set given logger as the underlying implementation.
	Set(loggerv2 grpclog.LoggerV2)
	// Reset `discard` logger as the underlying implementation.
	Reset()
}

// replaceGrpcLoggerV2 creates and configures SettableLoggerV2 as grpc logger.
func replaceGrpcLoggerV2() settableLoggerV2 {
	settable := &settableLoggerV2Impl{}
	settable.Reset()
	grpclog.SetLoggerV2(settable)
	return settable
}

// settableLoggerV2Impl implements settableLoggerV2
type settableLoggerV2Impl struct {
	log grpclog.LoggerV2
	mu  sync.RWMutex
}

func (s *settableLoggerV2Impl) Set(log grpclog.LoggerV2) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log = log
}

func (s *settableLoggerV2Impl) Reset() {
	s.Set(grpclog.NewLoggerV2(io.Discard, io.Discard, io.Discard))
}

func (s *settableLoggerV2Impl) get() grpclog.LoggerV2 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.log
}

func (s *settableLoggerV2Impl) Info(args ...interface{}) {
	s.get().Info(args)
}

func (s *settableLoggerV2Impl) Infoln(args ...interface{}) {
	s.get().Infoln(args)
}

func (s *settableLoggerV2Impl) Infof(format string, args ...interface{}) {
	s.get().Infof(format, args)
}

func (s *settableLoggerV2Impl) Warning(args ...interface{}) {
	s.get().Warning(args)
}

func (s *settableLoggerV2Impl) Warningln(args ...interface{}) {
	s.get().Warningln(args)
}

func (s *settableLoggerV2Impl) Warningf(format string, args ...interface{}) {
	s.get().Warningf(format, args)
}

func (s *settableLoggerV2Impl) Error(args ...interface{}) {
	s.get().Error(args)
}

func (s *settableLoggerV2Impl) Errorln(args ...interface{}) {
	s.get().Errorln(args)
}

func (s *settableLoggerV2Impl) Errorf(format string, args ...interface{}) {
	s.get().Errorf(format, args)
}

func (s *settableLoggerV2Impl) Fatal(args ...interface{}) {
	s.get().Fatal(args)
}

func (s *settableLoggerV2Impl) Fatalln(args ...interface{}) {
	s.get().Fatalln(args)
}

func (s *settableLoggerV2Impl) Fatalf(format string, args ...interface{}) {
	s.get().Fatalf(format, args)
}

func (s *settableLoggerV2Impl) V(l int) bool {
	return s.get().V(l)
}
