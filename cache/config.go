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
	// UpstreamBufferSize caps the buffered channel that decouples the etcd watch loop from the (potentially slow) worker that calls broadcast.
	UpstreamBufferSize int
}

// TODO: tune via performance/load tests.
func defaultConfig() Config {
	return Config{
		PerWatcherBufferSize: 10,
		HistoryWindowSize:    2048,
		ResyncInterval:       50 * time.Millisecond,
		InitialBackoff:       50 * time.Millisecond,
		MaxBackoff:           2 * time.Second,
		UpstreamBufferSize:   1024,
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

func WithUpstreamBufferSize(n int) Option {
	return func(c *Config) { c.UpstreamBufferSize = n }
}
