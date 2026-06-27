package ansi

import (
	"fmt"
	"strings"
)

// URxvtExt returns an escape sequence for calling a URxvt perl extension with
// the given name and parameters.
//
//	OSC 777 ; extension_name ; param1 ; param2 ; ... ST
//	OSC 777 ; extension_name ; param1 ; param2 ; ... BEL
//
// See: https://man.archlinux.org/man/extra/rxvt-unicode/urxvt.7.en#XTerm_Operating_System_Commands
func URxvtExt(extension string, params ...string) string {
	return fmt.Sprintf("\x1b]777;%s;%s\x07", extension, strings.Join(params, ";"))
}
