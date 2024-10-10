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
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLogObserver_Timeout(t *testing.T) {
	logCore, logOb := NewLogObserver(zap.InfoLevel)

	logger := zap.New(logCore)
	logger.Info(t.Name())

	ctx, cancel := context.WithTimeout(context.TODO(), 100*time.Millisecond)
	_, err := logOb.ExpectAtLeastNTimes(ctx, "unknown", 1)
	cancel()
	assert.True(t, errors.Is(err, context.DeadlineExceeded))

	assert.Equal(t, 1, len(logOb.entries))
}

func TestLogObserver_Expect(t *testing.T) {
	logCore, logOb := NewLogObserver(zap.InfoLevel)

	logger := zap.New(logCore)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	resCh := make(chan []string, 1)
	go func() {
		defer close(resCh)

		res, err := logOb.ExpectAtLeastNTimes(ctx, t.Name(), 2)
		require.NoError(t, err)
		resCh <- res
	}()

	msgs := []string{"Hello " + t.Name(), t.Name() + ", World"}
	for _, msg := range msgs {
		logger.Info(msg)
		time.Sleep(40 * time.Millisecond)
	}

	res := <-resCh
	assert.Equal(t, 2, len(res))

	// The logged message should be like
	//
	// 2023-04-16T11:46:19.367+0800 INFO	Hello TestLogObserver_Expect
	// 2023-04-16T11:46:19.408+0800	INFO	TestLogObserver_Expect, World
	//
	// The prefix timestamp is unpredictable so we should assert the suffix
	// only.
	for idx := range msgs {
		expected := fmt.Sprintf("\tINFO\t%s\n", msgs[idx])
		assert.True(t, strings.HasSuffix(res[idx], expected))
	}

	assert.Equal(t, 2, len(logOb.entries))
}

// TestLogObserver_ExpectExactNTimes tests the behavior of ExpectExactNTimes.
// It covers 4 scenarios:
//
// 1. Occurrences in log is less than expected when duration ends.
// 2. Occurrences in log is exactly as expected when duration ends.
// 3. Occurrences in log is greater than expected when duration ends.
// 4. context deadline exceeded before duration ends
func TestLogObserver_ExpectExactNTimes(t *testing.T) {
	logCore, logOb := NewLogObserver(zap.InfoLevel)

	logger := zap.New(logCore)

	// Log messages at 0ms, 100ms, 200ms
	msgs := []string{"Hello " + t.Name(), t.Name() + ", World", t.Name()}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	wg := sync.WaitGroup{}

	// When duration ends at 50ms, there's only one message in the log.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := logOb.ExpectExactNTimes(ctx, t.Name(), 2, 50*time.Millisecond)
		assert.ErrorContains(t, err, "failed to expect, expected: 2, got: 1")
	}()

	// When duration ends at 150ms, there are 2 messages in the log.
	wg.Add(1)
	go func() {
		defer wg.Done()
		lines, err := logOb.ExpectExactNTimes(ctx, t.Name(), 2, 150*time.Millisecond)
		assert.NoError(t, err)
		assert.Len(t, lines, 2)
		for i := 0; i < 2; i++ {
			// lines should be like:
			// 2023-04-16T11:46:19.367+0800 INFO	Hello TestLogObserver_ExpectExactNTimes
			// 2023-04-16T11:46:19.408+0800	INFO	TestLogObserver_ExpectExactNTimes, World
			//
			// msgs:
			// Hello TestLogObserver_ExpectExactNTimes
			// TestLogObserver_ExpectExactNTimes, World
			// TestLogObserver_ExpectExactNTimes
			strings.HasSuffix(lines[i], msgs[i])
		}
	}()

	// Before duration ends at 250ms, there are already 3 messages in the log.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := logOb.ExpectExactNTimes(ctx, t.Name(), 2, 250*time.Millisecond)
		assert.ErrorContains(t, err,"failed to expect; too many occurrences; expected: 2, got:3")
	}()

	// context deadline exceeded at 300ms before the 400ms duration can end.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := logOb.ExpectExactNTimes(ctx, t.Name(), 3, 400*time.Millisecond)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	}()

	// Log messages at 0ms, 100ms, 200ms
	for _, msg := range msgs {
		logger.Info(msg)
		time.Sleep(100 * time.Millisecond)
	}

	wg.Wait()
}
