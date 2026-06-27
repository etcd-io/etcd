package config

type Issues struct {
	MaxIssuesPerLinter int  `mapstructure:"max-issues-per-linter"`
	MaxSameIssues      int  `mapstructure:"max-same-issues"`
	UniqByLine         bool `mapstructure:"uniq-by-line"`

	DiffFromRevision  string `mapstructure:"new-from-rev"`
	DiffFromMergeBase string `mapstructure:"new-from-merge-base"`
	DiffPatchFilePath string `mapstructure:"new-from-patch"`
	WholeFiles        bool   `mapstructure:"whole-files"`
	Diff              bool   `mapstructure:"new"`

	NeedFix bool `mapstructure:"fix"`
}
