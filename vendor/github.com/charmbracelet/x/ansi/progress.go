package ansi

import "strconv"

// ResetProgressBar is a sequence that resets the progress bar to its default
// state (hidden).
//
// OSC 9 ; 4 ; 0 BEL
//
// See: https://learn.microsoft.com/en-us/windows/terminal/tutorials/progress-bar-sequences
const ResetProgressBar = "\x1b]9;4;0\x07"

// SetProgressBar returns a sequence for setting the progress bar to a specific
// percentage (0-100) in the "default" state.
//
// OSC 9 ; 4 ; 1 Percentage BEL
//
// See: https://learn.microsoft.com/en-us/windows/terminal/tutorials/progress-bar-sequences
func SetProgressBar(percentage int) string {
	return "\x1b]9;4;1;" + strconv.Itoa(min(max(0, percentage), 100)) + "\x07"
}

// SetErrorProgressBar returns a sequence for setting the progress bar to a
// specific percentage (0-100) in the "Error" state..
//
// OSC 9 ; 4 ; 2 Percentage BEL
//
// See: https://learn.microsoft.com/en-us/windows/terminal/tutorials/progress-bar-sequences
func SetErrorProgressBar(percentage int) string {
	return "\x1b]9;4;2;" + strconv.Itoa(min(max(0, percentage), 100)) + "\x07"
}

// SetIndeterminateProgressBar is a sequence that sets the progress bar to the
// indeterminate state.
//
// OSC 9 ; 4 ; 3 BEL
//
// See: https://learn.microsoft.com/en-us/windows/terminal/tutorials/progress-bar-sequences
const SetIndeterminateProgressBar = "\x1b]9;4;3\x07"

// SetWarningProgressBar is a sequence that sets the progress bar to the
// "Warning" state.
//
// OSC 9 ; 4 ; 4 Percentage BEL
//
// See: https://learn.microsoft.com/en-us/windows/terminal/tutorials/progress-bar-sequences
func SetWarningProgressBar(percentage int) string {
	return "\x1b]9;4;4;" + strconv.Itoa(min(max(0, percentage), 100)) + "\x07"
}
