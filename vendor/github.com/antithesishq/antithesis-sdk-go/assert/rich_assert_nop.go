//go:build no_antithesis_sdk

package assert

func AlwaysGreaterThan[T Number](left, right T, message string, details map[string]any)             {}
func AlwaysGreaterThanOrEqualTo[T Number](left, right T, message string, details map[string]any)    {}
func SometimesGreaterThan[T Number](left, right T, message string, details map[string]any)          {}
func SometimesGreaterThanOrEqualTo[T Number](left, right T, message string, details map[string]any) {}
func AlwaysLessThan[T Number](left, right T, message string, details map[string]any)                {}
func AlwaysLessThanOrEqualTo[T Number](left, right T, message string, details map[string]any)       {}
func SometimesLessThan[T Number](left, right T, message string, details map[string]any)             {}
func SometimesLessThanOrEqualTo[T Number](left, right T, message string, details map[string]any)    {}

func AlwaysSome(named_bool []NamedBool, message string, details map[string]any)   {}
func SometimesAll(named_bool []NamedBool, message string, details map[string]any) {}

func NumericGuidanceRaw[T Number](left, right T,
	message, id string,
	classname, funcname, filename string,
	line int,
	behavior string,
	hit bool,
) {
}

func BooleanGuidanceRaw(
	named_bools []NamedBool,
	message, id string,
	classname, funcname, filename string,
	line int,
	behavior string,
	hit bool,
) {
}
