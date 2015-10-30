// Copyright 2015 CoreOS, Inc.
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

package rafthttp

import (
	"fmt"
	"sync"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/pkg/capnslog"
)

// defaultPeriod is the default period to aggregate log lines.
// Set 1s as default value because it lets log concise
// while printing out cached log lines in time.
const defaultPeriod = time.Second

// logLine represents a log line that could be printed out
// through capnslog.PackageLogger.
type logLine struct {
	level capnslog.LogLevel
	log   string
}

// aggregateLogger aggregates log lines happened in a time period, and
// prints out one line for each repeated log line at the end of the period.
type aggregateLogger struct {
	period time.Duration
	logger *capnslog.PackageLogger

	mu sync.Mutex
	// linem holds all log lines that are cached and have not been printed out.
	linem map[logLine]int
}

func (l *aggregateLogger) Warningf(format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.linem == nil {
		l.linem = make(map[logLine]int)
		// print out lines in linem after the period
		time.AfterFunc(l.period, l.print)
	}

	line := logLine{
		level: capnslog.WARNING,
		log:   fmt.Sprintf(format, args...),
	}
	l.linem[line]++
}

func (l *aggregateLogger) print() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for line, cnt := range l.linem {
		l.logger.Logf(line.level, "%s [aggregated %d repeated lines in past %v]", line.log, cnt, l.period)
	}
	l.linem = nil
}
