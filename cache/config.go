package cache

import "time"

type Config struct {
	ChannelSize    int
	BufferEntries  int
	DropThreshold  int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

func defaultConfig() Config {
	return Config{
		ChannelSize:    256,
		BufferEntries:  2048,
		DropThreshold:  5,
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     2 * time.Second,
	}
}

type Option func(*Config)

func WithChannelSize(n int) Option {
	return func(c *Config) { c.ChannelSize = n }
}
func WithBufferEntries(n int) Option {
	return func(c *Config) { c.BufferEntries = n }
}
func WithDropThreshold(n int) Option {
	return func(c *Config) { c.DropThreshold = n }
}

func WithInitialBackoff(d time.Duration) Option {
	return func(c *Config) { c.InitialBackoff = d }
}

func WithMaxBackoff(d time.Duration) Option {
	return func(c *Config) { c.MaxBackoff = d }
}
