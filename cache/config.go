package cache

import "time"

type Config struct {
	ChannelSize     int           // per‑watcher buffer
	GapWatchTimeout time.Duration // time to wait for replying missed revisions
	MaxGapWatch     int           // max concurrent gap replays allowed
	DropThreshold   int           // consecutive missed sends before evicting slow watchers
	Disable         bool
}

func defaultConfig() Config {
	return Config{
		ChannelSize:     256,
		GapWatchTimeout: 15 * time.Second,
		MaxGapWatch:     32,
		DropThreshold:   3,
	}
}

type Option func(*Config)

func WithChannelSize(n int) Option          { return func(c *Config) { c.ChannelSize = n } }
func WithGapTimeout(d time.Duration) Option { return func(c *Config) { c.GapWatchTimeout = d } }
func WithMaxGapWatch(n int) Option          { return func(c *Config) { c.MaxGapWatch = n } }
func WithDropThreshold(n int) Option        { return func(c *Config) { c.DropThreshold = n } }
func WithDisabledCache() Option             { return func(c *Config) { c.Disable = true } }
