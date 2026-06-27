package config

import "strings"

const (
	// placeholderBasePath used inside linters to evaluate relative paths.
	placeholderBasePath = "${base-path}"

	// placeholderConfigPath used inside linters to paths relative to configuration.
	placeholderConfigPath = "${config-path}"
)

func NewPlaceholderReplacer(cfg *Config) *strings.Replacer {
	return strings.NewReplacer(
		placeholderBasePath, cfg.GetBasePath(),
		placeholderConfigPath, cfg.GetConfigDir(),
	)
}
