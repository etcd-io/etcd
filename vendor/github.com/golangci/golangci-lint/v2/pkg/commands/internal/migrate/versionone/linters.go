package versionone

type Linters struct {
	Enable     []string `mapstructure:"enable"`
	Disable    []string `mapstructure:"disable"`
	EnableAll  *bool    `mapstructure:"enable-all"`
	DisableAll *bool    `mapstructure:"disable-all"`
	Fast       *bool    `mapstructure:"fast"`

	Presets []string `mapstructure:"presets"`
}
