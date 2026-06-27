package mirror

import "github.com/butuzov/mirror/internal/checker"

var (
	StringFunctions = []checker.Violation{
		{ // strings.Compare
			Targets:    checker.Strings,
			Type:       checker.Function,
			Package:    "strings",
			Caller:     "Compare",
			Args:       []int{0, 1},
			AltPackage: "bytes",
			AltCaller:  "Compare",

			Generate: &checker.Generate{
				Pattern: `Compare($0,$1)`,
				Returns: []string{"int"},
			},
		},
		{ // strings.Contains
			Targets:    checker.Strings,
			Type:       checker.Function,
			Package:    "strings",
			Caller:     "Contains",
			Args:       []int{0, 1},
			AltPackage: "bytes",
			AltCaller:  "Contains",

			Generate: &checker.Generate{
				Pattern: `Contains($0,$1)`,
				Returns: []string{"bool"},
			},
		},
		{ // strings.ContainsAny
			Targets:    checker.Strings,
			Type:       checker.Function,
			Package:    "strings",
			Caller:     "ContainsAny",
			Args:       []int{0},
			AltPackage: "bytes",
			AltCaller:  "ContainsAny",

			Generate: &checker.Generate{
				Pattern: `ContainsAny($0,"foobar")`,
				Returns: []string{"bool"},
			},
		},
		{ // strings.ContainsRune
			Targets:    checker.Strings,
			Type:       checker.Function,
			Package:    "strings",
			Caller:     "ContainsRune",
			Args:       []int{0},
			AltPackage: "bytes",
			AltCaller:  "ContainsRune",

			Generate: &checker.Generate{
				Pattern: `ContainsRune($0,'ф')`,
				Returns: []string{"bool"},
			},
		},
		{ // 	strings.Count
			Targets:    checker.Strings,
			Type:       checker.Function,
			Package:    "strings",
			Caller:     "Count",
			Args:       []int{0, 1},
			AltPackage: "bytes",
			AltCaller:  "Count",

			Generate: &checker.Generate{
				Pattern: `Count($0, $1)`,
				Returns: []string{"int"},
			},
		},
		{ // strings.EqualFold
			Targets:    checker.Strings,
			Type:       checker.Function,
			Package:    "strings",
			Caller:     "EqualFold",
			Args:       []int{0, 1},
			AltPackage: "bytes",
			AltCaller:  "EqualFold",

			Generate: &checker.Generate{
				Pattern: `EqualFold($0,$1)`,
				Returns: []string{"bool"},
			},
		},
		{ // strings.HasPrefix
			Targets:    checker.Strings,
			Type:       checker.Function,
			Package:    "strings",
			Caller:     "HasPrefix",
			Args:       []int{0, 1},
			AltPackage: "bytes",
			AltCaller:  "HasPrefix",

			Generate: &checker.Generate{
				Pattern: `HasPrefix($0,$1)`,
				Returns: []string{"bool"},
			},
		},
		{ // strings.HasSuffix
			Targets:    checker.Strings,
			Type:       checker.Function,
			Package:    "strings",
			Caller:     "HasSuffix",
			Args:       []int{0, 1},
			AltPackage: "bytes",
			AltCaller:  "HasSuffix",

			Generate: &checker.Generate{
				Pattern: `HasSuffix($0,$1)`,
				Returns: []string{"bool"},
			},
		},
		{ // strings.Index
			Targets:    checker.Strings,
			Type:       checker.Function,
			Package:    "strings",
			Caller:     "Index",
			Args:       []int{0, 1},
			AltPackage: "bytes",
			AltCaller:  "Index",

			Generate: &checker.Generate{
				Pattern: `Index($0,$1)`,
				Returns: []string{"int"},
			},
		},
		{ // strings.IndexAny
			Targets:    checker.Strings,
			Type:       checker.Function,
			Package:    "strings",
			Caller:     "IndexAny",
			Args:       []int{0},
			AltPackage: "bytes",
			AltCaller:  "IndexAny",

			Generate: &checker.Generate{
				Pattern: `IndexAny($0, "f")`,
				Returns: []string{"int"},
			},
		},
		{ // strings.IndexByte
			Targets:    checker.Strings,
			Type:       checker.Function,
			Package:    "strings",
			Caller:     "IndexByte",
			Args:       []int{0},
			AltPackage: "bytes",
			AltCaller:  "IndexByte",

			Generate: &checker.Generate{
				Pattern: `IndexByte($0, 'f')`,
				Returns: []string{"int"},
			},
		},
		{ // strings.IndexFunc
			Targets:    checker.Strings,
			Type:       checker.Function,
			Package:    "strings",
			Caller:     "IndexFunc",
			Args:       []int{0},
			AltPackage: "bytes",
			AltCaller:  "IndexFunc",

			Generate: &checker.Generate{
				Pattern: `IndexFunc($0, func(r rune) bool { return true })`,
				Returns: []string{"int"},
			},
		},
		{ // strings.IndexRune
			Targets:    checker.Strings,
			Type:       checker.Function,
			Package:    "strings",
			Caller:     "IndexRune",
			Args:       []int{0},
			AltPackage: "bytes",
			AltCaller:  "IndexRune",

			Generate: &checker.Generate{
				Pattern: `IndexRune($0, rune('ф'))`,
				Returns: []string{"int"},
			},
		},
		{ // strings.LastIndex
			Targets:    checker.Strings,
			Type:       checker.Function,
			Package:    "strings",
			Caller:     "LastIndex",
			Args:       []int{0, 1},
			AltPackage: "bytes",
			AltCaller:  "LastIndex",

			Generate: &checker.Generate{
				Pattern: `LastIndex($0,$1)`,
				Returns: []string{"int"},
			},
		},
		{ // strings.LastIndexAny
			Targets:    checker.Strings,
			Type:       checker.Function,
			Package:    "strings",
			Caller:     "LastIndexAny",
			Args:       []int{0},
			AltPackage: "bytes",
			AltCaller:  "LastIndexAny",

			Generate: &checker.Generate{
				Pattern: `LastIndexAny($0,"f")`,
				Returns: []string{"int"},
			},
		},
		{ // strings.LastIndexByte
			Targets:    checker.Strings,
			Type:       checker.Function,
			Package:    "strings",
			Caller:     "LastIndexByte",
			Args:       []int{0},
			AltPackage: "bytes",
			AltCaller:  "LastIndexByte",

			Generate: &checker.Generate{
				Pattern: `LastIndexByte($0, 'f')`,
				Returns: []string{"int"},
			},
		},
		{ // strings.LastIndexFunc
			Targets:    checker.Strings,
			Type:       checker.Function,
			Package:    "strings",
			Caller:     "LastIndexFunc",
			Args:       []int{0},
			AltPackage: "bytes",
			AltCaller:  "LastIndexFunc",

			Generate: &checker.Generate{
				Pattern: `LastIndexFunc($0, func(r rune) bool { return true })`,
				Returns: []string{"int"},
			},
		},
	}

	StringsBuilderMethods = []checker.Violation{
		{ // (*strings.Builder).Write
			Targets:   checker.Bytes,
			Type:      checker.Method,
			Package:   "strings",
			Struct:    "Builder",
			Caller:    "Write",
			Args:      []int{0},
			AltCaller: "WriteString",

			Generate: &checker.Generate{
				PreCondition: `builder := strings.Builder{}`,
				Pattern:      `Write($0)`,
				Returns:      []string{"int", "error"},
			},
		},
		{ // (*strings.Builder).WriteString
			Targets:   checker.Strings,
			Type:      checker.Method,
			Package:   "strings",
			Struct:    "Builder",
			Caller:    "WriteString",
			Args:      []int{0},
			AltCaller: "Write",

			Generate: &checker.Generate{
				PreCondition: `builder := strings.Builder{}`,
				Pattern:      `WriteString($0)`,
				Returns:      []string{"int", "error"},
			},
		},
		{ // (*strings.Builder).WriteString -> (*strings.Builder).WriteRune
			Targets:   checker.Strings,
			Type:      checker.Method,
			Package:   "strings",
			Struct:    "Builder",
			Caller:    "WriteString",
			Args:      []int{0},
			ArgsType:  checker.Rune,
			AltCaller: "WriteRune",
			Generate: &checker.Generate{
				SkipGenerate: true,
				Pattern:      `WriteString($0)`,
				Returns:      []string{"int", "error"},
			},
		},
		// { // (*strings.Builder).WriteString -> (*strings.Builder).WriteByte
		// 	Targets:   checker.Strings,
		// 	Type:      checker.Method,
		// 	Package:   "strings",
		// 	Struct:    "Builder",
		// 	Caller:    "WriteString",
		// 	Args:      []int{0},
		// 	ArgsType:  checker.Byte,
		// 	AltCaller: "WriteByte", // byte
		// },
	}
)
