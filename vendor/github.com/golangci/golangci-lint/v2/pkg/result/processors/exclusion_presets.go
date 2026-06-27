package processors

import "github.com/golangci/golangci-lint/v2/pkg/config"

var LinterExclusionPresets = map[string][]config.ExcludeRule{
	config.ExclusionPresetComments: {
		{
			// Annoying issue about not having a comment. The rare codebase has such comments.
			// CheckPackageComment, CheckExportedFunctionDocs, CheckExportedTypeDocs, CheckExportedVarDocs
			BaseRule: config.BaseRule{
				Text:              "(ST1000|ST1020|ST1021|ST1022)",
				Linters:           []string{"staticcheck"},
				InternalReference: "EXC0011",
			},
		},
		{
			// Annoying issue about not having a comment. The rare codebase has such comments.
			// rule: exported
			BaseRule: config.BaseRule{
				Text:              `exported (.+) should have comment( \(or a comment on this block\))? or be unexported`,
				Linters:           []string{"revive"},
				InternalReference: "EXC0012",
			},
		},
		{
			// Annoying issue about not having a comment. The rare codebase has such comments.
			// rule: package-comments
			BaseRule: config.BaseRule{
				Text:              `package comment should be of the form "(.+)..."`,
				Linters:           []string{"revive"},
				InternalReference: "EXC0013",
			},
		},
		{
			// Annoying issue about not having a comment. The rare codebase has such comments.
			// rule: exported
			BaseRule: config.BaseRule{
				Text:              `comment on exported (.+) should be of the form "(.+)..."`,
				Linters:           []string{"revive"},
				InternalReference: "EXC0014",
			},
		},
		{
			// Annoying issue about not having a comment. The rare codebase has such comments.
			// rule: package-comments
			BaseRule: config.BaseRule{
				Text:              `should have a package comment`,
				Linters:           []string{"revive"},
				InternalReference: "EXC0015",
			},
		},
	},
	config.ExclusionPresetStdErrorHandling: {
		{
			// Almost all programs ignore errors on these functions and in most cases it's ok.
			BaseRule: config.BaseRule{
				Text: "(?i)Error return value of .((os\\.)?std(out|err)\\..*|.*Close" +
					"|.*Flush|os\\.Remove(All)?|.*print(f|ln)?|os\\.(Un)?Setenv). is not checked",
				Linters:           []string{"errcheck"},
				InternalReference: "EXC0001",
			},
		},
	},
	config.ExclusionPresetCommonFalsePositives: {
		{
			// Too many false-positives on 'unsafe' usage.
			BaseRule: config.BaseRule{
				Text:              "G103: Use of unsafe calls should be audited",
				Linters:           []string{"gosec"},
				InternalReference: "EXC0006",
			},
		},
		{
			// Too many false-positives for parametrized shell calls.
			BaseRule: config.BaseRule{
				Text:              "G204: Subprocess launched with variable",
				Linters:           []string{"gosec"},
				InternalReference: "EXC0007",
			},
		},
		{
			// False positive is triggered by 'src, err := ioutil.ReadFile(filename)'.
			BaseRule: config.BaseRule{
				Text:              "G304: Potential file inclusion via variable",
				Linters:           []string{"gosec"},
				InternalReference: "EXC0010",
			},
		},
	},
	config.ExclusionPresetLegacy: {
		{
			// Common false positives.
			BaseRule: config.BaseRule{
				Text:              "(possible misuse of unsafe.Pointer|should have signature)",
				Linters:           []string{"govet"},
				InternalReference: "EXC0004",
			},
		},
		{
			// Developers tend to write in C-style with an explicit 'break' in a 'switch', so it's ok to ignore.
			// CheckScopedBreak
			BaseRule: config.BaseRule{
				Text:              "SA4011",
				Linters:           []string{"staticcheck"},
				InternalReference: "EXC0005",
			},
		},
		{
			// Duplicated errcheck checks.
			// Errors unhandled.
			BaseRule: config.BaseRule{
				Text:              "G104",
				Linters:           []string{"gosec"},
				InternalReference: "EXC0008",
			},
		},
		{
			// Too many issues in popular repos.
			BaseRule: config.BaseRule{
				Text:              "(G301|G302|G307): Expect (directory permissions to be 0750|file permissions to be 0600) or less",
				Linters:           []string{"gosec"},
				InternalReference: "EXC0009",
			},
		},
	},
}

func getLinterExclusionPresets(names []string) []config.ExcludeRule {
	var rules []config.ExcludeRule

	for _, name := range names {
		if p, ok := LinterExclusionPresets[name]; ok {
			rules = append(rules, p...)
		}
	}

	return rules
}
