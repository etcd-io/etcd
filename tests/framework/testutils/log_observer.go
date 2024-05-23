// Copyright 2023 The etcd Authors
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

package testutils

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	zapobserver "go.uber.org/zap/zaptest/observer"
)

type LogObserver struct {
	ob  *zapobserver.ObservedLogs
	enc zapcore.Encoder

	mu sync.Mutex
	// entries stores all the logged entries after syncLogs.
	entries []zapobserver.LoggedEntry
}

func NewLogObserver(level zapcore.LevelEnabler) (zapcore.Core, *LogObserver) {
	// align with zaptest
	enc := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())

	co, ob := zapobserver.New(level)
	return co, &LogObserver{
		ob:  ob,
		enc: enc,
	}
}

// Expect returns the first N lines containing the given string.
func (logOb *LogObserver) Expect(ctx context.Context, s string, count int) ([]string, error) {
	return logOb.ExpectFunc(ctx, func(log string) bool { return strings.Contains(log, s) }, count)
}

// ExpectFunc returns the first N line satisfying the function f.
func (logOb *LogObserver) ExpectFunc(ctx context.Context, filter func(string) bool, count int) ([]string, error) {
	i := 0
	res := make([]string, 0, count)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		entries := logOb.syncLogs()

		// The order of entries won't be changed because of append-only.
		// It's safe to skip scanned entries by reusing `i`.
		for ; i < len(entries); i++ {
			buf, err := logOb.enc.EncodeEntry(entries[i].Entry, entries[i].Context)
			if err != nil {
				return nil, fmt.Errorf("failed to encode entry: %w", err)
			}

			logInStr := buf.String()
			if filter(logInStr) {
				res = append(res, logInStr)
			}

			if len(res) >= count {
				return res, nil
			}
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// syncLogs is to take all the existing logged entries from zapobserver and
// truncate zapobserver's entries slice.
func (logOb *LogObserver) syncLogs() []zapobserver.LoggedEntry {
	logOb.mu.Lock()
	defer logOb.mu.Unlock()

	logOb.entries = append(logOb.entries, logOb.ob.TakeAll()...)
	return logOb.entries
}
