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

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	_, err := logOb.Expect(ctx, "unknown", 1)
	cancel()
	require.ErrorIs(t, err, context.DeadlineExceeded)

	assert.Len(t, logOb.entries, 1)
}

func TestLogObserver_Expect(t *testing.T) {
	logCore, logOb := NewLogObserver(zap.InfoLevel)

	logger := zap.New(logCore)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	resCh := make(chan []string, 1)
	go func() {
		defer close(resCh)

		res, err := logOb.Expect(ctx, t.Name(), 2)
		assert.NoError(t, err)
		resCh <- res
	}()

	msgs := []string{"Hello " + t.Name(), t.Name() + ", World"}
	for _, msg := range msgs {
		logger.Info(msg)
		time.Sleep(40 * time.Millisecond)
	}

	res := <-resCh
	assert.Len(t, res, 2)

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

	assert.Len(t, logOb.entries, 2)
}
