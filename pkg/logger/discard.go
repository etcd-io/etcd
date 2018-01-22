// Copyright 2018 The etcd Authors
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

package logger

import "log"

// assert that Logger satisfies grpclog.LoggerV2
var _ Logger = &discardLogger{}

// NewDiscardLogger returns a new Logger that discards everything except "fatal".
func NewDiscardLogger() Logger { return &discardLogger{} }

type discardLogger struct{}

func (l *discardLogger) Info(args ...interface{})                    {}
func (l *discardLogger) Infoln(args ...interface{})                  {}
func (l *discardLogger) Infof(format string, args ...interface{})    {}
func (l *discardLogger) Warning(args ...interface{})                 {}
func (l *discardLogger) Warningln(args ...interface{})               {}
func (l *discardLogger) Warningf(format string, args ...interface{}) {}
func (l *discardLogger) Error(args ...interface{})                   {}
func (l *discardLogger) Errorln(args ...interface{})                 {}
func (l *discardLogger) Errorf(format string, args ...interface{})   {}
func (l *discardLogger) Fatal(args ...interface{})                   { log.Fatal(args...) }
func (l *discardLogger) Fatalln(args ...interface{})                 { log.Fatalln(args...) }
func (l *discardLogger) Fatalf(format string, args ...interface{})   { log.Fatalf(format, args...) }
func (l *discardLogger) V(lvl int) bool {
	return false
}
