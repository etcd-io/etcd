package versionone

type Config struct {
	Version string `mapstructure:"version"` // From v2, to be able to detect already migrated config file.

	Run Run `mapstructure:"run"`

	Output Output `mapstructure:"output"`

	LintersSettings LintersSettings `mapstructure:"linters-settings"`
	Linters         Linters         `mapstructure:"linters"`
	Issues          Issues          `mapstructure:"issues"`
	Severity        Severity        `mapstructure:"severity"`
}

func NewConfig() *Config {
	return &Config{}
}
