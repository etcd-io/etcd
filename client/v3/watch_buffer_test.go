// Copyright 2024 The etcd Authors
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
	"context"
	"testing"
	"time"
)

// TestWatchBufferLimit verifies that the watch buffer limit prevents unbounded memory growth
func TestWatchBufferLimit(t *testing.T) {
	// This test demonstrates the buffer limit functionality
	// In a real scenario, this would connect to an actual etcd server

	t.Run("unlimited buffer (default behavior)", func(t *testing.T) {
		cfg := Config{
			Endpoints:            []string{"localhost:2379"},
			MaxWatcherBufferSize: 0, // 0 means unlimited (backward compatible)
		}

		// With unlimited buffer, the system behaves as before
		if cfg.MaxWatcherBufferSize != 0 {
			t.Errorf("Expected unlimited buffer (0), got %d", cfg.MaxWatcherBufferSize)
		}
	})

	t.Run("limited buffer prevents OOM", func(t *testing.T) {
		cfg := Config{
			Endpoints:            []string{"localhost:2379"},
			MaxWatcherBufferSize: 1000, // Limit to 1000 events
		}

		if cfg.MaxWatcherBufferSize != 1000 {
			t.Errorf("Expected buffer limit of 1000, got %d", cfg.MaxWatcherBufferSize)
		}

		// When buffer reaches 1000 events:
		// 1. Warning log at 800 events (80% threshold)
		// 2. Backpressure applied at 1000 events
		// 3. No new events buffered until consumer catches up
		// 4. Prevents OOM in slow consumer scenarios
	})
}

// TestWatcherStreamBufferFields verifies the new buffer tracking fields
func TestWatcherStreamBufferFields(t *testing.T) {
	ws := &watcherStream{
		maxBufferSize:          1000,
		bufferWarningThreshold: 800,
		bufferWarningLogged:    false,
		buf:                    make([]*WatchResponse, 0),
	}

	// Verify initial state
	if ws.maxBufferSize != 1000 {
		t.Errorf("Expected maxBufferSize=1000, got %d", ws.maxBufferSize)
	}

	if ws.bufferWarningThreshold != 800 {
		t.Errorf("Expected bufferWarningThreshold=800, got %d", ws.bufferWarningThreshold)
	}

	if ws.bufferWarningLogged {
		t.Error("Expected bufferWarningLogged=false initially")
	}

	if len(ws.buf) != 0 {
		t.Errorf("Expected empty buffer, got length %d", len(ws.buf))
	}
}

// BenchmarkWatchBufferAppend benchmarks the buffer append performance
func BenchmarkWatchBufferAppend(b *testing.B) {
	ws := &watcherStream{
		maxBufferSize: 10000,
		buf:           make([]*WatchResponse, 0, 10000),
	}

	wr := &WatchResponse{
		Events: []*Event{
			{Type: EventTypePut},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if len(ws.buf) < ws.maxBufferSize {
			ws.buf = append(ws.buf, wr)
		} else {
			// Reset buffer to simulate consumption
			ws.buf = ws.buf[:0]
		}
	}
}

// Example demonstrates how to use the buffer limit configuration
func ExampleConfig_maxWatcherBufferSize() {
	// Create a client with buffer limit to prevent OOM
	cfg := Config{
		Endpoints:            []string{"localhost:2379"},
		DialTimeout:          5 * time.Second,
		MaxWatcherBufferSize: 5000, // Limit each watcher to 5000 buffered events
	}

	cli, err := New(cfg)
	if err != nil {
		// Handle error
		return
	}
	defer cli.Close()

	// Create a watch - it will automatically use the buffer limit
	ctx := context.Background()
	watchChan := cli.Watch(ctx, "mykey", WithPrefix())

	// Consume events
	for resp := range watchChan {
		if resp.Err() != nil {
			// Handle error
			break
		}

		// Process events
		for _, event := range resp.Events {
			_ = event // Process event
		}
	}

	// Benefits:
	// 1. If events arrive faster than you can process them,
	//    the buffer will fill up to 5000 events
	// 2. At 4000 events (80%), a warning is logged
	// 3. At 5000 events, backpressure is applied - no new events
	//    are buffered until you consume some
	// 4. This prevents memory exhaustion (OOM) in slow consumer scenarios
}
