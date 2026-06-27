package mirror

import "github.com/butuzov/mirror/internal/checker"

var (
	BytesFunctions = []checker.Violation{
		{ // bytes.NewBuffer
			Targets:   checker.Bytes,
			Type:      checker.Function,
			Package:   "bytes",
			Caller:    "NewBuffer",
			Args:      []int{0},
			AltCaller: "NewBufferString",

			Generate: &checker.Generate{
				Pattern: `NewBuffer($0)`,
				Returns: []string{"*bytes.Buffer"},
			},
		},
		{ // bytes.NewBufferString
			Targets:   checker.Strings,
			Type:      checker.Function,
			Package:   "bytes",
			Caller:    "NewBufferString",
			Args:      []int{0},
			AltCaller: "NewBuffer",

			Generate: &checker.Generate{
				Pattern: `NewBufferString($0)`,
				Returns: []string{"*bytes.Buffer"},
			},
		},
		{ // bytes.Compare:
			Targets:    checker.Bytes,
			Type:       checker.Function,
			Package:    "bytes",
			Caller:     "Compare",
			Args:       []int{0, 1},
			AltPackage: "strings",
			AltCaller:  "Compare",

			Generate: &checker.Generate{
				Pattern: `Compare($0, $1)`,
				Returns: []string{"int"},
			},
		},
		{ // bytes.Contains:
			Targets:    checker.Bytes,
			Type:       checker.Function,
			Package:    "bytes",
			Caller:     "Contains",
			Args:       []int{0, 1},
			AltPackage: "strings",
			AltCaller:  "Contains",

			Generate: &checker.Generate{
				Pattern: `Contains($0, $1)`,
				Returns: []string{"bool"},
			},
		},
		{ // bytes.ContainsAny
			Targets:    checker.Bytes,
			Type:       checker.Function,
			Package:    "bytes",
			Caller:     "ContainsAny",
			Args:       []int{0},
			AltPackage: "strings",
			AltCaller:  "ContainsAny",

			Generate: &checker.Generate{
				Pattern: `ContainsAny($0, "f")`,
				Returns: []string{"bool"},
			},
		},
		{ // bytes.ContainsRune
			Targets:    checker.Bytes,
			Type:       checker.Function,
			Package:    "bytes",
			Caller:     "ContainsRune",
			Args:       []int{0},
			AltPackage: "strings",
			AltCaller:  "ContainsRune",

			Generate: &checker.Generate{
				Pattern: `ContainsRune($0, 'ф')`,
				Returns: []string{"bool"},
			},
		},
		{ // bytes.Count
			Targets:    checker.Bytes,
			Type:       checker.Function,
			Package:    "bytes",
			Caller:     "Count",
			Args:       []int{0, 1},
			AltPackage: "strings",
			AltCaller:  "Count",

			Generate: &checker.Generate{
				Pattern: `Count($0, $1)`,
				Returns: []string{"int"},
			},
		},
		{ // bytes.EqualFold
			Targets:    checker.Bytes,
			Type:       checker.Function,
			Package:    "bytes",
			Caller:     "EqualFold",
			Args:       []int{0, 1},
			AltPackage: "strings",
			AltCaller:  "EqualFold",

			Generate: &checker.Generate{
				Pattern: `EqualFold($0, $1)`,
				Returns: []string{"bool"},
			},
		},

		{ // bytes.HasPrefix
			Targets:    checker.Bytes,
			Type:       checker.Function,
			Package:    "bytes",
			Caller:     "HasPrefix",
			Args:       []int{0, 1},
			AltPackage: "strings",
			AltCaller:  "HasPrefix",

			Generate: &checker.Generate{
				Pattern: `HasPrefix($0, $1)`,
				Returns: []string{"bool"},
			},
		},
		{ // bytes.HasSuffix
			Targets:    checker.Bytes,
			Type:       checker.Function,
			Package:    "bytes",
			Caller:     "HasSuffix",
			Args:       []int{0, 1},
			AltPackage: "strings",
			AltCaller:  "HasSuffix",

			Generate: &checker.Generate{
				Pattern: `HasSuffix($0, $1)`,
				Returns: []string{"bool"},
			},
		},
		{ // bytes.Index
			Targets:    checker.Bytes,
			Type:       checker.Function,
			Package:    "bytes",
			Caller:     "Index",
			Args:       []int{0, 1},
			AltPackage: "strings",
			AltCaller:  "Index",

			Generate: &checker.Generate{
				Pattern: `Index($0, $1)`,
				Returns: []string{"int"},
			},
		},
		{ // bytes.IndexAny
			Targets:    checker.Bytes,
			Type:       checker.Function,
			Package:    "bytes",
			Caller:     "IndexAny",
			Args:       []int{0},
			AltPackage: "strings",
			AltCaller:  "IndexAny",

			Generate: &checker.Generate{
				Pattern: `IndexAny($0, "f")`,
				Returns: []string{"int"},
			},
		},
		{ // bytes.IndexByte
			Targets:    checker.Bytes,
			Type:       checker.Function,
			Package:    "bytes",
			Caller:     "IndexByte",
			Args:       []int{0},
			AltPackage: "strings",
			AltCaller:  "IndexByte",

			Generate: &checker.Generate{
				Pattern: `IndexByte($0, 'f')`,
				Returns: []string{"int"},
			},
		},
		{ // bytes.IndexFunc
			Targets:    checker.Bytes,
			Type:       checker.Function,
			Package:    "bytes",
			Caller:     "IndexFunc",
			Args:       []int{0},
			AltPackage: "strings",
			AltCaller:  "IndexFunc",

			Generate: &checker.Generate{
				Pattern: `IndexFunc($0, func(rune) bool {return true })`,
				Returns: []string{"int"},
			},
		},
		{ // bytes.IndexRune
			Targets:    checker.Bytes,
			Type:       checker.Function,
			Package:    "bytes",
			Caller:     "IndexRune",
			Args:       []int{0},
			AltPackage: "strings",
			AltCaller:  "IndexRune",

			Generate: &checker.Generate{
				Pattern: `IndexRune($0, rune('ф'))`,
				Returns: []string{"int"},
			},
		},
		{ // bytes.LastIndex
			Targets:    checker.Bytes,
			Type:       checker.Function,
			Package:    "bytes",
			Caller:     "LastIndex",
			Args:       []int{0, 1},
			AltPackage: "strings",
			AltCaller:  "LastIndex",

			Generate: &checker.Generate{
				Pattern: `LastIndex($0, $1)`,
				Returns: []string{"int"},
			},
		},
		{ // bytes.LastIndexAny
			Targets:    checker.Bytes,
			Type:       checker.Function,
			Package:    "bytes",
			Caller:     "LastIndexAny",
			Args:       []int{0},
			AltPackage: "strings",
			AltCaller:  "LastIndexAny",

			Generate: &checker.Generate{
				Pattern: `LastIndexAny($0, "ф")`,
				Returns: []string{"int"},
			},
		},
		{ // bytes.LastIndexByte
			Targets:    checker.Bytes,
			Type:       checker.Function,
			Package:    "bytes",
			Caller:     "LastIndexByte",
			Args:       []int{0},
			AltPackage: "strings",
			AltCaller:  "LastIndexByte",

			Generate: &checker.Generate{
				Pattern: `LastIndexByte($0, 'f')`,
				Returns: []string{"int"},
			},
		},
		{ // bytes.LastIndexFunc
			Targets:    checker.Bytes,
			Type:       checker.Function,
			Package:    "bytes",
			Caller:     "LastIndexFunc",
			Args:       []int{0},
			AltPackage: "strings",
			AltCaller:  "LastIndexFunc",

			Generate: &checker.Generate{
				Pattern: `LastIndexFunc($0, func(rune) bool {return true })`,
				Returns: []string{"int"},
			},
		},
	}

	BytesBufferMethods = []checker.Violation{
		{ // (*bytes.Buffer).Write
			Targets:   checker.Bytes,
			Type:      checker.Method,
			Package:   "bytes",
			Struct:    "Buffer",
			Caller:    "Write",
			Args:      []int{0},
			AltCaller: "WriteString",

			Generate: &checker.Generate{
				PreCondition: `bb := bytes.Buffer{}`,
				Pattern:      `Write($0)`,
				Returns:      []string{"int", "error"},
			},
		},
		{ // (*bytes.Buffer).WriteString
			Targets:   checker.Strings,
			Type:      checker.Method,
			Package:   "bytes",
			Struct:    "Buffer",
			Caller:    "WriteString",
			Args:      []int{0},
			AltCaller: "Write",

			Generate: &checker.Generate{
				PreCondition: `bb := bytes.Buffer{}`,
				Pattern:      `WriteString($0)`,
				Returns:      []string{"int", "error"},
			},
		},
		{ // (*bytes.Buffer).WriteString -> (*bytes.Buffer).WriteRune
			Targets:   checker.Strings,
			Type:      checker.Method,
			Package:   "bytes",
			Struct:    "Buffer",
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
		// { // (*bytes.Buffer).WriteString -> (*bytes.Buffer).WriteByte
		// 	Targets:   checker.Strings,
		// 	Type:      checker.Method,
		// 	Package:   "bytes",
		// 	Struct:    "Buffer",
		// 	Caller:    "WriteString",
		// 	Args:      []int{0},
		// 	ArgsType:  checker.Byte,
		// 	AltCaller: "WriteByte",
		// },
	}
)
