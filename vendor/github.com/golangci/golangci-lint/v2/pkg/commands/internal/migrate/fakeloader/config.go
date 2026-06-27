package fakeloader

// Config implements [config.BaseConfig].
// This only the stub for the real file loader.
type Config struct {
	Version string `mapstructure:"version"`

	cfgDir string // Path to the directory containing golangci-lint config file.
}

func NewConfig() *Config {
	return &Config{}
}

// SetConfigDir sets the path to directory that contains golangci-lint config file.
func (c *Config) SetConfigDir(dir string) {
	c.cfgDir = dir
}

func (*Config) IsInternalTest() bool {
	return false
}
