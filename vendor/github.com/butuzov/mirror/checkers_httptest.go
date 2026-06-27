package mirror

import "github.com/butuzov/mirror/internal/checker"

var HTTPTestMethods = []checker.Violation{
	{ // (*net/http/httptest.ResponseRecorder).Write
		Targets:   checker.Bytes,
		Type:      checker.Method,
		Package:   "net/http/httptest",
		Struct:    "ResponseRecorder",
		Caller:    "Write",
		Args:      []int{0},
		AltCaller: "WriteString",

		Generate: &checker.Generate{
			PreCondition: `h := httptest.ResponseRecorder{}`,
			Pattern:      `Write($0)`,
			Returns:      []string{"int", "error"},
		},
	},
	{ // (*net/http/httptest.ResponseRecorder).WriteString
		Targets:   checker.Strings,
		Type:      checker.Method,
		Package:   "net/http/httptest",
		Struct:    "ResponseRecorder",
		Caller:    "WriteString",
		Args:      []int{0},
		AltCaller: "Write",

		Generate: &checker.Generate{
			PreCondition: `h := httptest.ResponseRecorder{}`,
			Pattern:      `WriteString($0)`,
			Returns:      []string{"int", "error"},
		},
	},
}
