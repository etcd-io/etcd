//go:build no_antithesis_sdk

package assert

func Always(condition bool, message string, details map[string]any)              {}
func AlwaysOrUnreachable(condition bool, message string, details map[string]any) {}
func Sometimes(condition bool, message string, details map[string]any)           {}
func Unreachable(message string, details map[string]any)                         {}
func Reachable(message string, details map[string]any)                           {}
func AssertRaw(cond bool, message string, details map[string]any,
	classname, funcname, filename string, line int,
	hit bool, mustHit bool,
	assertType string, displayType string,
	id string,
) {
}
