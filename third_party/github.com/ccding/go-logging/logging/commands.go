// Copyright 2013, Cong Ding. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// author: Cong Ding <dinggnu@gmail.com>
//
package logging

// Logln receives log request from the client. The request includes a set of
// variables.
func (logger *Logger) Log(level Level, v ...interface{}) {
	// Don't delete this calling. The calling is used to keep the same
	// calldepth for all the logging functions. The calldepth is used to
	// get runtime information such as line number, function name, etc.
	logger.log(level, v...)
}

// Logf receives log request from the client. The request has a string
// parameter to describe the format of output.
func (logger *Logger) Logf(level Level, format string, v ...interface{}) {
	logger.logf(level, format, v...)
}

// Other quick commands for different level

func (logger *Logger) Critical(v ...interface{}) {
	logger.log(CRITICAL, v...)
}

func (logger *Logger) Fatal(v ...interface{}) {
	logger.log(CRITICAL, v...)
}

func (logger *Logger) Error(v ...interface{}) {
	logger.log(ERROR, v...)
}

func (logger *Logger) Warn(v ...interface{}) {
	logger.log(WARNING, v...)
}

func (logger *Logger) Warning(v ...interface{}) {
	logger.log(WARNING, v...)
}

func (logger *Logger) Info(v ...interface{}) {
	logger.log(INFO, v...)
}

func (logger *Logger) Debug(v ...interface{}) {
	logger.log(DEBUG, v...)
}

func (logger *Logger) Notset(v ...interface{}) {
	logger.log(NOTSET, v...)
}

func (logger *Logger) Criticalf(format string, v ...interface{}) {
	logger.logf(CRITICAL, format, v...)
}

func (logger *Logger) Fatalf(format string, v ...interface{}) {
	logger.logf(CRITICAL, format, v...)
}

func (logger *Logger) Errorf(format string, v ...interface{}) {
	logger.logf(ERROR, format, v...)
}

func (logger *Logger) Warnf(format string, v ...interface{}) {
	logger.logf(WARNING, format, v...)
}

func (logger *Logger) Warningf(format string, v ...interface{}) {
	logger.logf(WARNING, format, v...)
}

func (logger *Logger) Infof(format string, v ...interface{}) {
	logger.logf(INFO, format, v...)
}

func (logger *Logger) Debugf(format string, v ...interface{}) {
	logger.logf(DEBUG, format, v...)
}

func (logger *Logger) Notsetf(format string, v ...interface{}) {
	logger.logf(NOTSET, format, v...)
}
