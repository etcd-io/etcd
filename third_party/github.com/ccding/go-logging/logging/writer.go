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

import (
	"bytes"
	"fmt"
	"sync/atomic"
	"time"
)

// watcher watches the logger.queue channel, and writes the logs to output
func (logger *Logger) watcher() {
	var buf bytes.Buffer
	for {
		timeout := time.After(time.Second / 10)

		for i := 0; i < bufSize; i++ {
			select {
			case msg := <-logger.queue:
				fmt.Fprintln(&buf, msg)
			case req := <-logger.request:
				logger.flushReq(&buf, &req)
			case <-timeout:
				i = bufSize
			case <-logger.flush:
				logger.flushBuf(&buf)
				logger.flush <- true
				i = bufSize
			case <-logger.quit:
				// If quit signal received, cleans the channel
				// and writes all of them to io.Writer.
				for {
					select {
					case msg := <-logger.queue:
						fmt.Fprintln(&buf, msg)
					case req := <-logger.request:
						logger.flushReq(&buf, &req)
					case <-logger.flush:
						// do nothing
					default:
						logger.flushBuf(&buf)
						logger.quit <- true
						return
					}
				}
			}
		}
		logger.flushBuf(&buf)
	}
}

// flushBuf flushes the content of buffer to out and reset the buffer
func (logger *Logger) flushBuf(b *bytes.Buffer) {
	if len(b.Bytes()) > 0 {
		logger.out.Write(b.Bytes())
		b.Reset()
	}
}

// flushReq handles the request and writes the result to writer
func (logger *Logger) flushReq(b *bytes.Buffer, req *request) {
	if req.format == "" {
		msg := fmt.Sprint(req.v...)
		msg = logger.genLog(req.level, msg)
		fmt.Fprintln(b, msg)
	} else {
		msg := fmt.Sprintf(req.format, req.v...)
		msg = logger.genLog(req.level, msg)
		fmt.Fprintln(b, msg)
	}
}

// flushMsg is to print log to file, stdout, or others.
func (logger *Logger) flushMsg(message string) {
	if logger.sync {
		logger.wlock.Lock()
		defer logger.wlock.Unlock()
		fmt.Fprintln(logger.out, message)
	} else {
		logger.queue <- message
	}
}

// log records log v... with level `level'.
func (logger *Logger) log(level Level, v ...interface{}) {
	if int32(level) >= atomic.LoadInt32((*int32)(&logger.level)) {
		if logger.runtime || logger.sync {
			message := fmt.Sprint(v...)
			message = logger.genLog(level, message)
			logger.flushMsg(message)
		} else {
			r := new(request)
			r.level = level
			r.v = v
			logger.request <- *r
		}
	}
}

// logf records log v... with level `level'.
func (logger *Logger) logf(level Level, format string, v ...interface{}) {
	if int32(level) >= atomic.LoadInt32((*int32)(&logger.level)) {
		if logger.runtime || logger.sync {
			message := fmt.Sprintf(format, v...)
			message = logger.genLog(level, message)
			logger.flushMsg(message)
		} else {
			r := new(request)
			r.level = level
			r.format = format
			r.v = v
			logger.request <- *r
		}
	}
}
