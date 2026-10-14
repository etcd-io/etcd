// Copyright 2026 The etcd Authors
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

package clientv3

import (
	"reflect"
	"testing"
	"time"
)

func TestBlockLogger(t *testing.T) {
	const (
		logInterval  = 5 * time.Second
		logThreshold = 100 * time.Millisecond
	)

	type step struct {
		advance time.Duration
		wait    time.Duration
	}
	type logEntry struct {
		count  int
		wait   time.Duration
		window time.Duration
	}

	tests := []struct {
		name     string
		steps    []step
		wantLogs []logEntry
	}{
		{
			name: "does not log before interval elapses",
			steps: []step{
				{wait: logThreshold},
				{advance: time.Second, wait: logThreshold},
			},
		},
		{
			name: "does not log when accumulated wait stays below threshold",
			steps: []step{
				{wait: 20 * time.Millisecond},
				{advance: logInterval, wait: 20 * time.Millisecond},
			},
		},
		{
			name: "logs accumulated waits when interval elapses and threshold is exceeded",
			steps: []step{
				{wait: 40 * time.Millisecond},
				{advance: logInterval, wait: 80 * time.Millisecond},
			},
			wantLogs: []logEntry{
				{count: 2, wait: 120 * time.Millisecond, window: logInterval},
			},
		},
		{
			name: "logs once per accumulation window and resets after logging",
			steps: []step{
				{wait: 40 * time.Millisecond},
				{advance: logInterval, wait: 80 * time.Millisecond},
				{advance: logInterval, wait: 120 * time.Millisecond},
			},
			wantLogs: []logEntry{
				{count: 2, wait: 120 * time.Millisecond, window: logInterval},
				{count: 1, wait: 120 * time.Millisecond, window: logInterval},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Unix(0, 0)
			var logs []logEntry
			logger := newBlockLogger(logInterval, logThreshold, func() time.Time {
				return now
			}, func(eventCount int, timeWaiting time.Duration, window time.Duration) {
				logs = append(logs, logEntry{count: eventCount, wait: timeWaiting, window: window})
			})

			for _, step := range tt.steps {
				now = now.Add(step.advance)
				logger.recordWait(step.wait)
			}

			if !reflect.DeepEqual(logs, tt.wantLogs) {
				t.Fatalf("logs = %+v, want %+v", logs, tt.wantLogs)
			}
		})
	}
}
