package mirror

import "github.com/butuzov/mirror/internal/checker"

var UTF8Functions = []checker.Violation{
	{ // utf8.Valid
		Type:      checker.Function,
		Targets:   checker.Bytes,
		Package:   "unicode/utf8",
		Caller:    "Valid",
		Args:      []int{0},
		AltCaller: "ValidString",

		Generate: &checker.Generate{
			Pattern: `Valid($0)`,
			Returns: []string{"bool"},
		},
	},
	{ // utf8.ValidString
		Targets:   checker.Strings,
		Type:      checker.Function,
		Package:   "unicode/utf8",
		Caller:    "ValidString",
		Args:      []int{0},
		AltCaller: "Valid",

		Generate: &checker.Generate{
			Pattern: `ValidString($0)`,
			Returns: []string{"bool"},
		},
	},
	{ // utf8.FullRune
		Targets:   checker.Bytes,
		Type:      checker.Function,
		Package:   "unicode/utf8",
		Caller:    "FullRune",
		Args:      []int{0},
		AltCaller: "FullRuneInString",

		Generate: &checker.Generate{
			Pattern: `FullRune($0)`,
			Returns: []string{"bool"},
		},
	},
	{ // utf8.FullRuneInString
		Targets:   checker.Strings,
		Type:      checker.Function,
		Package:   "unicode/utf8",
		Caller:    "FullRuneInString",
		Args:      []int{0},
		AltCaller: "FullRune",

		Generate: &checker.Generate{
			Pattern: `FullRuneInString($0)`,
			Returns: []string{"bool"},
		},
	},

	{ // bytes.RuneCount
		Targets:   checker.Bytes,
		Type:      checker.Function,
		Package:   "unicode/utf8",
		Caller:    "RuneCount",
		Args:      []int{0},
		AltCaller: "RuneCountInString",

		Generate: &checker.Generate{
			Pattern: `RuneCount($0)`,
			Returns: []string{"int"},
		},
	},
	{ // bytes.RuneCountInString
		Targets:   checker.Strings,
		Type:      checker.Function,
		Package:   "unicode/utf8",
		Caller:    "RuneCountInString",
		Args:      []int{0},
		AltCaller: "RuneCount",

		Generate: &checker.Generate{
			Pattern: `RuneCountInString($0)`,
			Returns: []string{"int"},
		},
	},

	{ // bytes.DecodeLastRune
		Targets:   checker.Bytes,
		Type:      checker.Function,
		Package:   "unicode/utf8",
		Caller:    "DecodeLastRune",
		Args:      []int{0},
		AltCaller: "DecodeLastRuneInString",

		Generate: &checker.Generate{
			Pattern: `DecodeLastRune($0)`,
			Returns: []string{"rune", "int"},
		},
	},
	{ // utf8.DecodeLastRuneInString
		Targets:   checker.Strings,
		Type:      checker.Function,
		Package:   "unicode/utf8",
		Caller:    "DecodeLastRuneInString",
		Args:      []int{0},
		AltCaller: "DecodeLastRune",

		Generate: &checker.Generate{
			Pattern: `DecodeLastRuneInString($0)`,
			Returns: []string{"rune", "int"},
		},
	},
	{ // utf8.DecodeRune
		Targets:   checker.Bytes,
		Type:      checker.Function,
		Package:   "unicode/utf8",
		Caller:    "DecodeRune",
		Args:      []int{0},
		AltCaller: "DecodeRuneInString",

		Generate: &checker.Generate{
			Pattern: `DecodeRune($0)`,
			Returns: []string{"rune", "int"},
		},
	},
	{ // utf8.DecodeRuneInString
		Targets:   checker.Strings,
		Type:      checker.Function,
		Package:   "unicode/utf8",
		Args:      []int{0},
		Caller:    "DecodeRuneInString",
		AltCaller: "DecodeRune",

		Generate: &checker.Generate{
			Pattern: `DecodeRuneInString($0)`,
			Returns: []string{"rune", "int"},
		},
	},
}
