package versionone

import (
	"time"
)

// Run encapsulates the config options for running the linter analysis.
type Run struct {
	Timeout time.Duration `mapstructure:"timeout"`

	Concurrency *int `mapstructure:"concurrency"`

	Go *string `mapstructure:"go"`

	RelativePathMode *string `mapstructure:"relative-path-mode"`

	BuildTags           []string `mapstructure:"build-tags"`
	ModulesDownloadMode *string  `mapstructure:"modules-download-mode"`

	ExitCodeIfIssuesFound *int  `mapstructure:"issues-exit-code"`
	AnalyzeTests          *bool `mapstructure:"tests"`

	AllowParallelRunners *bool `mapstructure:"allow-parallel-runners"`
	AllowSerialRunners   *bool `mapstructure:"allow-serial-runners"`
}
