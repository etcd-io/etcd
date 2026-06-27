package ansi

import "strings"

// FinalTerm returns an escape sequence that is used for shell integrations.
// Originally, FinalTerm designed the protocol hence the name.
//
//	OSC 133 ; Ps ; Pm ST
//	OSC 133 ; Ps ; Pm BEL
//
// See: https://iterm2.com/documentation-shell-integration.html
func FinalTerm(pm ...string) string {
	return "\x1b]133;" + strings.Join(pm, ";") + "\x07"
}

// FinalTermPrompt returns an escape sequence that is used for shell
// integrations prompt marks. This is sent just before the start of the shell
// prompt.
//
// This is an alias for FinalTerm("A").
func FinalTermPrompt(pm ...string) string {
	if len(pm) == 0 {
		return FinalTerm("A")
	}
	return FinalTerm(append([]string{"A"}, pm...)...)
}

// FinalTermCmdStart returns an escape sequence that is used for shell
// integrations command start marks. This is sent just after the end of the
// shell prompt, before the user enters a command.
//
// This is an alias for FinalTerm("B").
func FinalTermCmdStart(pm ...string) string {
	if len(pm) == 0 {
		return FinalTerm("B")
	}
	return FinalTerm(append([]string{"B"}, pm...)...)
}

// FinalTermCmdExecuted returns an escape sequence that is used for shell
// integrations command executed marks. This is sent just before the start of
// the command output.
//
// This is an alias for FinalTerm("C").
func FinalTermCmdExecuted(pm ...string) string {
	if len(pm) == 0 {
		return FinalTerm("C")
	}
	return FinalTerm(append([]string{"C"}, pm...)...)
}

// FinalTermCmdFinished returns an escape sequence that is used for shell
// integrations command finished marks.
//
// If the command was sent after
// [FinalTermCmdStart], it indicates that the command was aborted. If the
// command was sent after [FinalTermCmdExecuted], it indicates the end of the
// command output. If neither was sent, [FinalTermCmdFinished] should be
// ignored.
//
// This is an alias for FinalTerm("D").
func FinalTermCmdFinished(pm ...string) string {
	if len(pm) == 0 {
		return FinalTerm("D")
	}
	return FinalTerm(append([]string{"D"}, pm...)...)
}
