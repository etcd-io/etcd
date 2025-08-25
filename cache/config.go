// Copyright 2025 The etcd Authors
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

package cache

import "time"

type Config struct {
	// PerWatcherBufferSize caps each watcher’s buffered channel.
	// Bigger values tolerate brief client slow-downs at the cost of extra memory.
	PerWatcherBufferSize int
	// HistoryWindowSize is the max events kept in memory for replay.
	// It defines how far back the cache can replay events to lagging watchers
	HistoryWindowSize int
	// ResyncInterval controls how often the demux attempts to catch a lagging watcher up by replaying events from History.
	ResyncInterval time.Duration
	// InitialBackoff is the first delay to wait before retrying an upstream etcd Watch after it ends with an error.
	InitialBackoff time.Duration
	// MaxBackoff caps the exponential back-off between successive upstream watch retries.
	MaxBackoff time.Duration
	// GetTimeout is the timeout applied to the first Get() used to bootstrap the cache.
	GetTimeout time.Duration
	// BTreeDegree controls the degree (branching factor) of the in-memory B-tree store.
	BTreeDegree int
}

// TODO: tune via performance/load tests.
func defaultConfig() Config {
	return Config{
		PerWatcherBufferSize: 10,
		HistoryWindowSize:    2048,
		ResyncInterval:       50 * time.Millisecond,
		InitialBackoff:       50 * time.Millisecond,
		MaxBackoff:           2 * time.Second,
		GetTimeout:           5 * time.Second,
		BTreeDegree:          32,
	}
}

type Option func(*Config)

func WithPerWatcherBufferSize(n int) Option {
	return func(c *Config) { c.PerWatcherBufferSize = n }
}

func WithHistoryWindowSize(n int) Option {
	return func(c *Config) { c.HistoryWindowSize = n }
}

func WithResyncInterval(d time.Duration) Option {
	return func(c *Config) { c.ResyncInterval = d }
}

func WithInitialBackoff(d time.Duration) Option {
	return func(c *Config) { c.InitialBackoff = d }
}

func WithMaxBackoff(d time.Duration) Option {
	return func(c *Config) { c.MaxBackoff = d }
}

func WithGetTimeout(d time.Duration) Option {
	return func(c *Config) { c.GetTimeout = d }
}

func WithBTreeDegree(n int) Option {
	return func(c *Config) { c.BTreeDegree = n }
}
