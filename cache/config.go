package cache

import "time"

type Config struct {
	PerWatcherBufferSize int
	HistoryWindowSize    int
	ResyncInterval       time.Duration
	InitialBackoff       time.Duration
	MaxBackoff           time.Duration
}

func defaultConfig() Config {
	return Config{
		PerWatcherBufferSize: 10,
		HistoryWindowSize:    2048,
		ResyncInterval:       50 * time.Millisecond,
		InitialBackoff:       50 * time.Millisecond,
		MaxBackoff:           2 * time.Second,
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
