package mirror

import "github.com/butuzov/mirror/internal/checker"

var OsFileMethods = []checker.Violation{
	{ // (*os.File).Write
		Targets:   checker.Bytes,
		Type:      checker.Method,
		Package:   "os",
		Struct:    "File",
		Caller:    "Write",
		Args:      []int{0},
		AltCaller: "WriteString",

		Generate: &checker.Generate{
			PreCondition: `f := &os.File{}`,
			Pattern:      `Write($0)`,
			Returns:      []string{"int", "error"},
		},
	},
	{ // (*os.File).WriteString
		Targets:   checker.Strings,
		Type:      checker.Method,
		Package:   "os",
		Struct:    "File",
		Caller:    "WriteString",
		Args:      []int{0},
		AltCaller: "Write",

		Generate: &checker.Generate{
			PreCondition: `f := &os.File{}`,
			Pattern:      `WriteString($0)`,
			Returns:      []string{"int", "error"},
		},
	},
}
