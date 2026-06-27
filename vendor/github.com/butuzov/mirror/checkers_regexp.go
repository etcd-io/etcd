package mirror

import "github.com/butuzov/mirror/internal/checker"

var (
	RegexpFunctions = []checker.Violation{
		{ // regexp.Match
			Targets:   checker.Bytes,
			Type:      checker.Function,
			Package:   "regexp",
			Caller:    "Match",
			Args:      []int{1},
			AltCaller: "MatchString",

			Generate: &checker.Generate{
				Pattern: `Match("foo", $0)`,
				Returns: []string{"bool", "error"},
			},
		},
		{ // regexp.MatchString
			Targets:   checker.Strings,
			Type:      checker.Function,
			Package:   "regexp",
			Caller:    "MatchString",
			Args:      []int{1},
			AltCaller: "Match",

			Generate: &checker.Generate{
				Pattern: `MatchString("foo", $0)`,
				Returns: []string{"bool", "error"},
			},
		},
	}

	RegexpRegexpMethods = []checker.Violation{
		{ // (*regexp.Regexp).Match
			Targets:   checker.Bytes,
			Type:      checker.Method,
			Package:   "regexp",
			Struct:    "Regexp",
			Caller:    "Match",
			Args:      []int{0},
			AltCaller: "MatchString",

			Generate: &checker.Generate{
				PreCondition: `re := regexp.MustCompile(".*")`,
				Pattern:      `Match($0)`,
				Returns:      []string{"bool"},
			},
		},
		{ // (*regexp.Regexp).MatchString
			Targets:   checker.Strings,
			Type:      checker.Method,
			Package:   "regexp",
			Struct:    "Regexp",
			Caller:    "MatchString",
			Args:      []int{0},
			AltCaller: "Match",

			Generate: &checker.Generate{
				PreCondition: `re := regexp.MustCompile(".*")`,
				Pattern:      `MatchString($0)`,
				Returns:      []string{"bool"},
			},
		},
		{ // (*regexp.Regexp).FindAllIndex
			Targets:   checker.Bytes,
			Type:      checker.Method,
			Package:   "regexp",
			Struct:    "Regexp",
			Caller:    "FindAllIndex",
			Args:      []int{0},
			AltCaller: "FindAllStringIndex",

			Generate: &checker.Generate{
				PreCondition: `re := regexp.MustCompile(".*")`,
				Pattern:      `FindAllIndex($0, 1)`,
				Returns:      []string{"[][]int"},
			},
		},
		{ // (*regexp.Regexp).FindAllStringIndex
			Targets:   checker.Strings,
			Type:      checker.Method,
			Package:   "regexp",
			Struct:    "Regexp",
			Caller:    "FindAllStringIndex",
			Args:      []int{0},
			AltCaller: "FindAllIndex",

			Generate: &checker.Generate{
				PreCondition: `re := regexp.MustCompile(".*")`,
				Pattern:      `FindAllStringIndex($0, 1)`,
				Returns:      []string{"[][]int"},
			},
		},
		{ // (*regexp.Regexp).FindAllSubmatchIndex
			Targets:   checker.Bytes,
			Type:      checker.Method,
			Package:   "regexp",
			Struct:    "Regexp",
			Caller:    "FindAllSubmatchIndex",
			Args:      []int{0},
			AltCaller: "FindAllStringSubmatchIndex",

			Generate: &checker.Generate{
				PreCondition: `re := regexp.MustCompile(".*")`,
				Pattern:      `FindAllSubmatchIndex($0, 1)`,
				Returns:      []string{"[][]int"},
			},
		},
		{ // (*regexp.Regexp).FindAllStringSubmatchIndex
			Targets:   checker.Strings,
			Type:      checker.Method,
			Package:   "regexp",
			Struct:    "Regexp",
			Caller:    "FindAllStringSubmatchIndex",
			Args:      []int{0},
			AltCaller: "FindAllSubmatchIndex",

			Generate: &checker.Generate{
				PreCondition: `re := regexp.MustCompile(".*")`,
				Pattern:      `FindAllStringSubmatchIndex($0, 1)`,
				Returns:      []string{"[][]int"},
			},
		},
		{ // (*regexp.Regexp).FindIndex
			Targets:   checker.Bytes,
			Type:      checker.Method,
			Package:   "regexp",
			Struct:    "Regexp",
			Caller:    "FindIndex",
			Args:      []int{0},
			AltCaller: "FindStringIndex",

			Generate: &checker.Generate{
				PreCondition: `re := regexp.MustCompile(".*")`,
				Pattern:      `FindIndex($0)`,
				Returns:      []string{"[]int"},
			},
		},
		{ // (*regexp.Regexp).FindStringIndex
			Targets:   checker.Strings,
			Type:      checker.Method,
			Package:   "regexp",
			Struct:    "Regexp",
			Caller:    "FindStringIndex",
			Args:      []int{0},
			AltCaller: "FindIndex",

			Generate: &checker.Generate{
				PreCondition: `re := regexp.MustCompile(".*")`,
				Pattern:      `FindStringIndex($0)`,
				Returns:      []string{"[]int"},
			},
		},
		{ // (*regexp.Regexp).FindSubmatchIndex
			Targets:   checker.Bytes,
			Type:      checker.Method,
			Package:   "regexp",
			Struct:    "Regexp",
			Caller:    "FindSubmatchIndex",
			Args:      []int{0},
			AltCaller: "FindStringSubmatchIndex",

			Generate: &checker.Generate{
				PreCondition: `re := regexp.MustCompile(".*")`,
				Pattern:      `FindSubmatchIndex($0)`,
				Returns:      []string{"[]int"},
			},
		},
		{ // (*regexp.Regexp).FindStringSubmatchIndex
			Targets:   checker.Strings,
			Type:      checker.Method,
			Package:   "regexp",
			Struct:    "Regexp",
			Caller:    "FindStringSubmatchIndex",
			Args:      []int{0},
			AltCaller: "FindSubmatchIndex",

			Generate: &checker.Generate{
				PreCondition: `re := regexp.MustCompile(".*")`,
				Pattern:      `FindStringSubmatchIndex($0)`,
				Returns:      []string{"[]int"},
			},
		},
	}
)
