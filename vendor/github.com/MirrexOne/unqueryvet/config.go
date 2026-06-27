package unqueryvet

import "github.com/MirrexOne/unqueryvet/pkg/config"

// Settings is a type alias for UnqueryvetSettings from the config package.
type Settings = config.UnqueryvetSettings

// DefaultSettings returns the default configuration for Unqueryvet.
func DefaultSettings() Settings {
	return config.DefaultSettings()
}
