package ansi

// Focus is an escape sequence to notify the terminal that it has focus.
// This is used with [FocusEventMode].
const Focus = "\x1b[I"

// Blur is an escape sequence to notify the terminal that it has lost focus.
// This is used with [FocusEventMode].
const Blur = "\x1b[O"
